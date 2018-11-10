package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) querySlot() pp.Response {
	key := []string{oc.slot, string(oc.action)} // The partial composite key is: slot_number + action
	iter, err := oc.stub.GetStateByPartialCompositeKey("", key)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
	defer iter.Close()

	if !iter.HasNext() {
		msg := fmt.Sprintf("[%s] No values exist for partial key %s", oc.txID, key)
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var resp []string
	for iter.HasNext() {
		respRange, err := iter.Next()
		if err != nil {
			msg := fmt.Sprintf("[%s] Failed during iteration on key %s: %s", oc.txID, key, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return shim.Error(msg)
		}
		item := string(respRange.Value)
		msg := fmt.Sprintf("[%s] Adding [%s] to the response payload for key %s", oc.txID, item, key)
		fmt.Fprintln(os.Stdout, msg)
		resp = append(resp, item)
	}

	respB, err := json.Marshal(resp)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal response: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success(respB)
}
