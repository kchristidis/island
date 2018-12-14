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
		msg := fmt.Sprintf("tx_id:%s\tevent_id:%s\tslot:%012d\t• invalid action: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action)
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}
}
