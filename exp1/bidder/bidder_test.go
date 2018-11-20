package bidder_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kchristidis/island/exp1/bidder"
	"github.com/kchristidis/island/exp1/bidder/bidderfakes"
	"github.com/kchristidis/island/exp1/crypto"
	"github.com/kchristidis/island/exp1/stats"
	"github.com/kchristidis/island/exp1/trace"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestBidder(t *testing.T) {
	g := NewGomegaWithT(t)

	path := filepath.Join("..", "trace", trace.Filename)
	m, err := trace.Load(path)
	require.NoError(t, err)

	pubkeypath := filepath.Join("..", "crypto", "pub.pem")
	pubkey, err := crypto.LoadPublic(pubkeypath)
	require.NoError(t, err)

	slotc := make(chan stats.Slot, 10)               // A large enough buffer so that we don't have to worry about draining it.
	transactionc := make(chan stats.Transaction, 10) // A large enough buffer so that we don't have to worry about draining it.

	t.Run("notifier registration fails", func(t *testing.T) {
		invoker := new(bidderfakes.FakeInvoker)

		slotnotifier := new(bidderfakes.FakeNotifier)
		slotnotifier.RegisterReturns(false)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		b := bidder.New(invoker, slotnotifier, pubkey, m[trace.IDs[0]], trace.IDs[0], slotc, transactionc, bfr, donec)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = b.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to register with slot notifier"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(bidderfakes.FakeInvoker)

		slotnotifier := new(bidderfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		b := bidder.New(invoker, slotnotifier, pubkey, m[trace.IDs[0]], trace.IDs[0], slotc, transactionc, bfr, donec)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = b.Run()
			close(deadc)
		}()

		close(donec)
		<-deadc

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(bidderfakes.FakeInvoker)

		slotnotifier := new(bidderfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		slot := 1

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		b := bidder.New(invoker, slotnotifier, pubkey, m[trace.IDs[0]], trace.IDs[0], slotc, transactionc, bfr, donec)

		// var err error
		deadc := make(chan struct{})
		go func() {
			err = b.Run()
			close(deadc)
		}()

		invoker.InvokeReturns(nil, nil)
		b.SlotQueue <- slot

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("slot %d has arrived", slot)))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'buy' for"))

		/* invoker.InvokeReturns(nil, errors.New("foo"))
		b.SlotQueue <- slot + 1
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'buy' for"))

		b.BuyQueue = nil
		b.SlotQueue <- slot + 1
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("Unable to push row %d to 'buy' queue", slot+1+1))) */

		close(donec)
		<-deadc

		// g.Expect(err).To(HaveOccurred())
	})
}
