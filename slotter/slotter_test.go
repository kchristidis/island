package slotter

import (
	"testing"

	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestSlotter(t *testing.T) {
	g := NewGomegaWithT(t)

	srcChan := make(chan int)
	doneChan := make(chan struct{})
	bfr := gbytes.NewBuffer()
	s := New(srcChan, doneChan, bfr)
	go s.Run()

	t.Run("register", func(t *testing.T) {
		require.True(t, s.Register(1, make(chan int, 1)))
		_, ok := s.Subs.Load(1)
		require.True(t, ok)
		require.False(t, s.Register(1, make(chan int, 1)))
	})

	t.Run("run", func(t *testing.T) {
		finalVal := 1
		go func() { // Producer
			srcChan <- finalVal
		}()

		var rxVal int
		go func() { // Subscriber
			ch, _ := s.Subs.Load(1)
			rxVal = <-ch.(chan int)
		}()

		// Signal gets set properly
		g.Eventually(func() int {
			return s.LastVal
		}, "1s", "50ms").Should(Equal(finalVal))

		// Subscriber gets set properly
		g.Eventually(func() int {
			return rxVal
		}, "1s", "50ms").Should(Equal(finalVal))
	})

	t.Run("close", func(t *testing.T) {
		close(doneChan)

		// Signal closes properly
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(ExitMsg))
	})
}
