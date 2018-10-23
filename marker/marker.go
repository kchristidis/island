package marker

import (
	"fmt"
	"io"
	"sync"
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

// Signaler ...
//go:generate counterfeiter . Signaler
type Signaler interface {
	Register(id int, queue chan uint64) bool
}

// Marker ...
type Marker struct {
	DoneChan    chan struct{}
	MarkQueue   chan int
	Period      int
	SDK         SDKer
	Signal      Signaler
	SignalQueue chan uint64
	Out         io.Writer

	ID    int
	first int // The block number that corresponds to the first row in the trace
	once  sync.Once
}

// New ...
func New(period int, sdkContext SDKer, signal Signaler, doneChan chan struct{}, out io.Writer) *Marker {
	return &Marker{
		DoneChan:    doneChan,
		ID:          ID,
		MarkQueue:   make(chan int, BufferLen),
		Period:      period,
		SDK:         sdkContext,
		Signal:      signal,
		SignalQueue: make(chan uint64, BufferLen),
		Out:         out,
	}
}

// Run ...
func (m *Marker) Run() error {
	msg := fmt.Sprintf("[%d] Marker exited", m.ID)
	defer fmt.Fprintln(m.Out, msg)

	if ok := m.Signal.Register(m.ID, m.SignalQueue); !ok {
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
		case blockNumber := <-m.SignalQueue:
			m.once.Do(func() {
				m.first = int(blockNumber)
			})

			slot := int(blockNumber) - m.first
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
