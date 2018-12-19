package stats

import (
	"fmt"
	"io"

	"github.com/kchristidis/island/chaincode/schema"
)

// We are collecting stats on three different keys:
// - eventID: type (string) | status (string) | latency in ms (int)
// - blockNum: fileSize (int)
// - slotNum: energy used (floa64) | hi (float64) | energy generated (float64) |  lo (float64) | energy traded (float64) |  ppu_traded (float64)

// Transaction ...
type Transaction struct {
	ID              string
	Type            string
	Status          string
	LatencyInMillis int64
	Attempt         int
}

// TransactionStats ...
var TransactionStats []Transaction

// Block ...
type Block struct {
	Number uint64
	Size   float32 // In KiB
}

// BlockStats ...
var BlockStats []Block

// Slot ...
type Slot struct {
	Number       int
	EnergyUse    float64
	PricePaid    float64
	EnergyGen    float64
	PriceSold    float64
	EnergyTraded float64
	PriceTraded  float64
}

// SlotStats ...
var (
	SlotStats       [schema.TraceLength]Slot
	LargestSlotSeen int
)

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
	defer fmt.Fprintln(c.Writer, "stats • exited")

	for {
		select {
		case newLine := <-c.TransactionChan:
			c.TransactionCalc(newLine, &TransactionStats)
		case newLine := <-c.BlockChan:
			c.BlockCalc(newLine, &BlockStats)
		case newLine := <-c.SlotChan:
			c.SlotCalc(newLine, &SlotStats)
		case <-c.DoneChan:
			// Don't exit until you make sure that the channels are drained first
			close(c.BlockChan)
			for newLine := range c.BlockChan {
				c.BlockCalc(newLine, &BlockStats)
			}
			close(c.TransactionChan)
			for newLine := range c.TransactionChan {
				c.TransactionCalc(newLine, &TransactionStats)
			}
			close(c.SlotChan)
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
func (c *Collector) SlotCalc(newLine Slot, aggStats *[schema.TraceLength]Slot) {
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

		if curLine.PricePaid < newLine.PricePaid {
			curLine.PricePaid = newLine.PricePaid
		}

		if curLine.PriceSold < newLine.PriceSold {
			curLine.PriceSold = newLine.PriceSold
		}

		curLine.EnergyTraded = newLine.EnergyTraded
		curLine.PriceTraded = newLine.PriceTraded
		curLine.EnergyUse -= newLine.EnergyTraded
		curLine.EnergyGen -= newLine.EnergyTraded

		(*aggStats)[slotNum] = curLine
	}
}
