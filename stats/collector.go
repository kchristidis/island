package stats

import (
	"fmt"
	"io"
)

// We are collecting stats on three different keys:
// - slotNum: energy used (floa64) | hi (float64) | energy generated (float64) |  lo (float64)
// - blockNum: fileSize (int)
// - txID: type (string) | latency in ms (int) | status (string)

// TraceLength ...
const TraceLength = 35036

// Slot ...
type Slot struct {
	Number    int
	EnergyUse float64
	PricePaid float64
	EnergyGen float64
	PriceSold float64
}

//
var (
	SlotStats       [TraceLength]Slot
	LargestSlotSeen int
)

// Block ...
type Block struct {
	Number int
	Size   int
}

// BlockStats ...
var BlockStats []Block

// Collector ...
type Collector struct {
	BlockChan chan Block    // Input
	DoneChan  chan struct{} // Input
	SlotChan  chan Slot     // Input
	Writer    io.Writer
}

// Run ...
func (c *Collector) Run() {
	defer fmt.Fprintln(c.Writer, "[stats collector] Exited")

	for {
		select {
		case newLine := <-c.SlotChan:
			c.SlotCalc(newLine, &SlotStats)
		case <-c.DoneChan:
			close(c.SlotChan)
			// Don't exit until you make sure that the slot chan is drained first
			for newLine := range c.SlotChan {
				c.SlotCalc(newLine, &SlotStats)
			}
			return
		}
	}
}

// SlotCalc ...
func (c *Collector) SlotCalc(newLine Slot, aggStats *[TraceLength]Slot) {
	/* msg := fmt.Sprintf(">>> About to consider slot %d: %.3f kWh used @ %.3f ç/kWh, %.3f kWh sold @ %.3f ç/kWh",
		newLine.Number, newLine.EnergyUse, newLine.PricePaid, newLine.EnergyGen, newLine.PriceSold)
	fmt.Fprintln(c.Writer, msg) */
	slotNum := newLine.Number
	if slotNum > LargestSlotSeen {
		LargestSlotSeen = slotNum
	}
	if ((*aggStats)[slotNum] == Slot{}) {
		(*aggStats)[slotNum] = newLine
	} else {
		curLine := aggStats[slotNum]
		curLine.EnergyUse += newLine.EnergyUse
		curLine.EnergyGen += newLine.EnergyGen
		(*aggStats)[slotNum] = curLine
	}
	/* msg = fmt.Sprintf(">>> At the end of this Calc run, the aggregate stats for slot %d look like this: %v", slotNum, SlotStats[slotNum])
	fmt.Fprintln(c.Writer, msg) */
}
