package blocknotifier

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
)

// Invoker ...
//go:generate counterfeiter . Invoker
type Invoker interface {
	Invoke(slot int, action string, dataB []byte) ([]byte, error)
}

// Querier ...
//go:generate counterfeiter . Querier
type Querier interface {
	QueryBlock(blockNumber uint64, options ...ledger.RequestOption) (*common.Block, error)
	QueryInfo(opts ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)
}

// Notifier ...
type Notifier struct {
	BlocksPerSlot    int           // How many blocks constitute a slot?
	ClockPeriod      time.Duration // How often do you invoke the clock method?
	DoneChan         chan struct{}
	Invoker          Invoker
	LastHeight       uint64 // Used by the block notifier
	LastSlot         int
	MostRecentHeight uint64 // A LastHeight variable used for the block size calculations
	OutChan          chan int
	Querier          Querier
	SleepDuration    time.Duration // How often do you check for new blocks?
	StartFromBlock   uint64
	Writer           io.Writer

	wg       sync.WaitGroup
	killChan chan struct{}
}

// New ...
func New(startblock uint64, blocksperslot int, clockperiod time.Duration, sleepduration time.Duration,
	invoker Invoker, querier Querier, heightc chan int,
	donec chan struct{}, writer io.Writer) *Notifier {
	return &Notifier{
		BlocksPerSlot:  blocksperslot,
		ClockPeriod:    clockperiod,
		DoneChan:       donec,
		Invoker:        invoker,
		OutChan:        heightc,
		Querier:        querier,
		SleepDuration:  sleepduration,
		StartFromBlock: startblock,
		Writer:         writer,

		killChan: make(chan struct{}),
	}
}

// Run ...
func (n *Notifier) Run() error {
	defer fmt.Fprintln(n.Writer, "[block notifier] Exited")

	msg := fmt.Sprint("[block notifier] Running")
	fmt.Fprintln(n.Writer, msg)

	defer func() {
		close(n.killChan)
		n.wg.Wait()
	}()

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ticker := time.NewTicker(n.ClockPeriod)
		for {
			select {
			case <-n.killChan:
				return
			case <-ticker.C:
				n.Invoker.Invoke(0, "clock", nil)
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
			inHeight := resp.BCI.GetHeight()

			if inHeight > n.MostRecentHeight {
				n.MostRecentHeight = inHeight
				msg := fmt.Sprintf("[block notifier] Block %d committed at the peer", int(inHeight))
				fmt.Fprintln(n.Writer, msg)
				// ATTN: We query the PREVIOUS block because the peer doesn't index the current block
				// as soon as an event for it is sent out. We therefore need to delay the query of the
				// current block, otherwise we'll error.
				block, err := n.Querier.QueryBlock(n.MostRecentHeight - 1)
				if err != nil {
					msg := fmt.Sprintf("[block notifier] Unable to get block %d's size: %s", n.MostRecentHeight-1, err.Error())
					fmt.Fprintln(n.Writer, msg)
					return err
				} else {
					_, err := proto.Marshal(block)
					if err != nil {
						msg := fmt.Sprintf("[block notifier] Unable to marshal block %d: %s", n.MostRecentHeight-1, err.Error())
						fmt.Fprintln(n.Writer, msg)
						return err
					}
					// msg := fmt.Sprintf("[block notifier] Block %d size in kiB: %d", n.MostRecentHeight-1, len(blockBytes)/1024)
					// fmt.Fprintln(n.Writer, msg)
				}
			}

			switch n.LastHeight {
			case 0: // If this is the case, then we haven't reached StartFromBlock yet
				if inHeight < n.StartFromBlock {
					continue
				}

				if inHeight != n.StartFromBlock {
					msg := fmt.Sprintf("[block notifier] WARNING: This is NOT the start block (%d)", int(n.LastHeight))
					fmt.Println(n.Writer, msg)
					return fmt.Errorf("[block notifier] Expected to start with block %d, got block %d instead", n.StartFromBlock, inHeight)
				}

				n.LastSlot = int(inHeight - n.StartFromBlock) // should be 0
				n.LastHeight = inHeight
				msg = fmt.Sprintf("[block notifier] Block corresponds to slot %d", n.LastSlot)
				fmt.Fprintln(n.Writer, msg)
				n.OutChan <- n.LastSlot
			default: // We're hitting this case only after we've reached StartFromBlock
				if inHeight <= n.LastHeight {
					continue
				}

				if int(inHeight-n.StartFromBlock)%n.BlocksPerSlot == 0 {
					n.LastSlot = int(inHeight-n.StartFromBlock) / n.BlocksPerSlot
					n.LastHeight = inHeight
					msg := fmt.Sprintf("[block notifier] Block corresponds to slot %d", n.LastSlot)
					fmt.Fprintln(n.Writer, msg)
					n.OutChan <- n.LastSlot
				}
			}
			time.Sleep(n.SleepDuration)
		}
	}
}
