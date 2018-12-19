package main

import (
	"crypto/rsa"
	"io"
	"sync"
	"time"

	"github.com/kchristidis/island/bidder"
	"github.com/kchristidis/island/blockchain"
	"github.com/kchristidis/island/blocknotifier"
	"github.com/kchristidis/island/regulator"
	"github.com/kchristidis/island/slotnotifier"
	"github.com/kchristidis/island/stats"
	"github.com/kchristidis/island/trace"
)

// The files that this simulation will write for metrics/plotting
const (
	OutputDir   = "output"
	OutputTran  = "tran.csv"
	OutputSlot  = "slot.csv"
	OutputBlock = "block.csv"
)

// BidderCount counts the number of bidders we have in the system.
const BidderCount = trace.IDCount

// StatChannelBuffer sets the buffer of the channels we use to pipe metrics into the
// stats collector from the agents. The larger the buffer of those channels, the less
// chances the stats collector will block when aggregating stats.
const StatChannelBuffer = 100

var (
	err error

	// Which iteration is this? Used to name the files we're writing results to.
	iter int
	// The prefix for all `Output*` files above
	outputprefix string

	// Track the duration of a simulation
	timestart time.Time

	bidders    [BidderCount]*bidder.Bidder
	regtor     *regulator.Regulator
	bnotifiers []*blocknotifier.Notifier
	snotifiers []*slotnotifier.Notifier

	sdkctx *blockchain.SDKContext

	// How many blocks constitute a slot? A sensitivity analysis parameter.
	blocksperslot int
	// The delay in blocks between the signal to MarkEnd and the signal to PostKey.
	// A sensitivity analysis parameter.
	blockoffset int
	// How often do we issue the Clock call to help with the creation of new blocks?
	clockperiod time.Duration
	// How often do we check for new blocks?
	sleepduration time.Duration
	// Upon receiving this block, the markend slot notifier will send its first signal,
	// and the whole thing will get going. The optional second slot notifier will be
	// triggered upon receiving block <startfromblock>+<blockoffset>.
	startfromblock uint64

	// Key pairs for the agents
	// N.B. For this PoC, have all agents use the same key-pair. This works just fine for
	// our time-based measurements. In an actual deployment, each agent would use their own keys.
	privkey *rsa.PrivateKey

	// The original trace is converted into this typed structure for easier processing
	tracemap map[int][][]float64

	// The channel(s) on which notifications are received from the block notifier(s)
	slotcs []chan int

	// The typed conduits for agents to pipe metrics into the stats collector
	statblockc     chan stats.Block
	statslotc      chan stats.Slot
	statstranc     chan stats.Transaction
	statscollector *stats.Collector

	donec      chan struct{} // Acts as a coordination signal for goroutines
	once       sync.Once     // Ensures that donec is only closed one.
	donestatsc chan struct{} // We want to kill the stats collector *after* all the stats submitting goroutines have returned.

	// Waitgroups for goroutine coordination
	// - wg1: bidders
	// - wg2: regulators, block/slot notifiers
	// - wg3: stats collector
	wg1, wg2, wg3 sync.WaitGroup

	writer io.Writer // For logging
)
