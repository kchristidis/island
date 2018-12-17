package blocknotifier

import (
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/kchristidis/island/chaincode/schema"
)

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

// Notifier queries a ledger for new block and posts slot notifications
// whenever the block that marks the beginning of a new slot is received.
type Notifier struct {
	BlocksPerSlot  int           // How many blocks constitute a slot in our experiment?
	ClockPeriod    time.Duration // How often do we invoke the clock method to help with the creation of new blocks?
	SleepDuration  time.Duration // How often do we check for new blocks?
	StartFromBlock uint64        // Which block will be the very first block of the first slot?

	// BlockChan chan stats.Block // Post block stat (size) notifications here
	SlotChan chan int // Post slot notifications here

	BlockHeightOfMostRecentSlot uint64 // Used by the slot notifier
	MostRecentSlot              int
	MostRecentBlockHeight       uint64 // Used by the block stats collector

	Invoker Invoker
	Querier Querier

	Writer io.Writer // Used for logging

	DoneChan  chan struct{}   // An external kill switch. Signals to all threads in this package that they should return.
	killChan  chan struct{}   // An internal kill switch. It can only be closed by this package, and it signals to the package's goroutines that they should exit.
	waitGroup *sync.WaitGroup // Ensures that the main thread in this package doesn't return before the goroutines it spawned also have as well.
}

// New returns a new notifier.
func New(blocksperslot int, clockperiod time.Duration, sleepduration time.Duration, startfromblock uint64,
	slotc chan int,
	invoker Invoker, querier Querier,
	writer io.Writer, donec chan struct{}) *Notifier {
	return &Notifier{
		BlocksPerSlot:  blocksperslot,
		ClockPeriod:    clockperiod,
		SleepDuration:  sleepduration,
		StartFromBlock: startfromblock,

		SlotChan: slotc,

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

	for {
		select {
		case <-n.DoneChan:
			return nil
		default:
			resp, err := n.Querier.QueryInfo()
			if err != nil {
				msg := fmt.Sprintf("block-notifier:%02d • cannot query ledger: %s", n.StartFromBlock, err.Error())
				fmt.Fprintln(n.Writer, msg)
				return err
			}

			inHeight := resp.BCI.GetHeight() - 1 // ATTN: Do not forget to decrement by 1
			if inHeight > n.MostRecentBlockHeight {
				n.MostRecentBlockHeight = inHeight
				// msg := fmt.Sprintf("block-notifier:%02d block:%012d • block committed at the peer", n.StartFromBlock, int(n.MostRecentBlockHeight))
				// fmt.Fprintln(n.Writer, msg)
			}

			// Slot notifier
			switch n.BlockHeightOfMostRecentSlot {
			case 0: // If this is the case, then we haven't reached StartFromBlock yet
				if inHeight < n.StartFromBlock {
					continue
				}

				if inHeight != n.StartFromBlock {
					msg := fmt.Sprintf("bloc-knotifier:%02d block:%012d • expected block %012d as start block", n.StartFromBlock, inHeight, n.StartFromBlock)
					fmt.Println(n.Writer, msg)
					return fmt.Errorf(msg)
				}

				n.MostRecentSlot = int(inHeight - n.StartFromBlock) // should be 0
				n.BlockHeightOfMostRecentSlot = inHeight
				msg = fmt.Sprintf("block-notifier:%02d block:%012d • new slot! block corresponds to slot %012d", n.StartFromBlock, inHeight, n.MostRecentSlot)
				fmt.Fprintln(n.Writer, msg)
				n.SlotChan <- n.MostRecentSlot
			default: // We're hitting this case only after we've reached StartFromBlock
				if inHeight <= n.BlockHeightOfMostRecentSlot {
					continue
				}

				if int(inHeight-n.StartFromBlock)%n.BlocksPerSlot == 0 {
					n.MostRecentSlot = int(inHeight-n.StartFromBlock) / n.BlocksPerSlot
					n.BlockHeightOfMostRecentSlot = inHeight
					msg := fmt.Sprintf("block-notifier:%02d block:%012d • new slot! block corresponds to slot %012d", n.StartFromBlock, inHeight, n.MostRecentSlot)
					fmt.Fprintln(n.Writer, msg)
					n.SlotChan <- n.MostRecentSlot
				}
			}
			time.Sleep(n.SleepDuration)
		}
	}
}
