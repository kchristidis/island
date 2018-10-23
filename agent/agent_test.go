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

	t.Run("slotter registration fails", func(t *testing.T) {
		sdk := new(agentfakes.FakeSDKer)
		slotsrc := new(agentfakes.FakeSlotter)
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], sdk, slotsrc, make(chan struct{}), bfr)

		slotsrc.RegisterReturns(false)

		var err error
		go func() {
			err = a.Run()
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Agent exited"))
		g.Eventually(err, "1s", "50ms").Should(HaveOccurred())
	})

	t.Run("done chan closes", func(t *testing.T) {
		sdk := new(agentfakes.FakeSDKer)
		slotsrc := new(agentfakes.FakeSlotter)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], sdk, slotsrc, donec, bfr)

		slotsrc.RegisterReturns(true)

		var err error
		go func() {
			err = a.Run()
		}()

		close(donec)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Agent exited"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("slotter works fine", func(t *testing.T) {
		sdk := new(agentfakes.FakeSDKer)
		slotsrc := new(agentfakes.FakeSlotter)
		donec := make(chan struct{})
		bfr := gbytes.NewBuffer()

		a := agent.New(csv.IDs[0], m[csv.IDs[0]], sdk, slotsrc, donec, bfr)

		slotsrc.RegisterReturns(true)
		sdk.InvokeReturns(nil, nil)

		go func() {
			a.Run()
		}()

		a.SlotQueue <- uint64(0)

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Processing row 0:"))
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Invoking 'buy' for"))

		sdk.InvokeReturns(nil, errors.New("foo"))
		a.SlotQueue <- uint64(1)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to invoke 'buy' for"))

		a.BuyQueue = nil
		a.SlotQueue <- uint64(2)
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to push row 2 to 'buy' queue"))

		close(donec)
	})
}
