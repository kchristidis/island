package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

// This is what we write in the chaincode's KV store.
type storeBlob struct {
	PrivKey    string
	PPU, Units float64
}

// This is what we return to the caller.
type respBlob struct {
	Message    string
	Slot       int
	PPU, Units float64
}

// Returns a marshalled `respBlob` object with the MCP for slot.
func (oc *opContext) markEnd() pp.Response {
	attrs := []string{oc.slot, oc.action, oc.txID}   // The composite key is: slot_number + action + tx_id
	wk, err := oc.stub.CreateCompositeKey("", attrs) // wk: write-key
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var storeVal storeBlob

	// Retrieve the markend regulator's private key
	storeVal.PrivKey = string(oc.data)

	// We have the private key, let's decrypt the bids and calculate the MCP.
	// ATTN: In a non-POC setting, we would persist this key to the ledger, and
	// have the clients do the decryption and calculate the clearing price locally.
	// In this POC we do persist the key to the ledger, *but* we perform the MCP
	// calculations on the chaincode.

	// Create the bid collections corresponding to that slot_number
	buyerBids := oc.newBidCollection("buy")
	sellerBids := oc.newBidCollection("sell")

	var respVal respBlob
	var respValB []byte

	slotNum, _ := strconv.Atoi(oc.slot)

	msg := fmt.Sprintf("[%s] Buyer bids for slot %d:", oc.txID, slotNum)
	fmt.Fprintln(os.Stdout, msg)
	if len(buyerBids) > 0 {
		for i, v := range buyerBids {
			fmt.Fprintf(os.Stdout, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(os.Stdout, "\tnone")
	}

	msg = fmt.Sprintf("[%s] Seller bids for slot %d:", oc.txID, slotNum)
	fmt.Fprintln(os.Stdout, msg)
	if len(sellerBids) > 0 {
		for i, v := range sellerBids {
			fmt.Fprintf(os.Stdout, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(os.Stdout, "\tnone")
	}

	// Settle the market for that slot
	if len(sellerBids) > 0 && len(buyerBids) > 0 {
		res, err := Settle(buyerBids, sellerBids)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot find clearing price for slot %s: %s",
				oc.txID, oc.slot, err.Error())
			fmt.Fprintln(os.Stdout, msg)
		} else {
			storeVal.PPU = res.PricePerUnit
			storeVal.Units = res.Units
			msg := fmt.Sprintf("[%s] %.6f kWh were cleared at %.3f ç/kWh in slot %s ✅",
				oc.txID, storeVal.Units, storeVal.PPU, oc.slot)
			fmt.Fprintln(os.Stdout, msg)

			respVal.Message = msg
			respVal.Slot = slotNum
			respVal.PPU = storeVal.PPU
			respVal.Units = storeVal.Units
		}
	} else {
		msg := fmt.Sprintf("No market for slot %d (buyer bids: %d, seller bids: %d) 😔",
			slotNum, len(buyerBids), len(sellerBids))
		fmt.Fprintln(os.Stdout, msg)

		respVal.Message = msg
		respVal.Slot = slotNum
	}

	storeValB, err := json.Marshal(storeVal)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal write-value for key %s: %s",
			oc.txID, wk, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(wk, storeValB); err != nil {
		msg := fmt.Sprintf("[%s] Cannot persist write-value for key %s: %s",
			oc.txID, wk, err.Error())
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

	respValB, err = json.Marshal(respVal)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal response: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success(respValB)
}

func (oc *opContext) newBidCollection(bidType string) (BidCollection, error) {
	var resp BidCollection

	bk := []string{oc.slot, bidType} // The partial composite key is: slot_nunber + bidType
	iter, err := oc.stub.GetStateByPartialCompositeKey("", bk)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key %s: %s", oc.txID, bk, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return nil, fmt.Errorf("%s", msg)
	}
	defer iter.Close()

	if !iter.HasNext() {
		msg := fmt.Sprintf("[%s] No values exist for partial key %s", oc.txID, bk)
		fmt.Fprintln(os.Stdout, msg)
		return nil, fmt.Errorf("%s", msg)
	}

	for iter.HasNext() {
		respRange, err := iter.Next()
		if err != nil {
			msg := fmt.Sprintf("[%s] Failed during iteration on key %s: %s", oc.txID, bk, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return nil, fmt.Errorf("%s", msg)
		}

		encBid := respRange.Value // This is the encrypted bid, i.e. `oc.data` in bid().

		var decBid []byte

		decBid, err = Decrypt(encBid, keyPair)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot decrypt encoded payload: %s", oc.txID, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return nil, fmt.Errorf("%s", msg)
		}

		var bidData []float64 // The expectation then is that `encBid` is an encrypted pair of float64's.

		if err := json.Unmarshal(decBid, &bidData); err != nil {
			msg := fmt.Sprintf("[%s] Cannot unmarshal encoded payload: %s", oc.txID, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return nil, fmt.Errorf("%s", msg)
		}

		bid := Bid{
			PricePerUnit: bidData[0],
			Units:        bidData[1],
		}

		slotNum, _ := strconv.Atoi(oc.slot)

		// Add bid to bid collection

		resp = append(resp, bid)

		msg := fmt.Sprintf("[%s] Added a bid to the collection for slot %d: %s", oc.txID, slotNum, bid)
		fmt.Fprintln(os.Stdout, msg)
	}

	return resp, nil
}
