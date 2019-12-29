package blocknotifier

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/stats"
)

// BufferLen sets the buffer length for the block channel.
const BufferLen = 100

//go:generate counterfeiter . Invoker

// Invoker is an interface that encapsulates the
// peer calls that are relevant to the notifier.
type Invoker interface {
	Invoke(args schema.OpContextInput) ([]byte, error)
}

//go:generate counterfeiter . Querier

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

	BlockChan  chan stats.Block   // Used to feed the stats collector
	BlockQueue chan *common.Block // Used to decouple the feeding of the stats collector from the main thread

	SlotChan chan int // Post slot notifications here

	ErrorThreshold int // How many failed query attempts can we tolerate before quitting?

	LargestBlockHeightObserved uint64 // Used by the block stats collector
	LargestSlotTriggered       int

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

		BlockChan:  blockc,
		BlockQueue: make(chan *common.Block, BufferLen),

		SlotChan: slotc,

		ErrorThreshold: 10,

		LargestSlotTriggered: -1,

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
			case block := <-n.BlockQueue:
				n.RecordStats(block)
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
			var block *common.Block
			var err error

			for i := 1; i <= n.ErrorThreshold; i++ {
				block, err = n.Querier.QueryBlock(math.MaxUint64)
				if err != nil {
					msg = fmt.Sprintf("block-notifier:%02d • cannot query ledger (errcnt:%d): %s", n.StartFromBlock, i, err.Error())
					fmt.Fprintln(n.Writer, msg)
					if i >= n.ErrorThreshold {
						return err
					}
					// time.Sleep(n.SleepDuration)
				} else {
					break
				}
			}

			// If we're here, we've successfully queried a block.
			if inHeight := block.GetHeader().GetNumber(); inHeight > 0 {
				if inHeight > n.LargestBlockHeightObserved {
					n.LargestBlockHeightObserved = inHeight
					n.BlockQueue <- block                                    // Feed that block to the stats collector
					if int(inHeight-n.StartFromBlock)%n.BlocksPerSlot == 0 { // Is is this block marking a new slot?
						n.LargestSlotTriggered = int(inHeight-n.StartFromBlock) / n.BlocksPerSlot
						msg = fmt.Sprintf("block-notifier:%02d block:%012d • new slot! block corresponds to slot %012d", n.StartFromBlock, inHeight, n.LargestSlotTriggered)
						fmt.Fprintln(n.Writer, msg)
						n.SlotChan <- n.LargestSlotTriggered
					}
				}
			}

			time.Sleep(n.SleepDuration)
		}
	}
}

// RecordStats on newly observed blocks.
func (n *Notifier) RecordStats(block *common.Block) error {
	height := block.GetHeader().GetNumber()
	var msg string

	/* if schema.StagingLevel <= schema.Debug {
		msg = fmt.Sprintf("block-notifier:%02d block:%012d • block committed at the peer", n.StartFromBlock, height)
		fmt.Fprintln(n.Writer, msg)
	} */

	blockB, err := proto.Marshal(block)
	if err != nil {
		msg = fmt.Sprintf("block-notifier:%02d • cannot marshal block %d: %s", n.StartFromBlock, height, err.Error())
		fmt.Fprintln(n.Writer, msg)
		return err
	}

	blockStat := stats.Block{
		Number: height,
		Size:   float32(len(blockB)) / 1024, // Size in KiB
	}

	select {
	case n.BlockChan <- blockStat:
		if schema.StagingLevel <= schema.Debug {
			msg = fmt.Sprintf("block-notifier:%02d block:%012d • pushed block to the stats collector (chlen:%d)", n.StartFromBlock, height, len(n.BlockChan))
			fmt.Fprintln(n.Writer, msg)
		}
	default:
	}

	return nil
}
