package blocknotifier

import (
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/kchristidis/island/exp2/stats"
)

// Invoker ...
//go:generate counterfeiter . Invoker
type Invoker interface {
	Invoke(txID string, slot int, action string, dataB []byte) ([]byte, error)
}

// Querier ...
//go:generate counterfeiter . Querier
type Querier interface {
	QueryBlock(blockNumber uint64, options ...ledger.RequestOption) (*common.Block, error)
	QueryInfo(opts ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)
}

// Notifier ...
type Notifier struct {
	BlocksPerSlot  int           // How many blocks constitute a slot in your experiment?
	ClockPeriod    time.Duration // How often do you invoke the clock method to help with the creation of new blocks?
	SleepDuration  time.Duration // How often do you check for new blocks?
	StartFromBlock uint64        // Which block will be the very first block of the first slot?

	BlockChan chan stats.Block // Post block stat (size) notifications here.
	SlotChan  chan int         // Post slot notifications here.

	BlockHeightOfMostRecentSlot uint64 // Used by the slot notifier.
	MostRecentSlot              int
	MostRecentBlockHeight       uint64 // Used by the block stats collector.

	Invoker Invoker
	Querier Querier

	Writer io.Writer // Used for logging.

	DoneChan  chan struct{}   // An external kill switch. Signals to all threads in this package that they should return.
	killChan  chan struct{}   // An internal kill switch. It can only be closed by this package, and it signals to the package's goroutines that they should exit.
	waitGroup *sync.WaitGroup // Ensures that the main thread in this package doesn't return before the goroutines it spawned also have as well.
}

// New ...
func New(blocksperslot int, clockperiod time.Duration, sleepduration time.Duration, startfromblock uint64,
	blockc chan stats.Block, slotc chan int,
	invoker Invoker, querier Querier,
	writer io.Writer, donec chan struct{}) *Notifier {
	return &Notifier{
		BlocksPerSlot:  blocksperslot,
		ClockPeriod:    clockperiod,
		SleepDuration:  sleepduration,
		StartFromBlock: startfromblock,

		BlockChan: blockc,
		SlotChan:  slotc,

		Invoker: invoker,
		Querier: querier,

		Writer: writer,

		DoneChan:  donec,
		killChan:  make(chan struct{}),
		waitGroup: new(sync.WaitGroup),
	}
}

// Run ...
func (n *Notifier) Run() error {
	defer fmt.Fprintln(n.Writer, "[block notifier] Exited")

	msg := fmt.Sprint("[block notifier] Running")
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
				txID := strconv.Itoa(rand.Intn(1E6))
				n.Invoker.Invoke(txID, 0, "clock", nil)
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
				msg := fmt.Sprintf("[block notifier] Unable to query ledger: %s", err.Error())
				fmt.Fprintln(n.Writer, msg)
				return err
			}
			inHeight := resp.BCI.GetHeight() - 1 // ATTN

			// Block stats collector
			if inHeight > n.MostRecentBlockHeight {
				n.MostRecentBlockHeight = inHeight
				msg := fmt.Sprintf("[block notifier] Block %d committed at the peer", int(inHeight))
				fmt.Fprintln(n.Writer, msg)
				// ATTN: We query the PREVIOUS block because the peer doesn't index the current block
				// as soon as an event for it is sent out. We therefore need to delay the query of the
				// current block, otherwise we'll error.
				block, err := n.Querier.QueryBlock(n.MostRecentBlockHeight - 1)
				if err != nil {
					msg := fmt.Sprintf("[block notifier] Unable to get block %d's size: %s", n.MostRecentBlockHeight-1, err.Error())
					fmt.Fprintln(n.Writer, msg)
					return err
				} else {
					blockBytes, err := proto.Marshal(block)
					if err != nil {
						msg := fmt.Sprintf("[block notifier] Unable to marshal block %d: %s", n.MostRecentBlockHeight-1, err.Error())
						fmt.Fprintln(n.Writer, msg)
						return err
					}
					// For the stats collector
					n.BlockChan <- stats.Block{
						Number:   int(n.MostRecentBlockHeight) - 1,
						SizeInKB: float32(len(blockBytes)) / 1024,
					}
					// msg := fmt.Sprintf("[block notifier] Block %d size in KiB: %d", n.MostRecentBlockHeight-1, len(blockBytes)/1024)
					// fmt.Fprintln(n.Writer, msg)
				}
			}

			// Slot notifier
			switch n.BlockHeightOfMostRecentSlot {
			case 0: // If this is the case, then we haven't reached StartFromBlock yet
				if inHeight < n.StartFromBlock {
					continue
				}

				if inHeight != n.StartFromBlock {
					msg := fmt.Sprintf("[block notifier] WARNING: This is NOT the start block (expected %d - got %d)",
						int(n.BlockHeightOfMostRecentSlot), int(inHeight))
					fmt.Println(n.Writer, msg)
					return fmt.Errorf("[block notifier] Expected to start with block %d, got block %d instead", n.StartFromBlock, inHeight)
				}

				n.MostRecentSlot = int(inHeight - n.StartFromBlock) // should be 0
				n.BlockHeightOfMostRecentSlot = inHeight
				msg = fmt.Sprintf("[block notifier] Block %d corresponds to slot %d", inHeight, n.MostRecentSlot)
				fmt.Fprintln(n.Writer, msg)
				n.SlotChan <- n.MostRecentSlot
			default: // We're hitting this case only after we've reached StartFromBlock
				if inHeight <= n.BlockHeightOfMostRecentSlot {
					continue
				}

				if int(inHeight-n.StartFromBlock)%n.BlocksPerSlot == 0 {
					n.MostRecentSlot = int(inHeight-n.StartFromBlock) / n.BlocksPerSlot
					n.BlockHeightOfMostRecentSlot = inHeight
					msg := fmt.Sprintf("[block notifier] Block corresponds to slot %d", n.MostRecentSlot)
					fmt.Fprintln(n.Writer, msg)
					n.SlotChan <- n.MostRecentSlot
				}
			}
			time.Sleep(n.SleepDuration)
		}
	}
}
