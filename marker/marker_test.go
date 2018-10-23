package marker_test

import (
	"errors"
	"testing"

	"github.com/kchristidis/exp2/marker"
	"github.com/kchristidis/exp2/marker/markerfakes"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func TestMarker(t *testing.T) {
	g := NewGomegaWithT(t)

	t.Run("signal registration errors", func(t *testing.T) {
		sdk := new(markerfakes.FakeSDKer)
		signal := new(markerfakes.FakeSignaler)
		bfr := gbytes.NewBuffer()

		m := marker.New(3, sdk, signal, make(chan struct{}), bfr)

		signal.RegisterReturns(false)

		var err error
		go func() {
			err = m.Run()
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Marker exited"))
		g.Eventually(err, "1s", "50ms").Should(HaveOccurred())
	})

	t.Run("close done chan", func(t *testing.T) {
		sdk := new(markerfakes.FakeSDKer)
		signal := new(markerfakes.FakeSignaler)
		doneChan := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := marker.New(3, sdk, signal, doneChan, bfr)

		signal.RegisterReturns(true)

		var err error
		go func() {
			err = m.Run()
		}()

		close(doneChan)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Marker exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("signal works fine", func(t *testing.T) {
		sdk := new(markerfakes.FakeSDKer)
		signal := new(markerfakes.FakeSignaler)
		doneChan := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := marker.New(3, sdk, signal, doneChan, bfr)

		signal.RegisterReturns(true)

		go func() {
			m.Run()
		}()

		m.SignalQueue <- uint64(0)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing slot 0"))

		sdk.InvokeReturns(nil, nil)
		m.SignalQueue <- uint64(2)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'markEnd' for slot 2"))

		sdk.InvokeReturns(nil, errors.New("foo"))
		m.SignalQueue <- uint64(5)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'markEnd' for slot 5:"))

		m.MarkQueue = nil
		m.SignalQueue <- uint64(8)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push slot 8 to 'mark' queue"))

		close(doneChan)
	})
}
