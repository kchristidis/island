package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
)

// BufferLen ...
const BufferLen = 100

// Delta ...
const Delta = 0.0001

// ToKWh ...
const ToKWh = 0.25

// Column names ...
const (
	Gen = iota
	Grid
	Use
	Lo
	Hi
)

// SDKer ...
//go:generate counterfeiter . SDKer
type SDKer interface {
	Invoke(slot int, action string, dataB []byte) ([]byte, error)
}

// Notifier ...
//go:generate counterfeiter . Notifier
type Notifier interface {
	Register(id int, queue chan int) bool
}

// Agent ...
type Agent struct {
	BuyQueue  chan int
	DoneChan  chan struct{}
	ID        int
	SDK       SDKer
	SellQueue chan int
	SlotQueue chan int
	Notifier  Notifier
	Trace     [][]float64
	Writer    io.Writer
}

// New ...
func New(id int, trace [][]float64, sdkctx SDKer, notifier Notifier, donec chan struct{}, writer io.Writer) *Agent {
	return &Agent{
		BuyQueue:  make(chan int, BufferLen),
		DoneChan:  donec,
		ID:        id,
		SellQueue: make(chan int, BufferLen),
		SDK:       sdkctx,
		SlotQueue: make(chan int, BufferLen),
		Notifier:  notifier,
		Trace:     trace,
		Writer:    writer,
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprintf("[%d] Agent exited", a.ID)
	defer fmt.Fprintln(a.Writer, msg)

	if ok := a.Notifier.Register(a.ID, a.SlotQueue); !ok {
		return fmt.Errorf("[%d] Unable to register with signaler", a.ID)
	}

	go func() {
		for {
			select {
			case rowIdx := <-a.BuyQueue:
				a.Buy(rowIdx)
			case <-a.DoneChan:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case rowIdx := <-a.SellQueue:
				a.Sell(rowIdx)
			case <-a.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case slot := <-a.SlotQueue:
			rowIdx := int(slot)
			msg := fmt.Sprintf("[%d] Processing row %d: %v", a.ID, rowIdx, a.Trace[rowIdx])
			fmt.Fprintln(a.Writer, msg)

			select {
			case a.BuyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[%d] Unable to push row %d to 'buy' queue (size: %d)", a.ID, rowIdx, len(a.BuyQueue))
				fmt.Fprintln(a.Writer, msg)
			}

			select {
			case a.SellQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[%d] Unable to push row %d to 'sell' queue (size: %d)", a.ID, rowIdx, len(a.BuyQueue))
				fmt.Fprintln(a.Writer, msg)
			}
		case <-a.DoneChan:
			return nil
		}
	}
}

// Buy ...
func (a *Agent) Buy(rowIdx int) {
	row := a.Trace[rowIdx]
	if row[Use] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())
		bid, _ := json.Marshal([]float64{ppu, row[Use] * ToKWh}) // first arg: PPU, second arg: QTY
		msg := fmt.Sprintf("[%d] Invoking 'buy' for %.3f kWh (%.3f) at %.3f ç/kWh", a.ID, row[Use]*ToKWh, row[Use], ppu)
		fmt.Fprintln(a.Writer, msg)
		if _, err := a.SDK.Invoke(rowIdx, "buy", bid); err != nil {
			msg := fmt.Sprintf("[%d] Unable to invoke 'buy' for row %d: %s\n", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
		}

	}
}

// Sell ...
func (a *Agent) Sell(rowIdx int) {
	row := a.Trace[rowIdx]
	if row[Gen] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())
		bid, _ := json.Marshal([]float64{ppu, row[Gen] * ToKWh}) // first arg: PPU, second arg: QTY
		msg := fmt.Sprintf("[%d] Invoking 'sell' for %.3f kWh (%.3f) at %.3f ç/kWh", a.ID, row[Gen]*ToKWh, row[Gen], ppu)
		fmt.Fprintln(a.Writer, msg)
		if _, err := a.SDK.Invoke(rowIdx, "sell", bid); err != nil {
			msg := fmt.Sprintf("[%d] Unable to invoke 'sell' for row %d: %s", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
		}
	}
}
