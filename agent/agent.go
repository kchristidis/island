package agent

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"

	"github.com/kchristidis/exp2/crypto"
	"github.com/kchristidis/exp2/stats"
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

// Invoker ...
//go:generate counterfeiter . Invoker
type Invoker interface {
	Invoke(slot int, action string, dataB []byte) ([]byte, error)
}

// Notifier ...
//go:generate counterfeiter . Notifier
type Notifier interface {
	Register(id int, queue chan int) bool
}

// Agent ...
type Agent struct {
	BuyQueue     chan int
	DoneChan     chan struct{}
	ID           int
	Invoker      Invoker
	PubKey       *rsa.PublicKey
	SellQueue    chan int
	SlotQueue    chan int // This is the agent's trigger, i.e. it's supposed to act whenever a new slot is created
	StatSlotChan chan stats.Slot
	Notifier     Notifier
	Trace        [][]float64
	Writer       io.Writer

	wg       sync.WaitGroup
	killChan chan struct{}
}

// New ...
func New(id int, trace [][]float64, invoker Invoker, notifier Notifier, pubkey *rsa.PublicKey, statslotc chan stats.Slot, donec chan struct{}, writer io.Writer) *Agent {
	return &Agent{
		BuyQueue:     make(chan int, BufferLen),
		DoneChan:     donec,
		ID:           id,
		Invoker:      invoker,
		PubKey:       pubkey,
		SellQueue:    make(chan int, BufferLen),
		SlotQueue:    make(chan int, BufferLen),
		StatSlotChan: statslotc,
		Notifier:     notifier,
		Trace:        trace[:5], // ATTN: Temporary modification
		Writer:       writer,

		killChan: make(chan struct{}),
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprintf("[agent %d] Exited", a.ID)
	defer fmt.Fprintln(a.Writer, msg)

	defer func() {
		close(a.killChan)
		a.wg.Wait()
	}()

	if ok := a.Notifier.Register(a.ID, a.SlotQueue); !ok {
		msg := fmt.Sprintf("[agent %d] Unable to register with slot notifier", a.ID)
		fmt.Fprintln(a.Writer, msg)
		return errors.New(msg)
	}

	msg = fmt.Sprintf("[agent %d] Registered with slot notifier", a.ID)
	fmt.Fprintln(a.Writer, msg)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-a.killChan:
				return
			case rowIdx := <-a.BuyQueue:
				a.Buy(rowIdx)
			case <-a.DoneChan:
				return
			}
		}
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-a.killChan:
				return
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
			msg := fmt.Sprintf("[agent %d] Processing row %d: %v", a.ID, rowIdx, a.Trace[rowIdx])
			fmt.Fprintln(a.Writer, msg)

			select {
			case a.BuyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[agent %d] Unable to push row %d to 'buy' queue (size: %d)", a.ID, rowIdx, len(a.BuyQueue))
				fmt.Fprintln(a.Writer, msg)
				return errors.New(msg)
			}

			select {
			case a.SellQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[agent %d] Unable to push row %d to 'sell' queue (size: %d)", a.ID, rowIdx, len(a.BuyQueue))
				fmt.Fprintln(a.Writer, msg)
				return errors.New(msg)
			}

			// Return when you're done processing your trace
			if rowIdx == len(a.Trace)-1 {
				return nil
			}
		case <-a.DoneChan:
			return nil
		}
	}
}

// Buy ...
func (a *Agent) Buy(rowIdx int) error {
	row := a.Trace[rowIdx]
	if row[Use] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())
		bid, _ := json.Marshal([]float64{ppu, row[Use] * ToKWh}) // first arg: PPU, second arg: QTY
		msg := fmt.Sprintf("[agent %d] Invoking 'buy' for %.3f kWh (%.3f) at %.3f ç/kWh @ slot %d", a.ID, row[Use]*ToKWh, row[Use], ppu, rowIdx)
		fmt.Fprintln(a.Writer, msg)
		encBid, err := crypto.Encrypt(bid, a.PubKey)
		if err != nil {
			msg := fmt.Sprintf("[agent %d] Unable to encrypt 'buy' for row %d: %s\n", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
			return errors.New(msg)
		}
		a.StatSlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyUse: row[Use] * ToKWh,
			PricePaid: row[Hi],
		}
		if _, err := a.Invoker.Invoke(rowIdx, "buy", encBid); err != nil {
			msg := fmt.Sprintf("[agent %d] Unable to invoke 'buy' for row %d: %s\n", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
			return errors.New(msg)
		}
	}
	return nil
}

// Sell ...
func (a *Agent) Sell(rowIdx int) error {
	row := a.Trace[rowIdx]
	if row[Gen] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())
		bid, _ := json.Marshal([]float64{ppu, row[Gen] * ToKWh}) // first arg: PPU, second arg: QTY
		msg := fmt.Sprintf("[agent %d] Invoking 'sell' for %.3f kWh (%.3f) at %.3f ç/kWh @ slot %d", a.ID, row[Gen]*ToKWh, row[Gen], ppu, rowIdx)
		fmt.Fprintln(a.Writer, msg)
		encBid, err := crypto.Encrypt(bid, a.PubKey)
		if err != nil {
			msg := fmt.Sprintf("[agent %d] Unable to encrypt 'sell' for row %d: %s\n", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
			return errors.New(msg)
		}
		a.StatSlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyGen: row[Gen] * ToKWh,
			PriceSold: row[Lo],
		}
		if _, err := a.Invoker.Invoke(rowIdx, "sell", encBid); err != nil {
			msg := fmt.Sprintf("[agent %d] Unable to invoke 'sell' for row %d: %s", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
			return errors.New(msg)
		}
	}
	return nil
}
