package marker_test

import (
	"errors"
	"testing"

	"github.com/kchristidis/exp2/marker"
	"github.com/kchristidis/exp2/marker/markerfakes"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestMarker(t *testing.T) {
	g := NewGomegaWithT(t)

	t.Run("slotter registration fails", func(t *testing.T) {
		sdk := new(markerfakes.FakeSDKer)
		slotsrc := new(markerfakes.FakeSlotter)
		bfr := gbytes.NewBuffer()

		m := marker.New(3, sdk, slotsrc, make(chan struct{}), bfr)

		slotsrc.RegisterReturns(false)

		var err error
		go func() {
			err = m.Run()
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Marker exited"))
		g.Eventually(err, "1s", "50ms").Should(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		sdk := new(markerfakes.FakeSDKer)
		slotsrc := new(markerfakes.FakeSlotter)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := marker.New(3, sdk, slotsrc, donec, bfr)

		slotsrc.RegisterReturns(true)

		var err error
		go func() {
			err = m.Run()
		}()

		close(donec)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Marker exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("slotter works fine", func(t *testing.T) {
		sdk := new(markerfakes.FakeSDKer)
		slotsrc := new(markerfakes.FakeSlotter)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := marker.New(3, sdk, slotsrc, donec, bfr)

		slotsrc.RegisterReturns(true)

		go func() {
			m.Run()
		}()

		m.SlotQueue <- uint64(0)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing slot 0"))

		sdk.InvokeReturns(nil, nil)
		m.SlotQueue <- uint64(2)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'markEnd' for slot 2"))

		sdk.InvokeReturns(nil, errors.New("foo"))
		m.SlotQueue <- uint64(5)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'markEnd' for slot 5:"))

		m.MarkQueue = nil
		m.SlotQueue <- uint64(8)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push slot 8 to 'mark' queue"))

		close(donec)
	})
}
