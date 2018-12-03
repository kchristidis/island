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

	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/cmap"
	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/stats"
)

// The column names in the trace
const (
	Gen = iota
	Grid
	Use
	Lo
	Hi
)

// ToKWh is a multiplier that converts the trace values into KWh units.
const ToKWh = 0.25

// BufferLen sets the buffer length for the slot/task channels, and the
// bid-keys cmap.
const BufferLen = 100

// RecentBidKeysKV is a type we create for the channel that will be used
// to feed the goroutine that updates the RecentBidKeys cmap.
type RecentBidKeysKV struct {
	Slot          int      // Primary key
	TxID          string   // Used in Exp1
	WriteKeyAttrs []string // Used in Exp3
}

//go:generate counterfeiter . Invoker

// Invoker is an interface that encapsulates the
// peer calls that are relevant to the bidder.
type Invoker interface {
	Invoke(args schema.OpContextInput) ([]byte, error)
}

//go:generate counterfeiter . Notifier

// Notifier is an interface that encapsulates the slot
// notifier calls that are relevant to the bidder.
type Notifier interface {
	Register(id int, queue chan int) bool
}

// Bidder issues bidding calls to the peer
// upon receiving slot notifications.
type Bidder struct {
	Invoker   Invoker
	Notifiers []Notifier

	ID           int
	Trace        [][]float64
	PrivKeyBytes []byte // The bidder's key pair

	// Used to feed the stats collector
	SlotChan        chan stats.Slot
	TransactionChan chan stats.Transaction

	// The bidder's trigger/input — a bidder acts whenever a new slot is
	// pushed through these channels. 1st SlotQueue is used for bids, 2nd
	// SlotQueue (optional) is used for PostKey calls.
	SlotQueues [2]chan int
	// Used to decouple bids and PostKey calls from the main thread
	BuyQueue  chan int
	SellQueue chan int

	PostKeyQueue chan int
	// Maintains a mapping between the bid for a rowIdx (key) and the
	// associated event/txID and write-key in the chaincode's KVS. We write
	// to this map during bids (buy/sells), and read from it when we post keys.
	RecentBidKeys      *cmap.Container
	RecentBidKeysQueue chan RecentBidKeysKV

	Writer io.Writer // For logging

	// An external kill switch. Signals to all threads in this package
	// that they should return.
	DoneChan chan struct{}
	// An internal kill switch. It can only be closed by this package,
	// and it signals to the package's goroutines that they should exit.
	killChan chan struct{}
	// Ensures that the main thread in this package doesn't return
	// before the goroutines it spawned also have as well.
	waitGroup *sync.WaitGroup

	// Handy references to the private and public keys
	privKey *rsa.PrivateKey
	pubKey  *rsa.PublicKey
}

// New returns a new bidder.
func New(invoker Invoker, slotBidNotifier Notifier, slotPostKeyNotifier Notifier,
	id int, privKeyBytes []byte, trace [][]float64,
	slotc chan stats.Slot, transactionc chan stats.Transaction,
	writer io.Writer, donec chan struct{}) *Bidder {

	notifiers := []Notifier{slotBidNotifier}
	// Detecting whether that second notifier is nil is actually tricky.
	// See: https://medium.com/golangspec/interfaces-in-go-part-i-4ae53a97479c (nil interface value)
	// As a hack for now, let's consult the `ExpNum` to figure out whether
	// this slice should carry one or two notifiers.
	switch schema.ExpNum {
	case 1, 3:
		notifiers = append(notifiers, slotPostKeyNotifier)
	default:
	}

	privKey, err := crypto.DeserializePrivate(privKeyBytes)
	if err != nil {
		panic(err.Error())
	}
	pubKey := &privKey.PublicKey

	var slotqs [2]chan int
	// Create two SlotQueues no matter what.
	// If len(notifiers) == 1, we will nil the second
	// SlotQueue before the for-select loop in the main
	// thread below.
	for i := 0; i < 2; i++ {
		slotqs[i] = make(chan int, BufferLen)
	}

	cmapKeys, _ := cmap.New(BufferLen)

	return &Bidder{
		Invoker:   invoker,
		Notifiers: notifiers,

		ID:           id,
		Trace:        trace[:5], // ATTN: Temporary modification
		PrivKeyBytes: privKeyBytes,

		SlotChan:        slotc,
		TransactionChan: transactionc,

		SlotQueues:         slotqs,
		BuyQueue:           make(chan int, BufferLen),
		SellQueue:          make(chan int, BufferLen),
		PostKeyQueue:       make(chan int, BufferLen),
		RecentBidKeys:      cmapKeys,
		RecentBidKeysQueue: make(chan RecentBidKeysKV, BufferLen),

		Writer: writer,

		DoneChan:  donec,
		killChan:  make(chan struct{}),
		waitGroup: new(sync.WaitGroup),

		privKey: privKey,
		pubKey:  pubKey,
	}
}

