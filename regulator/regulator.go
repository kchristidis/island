package regulator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/stats"
)

// BufferLen sets the buffer length for the slot and task channels.
const BufferLen = 100

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Invoker

// Invoker is an interface that encapsulates the
// peer calls that are relevant to the regulator.
type Invoker interface {
	Invoke(args schema.OpContextInput) ([]byte, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Notifier

// Notifier is an interface that encapsulates the slot
// notifier calls that are relevant to the regulator.
type Notifier interface {
	Register(id int, queue chan int) bool
}

// Regulator issues regulatory calls to the peer
// upon receiving slot notifications.
type Regulator struct {
	Invoker  Invoker
	Notifier Notifier

	PrivKeyBytes []byte // The regulator's key pair

	// Used to feed the stats collector
	SlotChan        chan stats.Slot
	TransactionChan chan stats.Transaction

	// The regulator's trigger/input — a regulator acts
	// whenever a new slot is pushed through this channel.
	SlotQueue chan int
	// A slot notification from SlotQueue ends up here, and
	// is then picked up by a goroutine.
	TaskQueue chan int
	// Used internally between the spawned goroutine and the
	// main thread in this package for the return of errors.
	ErrChan chan error

	Writer io.Writer

	// An external kill switch. Signals to all threads in this package
	// that they should return.
	DoneChan chan struct{}
	// An internal kill switch. It can only be closed by this package,
	// and it signals to the package's goroutines that they should exit.
	killChan chan struct{}
	// Ensures that the main thread in this package doesn't return
	// before the goroutines it spawned have.
	waitGroup *sync.WaitGroup
}

// New returns a new regulator.
func New(
	invoker Invoker, slotnotifier Notifier,
	privKeyBytes []byte,
	slotc chan stats.Slot, transactionc chan stats.Transaction,
	writer io.Writer, donec chan struct{}) *Regulator {
	return &Regulator{
		Invoker:  invoker,
		Notifier: slotnotifier,

		PrivKeyBytes: privKeyBytes,

		SlotChan:        slotc,
		TransactionChan: transactionc,

		SlotQueue: make(chan int, BufferLen),
		TaskQueue: make(chan int, BufferLen),
		ErrChan:   make(chan error),

		Writer: writer,

		DoneChan:  donec,
		killChan:  make(chan struct{}),
		waitGroup: new(sync.WaitGroup),
	}
}

// Run executes the regulator logic.
func (r *Regulator) Run() error {
	msg := fmt.Sprint("regulator • exited")
	defer fmt.Fprintln(r.Writer, msg)

	defer func() {
		close(r.killChan)
		r.waitGroup.Wait()
	}()

	if ok := r.Notifier.Register(-1, r.SlotQueue); !ok {
		msg := fmt.Sprintf("regulator • cannot register with slot notifier")
		fmt.Fprintln(r.Writer, msg)
		return errors.New(msg)
	}

	if schema.StagingLevel <= schema.Debug {
		msg := fmt.Sprint("regulator • registered with slot notifier")
		fmt.Fprintln(r.Writer, msg)
	}

	r.waitGroup.Add(1)
	go func() {
		defer r.waitGroup.Done()
		for {
			select {
			case <-r.killChan:
				return
			case slot := <-r.TaskQueue:
				affectedSlot := slot - 1
				eventID := fmt.Sprintf("%013d", rand.Intn(1E12))

				msg := fmt.Sprintf("regulator event_id:%s slot:%012d • about to invoke 'markEnd' - note! this will mark the end of slot %012d", eventID, slot, affectedSlot)
				fmt.Fprintln(r.Writer, msg)

				// We decrement the slot number because a markEnd call
				// @ slot N is supposed to mark the end of slot N-1.
				args := schema.OpContextInput{
					EventID: eventID,
					Action:  "markEnd",
					Slot:    affectedSlot,
				}

				// Does the regulator need to post its private key for slot
				// N-1 so that everyone else can verify the encrypted bids?
				switch schema.ExpNum {
				case 1, 3:
					args.Data = nil
				case 2:
					markEndInputVal := schema.MarkEndInput{
						PrivKey: r.PrivKeyBytes,
					}
					markEndInputValB, err := json.Marshal(markEndInputVal)
					if err != nil {
						msg := fmt.Sprintf("regulator event_id:%s slot:%012d • cannot encode 'markEnd' call to JSON: %s", eventID, slot, err.Error())
						fmt.Fprintln(r.Writer, msg)
						continue
					}
					args.Data = markEndInputValB
				}

				timeStart := time.Now()

				respB, err := r.Invoker.Invoke(args)

				// Update stats
				timeEnd := time.Now()
				elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)

				if err != nil {
					r.TransactionChan <- stats.Transaction{
						ID:              eventID,
						Type:            "markEnd",
						Status:          err.Error(),
						LatencyInMillis: elapsed,
					}
					msg := fmt.Sprintf("regulator event_id:%s slot:%012d • failure! cannot invoke 'markEnd': %s\n", eventID, slot, err)
					r.ErrChan <- errors.New(msg)
				} else {
					r.TransactionChan <- stats.Transaction{
						ID:              eventID,
						Type:            "markEnd",
						Status:          "success",
						LatencyInMillis: elapsed,
					}

					var markendOutputVal schema.MarkEndOutput
					if err := json.Unmarshal(respB, &markendOutputVal); err != nil {
						msg := fmt.Sprintf("regulator event_id:%s slot:%012d • cannot decode JSON response to 'markEnd' invocation: %s", eventID, slot, err.Error())
						fmt.Fprintln(r.Writer, msg)
						r.ErrChan <- errors.New(msg)
						continue
					}

					msg := fmt.Sprintf("regulator event_id:%s slot:%012d • invocation response: %s", eventID, slot, markendOutputVal.Message)
					fmt.Fprintln(r.Writer, msg)
					if slot > -1 { // The markEnd call @ -1 is useless.
						r.SlotChan <- stats.Slot{
							Number:       affectedSlot, // ATTN: markEnd @ slot N clears the market @ slot N-1.
							EnergyTraded: markendOutputVal.QuantityInKWh,
							PriceTraded:  markendOutputVal.PricePerUnitInCents,
						}
					}

				}
			case <-r.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case err := <-r.ErrChan:
			fmt.Fprintln(r.Writer, err.Error())
			// return err
		case slot := <-r.SlotQueue:
			msg := fmt.Sprintf("regulator slot:%012d • new slot!", slot)
			fmt.Fprintln(r.Writer, msg)
			if slot == 0 {
				msg := fmt.Sprintf("regulator slot:%012d • no point to act on slot 0 — skipping!", slot)
				fmt.Fprintln(r.Writer, msg)
				continue
			}
			select {
			case r.TaskQueue <- slot:
			default:
				msg := fmt.Sprintf("regulator slot:%012d • cannot push notification to task queue (size: %d)", slot, len(r.TaskQueue))
				fmt.Fprintln(r.Writer, msg)
				return errors.New(msg)
			}
		case <-r.DoneChan:
			select {
			// Cover the edge case in tests where you close the DoneChan to conclude
			// the test, and this branch is selected over the top-level ErrChan one
			case err := <-r.ErrChan:
				return err
			default:
				return nil
			}
		}
	}
}
