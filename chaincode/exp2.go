package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

var (
	buyerBids, sellerBids BidCollection // Set and cleared @ every slot
	results               = make(map[string]string)
)

func main() {
	if err := shim.Start(new(contract)); err != nil {
		println("Cannot establish handler with peer:", err)
	}
}

// contract satisfies the shim.Chaincode interface so that it is a valid Fabric contract.
type contract struct{}

// Init carries initialization logic for the chaincode. It is automatically invoked during chaincode instantiation.
func (c *contract) Init(stub shim.ChaincodeStubInterface) pp.Response {
	return shim.Success(nil) // A no-op for now
}

// Invoke is used whenever we wish to interact with the chaincode.
func (c *contract) Invoke(stub shim.ChaincodeStubInterface) pp.Response {
	op, err := newOpContext(stub)
	if err != nil {
		return shim.Error(err.Error())
	}

	return op.run()
}

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

	msg := fmt.Sprintf("[%s] Action '%s' @ slot '%s'", oc.txID, oc.action, oc.slot)
	fmt.Fprintln(os.Stdout, msg)

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

func (oc *opContext) invoke() pp.Response {
	switch oc.action {
	case "buy", "sell":
		return oc.bid()
	case "markEnd":
		return oc.markEnd()
	default:
		msg := fmt.Sprintf("[%s] Invalid action: %s", oc.txID, oc.action)
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
}

func (oc *opContext) bid() pp.Response {
	// Has this slot been marked already?
	k1 := []string{oc.slot, "markEnd"} // The partial composite key is: slot_number + "markEnd"
	iter, err := oc.stub.GetStateByPartialCompositeKey("", k1)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key %s: %s", oc.txID, k1, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
	defer iter.Close()

	if iter.HasNext() {
		msg := fmt.Sprintf("[%s] Slot %s has been marked already, aborting 🛑", oc.txID, oc.slot)
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	// Proceed as usual
	attrs := []string{oc.slot, oc.action, oc.txID} // The composite key is: slot_number + action + tx_id
	k2, err := oc.stub.CreateCompositeKey("", attrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(k2, oc.data); err != nil {
		msg := fmt.Sprintf("[%s] Cannot write value for key %s: %s", oc.txID, k2, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var bidData []float64 // The expectation then is that `oc.data` is a pair of float64's.
	if err := json.Unmarshal(oc.data, &bidData); err != nil {
		msg := fmt.Sprintf("[%s] Cannot unmarshal encoded payload: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	bid := Bid{
		PricePerUnit: bidData[0],
		Units:        bidData[1],
	}

	// Add bid to bid collection
	switch oc.action {
	case "buy":
		buyerBids = append(buyerBids, bid)
	case "sell":
		sellerBids = append(sellerBids, bid)
	}

	// Notify listeners that the event has been executed
	err = oc.stub.SetEvent(oc.eventID, nil)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success([]byte(k2))
}

func (oc *opContext) markEnd() pp.Response {
	attrs := []string{oc.slot, oc.action, oc.txID} // The composite key is: slot_number + action + tx_id
	key, err := oc.stub.CreateCompositeKey("", attrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var writeVal struct {
		privKey    string
		ppu, units float64
	}

	var respPayload []byte

	// Settle the market for that slot
	if len(sellerBids) > 0 && len(buyerBids) > 0 {
		//msg := fmt.Sprintf("Buyer bids: %v", buyerBids)
		// fmt.Fprintln(os.Stdout, msg)
		// msg = fmt.Sprintf("Seller bids: %v", sellerBids)
		// fmt.Fprintln(os.Stdout, msg)

		res, err := Settle(buyerBids, sellerBids)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot find clearing price for slot %s: %s", oc.txID, oc.slot, err.Error())
			fmt.Fprintln(os.Stdout, msg)
		} else {
			writeVal.ppu = res.PricePerUnit
			writeVal.units = res.Units
			msg := fmt.Sprintf("[%s] %.2f kWh were cleared at %.2f ç/kWh in slot %s ✅", oc.txID, writeVal.units, writeVal.ppu, oc.slot)
			fmt.Fprintln(os.Stdout, msg)
			respPayload = []byte(msg)

			// Reset for the next slot
			buyerBids = BidCollection{}
			sellerBids = BidCollection{}
		}
	}

	writeVal.privKey = string(oc.data)

	writeBytes, err := json.Marshal(writeVal)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal write-value for key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(key, writeBytes); err != nil {
		msg := fmt.Sprintf("[%s] Cannot persist write-value for key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	// Notify listeners that the event has been executed
	err = oc.stub.SetEvent(oc.eventID, nil)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success(respPayload)
}

func (oc *opContext) query() pp.Response {
	key := []string{oc.slot, string(oc.action)} // The partial composite key is: slot_number + action
	iter, err := oc.stub.GetStateByPartialCompositeKey("", key)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create iterator for key %s: %s", oc.txID, key, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}
	defer iter.Close()

	if !iter.HasNext() {
		msg := fmt.Sprintf("[%s] No values exist for partial key %s", oc.txID, key)
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var resp []string
	for iter.HasNext() {
		respRange, err := iter.Next()
		if err != nil {
			msg := fmt.Sprintf("[%s] Failed during iteration on key %s: %s", oc.txID, key, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return shim.Error(msg)
		}
		item := string(respRange.Value)
		msg := fmt.Sprintf("[%s] Adding [%s] to the response payload for key %s", oc.txID, item, key)
		fmt.Fprintln(os.Stdout, msg)
		resp = append(resp, item)
	}

	respB, err := json.Marshal(resp)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot marshal response: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	return shim.Success(respB)
}
