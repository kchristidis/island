package markend_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/markend"
	"github.com/kchristidis/island/markend/markendfakes"
	"github.com/kchristidis/island/stats"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
	g := NewGomegaWithT(t)

	var respVal struct {
		msg        string
		slot       int
		ppu, units float64
	}

	privkeypath := filepath.Join("..", "crypto", "priv.pem")
	privkey, err := crypto.LoadPrivate(privkeypath)
	require.NoError(t, err)
	privkeybytes := crypto.SerializePrivate(privkey)
	slotc := make(chan stats.Slot, 10)               // A large enough buffer so that we don't have to worry about draining it.
	transactionc := make(chan stats.Transaction, 10) // A large enough buffer so that we don't have to worry about draining it.

	t.Run("notifier registration fails", func(t *testing.T) {
		invoker := new(markendfakes.FakeInvoker)
		slotnotifier := new(markendfakes.FakeNotifier)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		m := markend.New(invoker, slotnotifier, privkeybytes, slotc, transactionc, bfr, donec)

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

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		m := markend.New(invoker, slotnotifier, privkeybytes, slotc, transactionc, bfr, donec)

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

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		m := markend.New(invoker, slotnotifier, privkeybytes, slotc, transactionc, bfr, donec)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = m.Run()
			close(deadc)
		}()

		respVal.slot = -1
		respB, _ := json.Marshal(respVal)
		invoker.InvokeReturns(respB, nil)
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

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		m := markend.New(invoker, slotnotifier, privkeybytes, slotc, transactionc, bfr, donec)

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

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		m := markend.New(invoker, slotnotifier, privkeybytes, slotc, transactionc, bfr, donec)

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
