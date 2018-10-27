package main

import (
	"crypto/rsa"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kchristidis/exp2/agent"
	"github.com/kchristidis/exp2/blockchain"
	"github.com/kchristidis/exp2/blocknotifier"
	"github.com/kchristidis/exp2/crypto"
	"github.com/kchristidis/exp2/csv"
	"github.com/kchristidis/exp2/markend"
	"github.com/kchristidis/exp2/slotnotifier"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		err           error
		agents        []*agent.Agent
		bnotifier     *blocknotifier.Notifier
		blocksperslot int
		clockperiod   time.Duration
		donec         chan struct{}
		heightc       chan int
		markendagent  *markend.Agent
		once          sync.Once
		privkeybytes  []byte
		pubkey        *rsa.PublicKey
		sdkctx        *blockchain.SDKContext
		snotifier     *slotnotifier.Notifier
		startblock    uint64
		trace         map[int][][]float64
		wg1, wg2      sync.WaitGroup
		writer        io.Writer
	)

	// Stats
	/* var (
		BytesPerBlock                 []int
		LatenciesPerBidInMilliseconds []int
	) */

	// Global vars

	blocksperslot = 3
	clockperiod = 500 * time.Millisecond
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
	for i, ID := range csv.IDs { // ATTN: Temporary modification: []int{171, 1103}
		agents[i] = agent.New(ID, trace[ID], sdkctx, snotifier, pubkey, donec, writer)
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

	bnotifier = &blocknotifier.Notifier{
		BlocksPerSlot:  blocksperslot,
		ClockPeriod:    clockperiod,
		DoneChan:       donec,
		Invoker:        sdkctx,
		OutChan:        heightc,
		Querier:        sdkctx.LedgerClient,
		StartFromBlock: startblock,
		SleepDuration:  100 * time.Millisecond,
		Writer:         writer,
	}

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

	/* if resp, err := sc.Query(2, "bid"); err != nil {
		return err
	} else {
		fmt.Fprintf(os.Stdout, "Response from chaincode query for '2/bid': %s\n", resp)
	} */

	wg1.Wait()
	once.Do(func() {
		close(donec)
	})
	wg2.Wait()

	fmt.Fprintf(os.Stdout, "Run completed successfully 😎")

	println()

	return nil
}
