package regulator_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/crypto"
	"github.com/kchristidis/island/regulator"
	"github.com/kchristidis/island/regulator/regulatorfakes"
	"github.com/kchristidis/island/stats"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/require"

	. "github.com/onsi/gomega"
)

func TestRegulator(t *testing.T) {
	g := NewGomegaWithT(t)

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

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		r := regulator.New(
			invoker, slotnotifier,
			privkeybytes,
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
	})

	t.Run("done chan closes", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		r := regulator.New(
			invoker, slotnotifier,
			privkeybytes,
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
	})

	t.Run("notifier works fine", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)
		slot := 5
		markendOutputVal := schema.MarkEndOutput{
			Slot: slot,
		}
		markendOutputValB, _ := json.Marshal(markendOutputVal)
		invoker.InvokeReturns(markendOutputValB, nil)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		r := regulator.New(
			invoker, slotnotifier,
			privkeybytes,
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
		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(fmt.Sprintf("Invoking 'markEnd' @ slot %d", slot)))

		g.Eventually(func() int {
			args := invoker.InvokeArgsForCall(0)
			return args.Slot
		}, "1s", "50ms").Should(Equal(slot - 1))

		close(donec)
		<-deadc
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("invocation returns error", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)
		invoker.InvokeReturns(nil, errors.New("foo"))

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		slot := 5

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		r := regulator.New(
			invoker, slotnotifier,
			privkeybytes,
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
	})

	t.Run("invocation fails", func(t *testing.T) {
		invoker := new(regulatorfakes.FakeInvoker)

		slotnotifier := new(regulatorfakes.FakeNotifier)
		slotnotifier.RegisterReturns(true)

		slot := 5

		bfr := gbytes.NewBuffer()
		donec := make(chan struct{})

		r := regulator.New(
			invoker, slotnotifier,
			privkeybytes,
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
	})
}
