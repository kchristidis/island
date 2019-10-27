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
	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/stats"
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

// Notifier queries a ledger for new blocks, and posts slot notifications
// whenever the block that marks the beginning of a new slot is received.
type Notifier struct {
	BlocksPerSlot  int           // How many blocks constitute a slot in our experiment?
	ClockPeriod    time.Duration // How often do we invoke the clock method to help with the creation of new blocks?
	SleepDuration  time.Duration // How often do we check for new blocks?
	StartFromBlock uint64        // Which block will be the very first block of the first slot?

	BlockChan chan stats.Block // Used to feed the stats collector

	SlotChan chan int // Post slot notifications here

	BlockHeightOfMostRecentSlot uint64 // Used by the slot notifier
	MostRecentSlot              int
	MostRecentBlockHeight       uint64 // Used by the block stats collector

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

		BlockChan: blockc,

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
				msg = fmt.Sprintf("block-notifier:%02d • cannot query ledger: %s", n.StartFromBlock, err.Error())
				fmt.Fprintln(n.Writer, msg)
				return err
			}

			inHeight := resp.BCI.GetHeight() - 1 // ATTN: Do not forget to decrement by 1
			if inHeight > n.MostRecentBlockHeight {
				n.MostRecentBlockHeight = inHeight

				if schema.StagingLevel <= schema.Debug {
					msg = fmt.Sprintf("block-notifier:%02d block:%012d • block committed at the peer", n.StartFromBlock, int(n.MostRecentBlockHeight))
					fmt.Fprintln(n.Writer, msg)
				}

				/*
					As a temporary solution to this problem "cannot get block 59797's size:
					QueryBlock failed: Transaction processing for endorser [localhost:7051]:
					Chaincode status Code: (500) UNKNOWN. Description: Failed to get block
					number 59797, error Error getting envelope(unexpected EOF)", we query a
					lagging height.
				*/

				if inHeight >= n.StartFromBlock {
					laggingHeight := inHeight - n.StartFromBlock
					block, err := n.Querier.QueryBlock(laggingHeight)
					if err != nil {
						msg = fmt.Sprintf("block-notifier:%02d • cannot get block %d's size: %s", n.StartFromBlock, laggingHeight, err.Error())
						fmt.Fprintln(n.Writer, msg)
						return err
					}
					blockB, err := proto.Marshal(block)
					if err != nil {
						msg = fmt.Sprintf("block-notifier:%02d • cannot marshal block %d: %s", n.StartFromBlock, laggingHeight, err.Error())
						fmt.Fprintln(n.Writer, msg)
						return err
					}

					blockStat := stats.Block{
						Number: laggingHeight,
						Size:   float32(len(blockB)) / 1024, // Size in KiB
					}
					select {
					case n.BlockChan <- blockStat:
					default:
					}

					if schema.StagingLevel <= schema.Debug {
						msg = fmt.Sprintf("block-notifier:%02d block:%012d • pushed block to the stats collector (len: %d)", n.StartFromBlock, laggingHeight, len(n.BlockChan))
						fmt.Fprintln(n.Writer, msg)
					}
				}
			}

			// Slot notifier
			switch n.BlockHeightOfMostRecentSlot {
			case 0: // If this is the case, then we haven't reached StartFromBlock yet
				if inHeight < n.StartFromBlock {
					continue
				}

				if inHeight != n.StartFromBlock {
					msg = fmt.Sprintf("block-notifier:%02d block:%012d • expected block %012d as start block", n.StartFromBlock, inHeight, n.StartFromBlock)
					fmt.Println(n.Writer, msg)
					return fmt.Errorf(msg)
				}

				n.MostRecentSlot = int(inHeight - n.StartFromBlock) // This should be 0
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
