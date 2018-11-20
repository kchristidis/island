package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) invoke() pp.Response {
	switch oc.action {
	case "buy", "sell":
		return oc.bid()
	case "markEnd":
		return oc.markEnd()
	case "revealKeys":
		return oc.revealKeys()
	case "clock":
		return oc.clock()
	default:
		msg := fmt.Sprintf("[%s] Invalid action: %s", oc.txID, oc.action)
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
}
