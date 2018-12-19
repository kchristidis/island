package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kchristidis/island/chaincode/schema"

	"github.com/kchristidis/island/bidder"
	"github.com/kchristidis/island/blockchain"
	"github.com/kchristidis/island/blocknotifier"
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
	outputprefix = fmt.Sprintf("exp-%02d-run-%02d", schema.ExpNum, iter)

	timestart = time.Now()

	startfromblock = uint64(10)

	statblockc = make(chan stats.Block, StatChannelBuffer)
	statslotc = make(chan stats.Slot, StatChannelBuffer)
	statstranc = make(chan stats.Transaction, StatChannelBuffer)

	donec = make(chan struct{})
	donestatsc = make(chan struct{})

	tracepath := filepath.Join("trace", trace.Filename)
	tracemap, err = trace.Load(tracepath)
	if err != nil {
		return nil
	}

	privkeypath := filepath.Join("crypto", "priv.pem")
	privkey, err = crypto.LoadPrivate(privkeypath)
	if err != nil {
		return nil
	}
	privkeybytes := crypto.SerializePrivate(privkey)

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

	statscollector = &stats.Collector{
		BlockChan:       statblockc,
		SlotChan:        statslotc,
		TransactionChan: statstranc,
		Writer:          writer,
		DoneChan:        donestatsc,
	}
	wg3.Add(1)
	go func() {
		statscollector.Run()
		wg3.Done()
	}()

	switch schema.ExpNum {
	case 1, 3:
		for i := 0; i < 2; i++ {
			// 1st for markend+bid calls
			// 2nd one for postkey calls
			slotcs = append(slotcs, make(chan int))
			snotifiers = append(snotifiers, slotnotifier.New(slotcs[i], writer, donec))
		}
	case 2:
		slotcs = append(slotcs, make(chan int))
		snotifiers = []*slotnotifier.Notifier{slotnotifier.New(slotcs[0], writer, donec), nil}
	}

	regtor = regulator.New(sdkctx, snotifiers[0], privkeybytes, statslotc, statstranc, writer, donec)
	wg2.Add(1)
	go func() {
		if err := regtor.Run(); err != nil {
			once.Do(func() {
				msg := fmt.Sprint("regulator • closing donec")
				fmt.Fprintln(writer, msg)
				close(donec)
			})
		}
		wg2.Done()
	}()

	// for i, ID := range trace.IDs {
	for i, ID := range []int{171, 1103} { // For debugging only
		bidders[i] = bidder.New(sdkctx, snotifiers[0], snotifiers[1],
			ID, privkeybytes, tracemap[ID],
			statslotc, statstranc, writer, donec)
		wg1.Add(1)
		go func(i int) {
			if err = bidders[i].Run(); err != nil {
				once.Do(func() {
					msg := fmt.Sprintf("bidder:%04d • closing donec", bidders[i].ID)
					fmt.Fprintln(writer, msg)
					close(donec)
				})
			}
			wg1.Done()
		}(i)
	}

	// The size of the slotcs slice dictates how many block notifiers we need

	bnotifiers = append(bnotifiers, blocknotifier.New(
		schema.BlocksPerSlot, schema.ClockPeriod, schema.SleepDuration, startfromblock,
		statblockc, slotcs[0],
		sdkctx, sdkctx.LedgerClient,
		writer, donec,
	))

	if len(slotcs) > 1 {
		nilchan := make(chan stats.Block) // This ensures that only the first blocknotifier feeds the stats collector
		bnotifiers = append(bnotifiers, blocknotifier.New(
			schema.BlocksPerSlot, schema.ClockPeriod, schema.SleepDuration, startfromblock+uint64(schema.BlockOffset),
			nilchan, slotcs[1],
			sdkctx, sdkctx.LedgerClient,
			writer, donec,
		))
	}

	for i := range bnotifiers {
		wg2.Add(1)
		go func(i int) {
			if err := bnotifiers[i].Run(); err != nil {
				once.Do(func() {
					msg := fmt.Sprintf("block-notifier:%d • closing donec", i)
					fmt.Fprintln(writer, msg)
					close(donec)
				})
			}
			wg2.Done()
		}(i)
	}

	for i := range snotifiers {
		if snotifiers[i] == nil {
			continue
		}
		wg2.Add(1)
		go func(i int) {
			snotifiers[i].Run()
			wg2.Done()
		}(i)
	}

	// Start the simulation

	// In the green path, the *bidders* will run their entire trace, then exit.
	// We then close `donec`, then wait for all the other goroutines to conclude.
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
		close(donec)
	})

	wg2.Wait()

	fmt.Fprintln(writer, "main • run completed")

	return metrics()
}
