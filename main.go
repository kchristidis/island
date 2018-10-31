package main

import (
	"crypto/rsa"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		err            error
		agents         []*agent.Agent
		bnotifier      *blocknotifier.Notifier
		blocksperslot  int
		clockperiod    time.Duration
		donec          chan struct{}
		heightc        chan int
		markendagent   *markend.Agent
		once           sync.Once
		privkeybytes   []byte
		pubkey         *rsa.PublicKey
		sdkctx         *blockchain.SDKContext
		sleepduration  time.Duration
		snotifier      *slotnotifier.Notifier
		startblock     uint64
		statscollector *stats.Collector
		statslotc      chan stats.Slot
		trace          map[int][][]float64
		wg1, wg2       sync.WaitGroup
		writer         io.Writer
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
	statslotc = make(chan stats.Slot)
	donec = make(chan struct{}) // acts as a coordination signal for goroutines
	startblock = uint64(10)
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
		DoneChan: donec,
		SlotChan: statslotc,
		Writer:   writer,
	}
	go statscollector.Run()

	// Set up the slot notifier

	heightc = make(chan int)
	snotifier = slotnotifier.New(heightc, donec, writer)

	// Set up and launch the agents

	markendagent = markend.New(sdkctx, snotifier, privkeybytes, donec, writer)
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
		agents[i] = agent.New(ID, trace[ID], sdkctx, snotifier, pubkey, statslotc, donec, writer)
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

	bnotifier = blocknotifier.New(startblock, blocksperslot, clockperiod, sleepduration, sdkctx, sdkctx.LedgerClient, heightc, donec, writer)

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

	wg1.Wait()
	once.Do(func() {
		close(donec)
	})
	wg2.Wait()

	println()

	fmt.Fprintln(os.Stdout, "Run completed successfully 😎")

	println()

	fmt.Fprintln(os.Stdout, "Time to collect & print results...")

	for i := 0; i <= stats.LargestSlotSeen; i++ {
		fmt.Fprintf(os.Stdout,
			"[%d] %.3f kWh bought from the grid @ %.3f ç/kWh - %.3f kWh bought from the grid @ %.3f ç/kWh\n",
			stats.SlotStats[i].Number,
			stats.SlotStats[i].EnergyUse, stats.SlotStats[i].PricePaid,
			stats.SlotStats[i].EnergyGen, stats.SlotStats[i].PriceSold,
		)
	}

	/* if resp, err := sdkctx.Query(2, "bid"); err != nil {
		return err
	} else {
		fmt.Fprintf(os.Stdout, "Response from chaincode query: %s\n", resp)
	} */

	println()

	fmt.Fprintf(os.Stdout, "Number of goroutines still running: %d\n", runtime.NumGoroutine())

	return nil
}
