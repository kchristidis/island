package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func main() {
	w = os.Stdout
	if err := shim.Start(new(Contract)); err != nil {
		msg := fmt.Sprintf("Cannot establish handler with peer: %s", err)
		fmt.Fprintln(w, msg)
	}
}

// Contract satisfies the shim.Chaincode interface
// so that it is a valid Fabric chaincode.
type Contract struct{}

// Init carries initialization logic for the chaincode.
// It is automatically invoked during chaincode instantiation.
func (c *Contract) Init(stub shim.ChaincodeStubInterface) pp.Response {
	return shim.Success(nil)
}

// Invoke is used whenever we wish to interact with the chaincode.
func (c *Contract) Invoke(stub shim.ChaincodeStubInterface) pp.Response {
	op, err := newOpContext(stub)
	if err != nil {
		return shim.Error(err.Error())
	}

	return op.run()
}
