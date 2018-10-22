package signal

import (
	"fmt"
	"io"
	"sync"
)

// ExitMsg ...
const ExitMsg = "Signal exited"

// Signal ...
type Signal struct {
	DoneChan   <-chan struct{}
	LastVal    uint64
	Out        io.Writer
	SourceChan <-chan uint64
	Subs       *sync.Map

	once sync.Once
}

// New ...
func New(srcChan chan uint64, doneChan chan struct{}, out io.Writer) *Signal {
	return &Signal{
		DoneChan:   doneChan,
		Out:        out,
		SourceChan: srcChan,
		Subs:       new(sync.Map),
	}
}

// Register ...
func (s *Signal) Register(id int, queue chan uint64) bool {
	_, loaded := s.Subs.LoadOrStore(id, queue)
	return !loaded
}

// Run ...
func (s *Signal) Run() {
	defer fmt.Fprintln(s.Out, ExitMsg)

	for {
		select {
		case <-s.DoneChan:
			return
		case newVal := <-s.SourceChan:
			if newVal > s.LastVal {
				s.LastVal = newVal
				s.Subs.Range(func(k, v interface{}) bool {
					select {
					case v.(chan uint64) <- s.LastVal:
						return true
					default:
						return false
					}
				})
			}
		}
	}
}
