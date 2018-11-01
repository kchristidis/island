package markend

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/kchristidis/exp2/stats"
)

// BufferLen ...
const BufferLen = 100

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
	Invoker  Invoker
	Notifier Notifier

	PrivKeyBytes []byte

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
	privKeyBytes []byte, transactionc chan stats.Transaction,
	writer io.Writer, donec chan struct{}) *Agent {
	return &Agent{
		Invoker:  invoker,
		Notifier: slotnotifier,

		PrivKeyBytes:    privKeyBytes,
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
				resp, err := a.Invoker.Invoke(slot, "markEnd", a.PrivKeyBytes)
				if err != nil {
					msg := fmt.Sprintf("[markend agent] Unable to invoke 'markEnd' for slot %d: %s\n", slot, err)
					a.ErrChan <- errors.New(msg)
				} else {
					msg := fmt.Sprintf("[markend agent] invocation response:\n\t%s\n", resp)
					fmt.Fprintf(a.Writer, msg)
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
