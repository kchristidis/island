package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

// Returns the composite write-key.
func (oc *opContext) bid() pp.Response {
	var err error

	// Has this slot been marked already?
	rk := []string{oc.slot, "markEnd"} // The partial composite key is: slot_number + "markEnd"
	iter, err := oc.stub.GetStateByPartialCompositeKey("", rk)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key %s: %s", oc.txID, rk, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
	defer iter.Close()

	if iter.HasNext() {
		msg := fmt.Sprintf("[%s] Slot %s has been marked already, aborting 🛑", oc.txID, oc.slot)
		fmt.Fprintln(os.Stdout, msg)

		// Update the relevant metrics
		slotNum, _ := strconv.Atoi(oc.slot)
		aggStats.LateTXsCount[slotNum]++
		switch oc.action {
		case "buy":
			aggStats.LateBuysCount[slotNum]++
		case "sell":
			aggStats.LateSellsCount[slotNum]++
		}

		return shim.Error(msg)
	}

	// Proceed as usual
	attrs := []string{oc.slot, oc.action, oc.txID} // The composite key is: slot_number + action + tx_id
	wk, err := oc.stub.CreateCompositeKey("", attrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(wk, oc.data); err != nil {
		msg := fmt.Sprintf("[%s] Cannot write value for key %s: %s", oc.txID, wk, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if EnableEvents {
		// Notify listeners that the event has been executed
		err = oc.stub.SetEvent(oc.eventID, nil)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return shim.Error(msg)
		}
	}

	return shim.Success([]byte(wk))
}
