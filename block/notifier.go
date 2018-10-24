package block

import (
	"fmt"
	"io"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
)

// LedgerQuerier ...
//go:generate counterfeiter . LedgerQuerier
type LedgerQuerier interface {
	QueryInfo(opts ...ledger.RequestOption) (*fab.BlockchainInfoResponse, error)
}

// Notifier ...
type Notifier struct {
	DoneChan       chan struct{}
	LastHeight     uint64
	LastSlot       int
	LedgerSource   LedgerQuerier
	Period         int
	OutChan        chan int
	SleepDuration  time.Duration
	StartFromBlock uint64
	Writer         io.Writer
}

// Run ...
func (n *Notifier) Run() error {
	for {
		select {
		case <-n.DoneChan:
			return nil
		default:
			resp, err := n.LedgerSource.QueryInfo()
			if err != nil {
				msg := fmt.Sprintf("Unable to query ledger: %s", err.Error())
				fmt.Fprintln(n.Writer, msg)
				return err
			}

			if inHeight := resp.BCI.GetHeight(); inHeight >= n.StartFromBlock {
				msg := fmt.Sprintf("Block %d received", int(inHeight))
				fmt.Fprintln(n.Writer, msg)
				switch n.LastHeight {
				case 0: // nil value for LastHeight
					if inHeight != n.StartFromBlock {
						msg := fmt.Sprintf("WARNING: This is NOT the start block (%d)", int(n.LastHeight))
						fmt.Println(n.Writer, msg)
						return fmt.Errorf("Expected to start with block %d, got block %d instead", n.StartFromBlock, inHeight)
					}
					msg := fmt.Sprintf("This is the start block")
					fmt.Println(n.Writer, msg)
					n.LastSlot = int(inHeight - n.StartFromBlock) // should be 0
					n.LastHeight = inHeight
					n.OutChan <- n.LastSlot
					continue
				default:
					if int(inHeight-n.LastHeight)%n.Period == 0 {
						n.LastSlot = int(inHeight-n.LastHeight) / n.Period
						n.LastHeight = inHeight
						msg := fmt.Sprintf("Corresponds to slot %d", n.LastSlot)
						fmt.Fprintln(n.Writer, msg)
						n.OutChan <- n.LastSlot
					}
				}
			}
			time.Sleep(n.SleepDuration)
		}
	}
}
