package agent_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kchristidis/island/agent"
	"github.com/kchristidis/island/agent/agentfakes"
	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/stats"
	"github.com/kchristidis/island/trace"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
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
		invoker := new(agentfakes.FakeInvoker)
		slotnotifier := new(agentfakes.FakeNotifier)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		a := agent.New(invoker, slotnotifier, pubkey, m[trace.IDs[0]], trace.IDs[0], slotc, transactionc, bfr, donec)

		slotnotifier.RegisterReturns(false)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = a.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to register with slot notifier"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(agentfakes.FakeInvoker)
		slotnotifier := new(agentfakes.FakeNotifier)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		a := agent.New(invoker, slotnotifier, pubkey, m[trace.IDs[0]], trace.IDs[0], slotc, transactionc, bfr, donec)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = a.Run()
			close(deadc)
		}()

		close(donec)
		<-deadc

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(agentfakes.FakeInvoker)
		slotnotifier := new(agentfakes.FakeNotifier)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		a := agent.New(invoker, slotnotifier, pubkey, m[trace.IDs[0]], trace.IDs[0], slotc, transactionc, bfr, donec)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = a.Run()
			close(deadc)
		}()

		invoker.InvokeReturns(nil, nil)
		a.SlotQueue <- 0

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing row 0:"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'buy' for"))

		invoker.InvokeReturns(nil, errors.New("foo"))
		a.SlotQueue <- 1
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'buy' for"))

		a.BuyQueue = nil
		a.SlotQueue <- 2
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push row 2 to 'buy' queue"))

		close(donec)
		<-deadc

		g.Expect(err).To(HaveOccurred())
	})
}
