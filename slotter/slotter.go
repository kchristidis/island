package slotter

import (
	"fmt"
	"io"
	"sync"
)

// Slotter ...
type Slotter struct {
	DoneChan   <-chan struct{}
	LastVal    int
	Out        io.Writer
	SourceChan <-chan int
	Subs       *sync.Map

	once sync.Once
}

// New ...
func New(srcChan chan int, doneChan chan struct{}, out io.Writer) *Slotter {
	return &Slotter{
		DoneChan:   doneChan,
		Out:        out,
		SourceChan: srcChan,
		Subs:       new(sync.Map),
	}
}

// Register ...
func (s *Slotter) Register(id int, queue chan int) bool {
	_, loaded := s.Subs.LoadOrStore(id, queue)
	return !loaded
}

// Run ...
func (s *Slotter) Run() {
	defer fmt.Fprintln(s.Out, "Slotter exited")

	for {
		select {
		case <-s.DoneChan:
			return
		case newVal := <-s.SourceChan:
			if newVal > s.LastVal {
				s.LastVal = newVal
				s.Subs.Range(func(k, v interface{}) bool {
					select {
					case v.(chan int) <- s.LastVal:
						return true
					default:
						return false
					}
				})
			}
		}
	}
}
