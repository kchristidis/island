package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/kchristidis/exp2/agent"
	"github.com/kchristidis/exp2/blockchain"
	"github.com/kchristidis/exp2/blocknotifier"
	"github.com/kchristidis/exp2/crypto"
	"github.com/kchristidis/exp2/csv"
	"github.com/kchristidis/exp2/markend"
	"github.com/kchristidis/exp2/slotnotifier"
	"github.com/kchristidis/exp2/stats"
)

// Constants ...
const (
	StatChannelBuffer = 100
	TraceLength       = 35036
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
		trace        map[int][][]float64

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

	tracepath := filepath.Join("csv", csv.Filename)
	trace, err = csv.Load(tracepath)
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
		ChaincodeID: "exp2",

		ChannelConfigPath:   os.Getenv("GOPATH") + "/src/github.com/kchristidis/exp2/fixtures/artifacts/clark-channel.tx",
		ChaincodeGoPath:     os.Getenv("GOPATH"),
		ChaincodeSourcePath: "github.com/kchristidis/exp2/chaincode/",
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

	agents = make([]*agent.Agent, csv.IDCount)
	// for i, ID := range []int{171, 1103} // ATTN: Temporary modification:
	for i, ID := range csv.IDs {
		agents[i] = agent.New(sdkctx, snotifier, pubkey, trace[ID], ID, statslotc, statstranc, writer, donec)
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

	println()

	for _, b := range stats.BlockStats {
		fmt.Fprintf(writer,
			"[block: %012d]"+
				"\t%f kiB\n",
			b.Number,
			b.SizeInKB,
		)
	}

	println()

	for i := 0; i <= stats.LargestSlotSeen; i++ {
		fmt.Fprintf(writer,
			"[slot: %012d]"+
				"\t%.3f kWh bought from the grid @ %.3f ç/kWh"+
				"\t\t%.3f kWh sold to grid @ %.3f ç/kWh"+
				"\t\t%.3f kWh of demand met internally @ %.3f ç/kWh\n",
			stats.SlotStats[i].Number,
			stats.SlotStats[i].EnergyUse, stats.SlotStats[i].PricePaid,
			stats.SlotStats[i].EnergyGen, stats.SlotStats[i].PriceSold,
			stats.SlotStats[i].EnergyTraded, stats.SlotStats[i].PriceTraded,
		)
	}

	println()

	for _, tx := range stats.TransactionStats {
		idNum, _ := strconv.Atoi(tx.ID)
		fmt.Fprintf(writer,
			"[txID: %012d]"+
				"\tlatency:%d ms"+
				"\t\ttype:%s"+
				"\t\tstatus:%s\n",
			idNum,
			tx.LatencyInMillis,
			tx.Type,
			tx.Status,
		)
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

	fmt.Fprintln(writer, "Late transactions:")

	for i := 0; i < stats.LargestSlotSeen+1; i++ {
		fmt.Fprintf(writer,
			"[slot: %012d]"+
				"\toverall: %d"+
				"\t\tbuys: %d"+
				"\t\tsells: %d\n",
			i,
			aggStats.LateTXsCount[i],
			aggStats.LateBuysCount[i],
			aggStats.LateSellsCount[i],
		)
	}

	println()

	fmt.Fprintf(writer, "Number of goroutines still running: %d\n", runtime.NumGoroutine())

	return nil
}
