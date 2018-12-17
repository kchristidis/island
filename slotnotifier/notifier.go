package slotnotifier

import (
	"fmt"
	"io"
	"math/rand"
	"sync"
)

// Notifier receives slot notifications and fans them out to its subscribers.
type Notifier struct {
	SourceChan <-chan int

	Subs    *sync.Map
	LastVal int

	ID int

	Writer   io.Writer
	DoneChan <-chan struct{}
}

// New returns a new notifier.
func New(sourcec chan int, writer io.Writer, donec chan struct{}) *Notifier {
	// We use this to differentiate between the
	// bid/markEnd notifier, and the postKey one.
	randomID := int(rand.Float32() * 100)

	return &Notifier{
		SourceChan: sourcec, // The channel on which slot notifications are received from the block notifier.

		Subs:    new(sync.Map),
		LastVal: -1, // -1 because we want the event for slot 0 to go through.

		ID: randomID,

		Writer: writer, // Used for logging.

		DoneChan: donec, // An external kill switch. Signals to all threads in this package that they should return.
	}
}

// Register allows a subscriber to register with this notifier.
func (n *Notifier) Register(id int, queue chan int) bool {
	_, loaded := n.Subs.LoadOrStore(id, queue)
	return !loaded
}

// Run executes the notifier logic.
func (n *Notifier) Run() {
	defer func() {
		msg := fmt.Sprintf("slot-notifier:%02d • exited", n.ID)
		fmt.Fprintln(n.Writer, msg)
	}()

	msg := fmt.Sprintf("slot-notifier:%02d • running", n.ID)
	fmt.Fprintln(n.Writer, msg)

	for {
		select {
		case <-n.DoneChan:
			return
		case newVal := <-n.SourceChan:
			msg := fmt.Sprintf("slot-notifier:%02d slot:%012d • received new slot from block notifier", n.ID, newVal)
			fmt.Fprintln(n.Writer, msg)
			if newVal > n.LastVal {
				n.LastVal = newVal
				n.Subs.Range(func(k, v interface{}) bool {
					select {
					case v.(chan int) <- n.LastVal:
						/* id := k.(int)
						var msg string
						if id == -1 {
							msg = fmt.Sprintf("slot-notifier:%02d slot:%012d • sent new slot to regulator", n.ID, newVal)
						} else {
							msg = fmt.Sprintf("slot-notifier:%02d slot:%012d • sent new slot to agent %d", n.ID, newVal, id)
						}
						fmt.Fprintln(n.Writer, msg) */
						return true
					default:
						return false
					}
				})
			}
		}
	}
}
