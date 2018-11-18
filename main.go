package main

import (
	"crypto/rsa"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/kchristidis/island/agent"
	"github.com/kchristidis/island/blockchain"
	"github.com/kchristidis/island/blocknotifier"
	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/markend"
	"github.com/kchristidis/island/slotnotifier"
	"github.com/kchristidis/island/stats"
	"github.com/kchristidis/island/trace"
)

// Constants ...
const (
	StatChannelBuffer = 100
	TraceLength       = 35036

	OutputDir   = "output"
	OutputTran  = "tran.csv"
	OutputSlot  = "slot.csv"
	OutputBlock = "block.csv"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		err error

		agents       []*agent.Agent
		markendagent *markend.Agent
		bnotifier    *blocknotifier.Notifier
		snotifier    *slotnotifier.Notifier
		sdkctx       *blockchain.SDKContext

		blocksperslot  int
		clockperiod    time.Duration
		sleepduration  time.Duration
		startfromblock uint64

		privkeybytes []byte
		pubkey       *rsa.PublicKey
		tracemap     map[int][][]float64

		slotc chan int

		statsblockc    chan stats.Block
		statslotc      chan stats.Slot
		statstranc     chan stats.Transaction
		statscollector *stats.Collector

		donec, donestatsc chan struct{}
		once              sync.Once
		wg1, wg2, wg3     sync.WaitGroup

		writer io.Writer
	)

	// Stats
	/* var (
		BytesPerBlock                 []int
		LatenciesPerBidInMilliseconds []int
	) */

	// Global vars

	blocksperslot = 3
	clockperiod = 500 * time.Millisecond
	sleepduration = 100 * time.Millisecond
	startfromblock = uint64(10)

	statsblockc = make(chan stats.Block, StatChannelBuffer)
	statslotc = make(chan stats.Slot, StatChannelBuffer)
	statstranc = make(chan stats.Transaction, StatChannelBuffer)

	donec = make(chan struct{})      // Acts as a coordination signal for goroutines.
	donestatsc = make(chan struct{}) // We want to kill the stats collector *after* all the stats submitting goroutines have returned.

	writer = os.Stdout

	// Load the trace

	tracepath := filepath.Join("trace", trace.Filename)
	tracemap, err = trace.Load(tracepath)
	if err != nil {
		return nil
	}

	// Load the keys

	pubkeypath := filepath.Join("crypto", "pub.pem")
	pubkey, err = crypto.LoadPublic(pubkeypath)
	if err != nil {
		return nil
	}

	privkeypath := filepath.Join("crypto", "priv.pem")
	privkey, err := crypto.LoadPrivate(privkeypath)
	if err != nil {
		return nil
	}
	privkeybytes = crypto.SerializePrivate(privkey)

	// Set up the SDK

	sdkctx = &blockchain.SDKContext{
		SDKConfigFile: "config.yaml",

		OrgName:  "clark",
		OrgAdmin: "Admin",
		UserName: "User1",

		OrdererID:   "joe.example.com",
		ChannelID:   "clark-channel",
		ChaincodeID: "island",

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

	// Set up the stats collector

	statscollector = &stats.Collector{
		BlockChan:       statsblockc,
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

	// Set up the slot notifier

	slotc = make(chan int)
	snotifier = slotnotifier.New(slotc, writer, donec)

	// Set up and launch the agents

	markendagent = markend.New(sdkctx, snotifier, privkeybytes, statslotc, statstranc, writer, donec)
	wg2.Add(1)
	go func() {
		if err := markendagent.Run(); err != nil {
			once.Do(func() {
				close(donec)
			})
		}
		wg2.Done()
	}()

	agents = make([]*agent.Agent, trace.IDCount)
	// for i, ID := range []int{171, 1103} // ATTN: Temporary modification:
	for i, ID := range trace.IDs {
		agents[i] = agent.New(sdkctx, snotifier, pubkey, tracemap[ID], ID, statslotc, statstranc, writer, donec)
		wg1.Add(1)
		go func(i int) {
			if err = agents[i].Run(); err != nil {
				once.Do(func() {
					close(donec)
				})
			}
			wg1.Done()
		}(i)
	}

	// Set up and launch the block notifier

	bnotifier = blocknotifier.New(
		blocksperslot, clockperiod, sleepduration, startfromblock,
		statsblockc, slotc,
		sdkctx, sdkctx.LedgerClient,
		writer, donec,
	)

	wg2.Add(1)
	go func() {
		if err := bnotifier.Run(); err != nil {
			once.Do(func() {
				close(donec)
			})
		}
		wg2.Done()
	}()

	// Launch the slot notifier

	wg2.Add(1)
	go func() {
		snotifier.Run()
		wg2.Done()
	}()

	// Start the simulation

	// In the green path, the agents will run their entire trace, then exit.
	// This will allows us to close the donec via the once.Do construct below.
	// Then we wait for all the other goroutines to conclude.
	// This is why we use two separate waitgroups.

	println()

	wg1.Wait()
	once.Do(func() {
		fmt.Fprintln(os.Stdout, "Closing donec...")
		close(donec)
	})
	wg2.Wait()

	fmt.Fprintln(os.Stdout, "Run completed successfully 😎")

	println()

	fmt.Fprintln(os.Stdout, "Time to collect & print the results...")

	println()

	fmt.Fprintln(os.Stdout, "Closing donestatsc...")
	close(donestatsc)
	wg3.Wait()

	tranfile, err := os.Create(filepath.Join(OutputDir, OutputTran))
	if err != nil {
		return err
	}
	defer tranfile.Close()

	tranwriter := csv.NewWriter(tranfile)
	if err := tranwriter.Write([]string{"tx ID", "latency (ms)", "tx type", "tx status"}); err != nil {
		return err
	}
	defer func() error {
		tranwriter.Flush()
		if err := tranwriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	blockfile, err := os.Create(filepath.Join(OutputDir, OutputBlock))
	if err != nil {
		return err
	}
	defer blockfile.Close()

	blockwriter := csv.NewWriter(blockfile)
	if err := blockwriter.Write([]string{"block num", "filesize (KiB)"}); err != nil {
		return err
	}
	defer func() error {
		blockwriter.Flush()
		if err := blockwriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	slotfile, err := os.Create(filepath.Join(OutputDir, OutputSlot))
	if err != nil {
		return err
	}
	defer slotfile.Close()

	slotwriter := csv.NewWriter(slotfile)
	if err := slotwriter.Write([]string{"slot num",
		"bfg qty (kWh)", "bfg ppu (ç/kWh)",
		"stg qty (kWh)", "stg ppu (ç/kWh)",
		"dmi qty (kWh)", "dmi ppu (ç/kWh)",
		"late cnt (all)", "late cnt (buy)", "late cnt (sell)"}); err != nil {
		return err
	}
	defer func() error {
		slotwriter.Flush()
		if err := slotwriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	println()

	fmt.Fprintln(writer, "Transaction stats")
	for _, tx := range stats.TransactionStats {
		idNum, _ := strconv.Atoi(tx.ID)
		idVal := fmt.Sprintf("%012d", idNum)
		latVal := fmt.Sprintf("%d", tx.LatencyInMillis)
		fmt.Fprintf(writer,
			"[txID: %s]"+
				"\tlatency:%s ms"+
				"\t\ttype:%s"+
				"\t\tstatus:%s\n",
			idVal,
			latVal,
			tx.Type,
			tx.Status,
		)
		if err := tranwriter.Write([]string{idVal, latVal, tx.Type, tx.Status}); err != nil {
			return err
		}
	}

	println()

	fmt.Fprintln(writer, "Block stats")
	for _, b := range stats.BlockStats {
		if uint64(b.Number) < startfromblock {
			continue
		}
		numVal := fmt.Sprintf("%012d", b.Number)
		sizeVal := fmt.Sprintf("%.1f", b.SizeInKB)
		fmt.Fprintf(writer,
			"[block: %s]"+
				"\t%s KiB\n",
			numVal,
			sizeVal,
		)
		if err := blockwriter.Write([]string{numVal, sizeVal}); err != nil {
			return err
		}
	}

	println()

	type aggregateStats struct {
		LateTXsCount, LateBuysCount, LateSellsCount [TraceLength]int
	}

	var (
		aggStats aggregateStats
		resp     []byte
	)

	if resp, err = sdkctx.Query(-1, "aggregate"); err != nil {
		return err
	}

	if err := json.Unmarshal(resp, &aggStats); err != nil {
		msg := fmt.Sprintf("Cannot unmarshal returned response: %s", err.Error())
		fmt.Fprintln(writer, msg)
	}

	fmt.Fprintln(writer, "Slot stats")
	for i := 0; i <= stats.LargestSlotSeen; i++ {
		slotVal := fmt.Sprintf("%012d", stats.SlotStats[i].Number)
		bfgQtyVal := fmt.Sprintf("%.3f", stats.SlotStats[i].EnergyUse)
		bfgPpuVal := fmt.Sprintf("%.3f", stats.SlotStats[i].PricePaid)
		stgQtyVal := fmt.Sprintf("%.3f", stats.SlotStats[i].EnergyGen)
		stgPpuVal := fmt.Sprintf("%.3f", stats.SlotStats[i].PriceSold)
		dmiQtyVal := fmt.Sprintf("%.3f", stats.SlotStats[i].EnergyTraded)
		dmiPpuVal := fmt.Sprintf("%.3f", stats.SlotStats[i].PriceTraded)
		lateAllVal := fmt.Sprintf("%d", aggStats.LateTXsCount[i])
		lateBuyVal := fmt.Sprintf("%d", aggStats.LateBuysCount[i])
		lateSellVal := fmt.Sprintf("%d", aggStats.LateSellsCount[i])
		fmt.Fprintf(writer,
			"[slot: %s]"+
				"\t%s kWh bought from the grid @ %s ç/kWh"+
				"\t\t%s kWh sold to grid @ %s ç/kWh"+
				"\t\t%s kWh of demand met internally @ %s ç/kWh"+
				"\t\t%s late transactions (total)"+
				"\t\t%s late buy transactions"+
				"\t\t%s late sell transactions\n",
			slotVal,
			bfgQtyVal, bfgPpuVal,
			stgQtyVal, stgPpuVal,
			dmiQtyVal, dmiPpuVal,
			lateAllVal, lateBuyVal, lateSellVal,
		)
		if err := slotwriter.Write([]string{slotVal,
			bfgQtyVal, bfgPpuVal,
			stgQtyVal, stgPpuVal,
			dmiQtyVal, dmiPpuVal,
			lateAllVal, lateBuyVal, lateSellVal}); err != nil {
			return err
		}
	}

	println()

	fmt.Fprintf(writer, "Number of goroutines still running: %d\n", runtime.NumGoroutine())

	return nil
}
