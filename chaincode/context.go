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
		msg := fmt.Sprintf("cannot decode JSON op-context: %s", err.Error())
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
		msg = fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • incoming action!", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action)
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
		msg := fmt.Sprintf("tx_id:%s event_id:%s slot:%012d action:%s • invalid function: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action, oc.fn)
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}
}
