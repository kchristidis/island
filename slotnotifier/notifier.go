package slotnotifier

import (
	"fmt"
	"io"
	"sync"
)

// Notifier ...
type Notifier struct {
	SourceChan <-chan int

	Subs    *sync.Map
	LastVal int

	Writer   io.Writer
	DoneChan <-chan struct{}
}

// New ...
func New(sourcec chan int, writer io.Writer, donec chan struct{}) *Notifier {
	return &Notifier{
		SourceChan: sourcec, // The channel on which slot notifications are received from the block notifier.

		Subs:    new(sync.Map),
		LastVal: -1, // -1 because we want the event for slot 0 to go through.

		Writer: writer, // Used for logging.

		DoneChan: donec, // An external kill switch. Signals to all threads in this package that they should return.
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
