package markend_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kchristidis/exp2/crypto"
	"github.com/kchristidis/exp2/markend"
	"github.com/kchristidis/exp2/markend/markendfakes"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
	g := NewGomegaWithT(t)

	privkeypath := filepath.Join("..", "crypto", "priv.pem")
	privkey, err := crypto.LoadPrivate(privkeypath)
	require.NoError(t, err)
	privkeybytes := crypto.SerializePrivate(privkey)

	t.Run("notifier registration fails", func(t *testing.T) {
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		m := markend.New(invoker, slotnotifier, privkeybytes, donec, bfr)

		slotnotifier.RegisterReturns(false)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to register with slot notifier"))
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

		m := markend.New(invoker, slotnotifier, privkeybytes, donec, bfr)

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

		m := markend.New(invoker, slotnotifier, privkeybytes, donec, bfr)

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

		m := markend.New(invoker, slotnotifier, privkeybytes, donec, bfr)

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

		m := markend.New(invoker, slotnotifier, privkeybytes, donec, bfr)

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
