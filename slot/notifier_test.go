package slot

import (
	"testing"

	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestNotifier(t *testing.T) {
	g := NewGomegaWithT(t)

	sourcec := make(chan int)
	donec := make(chan struct{})
	bfr := gbytes.NewBuffer()
	n := New(sourcec, donec, bfr)
	go n.Run()

	t.Run("register", func(t *testing.T) {
		require.True(t, n.Register(1, make(chan int, 1)))
		_, ok := n.Subs.Load(1)
		require.True(t, ok)
		require.False(t, n.Register(1, make(chan int, 1)))
	})

	t.Run("run", func(t *testing.T) {
		finalVal := 1
		go func() { // Producer
			sourcec <- finalVal
		}()

		var rxVal int
		go func() { // Subscriber
			ch, _ := n.Subs.Load(1)
			rxVal = <-ch.(chan int)
		}()

		// Notifier gets set properly
		g.Eventually(func() int {
			return n.LastVal
		}, "1s", "50ms").Should(Equal(finalVal))

		// Subscriber gets set properly
		g.Eventually(func() int {
			return rxVal
		}, "1s", "50ms").Should(Equal(finalVal))
	})

	t.Run("close", func(t *testing.T) {
		close(donec)

		// Signal closes properly
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Slot notifier exited"))
	})
}
