package slotnotifier

import (
	"fmt"
	"io"
	"sync"
)

// Notifier ...
type Notifier struct {
	DoneChan   <-chan struct{}
	LastVal    int
	SourceChan <-chan int
	Subs       *sync.Map
	Writer     io.Writer
}

// New ...
func New(sourcec chan int, donec chan struct{}, writer io.Writer) *Notifier {
	return &Notifier{
		DoneChan:   donec,
		SourceChan: sourcec,
		Subs:       new(sync.Map),
		Writer:     writer,
	}
}

// Register ...
func (n *Notifier) Register(id int, queue chan int) bool {
	_, loaded := n.Subs.LoadOrStore(id, queue)
	return !loaded
}

// Run ...
func (n *Notifier) Run() {
	defer fmt.Fprintln(n.Writer, "[slot notifier] Exited")

	msg := fmt.Sprint("[slot notifier] Running")
	fmt.Fprintln(n.Writer, msg)

	for {
		select {
		case <-n.DoneChan:
			return
		case newVal := <-n.SourceChan:
			msg := fmt.Sprintf("[slot notifier] Received slot %d from block notifier", newVal)
			fmt.Fprintln(n.Writer, msg)
			if newVal > n.LastVal {
				n.LastVal = newVal
				n.Subs.Range(func(k, v interface{}) bool {
					select {
					case v.(chan int) <- n.LastVal:
						// msg := fmt.Sprintf("[slot notifier] Sent slot %d to agent %d", newVal, k.(int))
						// fmt.Fprintln(n.Writer, msg)
						return true
					default:
						return false
					}
				})
			}
		}
	}
}
