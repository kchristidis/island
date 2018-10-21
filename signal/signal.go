package signal

import (
	"fmt"
	"io"
	"sync"
)

// ExitMsg ...
const ExitMsg = "Signal exited"

// BufferLen ...
const BufferLen = 1000

// Signal ...
type Signal struct {
	Source  <-chan uint64
	Done    <-chan struct{}
	Out     io.Writer
	LastVal uint64
	Subs    *sync.Map
	once    sync.Once
}

// New ...
func New(srcChan chan uint64, doneChan chan struct{}, out io.Writer) *Signal {
	return &Signal{
		Source: srcChan,
		Done:   doneChan,
		Out:    out,
		Subs:   new(sync.Map),
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
		case <-s.Done:
			return
		case newVal := <-s.Source:
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
