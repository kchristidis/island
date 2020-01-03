package blocknotifier

import (
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/stats"
)

// BufferLen sets the buffer length for the block channel.
const BufferLen = 100

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Invoker

// Invoker is an interface that encapsulates the
// peer calls that are relevant to the notifier.
type Invoker interface {
	Invoke(args schema.OpContextInput) ([]byte, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Querier

// Querier is an interface that encapsulates the
// ledger calls that are relevant to the notifier.
type Querier interface {
	QueryBlock(blockNumber uint64, options ...ledger.RequestOption) (*common.Block, error)
	QueryInfo(opts ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)
}

// Notifier queries a ledger for new blocks, and posts slot notifications
// whenever the block that marks the beginning of a new slot is received.
type Notifier struct {
	BlocksPerSlot  int           // How many blocks constitute a slot in our experiment?
	ClockPeriod    time.Duration // How often do we invoke the clock method to help with the creation of new blocks?
	SleepDuration  time.Duration // How often do we check for new blocks?
	StartFromBlock uint64        // Which block will be the very first block of the first slot?

	BlockNumberChan chan int64       // Used to decouple the GetBlock goroutine from the main thread
	BlockChan       chan stats.Block // Used by the GetBlock goroutine to feed the stats collector

	SlotChan chan int // Post slot notifications here

	LargestBlockNumberObserved int64 // This is updated by the main thread
	LargestBlockNumberQueried  int64 // This is updated by the BlockInfo goroutine
	LargestSlotNumberTriggered int   // This is updated by the main thread

	Once    sync.Once
	Started bool // Dictates whether we check for equality between CurrentBlockNumber and LargestBlockNumberObserved

	Invoker Invoker
	Querier Querier

	Writer io.Writer // Used for logging

	DoneChan  chan struct{}   // An external kill switch. Signals to all threads in this package that they should return.
	killChan  chan struct{}   // An internal kill switch. It can only be closed by this package, and it signals to the package's goroutines that they should exit.
	waitGroup *sync.WaitGroup // Ensures that the main thread in this package doesn't return before the goroutines it spawned have.
}

// New returns a new notifier.
func New(blocksperslot int, clockperiod time.Duration, sleepduration time.Duration, startfromblock uint64,
	blockc chan stats.Block, slotc chan int,
	invoker Invoker, querier Querier,
	writer io.Writer, donec chan struct{}) *Notifier {
	return &Notifier{
		BlocksPerSlot:  blocksperslot,
		ClockPeriod:    clockperiod,
		SleepDuration:  sleepduration,
		StartFromBlock: startfromblock,

		BlockNumberChan: make(chan int64, BufferLen),
		BlockChan:       blockc,

		SlotChan: slotc,

		LargestBlockNumberObserved: -1,
		LargestBlockNumberQueried:  -1,
		LargestSlotNumberTriggered: -1,

		Invoker: invoker,
		Querier: querier,

		Writer: writer,

		DoneChan:  donec,
		killChan:  make(chan struct{}),
		waitGroup: new(sync.WaitGroup),
	}
}

// Run executes the notifier logic.
func (n *Notifier) Run() error {
	defer func() {
		msg := fmt.Sprintf("block-notifier:%02d • exited", n.StartFromBlock)
		fmt.Fprintln(n.Writer, msg)
	}()

	msg := fmt.Sprintf("block-notifier:%02d • running", n.StartFromBlock)
	fmt.Fprintln(n.Writer, msg)

	defer func() {
		close(n.killChan)
		n.waitGroup.Wait()
	}()

	n.waitGroup.Add(1)
	go func() {
		defer n.waitGroup.Done()
		ticker := time.NewTicker(n.ClockPeriod)
		for {
			select {
			case <-n.killChan:
				return
			case <-ticker.C:
				args := schema.OpContextInput{
					EventID: strconv.Itoa(rand.Intn(1E12)),
					Action:  "clock",
				}
				n.Invoker.Invoke(args)
			case <-n.DoneChan:
				return
			}
		}
	}()

	n.waitGroup.Add(1)
	go func() {
		defer n.waitGroup.Done()
		for {
			select {
			case <-n.killChan:
				return
			case blockNumber := <-n.BlockNumberChan:
				n.GetBlock(blockNumber)
			case <-n.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case <-n.DoneChan:
			return nil
		default:
			resp, err := n.Querier.QueryInfo()
			if err != nil {
				msg = fmt.Sprintf("block-notifier:%02d • cannot query info: %s", n.StartFromBlock, err.Error())
				fmt.Fprintln(n.Writer, msg)
				return err
			}

			CurrentBlockNumber := int64(resp.BCI.GetHeight() - 1)

			if CurrentBlockNumber > n.LargestBlockNumberObserved {
				n.LargestBlockNumberObserved = CurrentBlockNumber
				if schema.EnableBlockStatsCollection {
					n.BlockNumberChan <- n.LargestBlockNumberObserved
				}
				if n.LargestBlockNumberObserved >= int64(n.StartFromBlock) {
					slot := int((n.LargestBlockNumberObserved - int64(n.StartFromBlock)) / int64(n.BlocksPerSlot))
					if slot > n.LargestSlotNumberTriggered {
						n.LargestSlotNumberTriggered = slot
						msg = fmt.Sprintf("block-notifier:%02d block:%012d • new slot! block triggered slot %012d", n.StartFromBlock, n.LargestBlockNumberObserved, n.LargestSlotNumberTriggered)
						fmt.Fprintln(n.Writer, msg)
						n.SlotChan <- n.LargestSlotNumberTriggered
					}
				}
			}

			time.Sleep(n.SleepDuration)
		}
	}
}

// GetBlock gets all blocks in the (n.LargestBlockNumberQueried, blockNumber] interval and feeds them to the stats collector.
func (n *Notifier) GetBlock(blockNumber int64) {
	var msg string
	for i := n.LargestBlockNumberQueried + 1; i <= blockNumber; i++ {
		block, err := n.Querier.QueryBlock(uint64(i))
		if err != nil {
			msg = fmt.Sprintf("block-notifier:%02d block:%012d • cannot query block: %s", n.StartFromBlock, i, err.Error())
			fmt.Fprintln(n.Writer, msg)
			continue
		}
		blockB, err := proto.Marshal(block)
		if err != nil {
			msg = fmt.Sprintf("block-notifier:%02d • cannot marshal block %d: %s", n.StartFromBlock, i, err.Error())
			fmt.Fprintln(n.Writer, msg)
			continue
		}
		blockStat := stats.Block{
			Number: uint64(i),
			Size:   float32(len(blockB)) / 1024, // Size in KiB
		}
		select {
		case n.BlockChan <- blockStat:
			if schema.StagingLevel <= schema.Debug {
				msg = fmt.Sprintf("block-notifier:%02d block:%012d • pushed block to the stats collector (chlen:%d)", n.StartFromBlock, i, len(n.BlockChan))
				fmt.Fprintln(n.Writer, msg)
			}
		default:
		}
	}
	n.LargestBlockNumberQueried = blockNumber
}
