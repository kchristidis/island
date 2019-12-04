package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"

	"github.com/kchristidis/island/chaincode/schema"
)

// Get retrieves the value associated with the given composite key.
func (oc *opContext) Get(keyAttrs []string) ([]byte, error) {
	key, err := oc.stub.CreateCompositeKey("", keyAttrs)
	if err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot create key w/ attributes %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, keyAttrs, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicKeyCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}

	valB, err := oc.stub.GetState(key)
	if err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot read key %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, key, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicGetStateCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}

	if schema.StagingLevel <= schema.Debug {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • read key w/ attributes %s successfully", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, keyAttrs)
		fmt.Fprintln(w, msg)
	}

	return valB, nil
}

// Put persists the value for a given composite key.
func (oc *opContext) Put(keyAttrs []string, valB []byte) error {
	key, err := oc.stub.CreateCompositeKey("", keyAttrs)
	if err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot create key w/ attributes %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, keyAttrs, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicKeyCount[oc.args.Slot]++
		return errors.New(msg)
	}

	if err := oc.stub.PutState(key, valB); err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot persist value to key %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, key, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicPutStateCount[oc.args.Slot]++
		return errors.New(msg)
	}

	if schema.StagingLevel <= schema.Debug {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • wrote to key w/ attributes %s successfully", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, keyAttrs)
		fmt.Fprintln(w, msg)
	}

	return nil
}

// Iter returns the iterator associated with a given partial composite key.
func (oc *opContext) Iter(keyAttrs []string) (shim.StateQueryIteratorInterface, error) {
	iter, err := oc.stub.GetStateByPartialCompositeKey("", keyAttrs)
	if err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot create iterator for key w/ attributes %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, keyAttrs, err.Error())
		fmt.Fprintln(w, msg)
		return nil, errors.New(msg)
	}
	return iter, nil
}

// Split returns the components (key attributes) of a composite key.
func (oc *opContext) Split(key string) ([]string, error) {
	_, keyAttrs, err := oc.stub.SplitCompositeKey(key)
	if err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot split key %s: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, key, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicKeyCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}
	return keyAttrs, nil
}

// Marshal wraps the JSON encoder and logs any errors to the writer.
func (oc *opContext) Marshal(val interface{}) ([]byte, error) {
	valB, err := json.Marshal(val)
	if err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot encode to JSON: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicMarshalCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}
	return valB, nil
}

// Unmarshal wraps the JSON decoder and logs any errors to the writer.
func (oc *opContext) Unmarshal(valB []byte, val interface{}) error {
	if err := json.Unmarshal(valB, &val); err != nil {
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot decode JSON: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicMarshalCount[oc.args.Slot]++
		return errors.New(msg)
	}
	return nil
}

// Event sends an event.
func (oc *opContext) Event() error {
	if schema.EnableEvents {
		// Notify listeners that the event has been executed
		err := oc.stub.SetEvent(oc.args.EventID, nil)
		if err != nil {
			msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • cannot create event: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, err.Error())
			fmt.Fprintln(w, msg)
			return errors.New(msg)
		}
	}
	return nil
}
