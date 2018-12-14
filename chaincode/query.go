package main

import (
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) query() pp.Response {
	switch oc.args.Action {
	case "metrics":
		return oc.metrics()
	case "slotValues":
		return oc.slotvalues()
	default:
		msg := fmt.Sprintf("tx_id:%s\tevent_id:%s\tslot:%012d\t• invalid query action: %s", oc.txID, oc.args.EventID, oc.args.Slot, oc.args.Action)
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}
}