// Run executes the bidder logic.
func (b *Bidder) Run() error {
	msg := fmt.Sprintf("[bidder %04d] Exited", b.ID)
	defer fmt.Fprintln(b.Writer, msg)

	defer func() {
		close(b.killChan)
		b.waitGroup.Wait()
	}()

	for i := range b.Notifiers {
		if ok := b.Notifiers[i].Register(b.ID, b.SlotQueues[i]); !ok {
			msg := fmt.Sprintf("[bidder %04d] Unable to register with slot notifier %d", b.ID, i)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}
		msg = fmt.Sprintf("[bidder %04d] Registered with slot notifier %d", b.ID, i)
		fmt.Fprintln(b.Writer, msg)
	}

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

	if len(b.Notifiers) == 2 {
		b.waitGroup.Add(1)
		go func() {
			defer b.waitGroup.Done()
			for {
				select {
				case <-b.killChan:
					return
				case rowIdx := <-b.PostKeyQueue:
					b.PostKey(rowIdx)
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
				case newVal := <-b.RecentBidKeysQueue:
					oldVals, ok := b.RecentBidKeys.Get(newVal.Slot)
					if !ok {
						b.RecentBidKeys.Put(newVal.Slot, map[string][]string{
							newVal.TxID: newVal.WriteKeyAttrs,
						})
						continue
					}
					oldVals.(map[string][]string)[newVal.TxID] = newVal.WriteKeyAttrs
					b.RecentBidKeys.Put(newVal.Slot, oldVals)
				case <-b.DoneChan:
					return
				}
			}
		}()
	}

	if len(b.Notifiers) == 1 {
		b.SlotQueues[1] = nil // You want that channel to always block
	}

	for {
		select {
		case bidSlot := <-b.SlotQueues[0]:
			rowIdx := int(bidSlot)
			msg := fmt.Sprintf("[bidder %04d] Got notified that slot %d has arrived - processing row %d for bidding: %v", b.ID, bidSlot, rowIdx, b.Trace[rowIdx])
			fmt.Fprintln(b.Writer, msg)

			select {
			case b.BuyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[bidder %04d] Unable to push row %d to 'buy' queue (size: %d)", b.ID, rowIdx, len(b.BuyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			select {
			case b.SellQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[bidder %04d] Unable to push row %d to 'sell' queue (size: %d)", b.ID, rowIdx, len(b.BuyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			// Return when you're done processing your trace
			if rowIdx == len(b.Trace)-1 {
				return nil
			}
		case postKeySlot := <-b.SlotQueues[1]:
			rowIdx := int(postKeySlot)
			msg := fmt.Sprintf("[bidder %04d] Got notified that slot %d has arrived - processing row %d for key posting: %v", b.ID, postKeySlot, rowIdx, b.Trace[rowIdx])
			fmt.Fprintln(b.Writer, msg)

			select {
			case b.PostKeyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("[bidder %04d] Unable to push row %d to 'postKey' queue (size: %d)", b.ID, rowIdx, len(b.PostKeyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}
		case <-b.DoneChan:
			return nil
		}
	}
}

// Buy allows a bidder place a buy offer.
func (b *Bidder) Buy(rowIdx int) error {
	row := b.Trace[rowIdx]
	txID := strconv.Itoa(rand.Intn(1E6))
	if row[Use] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())

		bidInputVal := schema.BidInput{
			PricePerUnitInCents: ppu,
			QuantityInKWh:       row[Use] * ToKWh,
		}

		bidInputValB, err := json.Marshal(bidInputVal)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Cannot encode 'buy' bid for slot %d as a JSON object: %s", b.ID, rowIdx, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		encBidInputValB, err := crypto.Encrypt(bidInputValB, b.pubKey)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Unable to encrypt 'buy' for row %d: %s\n",
				b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		msg := fmt.Sprintf("[bidder %04d] Invoking 'buy' for %.3f kWh (%.3f kW) at %.3f ç/kWh @ slot %d", b.ID, row[Use]*ToKWh, row[Use], ppu, rowIdx)
		fmt.Fprintln(b.Writer, msg)

		b.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyUse: bidInputVal.QuantityInKWh,
			PricePaid: row[Hi],
		}

		args := schema.OpContextInput{
			EventID: txID,
			Action:  "buy",
			Slot:    rowIdx,
			Data:    encBidInputValB,
		}

		timeStart := time.Now()

		respB, err := b.Invoker.Invoke(args)

		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)

		if err != nil {
			b.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "buy",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[bidder %04d] Unable to invoke 'buy' for row %d: %s\n", b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		// Update the cmap for post-key calls
		switch schema.ExpNum {
		case 1, 3:
			// Extract the write-key
			var bidOutputVal schema.BidOutput
			if err := json.Unmarshal(respB, &bidOutputVal); err != nil {
				b.TransactionChan <- stats.Transaction{
					ID:              txID,
					Type:            "buy",
					Status:          err.Error(),
					LatencyInMillis: elapsed,
				}
				msg := fmt.Sprintf("[bidder %04d] Cannot unmarshal JSON-encoded response associated with tx ID %s: %s", b.ID, txID, err.Error())
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			b.RecentBidKeysQueue <- RecentBidKeysKV{
				Slot:          rowIdx,
				TxID:          txID,
				WriteKeyAttrs: bidOutputVal.WriteKeyAttrs,
			}
		default:
		}

		b.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "buy",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}
	return nil
}

// Sell allows a bidder to place a sell offer.
func (b *Bidder) Sell(rowIdx int) error {
	txID := strconv.Itoa(rand.Intn(1E6))
	row := b.Trace[rowIdx]
	if row[Gen] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())

		bidInputVal := schema.BidInput{
			PricePerUnitInCents: ppu,
			QuantityInKWh:       row[Gen] * ToKWh,
		}

		bidInputValB, err := json.Marshal(bidInputVal)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Cannot encode 'sell' bid for slot %d as a JSON object: %s", b.ID, rowIdx, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		encBidInputValB, err := crypto.Encrypt(bidInputValB, b.pubKey)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Unable to encrypt 'sell' for row %d: %s\n", b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		msg := fmt.Sprintf("[bidder %04d] Invoking 'sell' for %.3f kWh (%.3f kW) at %.3f ç/kWh @ slot %d", b.ID, row[Gen]*ToKWh, row[Gen], ppu, rowIdx)
		fmt.Fprintln(b.Writer, msg)

		b.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyGen: bidInputVal.QuantityInKWh,
			PriceSold: row[Lo],
		}

		args := schema.OpContextInput{
			EventID: txID,
			Action:  "sell",
			Slot:    rowIdx,
			Data:    encBidInputValB,
		}

		timeStart := time.Now()

		respB, err := b.Invoker.Invoke(args)

		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)

		if err != nil {
			b.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "sell",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[bidder %04d] Unable to invoke 'sell' for row %d: %s", b.ID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		// Update the cmap for post-key calls
		switch schema.ExpNum {
		case 1, 3:
			// Extract the write-key
			var bidOutputVal schema.BidOutput
			if err := json.Unmarshal(respB, &bidOutputVal); err != nil {
				b.TransactionChan <- stats.Transaction{
					ID:              txID,
					Type:            "sell",
					Status:          err.Error(),
					LatencyInMillis: elapsed,
				}
				msg := fmt.Sprintf("[bidder %04d] Cannot unmarshal JSON-encoded response associated with tx ID %s: %s", b.ID, txID, err.Error())
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			b.RecentBidKeysQueue <- RecentBidKeysKV{
				Slot:          rowIdx,
				TxID:          txID,
				WriteKeyAttrs: bidOutputVal.WriteKeyAttrs,
			}
		default:
		}

		b.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "sell",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}
	return nil
}

