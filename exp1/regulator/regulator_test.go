package regulator_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kchristidis/island/exp1/crypto"
	"github.com/kchristidis/island/exp1/regulator"
	"github.com/kchristidis/island/exp1/regulator/regulatorfakes"
	"github.com/kchristidis/island/exp1/stats"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

type resp struct {
	msg        string
	slot       int
	ppu, units float64
}

func TestRegulator(t *testing.T) {
	g := NewGomegaWithT(t)

	id := 1

	privkeypath := filepath.Join("..", "crypto", "priv.pem")
	privkey, err := crypto.LoadPrivate(privkeypath)
	require.NoError(t, err)
	privkeybytes := crypto.SerializePrivate(privkey)

	slotc := make(chan stats.Slot, 10)               // A large enough buffer so that we don't have to worry about draining it.
	transactionc := make(chan stats.Transaction, 10) // A large enough buffer so that we don't have to worry about draining it.

	t.Run("notifier registration fails", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(false)

		types := []string{regulator.MarkEnd, regulator.RevealKeys}

		for _, regType := range types {
			bfr := gbytes.NewBuffer()
			donec := make(chan struct{})

			r := regulator.New(
				invoker, slotnotifier,
				id, regType, privkeybytes,
				slotc, transactionc,
				bfr, donec,
			)

			var err error
			deadc := make(chan struct{})
			go func() {
				err = r.Run()
				close(deadc)
			}()

			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to register with slot notifier"))
			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))

			close(donec)
			<-deadc
			g.Expect(err).To(HaveOccurred())
		}
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		types := []string{regulator.MarkEnd, regulator.RevealKeys}

		for _, regType := range types {
			bfr := gbytes.NewBuffer()
			donec := make(chan struct{})

			r := regulator.New(
				invoker, slotnotifier,
				id, regType, privkeybytes,
				slotc, transactionc,
				bfr, donec,
			)

			var err error
			deadc := make(chan struct{})
			go func() {
				err = r.Run()
				close(deadc)
			}()

			close(donec)
			<-deadc

			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Exited"))
			g.Expect(err).ToNot(HaveOccurred())
		}
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)
		slot := 5
		respVal := resp{
			slot: slot,
		}
		respB, _ := json.Marshal(respVal)
		invoker.InvokeReturns(respB, nil)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		types := []string{regulator.MarkEnd, regulator.RevealKeys}

		for _, regType := range types {
			bfr := gbytes.NewBuffer()
			donec := make(chan struct{})

			r := regulator.New(
				invoker, slotnotifier,
				id, regType, privkeybytes,
				slotc, transactionc,
				bfr, donec,
			)

			var err error
			deadc := make(chan struct{})
			go func() {
				err = r.Run()
				close(deadc)
			}()

			r.SlotQueue <- slot

			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("Got notified that slot %d has arrived", slot)))
			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("Invoking @ slot %d", slot)))

			switch r.Type {
			case regulator.MarkEnd:
				g.Eventually(func() int {
					_, slot, _, _ := invoker.InvokeArgsForCall(0)
					return slot
				}, "1s", "50ms").Should(Equal(slot - 1))
			case regulator.RevealKeys:
				g.Eventually(func() int {
					_, slot, _, _ := invoker.InvokeArgsForCall(1)
					return slot
				}, "1s", "50ms").Should(Equal(slot))

			}

			close(donec)
			<-deadc
			g.Expect(err).NotTo(HaveOccurred())
		}
	})

	t.Run("invocation returns error", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)
		invoker.InvokeReturns(nil, errors.New("foo"))

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		slot := 5

		types := []string{regulator.MarkEnd, regulator.RevealKeys}

		for _, regType := range types {
			bfr := gbytes.NewBuffer()
			donec := make(chan struct{})

			r := regulator.New(
				invoker, slotnotifier,
				id, regType, privkeybytes,
				slotc, transactionc,
				bfr, donec,
			)

			var err error
			deadc := make(chan struct{})
			go func() {
				err = r.Run()
				close(deadc)
			}()

			r.SlotQueue <- slot

			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("Unable to invoke @ slot %d", slot)))

			close(donec)
			<-deadc
			g.Expect(err).To(HaveOccurred())
		}
	})

	t.Run("invocation fails", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		slot := 5

		types := []string{regulator.MarkEnd, regulator.RevealKeys}

		for _, regType := range types {
			bfr := gbytes.NewBuffer()
			donec := make(chan struct{})

			r := regulator.New(
				invoker, slotnotifier,
				id, regType, privkeybytes,
				slotc, transactionc,
				bfr, donec,
			)
			r.TaskQueue = nil

			var err error
			deadc := make(chan struct{})
			go func() {
				err = r.Run()
				close(deadc)
			}()

			r.SlotQueue <- slot

			g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("Unable to push notification of slot %d to task queue", slot)))

			close(donec)
			<-deadc
			g.Expect(err).To(HaveOccurred())
		}
	})
}
