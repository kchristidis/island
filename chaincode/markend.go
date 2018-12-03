package main

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
	"github.com/kchristidis/island/chaincode/schema"
)

// Marks the end of a slot.
// - In case of experiment 3, deserializes the private key in `oc.args.Data`
// - In case of experiments 1 or 2, retrieves the private keys for `oc.args.Slot`
// 	 	posted in the chaincode's KV store
// - Decodes the posted bids
// - Creates a bid collection for buyers and sellers for slot`oc.args.Slot`
// - Calculates the MCP for `oc.args.Slot`
// - Creates write-key <slot_number>-<markend>-<tx_id>
// - Writes JSON-encoded `schema.MarkEndOutput` to write-key
func (oc *opContext) markEnd() pp.Response {
	keyAttrs := []string{strconv.Itoa(oc.args.Slot), "-", oc.args.Action, "-", oc.txID}

	markEndOutputVal := schema.MarkEndOutput{
		WriteKeyAttrs: keyAttrs,
		Slot:          oc.args.Slot,
	}

	var keyPair *rsa.PrivateKey
	var err error

	if schema.ExpNum == 2 {
		// Retrieve the markend regulator's private key
		var markEndInputVal schema.MarkEndInput
		if err := oc.Unmarshal(oc.args.Data, &markEndInputVal); err != nil {
			return shim.Error(err.Error())
		}
		markEndOutputVal.PrivKey = markEndInputVal.PrivKey
		keyPair, err = DeserializePrivate(markEndOutputVal.PrivKey)
		if err != nil {
			msg := fmt.Sprintf("Cannot load key pair: %s", err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			return shim.Error(msg)
		}
	}

	// Decrypt the bids using the private key and calculate the MCP.
	// ATTN: In a non-POC setting, we would just persist this key to the ledger, and
	// have the clients do the decryption and calculate the clearing price locally.
	// In this POC we both persist the key to the ledger, *and* have the chaincode
	// calculate the MCP.

	// Create the bid collections corresponding to that slot_number
	var buyerBids, sellerBids BidCollection

	switch schema.ExpNum {
	case 1:
		buyerBids, err = oc.newBidCollection1("buy")
		if err != nil {
			metricsOutputVal.ProblematicBidCalcCount[oc.args.Slot]++
			return shim.Error(err.Error())
		}
		sellerBids, err = oc.newBidCollection1("sell")
		if err != nil {
			metricsOutputVal.ProblematicBidCalcCount[oc.args.Slot]++
			return shim.Error(err.Error())
		}
	case 2:
		buyerBids, err = oc.newBidCollection2("buy", keyPair)
		if err != nil {
			metricsOutputVal.ProblematicBidCalcCount[oc.args.Slot]++
			return shim.Error(err.Error())
		}
		sellerBids, err = oc.newBidCollection2("sell", keyPair)
		if err != nil {
			metricsOutputVal.ProblematicBidCalcCount[oc.args.Slot]++
			return shim.Error(err.Error())
		}
	case 3:
		buyerBids, err = oc.newBidCollection3("buy")
		if err != nil {
			metricsOutputVal.ProblematicBidCalcCount[oc.args.Slot]++
			return shim.Error(err.Error())
		}
		sellerBids, err = oc.newBidCollection3("sell")
		if err != nil {
			metricsOutputVal.ProblematicBidCalcCount[oc.args.Slot]++
			return shim.Error(err.Error())
		}
	}

	msg := fmt.Sprintf("[%s] Buyer bids for slot %d:", oc.txID, markEndOutputVal.Slot)
	fmt.Fprintln(w, msg)
	if len(buyerBids) > 0 {
		for i, v := range buyerBids {
			fmt.Fprintf(w, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(w, "\tnone")
	}

	msg = fmt.Sprintf("[%s] Seller bids for slot %d:", oc.txID, markEndOutputVal.Slot)
	fmt.Fprintln(w, msg)
	if len(sellerBids) > 0 {
		for i, v := range sellerBids {
			fmt.Fprintf(w, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(w, "\tnone")
	}

	// Settle the market for that slot
	if len(sellerBids) > 0 && len(buyerBids) > 0 {
		res, err := Settle(buyerBids, sellerBids)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot find clearing price for slot %d: %s", oc.txID, oc.args.Slot, err.Error())
			fmt.Fprintln(w, msg)

			markEndOutputVal.Message = msg
		} else { // This is our happy path
			markEndOutputVal.PricePerUnitInCents = res.PricePerUnit
			markEndOutputVal.QuantityInKWh = res.Units
			msg := fmt.Sprintf("[%s] %.6f kWh were cleared at %.3f ç/kWh in slot %d ✅", oc.txID, markEndOutputVal.QuantityInKWh, markEndOutputVal.PricePerUnitInCents, markEndOutputVal.Slot)
			fmt.Fprintln(w, msg)

			markEndOutputVal.Message = msg
		}
	} else {
		msg := fmt.Sprintf("No market for slot %d (buyer bids: %d, seller bids: %d) 😔", markEndOutputVal.Slot, len(buyerBids), len(sellerBids))
		fmt.Fprintln(w, msg)

		markEndOutputVal.Message = msg
	}

	markEndOutputValB, err := oc.Marshal(&markEndOutputVal)
	if err != nil {
		return shim.Error(err.Error())
	}

	if err := oc.Put(keyAttrs, markEndOutputValB); err != nil {
		return shim.Error(err.Error())
	}

	if err := oc.Event(); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(markEndOutputValB)
}

func (oc *opContext) newBidCollection1(bidType string) (BidCollection, error) {
	var resp BidCollection

	keyAttrs := []string{strconv.Itoa(oc.args.Slot), "-", bidType}

	// Populate the encrypted bids and corresponding private keys maps.
	var encBidVal, postKeyVal map[string][]byte
	encBidValB, err := oc.Get(keyAttrs)
	if err != nil {
		return nil, err
	}
	if err := oc.Unmarshal(encBidValB, &encBidVal); err != nil {
		return nil, err
	}

	postKeyValB, err := oc.Get(append(keyAttrs, "-", schema.PostKeySuffix))
	if err != nil {
		return nil, err
	}
	if err := oc.Unmarshal(postKeyValB, &postKeyVal); err != nil {
		return nil, err
	}

	// Start debug
	dbg := fmt.Sprintf("Bid map:\n%+v\n\nPostkey map:\n%+v\n\n", encBidVal, postKeyVal)
	fmt.Fprintln(w, dbg)
	// End debug

	// - Iterate over the items in encBidVal
	// - Retrieve the corresponding private key from postKeyVal
	// - Decrypt and add to bid collection

	for bidTxID, encBidInputValB := range encBidVal {
		postKeyInputValB, ok := postKeyVal[bidTxID]
		if !ok {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue // ATTN: We do not return
		}
		var postKeyInputVal schema.PostKeyInput
		if err := oc.Unmarshal(postKeyInputValB, &postKeyInputVal); err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}
		keyPair, err := DeserializePrivate(postKeyInputVal.PrivKey)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot retrieve key-pair from 'postKey' key: %s", oc.txID, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}

		// Decrypt the bid
		bidInputValB, err := Decrypt(encBidInputValB, keyPair)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot decrypt encoded payload for 'bid' call: %s", oc.txID, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue // ATTN: We do not return
		}

		var bidInputVal schema.BidInput
		if err := oc.Unmarshal(bidInputValB, &bidInputVal); err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue // ATTN: We do not return
		}

		// Add bid to bid collection
		bid := Bid{
			PricePerUnit: bidInputVal.PricePerUnitInCents,
			Units:        bidInputVal.QuantityInKWh,
		}
		resp = append(resp, bid)
		msg := fmt.Sprintf("[%s] Added a bid to the collection for slot %d: %s", oc.txID, oc.args.Slot, bid)
		fmt.Fprintln(w, msg)
	}

	return resp, nil
}

func (oc *opContext) newBidCollection2(bidType string, keyPair *rsa.PrivateKey) (BidCollection, error) {
	var resp BidCollection

	keyAttrs := []string{strconv.Itoa(oc.args.Slot), "-", bidType}
	iter, err := oc.Iter([]string{strconv.Itoa(oc.args.Slot), "-", bidType})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	if !iter.HasNext() {
		msg := fmt.Sprintf("[%s] No values exist for partial bid-key w/ attributes %s", oc.txID, keyAttrs)
		fmt.Fprintln(w, msg)
		return resp, nil
	}

	for iter.HasNext() {
		bidKV, err := iter.Next() // This holds a bid
		if err != nil {
			msg := fmt.Sprintf("[%s] Failed during iteration on bid-key w/ attributes %s: %s", oc.txID, keyAttrs, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicIterCount[oc.args.Slot]++
			return nil, errors.New(msg)
		}

		encBidInputValB := bidKV.GetValue()
		bidInputValB, err := Decrypt(encBidInputValB, keyPair)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot decrypt encoded payload for 'bid' call: %s", oc.txID, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue // ATTN: We do not return
		}

		var bidInputVal schema.BidInput
		if err := oc.Unmarshal(bidInputValB, &bidInputVal); err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}

		// Add bid to bid collection
		bid := Bid{
			PricePerUnit: bidInputVal.PricePerUnitInCents,
			Units:        bidInputVal.QuantityInKWh,
		}
		resp = append(resp, bid)
		msg := fmt.Sprintf("[%s] Added a bid to the collection for slot %d: %s", oc.txID, oc.args.Slot, bid)
		fmt.Fprintln(w, msg)
	}

	return resp, nil
}

