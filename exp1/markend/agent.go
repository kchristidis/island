package markend

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

	"github.com/kchristidis/island/exp2/stats"
)

// BufferLen ...
const BufferLen = 100

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

	PrivKeyBytes []byte

	SlotChan        chan stats.Slot        // Used to feed the stats collector.
	TransactionChan chan stats.Transaction // Used to feed the stats collector.

	SlotQueue chan int   // The agent's trigger/input — an agent acts whenever a new slot is pushed through this channel.
	MarkQueue chan int   // A slot notification from SlotQueue ends up here, and is then picked up by a goroutine.
	ErrChan   chan error // Used internally between the spawned goroutine and the main thread in this package for the return of errors.

	Writer io.Writer

	DoneChan  chan struct{}   // An external kill switch. Signals to all threads in this package that they should return.
	killChan  chan struct{}   // An internal kill switch. It can only be closed by this package, and it signals to the package's goroutines that they should exit.
	waitGroup *sync.WaitGroup // Ensures that the main thread in this package doesn't return before the goroutines it spawned also have as well.
}

// New ...
func New(
	invoker Invoker, slotnotifier Notifier,
	privKeyBytes []byte,
	slotc chan stats.Slot, transactionc chan stats.Transaction,
	writer io.Writer, donec chan struct{}) *Agent {
	return &Agent{
		Invoker:  invoker,
		Notifier: slotnotifier,

		PrivKeyBytes: privKeyBytes,

		SlotChan:        slotc,
		TransactionChan: transactionc,

		SlotQueue: make(chan int, BufferLen),
		MarkQueue: make(chan int, BufferLen),
		ErrChan:   make(chan error),

		Writer: writer,

		DoneChan:  donec,
		killChan:  make(chan struct{}),
		waitGroup: new(sync.WaitGroup),
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprint("[markend agent] Exited")
	defer fmt.Fprintln(a.Writer, msg)

	defer func() {
		close(a.killChan)
		a.waitGroup.Wait()
	}()

	if ok := a.Notifier.Register(-1, a.SlotQueue); !ok {
		msg := fmt.Sprint("[markend agent] Unable to register with slot notifier")
		fmt.Fprintln(a.Writer, msg)
		return errors.New(msg)
	}

	msg = fmt.Sprintf("[markend agent] Registered with slot notifier")
	fmt.Fprintln(a.Writer, msg)

	a.waitGroup.Add(1)
	go func() {
		defer a.waitGroup.Done()
		for {
			select {
			case <-a.killChan:
				return
			case slot := <-a.MarkQueue:
				msg := fmt.Sprintf("[markend agent] Invoking 'markEnd' for slot %d", slot)
				fmt.Fprintln(a.Writer, msg)
				txID := strconv.Itoa(rand.Intn(1E6))
				timeStart := time.Now()
				resp, err := a.Invoker.Invoke(txID, slot, "markEnd", a.PrivKeyBytes)
				if err != nil {
					msg := fmt.Sprintf("[markend agent] Unable to invoke 'markEnd' for slot %d: %s\n", slot, err)
					// Update stats
					timeEnd := time.Now()
					elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
					a.TransactionChan <- stats.Transaction{
						ID:              txID,
						Type:            "markend",
						Status:          err.Error(),
						LatencyInMillis: elapsed,
					}
					a.ErrChan <- errors.New(msg)
				} else {
					// Update stats
					timeEnd := time.Now()
					elapsed := int64(timeEnd.Sub(timeStart) / time.Millisecond)
					a.TransactionChan <- stats.Transaction{
						ID:              txID,
						Type:            "markend",
						Status:          "success",
						LatencyInMillis: elapsed,
					}

					// This is the schema we expect from the markEnd response.
					var respVal struct {
						Msg        string
						Slot       int
						PPU, Units float64
					}

					if err := json.Unmarshal(resp, &respVal); err != nil {
						msg := fmt.Sprintf("[%s] Cannot unmarshal returned response: %s", txID, err.Error())
						fmt.Fprintln(os.Stdout, msg)
						a.ErrChan <- errors.New(msg)
					}

					msg := fmt.Sprintf("[markend agent] invocation response:\n\t%s\n", respVal.Msg)
					fmt.Fprintf(a.Writer, msg)
					if slot > -1 { // The markEnd call @ -1 is useless.
						a.SlotChan <- stats.Slot{
							Number:       slot,
							EnergyTraded: respVal.Units, // It's already in kWh, no need to convert
							PriceTraded:  respVal.PPU,
						}
					}
				}
			case <-a.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case err := <-a.ErrChan:
			fmt.Fprintln(a.Writer, err.Error())
			return err
		case slot := <-a.SlotQueue:
			prevSlot := slot - 1
			msg := fmt.Sprintf("[markend agent] Processing slot %d", prevSlot)
			fmt.Fprintln(a.Writer, msg)
			select {
			case a.MarkQueue <- prevSlot:
			default:
				msg := fmt.Sprintf("[markend agent] Unable to push slot %d to 'mark' queue (size: %d)", prevSlot, len(a.MarkQueue))
				fmt.Fprintln(a.Writer, msg)
				return errors.New(msg)
			}
		case <-a.DoneChan:
			select { // Cover the edge case in tests where you close the DoneChan to conclude the test, and this branch is selected over the top-level ErrChan one
			case err := <-a.ErrChan:
				return err
			default:
				return nil
			}
		}
	}
}
