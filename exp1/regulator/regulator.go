package regulator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/kchristidis/island/exp1/stats"
)

// Constants ...
const (
	MarkEnd    string = "markEnd"
	RevealKeys string = "revealKeys"

	BufferLen = 100
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

// Regulator ...
type Regulator struct {
	Invoker  Invoker
	Notifier Notifier

	ID           int
	Type         string // Allowed values:  MarkEnd, RevealKeys
	PrivKeyBytes []byte

	SlotChan        chan stats.Slot        // Used to feed the stats collector.
	TransactionChan chan stats.Transaction // Used to feed the stats collector.

	SlotQueue chan int   // The regulator's trigger/input — a regulator acts whenever a new slot is pushed through this channel.
	TaskQueue chan int   // A slot notification from SlotQueue ends up here, and is then picked up by a goroutine.
	ErrChan   chan error // Used internally between the spawned goroutine and the main thread in this package for the return of errors.

	Writer io.Writer

	DoneChan  chan struct{}   // An external kill switch. Signals to all threads in this package that they should return.
	killChan  chan struct{}   // An internal kill switch. It can only be closed by this package, and it signals to the package's goroutines that they should exit.
	waitGroup *sync.WaitGroup // Ensures that the main thread in this package doesn't return before the goroutines it spawned also have as well.
}

// New ...
func New(
	invoker Invoker, slotnotifier Notifier,
	id int, regType string, privKeyBytes []byte,
	slotc chan stats.Slot, transactionc chan stats.Transaction,
	writer io.Writer, donec chan struct{}) *Regulator {
	return &Regulator{
		Invoker:  invoker,
		Notifier: slotnotifier,

		ID:           id,
		Type:         regType,
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

// Run ...
func (r *Regulator) Run() error {
	msg := fmt.Sprintf("[regulator %s-%d] Exited", r.Type, r.ID)
	defer fmt.Fprintln(r.Writer, msg)

	defer func() {
		close(r.killChan)
		r.waitGroup.Wait()
	}()

	if ok := r.Notifier.Register(r.ID, r.SlotQueue); !ok {
		msg := fmt.Sprintf("[regulator %s-%2d] Unable to register with slot notifier", r.Type, r.ID)
		fmt.Fprintln(r.Writer, msg)
		return errors.New(msg)
	}

	msg = fmt.Sprintf("[regulator %s-%02d] Registered with slot notifier", r.Type, r.ID)
	fmt.Fprintln(r.Writer, msg)

	r.waitGroup.Add(1)
	go func() {
		defer r.waitGroup.Done()
		for {
			select {
			case <-r.killChan:
				return
			case slot := <-r.TaskQueue:
				msg := fmt.Sprintf("[regulator %s-%02d] Invoking @ slot %d", r.Type, r.ID, slot)
				fmt.Fprintln(r.Writer, msg)
				txID := strconv.Itoa(rand.Intn(1E6))
				timeStart := time.Now()
				var resp []byte
				var err error
				switch r.Type {
				case MarkEnd:
					// We decrement the slot number because a markEnd call
					// @ slot N is supposed to mark the end of slot N.
					resp, err = r.Invoker.Invoke(txID, slot-1, r.Type, r.PrivKeyBytes)
				case RevealKeys:
					resp, err = r.Invoker.Invoke(txID, slot, r.Type, r.PrivKeyBytes)
				}
				if err != nil {
					msg := fmt.Sprintf("[regulator %s-%02d] Unable to invoke @ slot %d: %s\n", r.Type, r.ID, slot, err)
					// Update stats
					timeEnd := time.Now()
					elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
					r.TransactionChan <- stats.Transaction{
						ID:              txID,
						Type:            r.Type,
						Status:          err.Error(),
						LatencyInMillis: elapsed,
					}
					r.ErrChan <- errors.New(msg)
				} else {
					// Update stats
					timeEnd := time.Now()
					elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
					r.TransactionChan <- stats.Transaction{
						ID:              txID,
						Type:            r.Type,
						Status:          "success",
						LatencyInMillis: elapsed,
					}

					switch r.Type {
					case MarkEnd:
						// This is the schema we expect from the markEnd response.
						var respVal struct {
							Msg        string
							Slot       int
							PPU, Units float64
						}

						if err := json.Unmarshal(resp, &respVal); err != nil {
							msg := fmt.Sprintf("[%s] Cannot unmarshal returned response: %s", txID, err.Error())
							fmt.Fprintln(os.Stdout, msg)
							r.ErrChan <- errors.New(msg)
						}

						msg := fmt.Sprintf("[regulator %s-%02d] Invocation response:\n\t%s\n", r.Type, r.ID, respVal.Msg)
						fmt.Fprintf(r.Writer, msg)
						if slot > -1 { // The markEnd call @ -1 is useless.
							r.SlotChan <- stats.Slot{
								Number:       slot - 1,      // ATTN: The markEnd call @ slot N clears the market @ slot N-1.
								EnergyTraded: respVal.Units, // It's already in kWh, no need to convert
								PriceTraded:  respVal.PPU,
							}
						}
					case RevealKeys:
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
			return err
		case slot := <-r.SlotQueue:
			msg := fmt.Sprintf("[regulator %s-%02d] Got notified that slot %d has arrived", r.Type, r.ID, slot)
			fmt.Fprintln(r.Writer, msg)
			select {
			case r.TaskQueue <- slot:
			default:
				msg := fmt.Sprintf("[regulator %s-%02d] Unable to push notification of slot %d to task queue (size: %d)",
					r.Type, r.ID, slot, len(r.TaskQueue))
				fmt.Fprintln(r.Writer, msg)
				return errors.New(msg)
			}
		case <-r.DoneChan:
			select { // Cover the edge case in tests where you close the DoneChan to conclude the test, and this branch is selected over the top-level ErrChan one
			case err := <-r.ErrChan:
				return err
			default:
				return nil
			}
		}
	}
}
