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
		msg := fmt.Sprintf("[%s] Cannot create key w/ attributes %s: %s", oc.txID, keyAttrs, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicKeyCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}

	valB, err := oc.stub.GetState(key)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot read key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicGetStateCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}

	return valB, nil
}

// Put persists the value for a given composite key.
func (oc *opContext) Put(keyAttrs []string, valB []byte) error {
	key, err := oc.stub.CreateCompositeKey("", keyAttrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key w/ attributes %s: %s", oc.txID, keyAttrs, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicKeyCount[oc.args.Slot]++
		return errors.New(msg)
	}

	if err := oc.stub.PutState(key, valB); err != nil {
		msg := fmt.Sprintf("[%s] Cannot persist value to key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicPutStateCount[oc.args.Slot]++
		return errors.New(msg)
	}

	return nil
}

// Iter returns the iterator associated with a given partial composite key.
func (oc *opContext) Iter(keyAttrs []string) (shim.StateQueryIteratorInterface, error) {
	iter, err := oc.stub.GetStateByPartialCompositeKey("", keyAttrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key w/ attributes %s: %s", oc.txID, keyAttrs, err.Error())
		fmt.Fprintln(w, msg)
		return nil, errors.New(msg)
	}
	return iter, nil
}

// Split returns the components (key attributes) of a composite key.
func (oc *opContext) Split(key string) ([]string, error) {
	_, keyAttrs, err := oc.stub.SplitCompositeKey(key)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot split key %s: %s", oc.txID, key, err.Error())
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
		msg := fmt.Sprintf("[%s] Cannot encode as JSON object: %s", oc.txID, err.Error())
		fmt.Fprintln(w, msg)
		metricsOutputVal.ProblematicMarshalCount[oc.args.Slot]++
		return nil, errors.New(msg)
	}
	return valB, nil
}

// Unmarshal wraps the JSON decoded and logs any errors to the writer.
func (oc *opContext) Unmarshal(valB []byte, val interface{}) error {
	if err := json.Unmarshal(valB, &val); err != nil {
		msg := fmt.Sprintf("[%s] Cannot unmarshal JSON-encoded object: %s", oc.txID, err.Error())
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
			msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
			fmt.Fprintln(w, msg)
			return errors.New(msg)
		}
	}
	return nil
}
