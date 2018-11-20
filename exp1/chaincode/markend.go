package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) markEnd() pp.Response {
	attrs := []string{oc.slot, oc.action, oc.txID} // The composite key is: slot_number + action + tx_id
	key, err := oc.stub.CreateCompositeKey("", attrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	// This is what we write in the chaincode's KV store.
	var writeVal struct {
		PrivKey    string
		PPU, Units float64
	}

	// ATTN: In a non-POC setting, we would persist this key to the ledger, and
	// have the clients do the decryption and calculate the clearing price locally.
	// In this POC we persist the key to the ledger, but also perform the MCP
	// calculations on the chaincode.
	writeVal.PrivKey = string(oc.data)

	// This is what we return to the caller.
	var respVal struct {
		Msg        string
		Slot       int
		PPU, Units float64
	}

	var respB []byte

	slotNum, _ := strconv.Atoi(oc.slot)

	bbMutex.RLock()
	defer bbMutex.RUnlock()
	sbMutex.RLock()
	defer sbMutex.RUnlock()

	msg := fmt.Sprintf("[%s] Buyer bids for slot %d:", oc.txID, slotNum)
	fmt.Fprintln(os.Stdout, msg)
	if len(buyerBids[slotNum]) > 0 {
		for i, v := range buyerBids[slotNum] {
			fmt.Fprintf(os.Stdout, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(os.Stdout, "\tnone")
	}

	msg = fmt.Sprintf("[%s] Seller bids for slot %d:", oc.txID, slotNum)
	fmt.Fprintln(os.Stdout, msg)
	if len(sellerBids[slotNum]) > 0 {
		for i, v := range sellerBids[slotNum] {
			fmt.Fprintf(os.Stdout, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(os.Stdout, "\tnone")
	}

	// Settle the market for that slot
	if len(sellerBids[slotNum]) > 0 && len(buyerBids[slotNum]) > 0 {
		res, err := Settle(buyerBids[slotNum], sellerBids[slotNum])
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot find clearing price for slot %s: %s", oc.txID, oc.slot, err.Error())
			fmt.Fprintln(os.Stdout, msg)
		} else {
			writeVal.PPU = res.PricePerUnit
			writeVal.Units = res.Units
			msg := fmt.Sprintf("[%s] %.6f kWh were cleared at %.3f ç/kWh in slot %s ✅", oc.txID, writeVal.Units, writeVal.PPU, oc.slot)
			fmt.Fprintln(os.Stdout, msg)

			respVal.Msg = msg
			respVal.Slot = slotNum
			respVal.PPU = writeVal.PPU
			respVal.Units = writeVal.Units
		}
	} else {
		msg := fmt.Sprintf("No market for slot %d (buyer bids: %d, seller bids: %d) 😔", slotNum, len(buyerBids[slotNum]), len(sellerBids[slotNum]))
		fmt.Fprintln(os.Stdout, msg)

		respVal.Msg = msg
		respVal.Slot = slotNum
	}

	writeBytes, err := json.Marshal(writeVal)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal write-value for key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(key, writeBytes); err != nil {
		msg := fmt.Sprintf("[%s] Cannot persist write-value for key %s: %s", oc.txID, key, err.Error())
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

	respB, err = json.Marshal(respVal)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal response: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success(respB)
}
