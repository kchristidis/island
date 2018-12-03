package main

import (
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) invoke() pp.Response {
	switch oc.args.Action {
	case "buy", "sell":
		return oc.bid()
	case "postKey":
		return oc.postKey()
	case "markEnd":
		return oc.markEnd()
	case "clock":
		return oc.clock()
	default:
		msg := fmt.Sprintf("[%s] Invalid action: %s", oc.txID, oc.args.Action)
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}
}
