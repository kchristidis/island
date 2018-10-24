package agent_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kchristidis/exp2/agent"
	"github.com/kchristidis/exp2/agent/agentfakes"
	"github.com/kchristidis/exp2/csv"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestAgent(t *testing.T) {
	g := NewGomegaWithT(t)

	path := filepath.Join("..", "csv", csv.Filename)
	m, err := csv.Load(path)
	require.NoError(t, err)

	t.Run("notifier registration fails", func(t *testing.T) {
		invoker := new(agentfakes.FakeInvoker)
		slotnotifier := new(agentfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(false)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = a.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("unable to register with signaler"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("exited"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(agentfakes.FakeInvoker)
		slotnotifier := new(agentfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = a.Run()
			close(deadc)
		}()

		close(donec)
		<-deadc

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(agentfakes.FakeInvoker)
		slotnotifier := new(agentfakes.FakeNotifier)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], invoker, slotnotifier, donec, bfr)

		slotnotifier.RegisterReturns(true)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = a.Run()
			close(deadc)
		}()

		invoker.InvokeReturns(nil, nil)
		a.SlotQueue <- 0

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("processing row 0:"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("invoking 'buy' for"))

		invoker.InvokeReturns(nil, errors.New("foo"))
		a.SlotQueue <- 1
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("unable to invoke 'buy' for"))

		a.BuyQueue = nil
		a.SlotQueue <- 2
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("unable to push row 2 to 'buy' queue"))

		close(donec)
		<-deadc

		g.Expect(err).To(HaveOccurred())
	})
}
