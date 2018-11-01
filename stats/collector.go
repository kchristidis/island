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

// SlotStats ...
var (
	SlotStats       [TraceLength]Slot
	LargestSlotSeen int
)

// Block ...
type Block struct {
	Number   int
	SizeInKB float32
}

// BlockStats ...
var BlockStats []Block

// Transaction ...
type Transaction struct {
	ID              string
	Type            string
	Status          string
	LatencyInMillis int
}

// TransactionStats ...
var TransactionStats []Transaction

// Collector ...
type Collector struct {
	BlockChan       chan Block // Input channels for stat aggregation.
	SlotChan        chan Slot
	TransactionChan chan Transaction

	Writer io.Writer // Used for logging.

	DoneChan chan struct{} // An external kill switch.
}

// Run ...
func (c *Collector) Run() {
	defer fmt.Fprintln(c.Writer, "[stats collector] Exited")

	for {
		select {
		case newLine := <-c.TransactionChan:
			c.TransactionCalc(newLine, &TransactionStats)
		case newLine := <-c.BlockChan:
			c.BlockCalc(newLine, &BlockStats)
		case newLine := <-c.SlotChan:
			c.SlotCalc(newLine, &SlotStats)
		case <-c.DoneChan:
			close(c.BlockChan)
			for newLine := range c.BlockChan {
				c.BlockCalc(newLine, &BlockStats)
			}
			close(c.SlotChan)
			// Don't exit until you make sure that the slot chan is drained first
			for newLine := range c.SlotChan {
				c.SlotCalc(newLine, &SlotStats)
			}
			return
		}
	}
}

// TransactionCalc ...
func (c *Collector) TransactionCalc(newLine Transaction, aggStats *[]Transaction) {
	*aggStats = append(*aggStats, newLine)
}

// BlockCalc ...
func (c *Collector) BlockCalc(newLine Block, aggStats *[]Block) {
	*aggStats = append(*aggStats, newLine)
}

// SlotCalc ...
func (c *Collector) SlotCalc(newLine Slot, aggStats *[TraceLength]Slot) {
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
}
