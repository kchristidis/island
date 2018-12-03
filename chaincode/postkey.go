package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
	"github.com/kchristidis/island/chaincode/schema"
)

// - Looks up read-key <slot_number>-<markend> and returns error if read-key is found
// - Experiment 1:
//		a. If write-key <PostKeyInput.ReadKey>-<PostKeySuffix> is not found, creates
//			map[string][]byte as value for that write-key, and writes JSON-encoded
//			`schema.PostKeyOutput` to map[bidder_id].
//		2. If write-key <PostKeyInput.ReadKey>-<PostKeySuffix> is found, it writes
//			JSON-encoded `schema.PostKeyOutput` to map[bidder_id] (the value for the
//			write-key will be a map).
// - Experiments 2, 3:
//		a. Creates write-key <PostKeyInput.ReadKey>-<PostKeySuffix>
//		b. Writes JSON-encoded `schema.PostKeyOutput` to write-key
func (oc *opContext) postKey() pp.Response {
	valB, err := oc.Get([]string{strconv.Itoa(oc.args.Slot), "-", "markEnd"})
	if err != nil {
		return shim.Error(err.Error())
	}
	if valB != nil {
		msg := fmt.Sprintf("[%s] Slot %d has been marked already, aborting 'postKey' 🛑", oc.txID, oc.args.Slot)
		fmt.Fprintln(w, msg)
		metricsOutputVal.LateTXsCount[oc.args.Slot]++
		metricsOutputVal.LateDecryptsCount[oc.args.Slot]++
		return shim.Error(msg)
	}

	// The slot has not been marked
	// Let's proceed as usual in order to post the key

	// Unmarshal the incoming data
	var postKeyInputVal schema.PostKeyInput
	if err := json.Unmarshal(oc.args.Data, &postKeyInputVal); err != nil {
		return shim.Error(err.Error())
	}

	// What is the key we wish to write to?
	keyAttrs := append(postKeyInputVal.ReadKeyAttrs, "-", schema.PostKeySuffix)

	// Let's work on what we'll return to the caller now
	// because it will be of use in the ExpNum 2,3 cases.

	// This is what we return to the caller
	postKeyOutputVal := schema.PostKeyOutput{
		WriteKeyAttrs: keyAttrs,
		PrivKey:       postKeyInputVal.PrivKey,
	}

	postKeyOutputValB, err := oc.Marshal(&postKeyOutputVal)
	if err != nil {
		return shim.Error(err.Error())
	}

	switch schema.ExpNum {
	case 1:
		// Does this key exist already?
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
		val[postKeyInputVal.TxID] = oc.args.Data // ATTN: It's not oc.txID as this would give us the ID of the *current* transaction
		newValB, err := oc.Marshal(val)
		if err != nil {
			return shim.Error(err.Error())
		}
		if err := oc.Put(keyAttrs, newValB); err != nil {
			return shim.Error(err.Error())
		}
	case 3:
		if err := oc.Put(keyAttrs, postKeyOutputValB); err != nil {
			return shim.Error(err.Error())
		}
	}

	if err := oc.Event(); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(postKeyOutputValB)
}
