package markend_test

import (
	"errors"
	"testing"

	"github.com/kchristidis/exp2/markend"
	"github.com/kchristidis/exp2/markend/markendfakes"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
	g := NewGomegaWithT(t)

	t.Run("notifier registration fails", func(t *testing.T) {
		sdkctx := new(markendfakes.FakeSDKer)
		slotnotifier := new(markendfakes.FakeNotifier)
		bfr := gbytes.NewBuffer()

		m := markend.New(3, sdkctx, slotnotifier, make(chan struct{}), bfr)

		slotnotifier.RegisterReturns(false)

		var err error
		go func() {
			err = m.Run()
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Markend agent exited"))
		g.Eventually(err, "1s", "50ms").Should(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		sdkctx := new(markendfakes.FakeSDKer)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(3, sdkctx, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		go func() {
			err = m.Run()
		}()

		close(donec)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Markend agent exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("notifier works fine", func(t *testing.T) {
		sdkctx := new(markendfakes.FakeSDKer)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(3, sdkctx, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		go func() {
			m.Run()
		}()

		m.SlotQueue <- 0

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing slot 0"))

		sdkctx.InvokeReturns(nil, nil)
		m.SlotQueue <- 2
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'markEnd' for slot 2"))

		sdkctx.InvokeReturns(nil, errors.New("foo"))
		m.SlotQueue <- 5
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'markEnd' for slot 5:"))

		m.MarkQueue = nil
		m.SlotQueue <- 8
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push slot 8 to 'mark' queue"))

		close(donec)
	})
}
