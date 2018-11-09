package agent

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"sync"
	"time"

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
	Invoke(txID string, slot int, action string, dataB []byte) ([]byte, error)
}

// Notifier ...
//go:generate counterfeiter . Notifier
type Notifier interface {
	Register(id int, queue chan int) bool
}

// Agent ...
type Agent struct {
	Invoker  Invoker
	Notifier Notifier

	PubKey *rsa.PublicKey
	Trace  [][]float64
	ID     int

	SlotChan        chan stats.Slot        // Used to feed the stats collector.
	TransactionChan chan stats.Transaction // Used to feed the stats collector.

	SlotQueue chan int // The agent's trigger/input — an agent acts whenever a new slot is pushed through this channel.
	BuyQueue  chan int // Used to decouple bids from the main thread.
	SellQueue chan int // As above.

	Writer io.Writer // For logging.

	DoneChan  chan struct{}   // An external kill switch. Signals to all threads in this package that they should return.
	killChan  chan struct{}   // An internal kill switch. It can only be closed by this package, and it signals to the package's goroutines that they should exit.
	waitGroup *sync.WaitGroup // Ensures that the main thread in this package doesn't return before the goroutines it spawned also have as well.
}

// New ...
func New(invoker Invoker, notifier Notifier,
	pubkey *rsa.PublicKey, trace [][]float64, id int,
	slotc chan stats.Slot, transactionc chan stats.Transaction,
	writer io.Writer, donec chan struct{}) *Agent {
	return &Agent{
		Invoker:  invoker,
		Notifier: notifier,

		PubKey: pubkey,
		Trace:  trace[:5], // ATTN: Temporary modification
		ID:     id,

		SlotChan:        slotc,
		TransactionChan: transactionc,

		SlotQueue: make(chan int, BufferLen),
		BuyQueue:  make(chan int, BufferLen),
		SellQueue: make(chan int, BufferLen),

		Writer: writer,

		DoneChan:  donec,
		killChan:  make(chan struct{}),
		waitGroup: new(sync.WaitGroup),
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprintf("[agent %d] Exited", a.ID)
	defer fmt.Fprintln(a.Writer, msg)

	defer func() {
		close(a.killChan)
		a.waitGroup.Wait()
	}()

	if ok := a.Notifier.Register(a.ID, a.SlotQueue); !ok {
		msg := fmt.Sprintf("[agent %d] Unable to register with slot notifier", a.ID)
		fmt.Fprintln(a.Writer, msg)
		return errors.New(msg)
	}

	msg = fmt.Sprintf("[agent %d] Registered with slot notifier", a.ID)
	fmt.Fprintln(a.Writer, msg)

	a.waitGroup.Add(1)
	go func() {
		defer a.waitGroup.Done()
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

	a.waitGroup.Add(1)
	go func() {
		defer a.waitGroup.Done()
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
	txID := strconv.Itoa(rand.Intn(1E6))
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
		a.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyUse: row[Use] * ToKWh,
			PricePaid: row[Hi],
		}
		timeStart := time.Now()
		if _, err := a.Invoker.Invoke(txID, rowIdx, "buy", encBid); err != nil {
			// Update stats
			timeEnd := time.Now()
			elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
			a.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "buy",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[agent %d] Unable to invoke 'buy' for row %d: %s\n", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
			return errors.New(msg)
		}
		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
		a.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "buy",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}
	return nil
}

// Sell ...
func (a *Agent) Sell(rowIdx int) error {
	txID := strconv.Itoa(rand.Intn(1E6))
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
		a.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyGen: row[Gen] * ToKWh,
			PriceSold: row[Lo],
		}
		timeStart := time.Now()
		if _, err := a.Invoker.Invoke(txID, rowIdx, "sell", encBid); err != nil {
			// Update stats
			timeEnd := time.Now()
			elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
			a.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "sell",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[agent %d] Unable to invoke 'sell' for row %d: %s", a.ID, rowIdx, err)
			fmt.Fprintln(a.Writer, msg)
			return errors.New(msg)
		}
		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
		a.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "sell",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}
	return nil
}
