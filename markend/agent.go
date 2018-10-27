package markend

import (
	"errors"
	"fmt"
	"io"
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
	DoneChan     chan struct{}
	ErrChan      chan error
	Invoker      Invoker
	MarkQueue    chan int
	Notifier     Notifier
	PrivKeyBytes []byte
	SlotQueue    chan int // this is the agent's trigger, i.e. it's supposed to act whenever a new slot is created
	Writer       io.Writer
}

// New ...
func New(invoker Invoker, notifier Notifier, privKeyBytes []byte, donec chan struct{}, writer io.Writer) *Agent {
	return &Agent{
		DoneChan:     donec,
		ErrChan:      make(chan error),
		Invoker:      invoker,
		MarkQueue:    make(chan int, BufferLen),
		Notifier:     notifier,
		PrivKeyBytes: privKeyBytes,
		SlotQueue:    make(chan int, BufferLen),
		Writer:       writer,
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprint("[markend agent] Exited")
	defer fmt.Fprintln(a.Writer, msg)

	if ok := a.Notifier.Register(-1, a.SlotQueue); !ok {
		msg := fmt.Sprint("[markend agent] Unable to register with slot notifier")
		fmt.Fprintln(a.Writer, msg)
		return errors.New(msg)
	}

	msg = fmt.Sprintf("[markend agent] Registered with slot notifier")
	fmt.Fprintln(a.Writer, msg)

	go func() {
		for {
			select {
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
			select { // cover the edge case in tests where you close the donechan to conclude the test, and this branch is selected over the top-level errchan one
			case err := <-a.ErrChan:
				return err
			default:
				return nil
			}
		}
	}
}
