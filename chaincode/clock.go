package main

import (
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
	"github.com/kchristidis/island/chaincode/schema"
)

// - Creates write-key <slot_number>-<clock>-<tx_id>
// - Writes `oc.args.Data` to key (N.B. this step can be ommitted, the passed-in `oc.args.Data` is nil)
func (oc *opContext) clock() pp.Response {
	keyAttrs := []string{strconv.Itoa(oc.args.Slot), "-", oc.args.Action, "-", oc.txID}
	if err := oc.Put(keyAttrs, oc.args.Data); err != nil {
		return shim.Error(err.Error())
	}

	clockOutputVal := schema.ClockOutput{
		WriteKeyAttrs: keyAttrs,
	}

	clockOutputValB, err := oc.Marshal(&clockOutputVal)
	if err != nil {
		return shim.Error(err.Error())
	}

	if err := oc.Event(); err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(clockOutputValB)
}
