package main

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
	"github.com/kchristidis/island/chaincode/schema"
)

// - Creates partial read-key: <slot_number>-<oc.args.Action>
// - Stores all values associated w/ that partial read-key to a slice of strings
// - Encodes slice of strings into JSON object
// - Returns JSON encoding
func (oc *opContext) slotvalues() pp.Response {
	keyAttrs := []string{strconv.Itoa(oc.args.Slot), "-", string(oc.args.Action)}
	iter, err := oc.Iter(keyAttrs)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer iter.Close()

	if !iter.HasNext() {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d • no values exist for partial key w/ attributes %s", oc.txID, oc.args.EventID, oc.args.Slot, keyAttrs)
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}

	var slotOutputVal schema.SlotOutput

	for iter.HasNext() {
		respRange, err := iter.Next()
		if err != nil {
			msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d • failed during iteration on key w/ attributes %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, keyAttrs, err.Error())
			fmt.Fprintln(w, msg)
			return shim.Error(msg)
		}
		item := respRange.Value
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d • adding tx_id:%s to the response payload for key w/ attributes %s", oc.txID, oc.args.EventID, oc.args.Slot, string(item), keyAttrs)
		fmt.Fprintln(w, msg)
		slotOutputVal.Values = append(slotOutputVal.Values, item)
	}

	slotOutputValB, err := oc.Marshal(slotOutputVal)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(slotOutputValB)
}
