package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// ErrNoPrice is returned when no equilibrium price can be found.
var ErrNoPrice = errors.New("cannot find a price that clears the market")

// Group is used to sort bidders into buyers and sellers.
type Group int

const (
	// Buyers place bids with a maximum buy price per unit.
	Buyers Group = iota
	// Sellers place bids with a minimum sell price per unit.
	Sellers
)

// Bid defines the number of units and the maximum/minimum
// per-unit price a buyer/seller is willing to pay/receive.
type Bid struct {
	PricePerUnit float64
	Units        float64
}

// Bid satisfies the fmt.Stringer interface.
func (b Bid) String() string {
	return fmt.Sprintf("%f units at a price of %f per unit", b.Units, b.PricePerUnit)
}

// BidCollection collects all the Bids placed by a group (buyers or sellers).
type BidCollection []Bid

// BidCollection satisfies the sort.Interface interface.
func (bc BidCollection) Len() int {
	return len(bc)
}

// BidCollection satisfies the sort.Interface interface.
func (bc BidCollection) Swap(i, j int) {
	bc[i], bc[j] = bc[j], bc[i]
}

// BidCollection satisfies the sort.Interface interface.
func (bc BidCollection) Less(i, j int) bool {
	return bc[i].PricePerUnit < bc[j].PricePerUnit
}

// Settle determines the clearing price and the number of units that can be
// traded, given a collection of bids from buyers and sellers. It returns an
// error if no equilibrium price can be found.
func Settle(buyers, sellers BidCollection) (Bid, error) {
	sb := Stack(buyers, Buyers)
	ss := Stack(sellers, Sellers)

	// If the highest buying price point is lower than the lowest selling price point, return error.
	if sb[len(sb)-1].PricePerUnit < ss[len(ss)-1].PricePerUnit {
		return Bid{PricePerUnit: 0.0, Units: 0.0}, ErrNoPrice
	}

	// We will definitely have a trade.
	var result []Bid

	// Remember that we can trade when sb[i] >= sb[j].
	for i := range sb { // We start with the lowest price point for buyers.
		for j := range ss { // We start with the highest price point for sellers.
			if sb[i].PricePerUnit < ss[j].PricePerUnit {
				continue
			}
			result = append(result, Bid{PricePerUnit: (sb[i].PricePerUnit + ss[j].PricePerUnit) / 2, Units: math.Min(sb[i].Units, ss[j].Units)})
		}
	}

	if len(result) == 1 {
		return result[0], nil
	}

	// Optimize for number of tradeable aunits.
	best := result[0]

	for i := 1; i < len(result); i++ {
		if result[i].Units > best.Units {
			best = result[i]
			continue
		} else if result[i].Units == best.Units {
			// Satisfy the Economic Efficiency property: the items should be
			// in the hands of those that value them the most.
			if result[i].PricePerUnit > best.PricePerUnit {
				best = result[i]
				continue
			}
		}
	}

	return best, nil
}

// Stack returns the number of units that would be settled at each price point of a given bid
// collection, *if* the overall supply (in the case of buyers) or demand (in the case of sellers)
// was adequate. The result is sorted so that that number of tradeable units decreases monotonically.
func Stack(bc BidCollection, groupType Group) BidCollection {
	switch groupType {
	case Buyers:
		// Sort the buyers in decreasing order of their bid.
		// As the bid price decreases, the number of requested units for purchase increases.
		sort.Sort(sort.Reverse(bc))
	case Sellers:
		// Sort the sellers in increasing order of their bid.
		// As the bid price increases, the number of offered units for sale increases.
		sort.Sort(bc)
	}

	bids := make([]Bid, bc.Len())

	for i := range bc {
		bids[i].PricePerUnit = bc[i].PricePerUnit
		if i == 0 {
			bids[i].Units = bc[i].Units
			continue
		}
		bids[i].Units = bids[i-1].Units + bc[i].Units
	}

	result := BidCollection(bids)

	// This sort effectively amounts to ordering by decreasing number of tradeable units.
	switch groupType {
	case Buyers:
		sort.Sort(result)
	case Sellers:
		sort.Sort(sort.Reverse(result))
	}

	return result
}
