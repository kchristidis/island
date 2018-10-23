package marker

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

// Slotter ...
//go:generate counterfeiter . Slotter
type Slotter interface {
	Register(id int, queue chan int) bool
}

// Marker ...
type Marker struct {
	DoneChan   chan struct{}
	ID         int
	MarkQueue  chan int
	Period     int
	SDK        SDKer
	SlotSource Slotter
	SlotQueue  chan int
	Out        io.Writer
}

// New ...
func New(period int, sdkContext SDKer, slotSource Slotter, doneChan chan struct{}, out io.Writer) *Marker {
	return &Marker{
		DoneChan:   doneChan,
		ID:         ID,
		MarkQueue:  make(chan int, BufferLen),
		Period:     period,
		SDK:        sdkContext,
		SlotSource: slotSource,
		SlotQueue:  make(chan int, BufferLen),
		Out:        out,
	}
}

// Run ...
func (m *Marker) Run() error {
	msg := fmt.Sprintf("[%d] Marker exited", m.ID)
	defer fmt.Fprintln(m.Out, msg)

	if ok := m.SlotSource.Register(m.ID, m.SlotQueue); !ok {
		return fmt.Errorf("[%d] Unable to register with signaler", m.ID)
	}

	go func() {
		for {
			select {
			case slot := <-m.MarkQueue:
				msg := fmt.Sprintf("[%d] Invoking 'markEnd' for slot %d", m.ID, slot)
				fmt.Fprintln(m.Out, msg)
				if _, err := m.SDK.Invoke(2, "markEnd", []byte("prvKey")); err != nil {
					msg := fmt.Sprintf("[%d] Unable to invoke 'markEnd' for slot %d: %s\n", m.ID, slot, err)
					fmt.Fprintf(m.Out, msg)
				}
			case <-m.DoneChan:
				return
			}
		}
	}()

	for {
		select {
		case slot := <-m.SlotQueue:
			msg := fmt.Sprintf("[%d] Processing slot %d", m.ID, slot)
			fmt.Fprintln(m.Out, msg)

			if (slot+1)%m.Period == 0 { // We add 1 because slot 0 is the 1st slot
				select {
				case m.MarkQueue <- slot:
				default:
					msg := fmt.Sprintf("[%d] Unable to push slot %d to 'mark' queue (size: %d)", m.ID, slot, len(m.MarkQueue))
					fmt.Fprintln(m.Out, msg)
				}
			}
		case <-m.DoneChan:
			return nil
		}
	}
}
