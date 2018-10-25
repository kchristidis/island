package blocknotifier

import (
	"fmt"
	"io"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
)

// Invoker ...
//go:generate counterfeiter . Invoker
type Invoker interface {
	Invoke(slot int, action string, dataB []byte) ([]byte, error)
}

// Querier ...
//go:generate counterfeiter . Querier
type Querier interface {
	QueryInfo(opts ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)
}

// Notifier ...
type Notifier struct {
	BlocksPerSlot  int           // How many blocks constitute a slot?
	ClockPeriod    time.Duration // How often do you invoke the clock method?
	DoneChan       chan struct{}
	Invoker        Invoker
	LastHeight     uint64
	LastSlot       int
	OutChan        chan int
	Querier        Querier
	SleepDuration  time.Duration
	StartFromBlock uint64
	Writer         io.Writer
}

// Run ...
func (n *Notifier) Run() error {
	msg := fmt.Sprint("[block notifier] Running")
	fmt.Fprintln(n.Writer, msg)

	go func() {
		ticker := time.NewTicker(n.ClockPeriod)
		for {
			select {
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
			switch n.LastHeight {
			case 0: // If this is the case, then we haven't reached StartFromBlock yet
				if inHeight < n.StartFromBlock {
					continue
				}
				msg := fmt.Sprintf("[block notifier] Block %d committed at the peer", int(inHeight))
				fmt.Fprintln(n.Writer, msg)

				if inHeight != n.StartFromBlock {
					msg := fmt.Sprintf("[block notifier] WARNING: This is NOT the start block (%d)", int(n.LastHeight))
					fmt.Println(n.Writer, msg)
					return fmt.Errorf("[block notifier] Expected to start with block %d, got block %d instead", n.StartFromBlock, inHeight)
				}

				msg = fmt.Sprint("[block notifier] This is the start block")
				fmt.Println(n.Writer, msg)

				n.LastSlot = int(inHeight - n.StartFromBlock) // should be 0
				n.LastHeight = inHeight
				n.OutChan <- n.LastSlot
			default: // We're hitting this case only after we've reached StartFromBlock
				if inHeight <= n.LastHeight {
					continue
				}

				if int(inHeight-n.LastHeight)%n.BlocksPerSlot == 0 {
					n.LastSlot = int(inHeight-n.LastHeight) / n.BlocksPerSlot
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