// PostKey allows a bidder to post the private key corresponding to the
// public key with which they posted an encrypted bid on the ledger.
func (b *Bidder) PostKey(rowIdx int) error {
	msg := fmt.Sprintf("[bidder %04d] Invoking 'postKey' @ slot %d", b.ID, rowIdx)
	fmt.Fprintln(b.Writer, msg)

	valMap, ok := b.RecentBidKeys.Get(rowIdx)
	if !ok {
		msg := fmt.Sprintf("[bidder %04d] Could not find any bids to post keys for @ slot %d", b.ID, rowIdx)
		fmt.Fprintln(b.Writer, msg)
		return errors.New(msg) // Nobody's actually consuming this error for now; that's OK.
	}

	msg = fmt.Sprintf("[bidder %04d] Found %d bids to post keys for @ slot %d ", b.ID, len(valMap.(map[string][]string)), rowIdx)
	fmt.Fprintln(b.Writer, msg)

	mapIdx := 0
	mapLen := len(valMap.(map[string][]string))
	for k, v := range valMap.(map[string][]string) {
		txID := strconv.Itoa(rand.Intn(1E6))
		mapIdx++

		msg := fmt.Sprintf("[bidder %04d] About to 'postKey' for bid %d of %d with tx ID %s and key w/ attributes %s", b.ID, mapIdx, mapLen, k, v)
		fmt.Fprintln(b.Writer, msg)

		postKeyInputVal := schema.PostKeyInput{
			ReadKeyAttrs: v,
			PrivKey:      b.PrivKeyBytes,
			TxID:         k, // ATTN: This is the tx ID of the associated bid!
		}

		postKeyInputValB, err := json.Marshal(postKeyInputVal)
		if err != nil {
			msg := fmt.Sprintf("[bidder %04d] Cannot encode 'postKey' payload for slot %d (bid %d of %d) as a JSON object: %s", b.ID, rowIdx, mapIdx, mapLen, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		args := schema.OpContextInput{
			EventID: txID,
			Action:  "postKey",
			Slot:    rowIdx,
			Data:    postKeyInputValB,
		}

		timeStart := time.Now()

		_, err = b.Invoker.Invoke(args)

		// Update stats
		timeEnd := time.Now()
		elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)

		if err != nil {
			b.TransactionChan <- stats.Transaction{
				ID:              txID,
				Type:            "postKey",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
			}
			msg := fmt.Sprintf("[bidder %04d] Unable to invoke 'postKey' for row %d (bid %d/%d): %s", b.ID, rowIdx, mapIdx, mapLen, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		// The green path
		b.TransactionChan <- stats.Transaction{
			ID:              txID,
			Type:            "postKey",
			Status:          "success",
			LatencyInMillis: elapsed,
		}
	}

	return nil
}
