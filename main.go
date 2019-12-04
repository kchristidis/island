package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kchristidis/island/bidder"
	"github.com/kchristidis/island/blockchain"
	"github.com/kchristidis/island/blocknotifier"
	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/regulator"
	"github.com/kchristidis/island/slotnotifier"
	"github.com/kchristidis/island/stats"
	"github.com/kchristidis/island/trace"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	writer = os.Stdout
	iter++
	outputPrefix = fmt.Sprintf("exp-%02d-run-%02d", schema.ExpNum, iter)

	timeStart = time.Now()

	startFromBlock = uint64(10)

	statsBlockC = make(chan stats.Block, StatChannelBuffer)
	statsSlotC = make(chan stats.Slot, StatChannelBuffer)
	statsTranC = make(chan stats.Transaction, StatChannelBuffer)

	doneC = make(chan struct{})
	doneStatsC = make(chan struct{})

	tracePath := filepath.Join("trace", trace.Filename)
	traceMap, err = trace.Load(tracePath)
	if err != nil {
		return nil
	}

	privKeyPath := filepath.Join("crypto", "priv.pem")
	privKey, err = crypto.LoadPrivate(privKeyPath)
	if err != nil {
		return nil
	}
	privKeyBytes := crypto.SerializePrivate(privKey)

	// Begin initializations

	println()
	msg := fmt.Sprintf("Simulating experiment %d...", schema.ExpNum)
	fmt.Fprintln(writer, msg)
	println()

	sdkctx = &blockchain.SDKContext{
		SDKConfigFile: "config.yaml",

		OrgName:  "clark",
		OrgAdmin: "Admin",
		UserName: "User1",

		OrdererID:   "joe.example.com",
		ChannelID:   "clark-channel",
		ChaincodeID: fmt.Sprintf("exp%d", schema.ExpNum),

		ChannelConfigPath:   os.Getenv("GOPATH") + "/src/github.com/kchristidis/island/fixtures/artifacts/clark-channel.tx",
		ChaincodeGoPath:     os.Getenv("GOPATH"),
		ChaincodeSourcePath: "github.com/kchristidis/island/chaincode/",
	}
	if err = sdkctx.Setup(); err != nil {
		return err
	}
	defer sdkctx.SDK.Close()
	if err = sdkctx.Install(); err != nil {
		return err
	}

	statsCollector = &stats.Collector{
		BlockChan:       statsBlockC,
		SlotChan:        statsSlotC,
		TransactionChan: statsTranC,
		Writer:          writer,
		DoneChan:        doneStatsC,
	}
	wg3.Add(1)
	go func() {
		statsCollector.Run()
		wg3.Done()
	}()

	switch schema.ExpNum {
	case 1, 3:
		for i := 0; i < 2; i++ {
			// 1st for markend+bid calls
			// 2nd one for postkey calls
			slotCs = append(slotCs, make(chan int))
			sNotifiers = append(sNotifiers, slotnotifier.New(slotCs[i], writer, doneC))
		}
	case 2:
		slotCs = append(slotCs, make(chan int))
		sNotifiers = []*slotnotifier.Notifier{slotnotifier.New(slotCs[0], writer, doneC), nil}
	}

	regtor = regulator.New(sdkctx, sNotifiers[0],
		privKeyBytes,
		statsSlotC, statsTranC, writer, doneC)
	wg2.Add(1)
	go func() {
		if err := regtor.Run(); err != nil {
			once.Do(func() {
				msg := fmt.Sprint("regulator • closing donec")
				fmt.Fprintln(writer, msg)
				close(doneC)
			})
		}
		wg2.Done()
	}()

	biddersList := trace.IDs
	if schema.StagingLevel <= schema.Debug {
		biddersList = biddersList[:10] // We only care about 10 participants.
	}

	for i, ID := range biddersList {
		bidders[i] = bidder.New(sdkctx, sNotifiers[0], sNotifiers[1],
			ID, privKeyBytes, traceMap[ID],
			statsSlotC, statsTranC, writer, doneC)
		wg1.Add(1)
		go func(i int) {
			if err = bidders[i].Run(); err != nil {
				once.Do(func() {
					msg := fmt.Sprintf("bidder:%04d • closing donec", bidders[i].ID)
					fmt.Fprintln(writer, msg)
					close(doneC)
				})
			}
			wg1.Done()
		}(i)
	}

	// The size of the slotCs slice dictates how many block notifiers we need

	bNotifiers = append(bNotifiers, blocknotifier.New(
		schema.BlocksPerSlot, schema.ClockPeriod, schema.SleepDuration, startFromBlock,
		statsBlockC, slotCs[0],
		sdkctx, sdkctx.LedgerClient,
		writer, doneC,
	))

	if len(slotCs) > 1 {
		nilChan := make(chan stats.Block) // This ensures that only the first blockNotifier feeds the stats collector
		bNotifiers = append(bNotifiers, blocknotifier.New(
			schema.BlocksPerSlot, schema.ClockPeriod, schema.SleepDuration, startFromBlock+uint64(schema.BlockOffset),
			nilChan, slotCs[1],
			sdkctx, sdkctx.LedgerClient,
			writer, doneC,
		))
	}

	for i := range bNotifiers {
		wg2.Add(1)
		go func(i int) {
			if err := bNotifiers[i].Run(); err != nil {
				once.Do(func() {
					msg := fmt.Sprintf("block-notifier:%d • closing donec", i)
					fmt.Fprintln(writer, msg)
					close(doneC)
				})
			}
			wg2.Done()
		}(i)
	}

	for i := range sNotifiers {
		if sNotifiers[i] == nil {
			continue
		}
		wg2.Add(1)
		go func(i int) {
			sNotifiers[i].Run()
			wg2.Done()
		}(i)
	}

	// Start the simulation

	// In the green path, the *bidders* will run their entire trace, then exit.
	// We then close `doneC`, and wait for all the other goroutines to conclude.
	// This is why we use two separate waitgroups: one for bidders, and one for
	// regulator/notifiers.
	// An unfortunate side-effect of this design is that we do not let the regulator
	// 'markEnd' the very last slot, since @ slot N the regulator closes slot N-1.
	// That means we missing out on the stats of the very last slot, which is why
	// we only output N-1 slots in our stats output ("cleared" slots).

	wg1.Wait()

	once.Do(func() {
		msg := fmt.Sprint("main • closing donec")
		fmt.Fprintln(writer, msg)
		close(doneC)
	})

	wg2.Wait()

	fmt.Fprintln(writer, "main • run completed")

	return metrics()
}
