package bidder

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

	"github.com/kchristidis/island/exp1/crypto"
	"github.com/kchristidis/island/exp1/stats"
)

// Constants ...
const (
	Gen = iota // The column names in the trace
	Grid
	Use
	Lo
	Hi

	BufferLen = 100  // The buffer for the slot and task channels
	ToKWh     = 0.25 // Used to convert the trace values into KWh
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

// Bidder ...
type Bidder struct {
	Invoker  Invoker
	Notifier Notifier

	PubKey *rsa.PublicKey
	Trace  [][]float64
	ID     int

	SlotChan        chan stats.Slot        // Used to feed the stats collector.
	TransactionChan chan stats.Transaction // Used to feed the stats collector.

	SlotQueue chan int // The bidder's trigger/input — a bidder acts whenever a new slot is pushed through this channel.
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
	writer io.Writer, donec chan struct{}) *Bidder {
	return &Bidder{
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
func (b *Bidder) Run() error {
	msg := fmt.Sprintf("[bidder %04d] Exited", b.ID)
	defer fmt.Fprintln(b.Writer, msg)

	defer func() {
		close(b.killChan)
		b.waitGroup.Wait()
	}()

	if ok := b.Notifier.Register(b.ID, b.SlotQueue); !ok {
		msg := fmt.Sprintf("[bidder %04d] Unable to register with slot notifier", b.ID)
		fmt.Fprintln(b.Writer, msg)
		return errors.New(msg)
	}

	msg = fmt.Sprintf("[bidder %04d] Registered with slot notifier", b.ID)
	fmt.Fprintln(b.Writer, msg)

	b.waitGroup.Add(1)
	go func() {
		defer b.waitGroup.Done()
		for {
			select {
			case <-b.killChan:
				return
			case rowIdx := <-b.BuyQueue:
				b.Buy(rowIdx)
			case <-b.DoneChan:
				return
			}
		}
	}()

	b.waitGroup.Add(1)
	go func() {
		defer b.waitGroup.Done()
		for {
			select {
			case <-b.killChan:
				return
			case rowIdx := <-b.SellQueue:
				b.Sell(rowIdx)
			case <-b.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case slot := <-b.SlotQueue:
			rowIdx := int(slot)
			msg := fmt.Sprintf("[bidder %04d] Got notified that slot %d has arrived - processing row %d: %v",
				b.ID, slot, rowIdx, b.Trace[rowIdx])
			fmt.Fprintln(b.Writer, msg)

			select {
			case b.BuyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[bidder %04d] Unable to push row %d to 'buy' queue (size: %d)",
					b.ID, rowIdx, len(b.BuyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			select {
			case b.SellQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[bidder %04d] Unable to push row %d to 'sell' queue (size: %d)",
					b.ID, rowIdx, len(b.BuyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			// Return when you're done processing your trace
			if rowIdx == len(b.Trace)-1 {
				return nil
			}
		case <-b.DoneChan:
			return nil
		}
	}
}

// Buy ...
func (b *Bidder) Buy(rowIdx int) error {
	row := b.Trace[rowIdx]
	txID := strconv.Itoa(rand.Intn(1E6))
	if row[Use] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())
		bid, _ := json.Marshal([]float64{ppu, row[Use] * ToKWh}) // First arg: PPU, second arg: QTY
		msg := fmt.Sprintf("[bidder %04d] Invoking 'buy' for %.3f kWh (%.3f) at %.3f ç/kWh @ slot %d",
			b.ID, row[Use]*ToKWh, row[Use], ppu, rowIdx)
		fmt.Fprintln(b.Writer, msg)
		encBid, err := crypto.Encrypt(bid, b.PubKey)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Unable to encrypt 'buy' for row %d: %s\n",
				b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}
		b.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyUse: row[Use] * ToKWh,
			PricePaid: row[Hi],
		}
		timeStart := time.Now()
		if _, err := b.Invoker.Invoke(txID, rowIdx, "buy", encBid); err != nil {
			// Update stats
			timeEnd := time.Now()
			elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
			b.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "buy",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[bidder %04d] Unable to invoke 'buy' for row %d: %s\n",
				b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}
		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
		b.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "buy",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}
	return nil
}

// Sell ...
func (b *Bidder) Sell(rowIdx int) error {
	txID := strconv.Itoa(rand.Intn(1E6))
	row := b.Trace[rowIdx]
	if row[Gen] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())
		bid, _ := json.Marshal([]float64{ppu, row[Gen] * ToKWh}) // First arg: PPU, second arg: QTY
		msg := fmt.Sprintf("[bidder %04d] Invoking 'sell' for %.3f kWh (%.3f) at %.3f ç/kWh @ slot %d",
			b.ID, row[Gen]*ToKWh, row[Gen], ppu, rowIdx)
		fmt.Fprintln(b.Writer, msg)
		encBid, err := crypto.Encrypt(bid, b.PubKey)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Unable to encrypt 'sell' for row %d: %s\n",
				b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}
		b.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyGen: row[Gen] * ToKWh,
			PriceSold: row[Lo],
		}
		timeStart := time.Now()
		if _, err := b.Invoker.Invoke(txID, rowIdx, "sell", encBid); err != nil {
			// Update stats
			timeEnd := time.Now()
			elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
			b.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "sell",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[bidder %04d] Unable to invoke 'sell' for row %d: %s",
				b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}
		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
		b.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "sell",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}
	return nil
}
