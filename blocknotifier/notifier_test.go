package blocknotifier_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/kchristidis/island/blocknotifier"
	"github.com/kchristidis/island/blocknotifier/blocknotifierfakes"
	"github.com/kchristidis/island/stats"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestNotifier(t *testing.T) {
	g := NewGomegaWithT(t)

	blockc := make(chan stats.Block, 10) // A large enough buffer so that we don't have to worry about draining it.
	slotc := make(chan int)

	invoker := new(blocknotifierfakes.FakeInvoker)
	invoker.InvokeReturns(nil, nil)
	querier := new(blocknotifierfakes.FakeQuerier)
	querier.QueryBlockReturns(new(common.Block), nil)
	bfr := gbytes.NewBuffer()

	blocksperslot := 3
	clockperiod := 500 * time.Millisecond
	sleepduration := 10 * time.Millisecond
	startfromblock := uint64(10)

	t.Run("early block received", func(t *testing.T) {
		donec := make(chan struct{})

		n := blocknotifier.New(blocksperslot, clockperiod, sleepduration, startfromblock, blockc, slotc, invoker, querier, bfr, donec)

		block := common.Block{
			Header: &common.BlockHeader{
				Number: n.StartFromBlock - 1,
			},
		}
		querier.QueryBlockReturns(&block, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Consistently(func() int {
			select {
			case val := <-n.SlotChan:
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

		n := blocknotifier.New(blocksperslot, clockperiod, sleepduration, startfromblock, blockc, slotc, invoker, querier, bfr, donec)

		block := common.Block{
			Header: &common.BlockHeader{
				Number: n.StartFromBlock,
			},
		}
		querier.QueryBlockReturns(&block, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		// I expect to receive the slot notification

		g.Eventually(func() int {
			select {
			case val := <-n.SlotChan:
				return val
			default:
				return -1
			}
		}, "1s", "50ms").Should(Equal(0))

		close(donec)
		<-deadc

		g.Expect(err).ToNot(HaveOccurred())

	})

	t.Run("query fails", func(t *testing.T) {
		donec := make(chan struct{})

		n := blocknotifier.New(blocksperslot, clockperiod, sleepduration, startfromblock, blockc, slotc, invoker, querier, bfr, donec)

		querier.QueryBlockReturns(nil, errors.New("foo"))

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say("error"))

		close(donec)
		<-deadc
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("non-period block received", func(t *testing.T) {
		donec := make(chan struct{})

		n := blocknotifier.New(blocksperslot, clockperiod, sleepduration, startfromblock, blockc, slotc, invoker, querier, bfr, donec)

		block := common.Block{
			Header: &common.BlockHeader{
				Number: n.StartFromBlock + 1,
			},
		}
		querier.QueryBlockReturns(&block, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Consistently(func() int {
			select {
			case val := <-n.SlotChan:
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

		n := blocknotifier.New(blocksperslot, clockperiod, sleepduration, startfromblock, blockc, slotc, invoker, querier, bfr, donec)

		block := common.Block{
			Header: &common.BlockHeader{
				Number: n.StartFromBlock + uint64(blocksperslot),
			},
		}
		querier.QueryBlockReturns(&block, nil)

		var err error
		deadc := make(chan struct{})
		go func() {
			err = n.Run()
			close(deadc)
		}()

		g.Eventually(func() int {
			select {
			case val := <-n.SlotChan:
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
