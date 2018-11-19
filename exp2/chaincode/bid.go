package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) bid() pp.Response {
	var err error

	// Has this slot been marked already?
	k1 := []string{oc.slot, "markEnd"} // The partial composite key is: slot_number + "markEnd"
	iter, err := oc.stub.GetStateByPartialCompositeKey("", k1)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key %s: %s", oc.txID, k1, err.Error())
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
	k2, err := oc.stub.CreateCompositeKey("", attrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(k2, oc.data); err != nil {
		msg := fmt.Sprintf("[%s] Cannot write value for key %s: %s", oc.txID, k2, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var decrData []byte
	decrData, err = Decrypt(oc.data, keyPair)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot decrypt encoded payload: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var bidData []float64 // The expectation then is that `oc.data` is an encrypted pair of float64's.
	if err := json.Unmarshal(decrData, &bidData); err != nil {
		msg := fmt.Sprintf("[%s] Cannot unmarshal encoded payload: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	bid := Bid{
		PricePerUnit: bidData[0],
		Units:        bidData[1],
	}

	slotNum, _ := strconv.Atoi(oc.slot)

	// Add bid to bid collection
	switch oc.action {
	case "buy":
		bbMutex.Lock()
		buyerBids[slotNum] = append(buyerBids[slotNum], bid)
		// This allows us to only keep the last 100 slots in memory.
		// We do not need the previous one as they have already been settled.
		if deleteKey := (slotNum - BufferLen); deleteKey >= 0 {
			if _, ok := buyerBids[deleteKey]; ok {
				delete(buyerBids, deleteKey)
			}
		}
		bbMutex.Unlock()
	case "sell":
		sbMutex.Lock()
		sellerBids[slotNum] = append(sellerBids[slotNum], bid)
		if deleteKey := (slotNum - BufferLen); deleteKey >= 0 {
			if _, ok := buyerBids[deleteKey]; ok {
				delete(buyerBids, deleteKey)
			}
		}
		sbMutex.Unlock()
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

	return shim.Success([]byte(k2))
}
