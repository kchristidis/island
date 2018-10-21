package signal

import (
	"sync"
	"testing"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/gomega"
)

func TestSignal(t *testing.T) {
	g := NewGomegaWithT(t)
	bfr := gbytes.NewBuffer()
	clc := make(chan struct{})
	src := make(chan uint64)

	s := &Signal{
		Source:  src,
		Close:   clc,
		Cond:    sync.NewCond(&sync.Mutex{}),
		LastVal: 0,
		Out:     bfr,
	}

	go s.Run()

	finalVal := uint64(1)

	go func() {
		src <- finalVal
	}()

	g.Eventually(func() uint64 {
		return s.LastVal
	}, "1s", "50ms").Should(Equal(finalVal))

	close(clc)

	g.Eventually(bfr, "1s", "50ms").Should(gbytes.Say(ExitMsg))
}
