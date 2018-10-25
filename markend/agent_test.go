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
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(false)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to register with signaler"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		close(donec)
		<-deadc

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		invoker.InvokeReturns(nil, nil)
		m.SlotQueue <- 0

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing slot -1"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'markEnd' for slot -1"))

		close(donec)
		<-deadc
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("invocation fails", func(t *testing.T) {
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		invoker.InvokeReturns(nil, errors.New("foo"))
		m.SlotQueue <- 0

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'markEnd' for slot -1"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("invocation fails", func(t *testing.T) {
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		m.MarkQueue = nil
		m.SlotQueue <- 0

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push slot -1 to 'mark' queue"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})
}
