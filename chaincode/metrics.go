package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

// - Encodes `aggStats` variable (type `aggregateStats`) as a JSON object
// - Returns JSON object
func (oc *opContext) metrics() pp.Response {
	metricsOutputValB, err := json.Marshal(&metricsOutputVal)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot encode response as a JSON object: %s", oc.txID, err.Error())
		fmt.Fprintln(w, msg)
		return shim.Error(msg)
	}

	return shim.Success(metricsOutputValB)
}
