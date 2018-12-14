package main

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
	"github.com/kchristidis/island/chaincode/schema"
)

// - Looks up read-key <slot_number>-<markend> and returns error if read-key is found
// - Experiment 1:
//		a. If write-key <slot_number>-<action> is not found, creates map[string][]byte
//			as value for that write-key, and persists encrypted bid (encrypted JSON
//			`BidInput` object) to map[bidder_id].
//		b. If write-key <slot_number>-<action> is found, it persists encrypted bid
//			(encrypted JSON `BidInput` object) to map[bidder_id] (the value for the
//			write-key will be a map).
// - Experiments 2, 3:
//		a. Creates write-key <slot_number>-<action>-<tx_id> for experiments 2, 3
//		b. Persists encrypted bid (encrypted JSON `BidInput` object) to write-key
func (oc *opContext) bid() pp.Response {
	valB, err := oc.Get([]string{strconv.Itoa(oc.args.Slot), "-", "markEnd"})
	if err != nil {
		return shim.Error(err.Error())
	}
	if valB != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • slot marked already, aborting 'bid' 🛑", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action)
		fmt.Fprintln(w, msg)
		metricsOutputVal.LateTXsCount[oc.args.Slot]++
		switch oc.args.Action {
		case "buy":
			metricsOutputVal.LateBuysCount[oc.args.Slot]++
		case "sell":
			metricsOutputVal.LateSellsCount[oc.args.Slot]++
		}
		return shim.Error(msg)
	}

	// The slot has not been marked
	// Let's proceed as usual in order to post the bid

	var keyAttrs []string

	switch schema.ExpNum {
	case 1:
		// Does the key to which we wish to write exist already?
		keyAttrs = []string{strconv.Itoa(oc.args.Slot), "-", oc.args.Action}
		var val map[string][]byte
		valB, err := oc.Get(keyAttrs)
		if err != nil {
			return shim.Error(err.Error())
		}
		if valB != nil {
			// This is the case where a JSON-encoded map exists
			if err := oc.Unmarshal(valB, &val); err != nil {
				return shim.Error(err.Error())
			}
		} else {
			val = make(map[string][]byte)
		}
		// This is the common path for both cases:
		// 1. When no value has been persisted under that key before, or
		// 2. When a value has been persisted before
		val[oc.args.EventID] = oc.args.Data
		newValB, err := oc.Marshal(val)
		if err != nil {
			return shim.Error(err.Error())
		}
		if err := oc.Put(keyAttrs, newValB); err != nil {
			return shim.Error(err.Error())
		}
	case 2, 3:
		// The composite key is: slot_number-action-tx_id
		keyAttrs = []string{strconv.Itoa(oc.args.Slot), "-", oc.args.Action, "-", oc.txID}
		if err := oc.Put(keyAttrs, oc.args.Data); err != nil {
			return shim.Error(err.Error())
		}
	}

	bidOutputVal := schema.BidOutput{
		WriteKeyAttrs: keyAttrs,
	}

	bidOutputValB, err := oc.Marshal(&bidOutputVal)
	if err != nil {
		return shim.Error(err.Error())
	}

	if err := oc.Event(); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(bidOutputValB)
}
