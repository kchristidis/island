package main

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

func main() {
	if err := shim.Start(new(contract)); err != nil {
		println("Cannot establish handler with peer:", err)
	}
}

// contract satisfies the shim.Chaincode interface so that it is a valid Fabric chaincode.
type contract struct{}

// Init carries initialization logic for the chaincode. It is automatically invoked during chaincode instantiation.
func (c *contract) Init(stub shim.ChaincodeStubInterface) pp.Response {
	var err error

	buyerBids = make(map[int]BidCollection)
	sellerBids = make(map[int]BidCollection)

	// N.B. For the purposes of this POC, we persist the private key in
	// the chaincode itself. This is of course a huge no-no in production.
	keyPair, err = DeserializePrivate(privBytes)
	if err != nil {
		msg := fmt.Sprintf("Cannot load keypair: %s", err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success(nil)
}

// Invoke is used whenever we wish to interact with the chaincode.
func (c *contract) Invoke(stub shim.ChaincodeStubInterface) pp.Response {
	op, err := newOpContext(stub)
	if err != nil {
		return shim.Error(err.Error())
	}

	return op.run()
}
