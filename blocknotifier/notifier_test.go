package blocknotifier_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/kchristidis/exp2/blocknotifier"
	"github.com/kchristidis/exp2/blocknotifier/blocknotifierfakes"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestNotifier(t *testing.T) {
	g := NewGomegaWithT(t)

	bfr := gbytes.NewBuffer()
	invoker := new(blocknotifierfakes.FakeInvoker)
	querier := new(blocknotifierfakes.FakeQuerier)
	resp := new(fab.BlockchainInfoResponse)

	outc := make(chan int)
	invoker.InvokeReturns(nil, nil)
	resp.BCI = new(common.BlockchainInfo)

	t.Run("early block received", func(t *testing.T) {
		donec := make(chan struct{})

		n := &blocknotifier.Notifier{
			BlocksPerSlot:  1,
			ClockPeriod:    500 * time.Millisecond,
			DoneChan:       donec,
			Invoker:        invoker,
			OutChan:        outc,
			Querier:        querier,
			SleepDuration:  10 * time.Millisecond,
			StartFromBlock: 10,
			Writer:         bfr,
		}

		resp.BCI.Height = n.StartFromBlock - 1
		querier.QueryInfoReturns(resp, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Consistently(func() int {
			select {
			case val := <-n.OutChan:
				return val
			default:
				return -1
			}
		}, "1s", "50ms").Should(Equal(-1))

		close(donec)
		<-deadc

		g.Expect(err).ToNot(HaveOccurred())

	})

	t.Run("start block received", func(t *testing.T) {
		donec := make(chan struct{})

		n := &blocknotifier.Notifier{
			BlocksPerSlot:  1,
			ClockPeriod:    500 * time.Millisecond,
			DoneChan:       donec,
			Invoker:        invoker,
			OutChan:        outc,
			Querier:        querier,
			SleepDuration:  10 * time.Millisecond,
			StartFromBlock: 10,
			Writer:         bfr,
		}

		resp.BCI.Height = n.StartFromBlock
		querier.QueryInfoReturns(resp, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		// I expect to reveive the slot notification

		g.Eventually(func() int {
			select {
			case val := <-n.OutChan:
				return val
			default:
				return -1
			}
		}, "1s", "50ms").Should(Equal(0))

		close(donec)
		<-deadc

		g.Expect(err).ToNot(HaveOccurred())

	})

	t.Run("first block received is larger than start", func(t *testing.T) {
		donec := make(chan struct{})

		n := &blocknotifier.Notifier{
			BlocksPerSlot:  1,
			ClockPeriod:    500 * time.Millisecond,
			DoneChan:       donec,
			Invoker:        invoker,
			OutChan:        outc,
			Querier:        querier,
			SleepDuration:  10 * time.Millisecond,
			StartFromBlock: 10,
			Writer:         bfr,
		}

		resp.BCI.Height = n.StartFromBlock + 1
		querier.QueryInfoReturns(resp, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Consistently(func() int {
			select {
			case val := <-n.OutChan:
				return val
			default:
				return -1
			}
		}, "1s", "50ms").Should(Equal(-1))

		g.Expect(err).To(HaveOccurred())

		close(donec)
		<-deadc
	})

	t.Run("query fails", func(t *testing.T) {
		donec := make(chan struct{})

		n := &blocknotifier.Notifier{
			BlocksPerSlot:  1,
			ClockPeriod:    500 * time.Millisecond,
			DoneChan:       donec,
			Invoker:        invoker,
			OutChan:        outc,
			Querier:        querier,
			SleepDuration:  10 * time.Millisecond,
			StartFromBlock: 10,
			Writer:         bfr,
		}

		querier.QueryInfoReturns(nil, errors.New("foo"))

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("Unable to query ledger:"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("non-period block received", func(t *testing.T) {
		donec := make(chan struct{})

		n := &blocknotifier.Notifier{
			BlocksPerSlot:  3,
			ClockPeriod:    500 * time.Millisecond,
			DoneChan:       donec,
			Invoker:        invoker,
			OutChan:        outc,
			Querier:        querier,
			SleepDuration:  10 * time.Millisecond,
			StartFromBlock: 10,
			Writer:         bfr,
		}

		n.LastHeight = n.StartFromBlock

		resp.BCI.Height = n.StartFromBlock + 1
		querier.QueryInfoReturns(resp, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Consistently(func() int {
			select {
			case val := <-n.OutChan:
				return val
			default:
				return -1
			}
		}, "1s", "50ms").Should(Equal(-1))

		close(donec)
		<-deadc

		g.Expect(err).ToNot(HaveOccurred())

	})

	t.Run("period block received", func(t *testing.T) {
		donec := make(chan struct{})

		n := &blocknotifier.Notifier{
			BlocksPerSlot:  3,
			ClockPeriod:    500 * time.Millisecond,
			DoneChan:       donec,
			Invoker:        invoker,
			OutChan:        outc,
			Querier:        querier,
			SleepDuration:  10 * time.Millisecond,
			StartFromBlock: 10,
			Writer:         bfr,
		}

		n.LastHeight = n.StartFromBlock

		resp.BCI.Height = n.StartFromBlock + uint64(n.BlocksPerSlot)
		querier.QueryInfoReturns(resp, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Eventually(func() int {
			select {
			case val := <-n.OutChan:
				return val
			default:
				return -1
			}
		}, "1s", "50ms").Should(Equal(1))

		close(donec)
		<-deadc

		g.Expect(err).ToNot(HaveOccurred())

	})
}
