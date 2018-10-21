package signal

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// ExitMsg ...
const ExitMsg = "Signal exited"

// Signal ...
type Signal struct {
	Source  <-chan uint64
	Close   <-chan struct{}
	Cond    *sync.Cond
	LastVal uint64
	Out     io.Writer
}

// Run ...
func (s *Signal) Run() {
	defer func() {
		if s.Out == nil {
			s.Out = os.Stdout
		}
		fmt.Fprintln(s.Out, ExitMsg)
	}()

	for {
		select {
		case <-s.Close:
			return
		case newVal := <-s.Source:
			if newVal > s.LastVal {
				s.Cond.L.Lock()
				s.LastVal = newVal
				s.Cond.L.Unlock()
				s.Cond.Broadcast()
			}
		}
	}
}
