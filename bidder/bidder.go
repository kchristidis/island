package bidder

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
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

// Used for the exponential backoff delay calculations in experiment 1.
var (
	s rand.Source
	r *rand.Rand
)

// RecentBidKeysKV is a type we create for the channel that will be used
// to feed the goroutine that updates the RecentBidKeys cmap.
type RecentBidKeysKV struct {
	Slot          int      // Primary key
	BidEventID    string   // Used in Exp1
	WriteKeyAttrs []string // Used in Exp3
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Invoker

// Invoker is an interface that encapsulates the
// peer calls that are relevant to the bidder.
type Invoker interface {
	Invoke(args schema.OpContextInput) ([]byte, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Notifier

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
	BuyQueue     chan int
	SellQueue    chan int
	PostKeyQueue chan int
	// Maintains a mapping between the bid for a rowIdx (key) and the
	// associated event ID and write-key in the chaincode's KVS. We write
	// to this map during bids (buy/sells), and read from it when we post
	// keys, so that we can associate the posted key with the right bid.
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
	// before the goroutines it spawned have.
	waitGroup *sync.WaitGroup

	// Handy references to the private and public keys
	privKey *rsa.PrivateKey
	pubKey  *rsa.PublicKey
}

// New returns a new bidder.
func New(invoker Invoker, slotBidNotifier Notifier, slotPostKeyNotifier Notifier,
	id int, privKeyBytes []byte, trace [][]float64,
	slotC chan stats.Slot, transactionC chan stats.Transaction,
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

	var slotQs [2]chan int
	// Create two SlotQueues no matter what.
	// If len(notifiers) == 1, we will nil the second
	// SlotQueue before the for-select loop in the main
	// thread below.
	for i := 0; i < 2; i++ {
		slotQs[i] = make(chan int, BufferLen)
	}

	cmapKeys, _ := cmap.New(BufferLen)

	s = rand.NewSource(time.Now().UnixNano())
	r = rand.New(s)

	return &Bidder{
		Invoker:   invoker,
		Notifiers: notifiers,

		ID:           id,
		Trace:        trace[:schema.TraceLength],
		PrivKeyBytes: privKeyBytes,

		SlotChan:        slotC,
		TransactionChan: transactionC,

		SlotQueues:         slotQs,
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
	msg := fmt.Sprintf("bidder:%04d • exited", b.ID)
	defer fmt.Fprintln(b.Writer, msg)

	defer func() {
		close(b.killChan)
		b.waitGroup.Wait()
	}()

	for i := range b.Notifiers {
		if ok := b.Notifiers[i].Register(b.ID, b.SlotQueues[i]); !ok {
			msg := fmt.Sprintf("bidder:%04d • cannot register with slot notifier %d", b.ID, i)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		if schema.StagingLevel <= schema.Debug {
			msg := fmt.Sprintf("bidder:%04d • registered with slot notifier %d", b.ID, i)
			fmt.Fprintln(b.Writer, msg)
		}
	}

	b.waitGroup.Add(1)
	go func() {
		defer b.waitGroup.Done()
		for {
			select {
			case <-b.killChan:
				return
			case rowIdx := <-b.BuyQueue:
				b.Buy(rowIdx) // Nobody's consuming the returned error for now - that's OK
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
				b.Sell(rowIdx) // Nobody's consuming the returned error for now - that's OK
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
					b.PostKey(rowIdx) // Nobody's consuming the returned error for now - that's OK
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
							newVal.BidEventID: newVal.WriteKeyAttrs,
						})
						continue
					}
					oldVals.(map[string][]string)[newVal.BidEventID] = newVal.WriteKeyAttrs
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

			if schema.StagingLevel <= schema.Debug {
				msg := fmt.Sprintf("bidder:%04d slot:%012d • new slot! processing row %d for bidding: %v", b.ID, bidSlot, rowIdx, b.Trace[rowIdx])
				fmt.Fprintln(b.Writer, msg)
			}

			select {
			case b.BuyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("bidder:%04d slot:%012d • cannot push row to'buy' queue (size: %d)", b.ID, rowIdx, len(b.BuyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			select {
			case b.SellQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("bidder:%04d slot:%012d • cannot push row to 'sell' queue (size: %d)", b.ID, rowIdx, len(b.BuyQueue))
				fmt.Fprintln(b.Writer, msg)
				return errors.New(msg)
			}

			// Return when you're done processing your trace
			if rowIdx == len(b.Trace)-1 {
				msg := fmt.Sprintf("bidder:%04d slot:%012d • done processing the trace! exiting", b.ID, rowIdx)
				fmt.Fprintln(b.Writer, msg)
				return nil
			}
		case postKeySlot := <-b.SlotQueues[1]:
			rowIdx := int(postKeySlot)

			if schema.StagingLevel <= schema.Debug {
				msg := fmt.Sprintf("bidder:%04d slot:%012d • new slot! processing row %d for key posting: %v", b.ID, postKeySlot, rowIdx, b.Trace[rowIdx])
				fmt.Fprintln(b.Writer, msg)
			}

			select {
			// For exp1:
			// A side-effect of calling for a 'postKey' call like this is that we may ask to
			// 'postKey' for slot N when the relevant bid/sell call is still being repeated.
			// We could probably mitigate this by tighter synchronization between the buy/sell
			// goroutines and the postkey one, but this would complicate the code significantly.
			case b.PostKeyQueue <- rowIdx:
			default:
				msg := fmt.Sprintf("bidder:%04d slot:%012d • cannot push row to 'postKey' queue (size: %d)", b.ID, rowIdx, len(b.PostKeyQueue))
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
	eventID := fmt.Sprintf("%013d", rand.Intn(1E12))
	row := b.Trace[rowIdx]

	if row[Use] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())

		bidInputVal := schema.BidInput{
			PricePerUnitInCents: ppu,
			QuantityInKWh:       row[Use] * ToKWh,
		}

		bidInputValB, err := json.Marshal(bidInputVal)
		if err != nil {
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d • cannot encode 'buy' bid to JSON: %s", b.ID, eventID, rowIdx, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		encBidInputValB, err := crypto.Encrypt(bidInputValB, b.pubKey)
		if err != nil {
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d • cannot encrypt 'buy' bid: %s", b.ID, eventID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		b.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyUse: bidInputVal.QuantityInKWh,
			PricePaid: row[Hi],
		}

		args := schema.OpContextInput{
			EventID: eventID,
			Action:  "buy",
			Slot:    rowIdx,
			Data:    encBidInputValB,
		}

		var respB []byte
		var elapsed int64
		var attempt int
		var delayBlocks int

		// A side-effect of the way this is written is that the same transaction ID
		// might be reported multiple times when we collect metrics. That is OK; the
		// attempt value changes.
		for i := 0; i <= schema.RetryCount; i++ {
			if schema.ExpNum == 1 {
				delayBlocks = r.Intn(int(schema.Alpha * math.Exp2(float64(i))))
				delayTimer := time.NewTimer(time.Duration(delayBlocks) * schema.BatchTimeout)
				<-delayTimer.C
			}

			attempt = i + 1
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • about to invoke 'buy' for %.6f kWh (%.6f kW) at %.6f ç/kWh", b.ID, eventID, rowIdx, attempt, delayBlocks, row[Use]*ToKWh, row[Use], ppu)
			fmt.Fprintln(b.Writer, msg)

			timeStart := time.Now()

			respB, err = b.Invoker.Invoke(args)

			// Update stats
			timeEnd := time.Now()
			elapsed = int64(timeEnd.Sub(timeStart) / time.Millisecond)

			if err != nil {
				b.TransactionChan <- stats.Transaction{
					ID:              eventID,
					Type:            "buy",
					Status:          err.Error(),
					LatencyInMillis: elapsed,
					Attempt:         attempt,
				}
				msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • failure! cannot invoke 'buy': %s", b.ID, eventID, rowIdx, attempt, delayBlocks, err)
				fmt.Fprintln(b.Writer, msg)

				if i == schema.RetryCount {
					return errors.New(msg)
				}
			} else {
				break
			}
		}

		// Extract the write-key
		var bidOutputVal schema.BidOutput
		if err := json.Unmarshal(respB, &bidOutputVal); err != nil {
			b.TransactionChan <- stats.Transaction{
				ID:              eventID,
				Type:            "buy",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
				Attempt:         attempt,
			}
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • cannot decode JSON response to 'buy' invocation: %s", b.ID, eventID, rowIdx, attempt, delayBlocks, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		b.TransactionChan <- stats.Transaction{
			ID:              eventID,
			Type:            "buy",
			Status:          "success",
			LatencyInMillis: elapsed,
			Attempt:         attempt,
		}

		msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • success! wrote 'buy' bid to key w/ attributes %s", b.ID, eventID, rowIdx, attempt, delayBlocks, bidOutputVal.WriteKeyAttrs)
		fmt.Fprintln(b.Writer, msg)

		// Update the cmap for post-key calls
		switch schema.ExpNum {
		case 1, 3:
			b.RecentBidKeysQueue <- RecentBidKeysKV{
				Slot:          rowIdx,
				BidEventID:    eventID,
				WriteKeyAttrs: bidOutputVal.WriteKeyAttrs,
			}
		default:
		}

	}
	return nil
}

// Sell allows a bidder to place a sell offer.
func (b *Bidder) Sell(rowIdx int) error {
	eventID := fmt.Sprintf("%013d", rand.Intn(1E12))
	row := b.Trace[rowIdx]

	if row[Gen] > 0 {
		ppu := row[Lo] + (row[Hi]-row[Lo])*(1.0-rand.Float64())

		bidInputVal := schema.BidInput{
			PricePerUnitInCents: ppu,
			QuantityInKWh:       row[Gen] * ToKWh,
		}

		bidInputValB, err := json.Marshal(bidInputVal)
		if err != nil {
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d • cannot encode 'buy' bid to JSON: %s", b.ID, eventID, rowIdx, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		encBidInputValB, err := crypto.Encrypt(bidInputValB, b.pubKey)
		if err != nil {
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d • cannot encrypt 'buy' bid: %s", b.ID, eventID, rowIdx, err)
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		b.SlotChan <- stats.Slot{
			Number:    rowIdx,
			EnergyGen: bidInputVal.QuantityInKWh,
			PriceSold: row[Lo],
		}

		args := schema.OpContextInput{
			EventID: eventID,
			Action:  "sell",
			Slot:    rowIdx,
			Data:    encBidInputValB,
		}

		var respB []byte
		var elapsed int64
		var attempt int
		var delayBlocks int

		for i := 0; i <= schema.RetryCount; i++ {
			if schema.ExpNum == 1 {
				delayBlocks = r.Intn(int(schema.Alpha * math.Exp2(float64(i))))
				delayTimer := time.NewTimer(time.Duration(delayBlocks) * schema.BatchTimeout)
				<-delayTimer.C
			}

			attempt = i + 1
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • about to invoke 'sell' for %.6f kWh (%.6f kW) at %.6f ç/kWh @ slot %d", b.ID, eventID, rowIdx, attempt, delayBlocks, row[Gen]*ToKWh, row[Gen], ppu, rowIdx)
			fmt.Fprintln(b.Writer, msg)

			timeStart := time.Now()

			respB, err = b.Invoker.Invoke(args)

			// Update stats
			timeEnd := time.Now()
			elapsed = int64(timeEnd.Sub(timeStart) / time.Millisecond)

			if err != nil {
				b.TransactionChan <- stats.Transaction{
					ID:              eventID,
					Type:            "sell",
					Status:          err.Error(),
					LatencyInMillis: elapsed,
					Attempt:         attempt,
				}
				msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • failure! cannot invoke 'sell': %s", b.ID, eventID, rowIdx, attempt, delayBlocks, err)
				fmt.Fprintln(b.Writer, msg)
				if i == schema.RetryCount {
					return errors.New(msg)
				}
			} else {
				break
			}
		}

		// Extract the write-key
		var bidOutputVal schema.BidOutput
		if err := json.Unmarshal(respB, &bidOutputVal); err != nil {
			b.TransactionChan <- stats.Transaction{
				ID:              eventID,
				Type:            "sell",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
				Attempt:         attempt,
			}
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • cannot decode JSON response to 'sell' invocation: %s", b.ID, eventID, rowIdx, attempt, delayBlocks, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		b.TransactionChan <- stats.Transaction{
			ID:              eventID,
			Type:            "sell",
			Status:          "success",
			LatencyInMillis: elapsed,
			Attempt:         attempt,
		}

		msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d attempt:%d blocks_waited:%02d • success! wrote 'sell' bid to key w/ attributes %s", b.ID, eventID, rowIdx, attempt, delayBlocks, bidOutputVal.WriteKeyAttrs)
		fmt.Fprintln(b.Writer, msg)

		// Update the cmap for post-key calls
		switch schema.ExpNum {
		case 1, 3:
			b.RecentBidKeysQueue <- RecentBidKeysKV{
				Slot:          rowIdx,
				BidEventID:    eventID,
				WriteKeyAttrs: bidOutputVal.WriteKeyAttrs,
			}
		default:
		}
	}
	return nil
}

// PostKey allows a bidder to post the private key corresponding to the
// public key with which they posted an encrypted bid on the ledger.
func (b *Bidder) PostKey(rowIdx int) error {
	if schema.StagingLevel <= schema.Debug {
		msg := fmt.Sprintf("bidder:%04d slot:%012d • about to invoke 'postKey'", b.ID, rowIdx)
		fmt.Fprintln(b.Writer, msg)
	}

	valMap, ok := b.RecentBidKeys.Get(rowIdx)
	if !ok {
		msg := fmt.Sprintf("bidder:%04d slot:%012d • cannot find any bids to post keys for", b.ID, rowIdx)
		fmt.Fprintln(b.Writer, msg)

		b.TransactionChan <- stats.Transaction{
			ID:              fmt.Sprintf("%013d", rand.Intn(1E12)),
			Type:            "postKey",
			Status:          "failure: no_bids",
			LatencyInMillis: -1, // We give an invalid value here on purpose
			Attempt:         0,  // As above
		}

		return errors.New(msg)
	}

	if schema.StagingLevel <= schema.Debug {
		msg := fmt.Sprintf("bidder:%04d slot:%012d • found %d bids to post keys for", b.ID, rowIdx, len(valMap.(map[string][]string)))
		fmt.Fprintln(b.Writer, msg)
	}

	mapIdx := 0
	mapLen := len(valMap.(map[string][]string))
	for k, v := range valMap.(map[string][]string) {
		eventID := fmt.Sprintf("%013d", rand.Intn(1E12))
		mapIdx++

		postKeyInputVal := schema.PostKeyInput{
			ReadKeyAttrs: v,
			PrivKey:      b.PrivKeyBytes,
			BidEventID:   k,
		}

		postKeyInputValB, err := json.Marshal(postKeyInputVal)
		if err != nil {
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d bid_idx:%d bid_count:%d• cannot encode to JSON the payload for 'postKey' call on bid w/ event_id %s: %s", b.ID, eventID, rowIdx, mapIdx, mapLen, k, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		args := schema.OpContextInput{
			EventID: eventID,
			Action:  "postKey",
			Slot:    rowIdx,
			Data:    postKeyInputValB,
		}

		var respB []byte
		var elapsed int64
		var attempt int
		var delayBlocks int

		for i := 0; i <= schema.RetryCount; i++ {
			if schema.ExpNum == 1 {
				delayBlocks = r.Intn(int(schema.Alpha * math.Exp2(float64(i))))
				delayTimer := time.NewTimer(time.Duration(delayBlocks) * schema.BatchTimeout)
				<-delayTimer.C
			}

			attempt = i + 1
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d bid_idx:%d bid_count:%d attempt:%d blocks_waited:%02d • about to 'postKey' for bid w/ event_id %s and key %s", b.ID, eventID, rowIdx, mapIdx, mapLen, attempt, delayBlocks, k, v)
			fmt.Fprintln(b.Writer, msg)

			timeStart := time.Now()

			respB, err = b.Invoker.Invoke(args)

			// Update stats
			timeEnd := time.Now()
			elapsed = int64(timeEnd.Sub(timeStart) / time.Millisecond)

			if err != nil {
				b.TransactionChan <- stats.Transaction{
					ID:              eventID,
					Type:            "postKey",
					Status:          err.Error(),
					LatencyInMillis: elapsed,
					Attempt:         attempt,
				}
				msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d bid_idx:%d bid_count:%d attempt:%d blocks_waited:%02d • failure! cannot invoke 'postKey' for bid w/ event_id %s: %s", b.ID, eventID, rowIdx, mapIdx, mapLen, attempt, delayBlocks, k, err.Error())
				fmt.Fprintln(b.Writer, msg)

				if i == schema.RetryCount {
					return errors.New(msg)
				}
			} else {
				break
			}
		}

		// Extract the write-key
		var postKeyOutputVal schema.PostKeyOutput
		if err := json.Unmarshal(respB, &postKeyOutputVal); err != nil {
			b.TransactionChan <- stats.Transaction{
				ID:              eventID,
				Type:            "postKey",
				Status:          err.Error(),
				LatencyInMillis: elapsed,
				Attempt:         attempt,
			}
			msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d bid_idx:%d bid_count:%d attempt:%d blocks_waited:%02d • cannot decode JSON response to 'postKey' invocation for bid w/ event_id %s: %s", b.ID, eventID, rowIdx, mapIdx, mapLen, attempt, delayBlocks, k, err.Error())
			fmt.Fprintln(b.Writer, msg)
			return errors.New(msg)
		}

		// The green path
		b.TransactionChan <- stats.Transaction{
			ID:              eventID,
			Type:            "postKey",
			Status:          "success",
			LatencyInMillis: elapsed,
			Attempt:         attempt,
		}

		msg := fmt.Sprintf("bidder:%04d event_id:%s slot:%012d bid_idx:%d bid_count:%d attempt:%d blocks_waited:%02d • success! wrote 'postKey' for bid w/ event_id %s to key w/ attributes %s", b.ID, eventID, rowIdx, mapIdx, mapLen, attempt, delayBlocks, k, postKeyOutputVal.WriteKeyAttrs)
		fmt.Fprintln(b.Writer, msg)
	}

	return nil
}
