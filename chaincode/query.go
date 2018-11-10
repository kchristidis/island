package main

import (
	pp "github.com/hyperledger/fabric/protos/peer"
)

func (oc *opContext) query() pp.Response {
	switch oc.action {
	case "aggregate":
		return oc.queryAggregate()
	default:
		return oc.querySlot()
	}
}
