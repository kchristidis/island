package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kchristidis/island/chaincode/schema"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

type opContext struct {
	stub shim.ChaincodeStubInterface
	txID string // ATTN: This is the ID assigned by the chaincode, not us
	fn   string // Basically: "invoke" or "query"

	args schema.OpContextInput
}

func newOpContext(stub shim.ChaincodeStubInterface) (*opContext, error) {
	args := stub.GetArgs()

	var OpContextInputVal schema.OpContextInput
	if err := json.Unmarshal(args[1], &OpContextInputVal); err != nil {
		msg := fmt.Sprintf("Cannot unmarshal JSON-encoded payload: %s", err.Error())
		fmt.Fprintln(w, msg)
		return nil, errors.New(msg)
	}

	oc := &opContext{
		stub: stub,
		txID: stub.GetTxID(),
		fn:   string(args[0]),
		args: OpContextInputVal,
	}

	var msg string

	switch oc.args.Action {
	case "buy", "sell", "postKey", "markEnd":
		msg = fmt.Sprintf("[%s] Incoming: %s for slot %d (event ID: %s)", oc.txID, oc.args.Action, oc.args.Slot, oc.args.EventID)
		fmt.Fprintln(w, msg)
	default:
		// Do not error on other actions, as they may correspond to valid queries.
	}

	return oc, nil
}

func (oc *opContext) run() pp.Response {
	switch oc.fn {
	case "invoke":
		return oc.invoke()
	case "query":
		return oc.query()
	default:
		msg := fmt.Sprintf("[%s] Invalid function: %s", oc.txID, oc.fn)
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}
}
