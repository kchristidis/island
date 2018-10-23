package markend

import (
	"fmt"
	"io"
)

// ID ...
const ID = -1

// BufferLen ...
const BufferLen = 100

// SDKer ...
//go:generate counterfeiter . SDKer
type SDKer interface {
	Invoke(slot int, action string, dataB []byte) ([]byte, error)
}

// Notifier ...
//go:generate counterfeiter . Notifier
type Notifier interface {
	Register(id int, queue chan int) bool
}

// Agent ...
type Agent struct {
	DoneChan  chan struct{}
	ID        int
	MarkQueue chan int
	Period    int
	SDK       SDKer
	Notifier  Notifier
	SlotQueue chan int
	Writer    io.Writer
}

// New ...
func New(period int, sdkctx SDKer, notifier Notifier, donec chan struct{}, writer io.Writer) *Agent {
	return &Agent{
		DoneChan:  donec,
		ID:        ID,
		MarkQueue: make(chan int, BufferLen),
		Notifier:  notifier,
		Period:    period,
		SDK:       sdkctx,
		SlotQueue: make(chan int, BufferLen),
		Writer:    writer,
	}
}

// Run ...
func (a *Agent) Run() error {
	msg := fmt.Sprintf("[%d] Markend agent exited", a.ID)
	defer fmt.Fprintln(a.Writer, msg)

	if ok := a.Notifier.Register(a.ID, a.SlotQueue); !ok {
		return fmt.Errorf("[%d] Unable to register with signaler", a.ID)
	}

	go func() {
		for {
			select {
			case slot := <-a.MarkQueue:
				msg := fmt.Sprintf("[%d] Invoking 'markEnd' for slot %d", a.ID, slot)
				fmt.Fprintln(a.Writer, msg)
				if _, err := a.SDK.Invoke(2, "markEnd", []byte("prvKey")); err != nil {
					msg := fmt.Sprintf("[%d] Unable to invoke 'markEnd' for slot %d: %s\n", a.ID, slot, err)
					fmt.Fprintf(a.Writer, msg)
				}
			case <-a.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case slot := <-a.SlotQueue:
			msg := fmt.Sprintf("[%d] Processing slot %d", a.ID, slot)
			fmt.Fprintln(a.Writer, msg)

			if (slot+1)%a.Period == 0 { // We add 1 because slot 0 is the 1st slot
				select {
				case a.MarkQueue <- slot:
				default:
					msg := fmt.Sprintf("[%d] Unable to push slot %d to 'mark' queue (size: %d)", a.ID, slot, len(a.MarkQueue))
					fmt.Fprintln(a.Writer, msg)
				}
			}
		case <-a.DoneChan:
			return nil
		}
	}
}
