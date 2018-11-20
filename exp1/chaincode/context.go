package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

type opContext struct {
	stub shim.ChaincodeStubInterface
	txID string
	fn   string

	eventID string
	action  string
	slot    string
	data    []byte
}

func newOpContext(stub shim.ChaincodeStubInterface) (*opContext, error) {
	args := stub.GetArgs()
	oc := &opContext{
		stub: stub,
		txID: stub.GetTxID(),
		fn:   string(args[0]),

		eventID: string(args[1]),
		action:  string(args[2]),
		slot:    string(args[3]),
		data:    args[4],
	}

	var msg string

	switch oc.action {
	case "buy", "sell", "markEnd", "revealKeys":
		msg = fmt.Sprintf("[%s] Action '%s' @ slot '%s'", oc.txID, oc.action, oc.slot)
		fmt.Fprintln(os.Stdout, msg)
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
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
}
