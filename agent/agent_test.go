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
	t.Log(m[csv.IDs[0]][0])

	t.Run("signal registration errors", func(t *testing.T) {
		sdk := new(agentfakes.FakeSDKer)
		signal := new(agentfakes.FakeSignaler)
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], sdk, signal, make(chan struct{}), bfr)

		signal.RegisterReturns(false)

		var err error
		go func() {
			err = a.Run()
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Agent exited"))
		g.Eventually(err, "1s", "50ms").Should(HaveOccurred())
	})

	t.Run("close done chan", func(t *testing.T) {
		sdk := new(agentfakes.FakeSDKer)
		signal := new(agentfakes.FakeSignaler)
		doneChan := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], sdk, signal, doneChan, bfr)

		signal.RegisterReturns(true)

		var err error
		go func() {
			err = a.Run()
		}()

		close(doneChan)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Agent exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("signal works fine", func(t *testing.T) {
		sdk := new(agentfakes.FakeSDKer)
		signal := new(agentfakes.FakeSignaler)
		doneChan := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], sdk, signal, doneChan, bfr)

		signal.RegisterReturns(true)
		sdk.InvokeReturns(nil, nil)

		go func() {
			a.Run()
		}()

		a.SignalQueue <- uint64(0)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing row 0:"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'buy' for"))

		sdk.InvokeReturns(nil, errors.New("foo"))
		a.SignalQueue <- uint64(0)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'buy' for"))

		a.BuyQueue = nil
		a.SignalQueue <- uint64(0)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push row 0 to 'buy' queue"))

		close(doneChan)
	})
}