func (oc *opContext) newBidCollection3(bidType string) (BidCollection, error) {
	var resp BidCollection

	keyAttrs := []string{strconv.Itoa(oc.args.Slot), "-", bidType}
	iter, err := oc.Iter(keyAttrs)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	if !iter.HasNext() {
		msg := fmt.Sprintf("[%s] No values exist for partial bid-key w/ attributes %s", oc.txID, keyAttrs)
		fmt.Fprintln(w, msg)
		return resp, nil
	}

	for iter.HasNext() {
		bidKV, err := iter.Next() // This holds a bid
		if err != nil {
			msg := fmt.Sprintf("[%s] Failed during iteration on bid-key w/ attributes %s: %s", oc.txID, keyAttrs, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicIterCount[oc.args.Slot]++
			return nil, errors.New(msg)
		}

		// Get the private key corresponding to this bid
		keyPrefixAttrs, err := oc.Split(bidKV.GetKey())
		if err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue // ATTN: We do not return
		}

		// Is this maybe a key holding a private key?
		// If so, we should ignore it.
		if len(keyPrefixAttrs) > 0 && (keyPrefixAttrs[len(keyPrefixAttrs)-1] == schema.PostKeySuffix) {
			continue
		}

		postKeyOutputValB, err := oc.Get(append(keyPrefixAttrs, "-", schema.PostKeySuffix))
		if err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}
		var postKeyOutputVal schema.PostKeyOutput
		if err := oc.Unmarshal(postKeyOutputValB, &postKeyOutputVal); err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}
		keyPair, err := DeserializePrivate(postKeyOutputVal.PrivKey)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot retrieve key-pair from 'postKey' key: %s", oc.txID, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}

		// Decrypt the bid
		encBidInputValB := bidKV.GetValue()
		bidInputValB, err := Decrypt(encBidInputValB, keyPair)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot decrypt encoded payload for 'bid' call: %s", oc.txID, err.Error())
			fmt.Fprintln(w, msg)
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}

		var bidInputVal schema.BidInput
		if err := oc.Unmarshal(bidInputValB, &bidInputVal); err != nil {
			metricsOutputVal.ProblematicDecryptCount[oc.args.Slot]++
			continue
		}

		// Add bid to bid collection
		bid := Bid{
			PricePerUnit: bidInputVal.PricePerUnitInCents,
			Units:        bidInputVal.QuantityInKWh,
		}
		resp = append(resp, bid)
		msg := fmt.Sprintf("[%s] Added a bid to the collection for slot %d: %s", oc.txID, oc.args.Slot, bid)
		fmt.Fprintln(w, msg)
	}

	return resp, nil
}
