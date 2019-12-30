package bidder_test

import (
	"path/filepath"
	"testing"

	"github.com/kchristidis/island/bidder"
	"github.com/kchristidis/island/bidder/bidderfakes"
	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/stats"
	"github.com/kchristidis/island/trace"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestBidder(t *testing.T) {
	g := NewGomegaWithT(t)

	path := filepath.Join("..", "trace", trace.Filename)
	m, err := trace.Load(path)
	require.NoError(t, err)

	privkeypath := filepath.Join("..", "crypto", "priv.pem")
	privkey, err := crypto.LoadPrivate(privkeypath)
	require.NoError(t, err)
	privkeybytes := crypto.SerializePrivate(privkey)

	slotc := make(chan stats.Slot, 10)               // A large enough buffer so that we don't have to worry about draining it.
	transactionc := make(chan stats.Transaction, 10) // A large enough buffer so that we don't have to worry about draining it.

	t.Run("notifier registration fails", func(t *testing.T) {
		invoker := new(bidderfakes.FakeInvoker)

		slotnotifier0 := new(bidderfakes.FakeNotifier)
		slotnotifier0.RegisterReturns(false)
		slotnotifier1 := new(bidderfakes.FakeNotifier)
		slotnotifier1.RegisterReturns(false)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		b := bidder.New(invoker, slotnotifier0, slotnotifier1, trace.IDs[0], privkeybytes, m[trace.IDs[0]], slotc, transactionc, bfr, donec)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = b.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("cannot register with slot notifier"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("exited"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(bidderfakes.FakeInvoker)

		slotnotifier0 := new(bidderfakes.FakeNotifier)
		slotnotifier0.RegisterReturns(true)
		slotnotifier1 := new(bidderfakes.FakeNotifier)
		slotnotifier1.RegisterReturns(true)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		b := bidder.New(invoker, slotnotifier0, slotnotifier1, trace.IDs[0], privkeybytes, m[trace.IDs[0]], slotc, transactionc, bfr, donec)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = b.Run()
			close(deadc)
		}()

		close(donec)
		<-deadc

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(bidderfakes.FakeInvoker)

		slotnotifier0 := new(bidderfakes.FakeNotifier)
		slotnotifier0.RegisterReturns(true)
		slotnotifier1 := new(bidderfakes.FakeNotifier)
		slotnotifier1.RegisterReturns(true)

		slot := 1

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		b := bidder.New(invoker, slotnotifier0, slotnotifier1, trace.IDs[0], privkeybytes, m[trace.IDs[0]], slotc, transactionc, bfr, donec)

		// var err error
		deadc := make(chan struct{})
		go func() {
			err = b.Run()
			close(deadc)
		}()

		invoker.InvokeReturns(nil, nil)
		b.SlotQueues[0] <- slot

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("new slot"))

		close(donec)
		<-deadc
	})
}
