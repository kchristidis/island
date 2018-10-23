package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sync"
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

// Signaler ...
//go:generate counterfeiter . Signaler
type Signaler interface {
	Register(id int, queue chan uint64) bool
}

// Agent ...
type Agent struct {
	BuyQueue    chan int
	DoneChan    chan struct{}
	ID          int
	SDK         SDKer
	SellQueue   chan int
	Signal      Signaler
	SignalQueue chan uint64
	Out         io.Writer
	Trace       [][]float64

	once  sync.Once
	first int // The block number that corresponds to the first row in the trace
}

// New ...
func New(id int, trace [][]float64, sdkContext SDKer, signal Signaler, doneChan chan struct{}, out io.Writer) *Agent {
	return &Agent{
		BuyQueue:    make(chan int, BufferLen),
		DoneChan:    doneChan,
		ID:          id,
		SellQueue:   make(chan int, BufferLen),
		SDK:         sdkContext,
		Signal:      signal,
		SignalQueue: make(chan uint64, BufferLen),
		Out:         out,
		Trace:       trace,
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprintf("[%d] Agent exited", a.ID)
	defer fmt.Fprintln(a.Out, msg)

	if ok := a.Signal.Register(a.ID, a.SignalQueue); !ok {
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
		case blockNumber := <-a.SignalQueue:
			a.once.Do(func() {
				a.first = int(blockNumber)
			})

			rowIdx := int(blockNumber) - a.first
			msg := fmt.Sprintf("[%d] Processing row %d: %v", a.ID, rowIdx, a.Trace[rowIdx])
			fmt.Fprintln(a.Out, msg)

			select {
			case a.BuyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[%d] Unable to push row %d to 'buy' queue (size: %d)", a.ID, rowIdx, len(a.BuyQueue))
				fmt.Fprintln(a.Out, msg)
			}

			select {
			case a.SellQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[%d] Unable to push row %d to 'sell' queue (size: %d)", a.ID, rowIdx, len(a.BuyQueue))
				fmt.Fprintln(a.Out, msg)
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
		fmt.Fprintln(a.Out, msg)
		if _, err := a.SDK.Invoke(rowIdx, "buy", bid); err != nil {
			msg := fmt.Sprintf("[%d] Unable to invoke 'buy' for row %d: %s\n", a.ID, rowIdx, err)
			fmt.Fprintln(a.Out, msg)
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
		fmt.Fprintln(a.Out, msg)
		if _, err := a.SDK.Invoke(rowIdx, "sell", bid); err != nil {
			msg := fmt.Sprintf("[%d] Unable to invoke 'sell' for row %d: %s", a.ID, rowIdx, err)
			fmt.Fprintln(a.Out, msg)
		}
	}
}
