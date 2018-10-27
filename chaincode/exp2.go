package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pp "github.com/hyperledger/fabric/protos/peer"
)

// EnableEvents ...
const EnableEvents = false

var (
	buyerBids, sellerBids map[int]BidCollection
	bbMutex, sbMutex      sync.RWMutex
)

// Stats
/* var (
	tradedEnergyPerSlot                  [35036]float32 // Goal: allocative efficiency
	tradedElectricityCostPerSlot         [35036]float32 // Goal: total electricity cost per slot and overall
	lateTransactionsPerslot              [35036]int     // Goal: percentage of transactions that didn't make it on time
	lateBidTransactionsPerSlot           [35036]int     // Same goal as above
	lateDecryptionKeyTransactionsPerSlot [35036]int     // As above
) */

func main() {
	if err := shim.Start(new(contract)); err != nil {
		println("Cannot establish handler with peer:", err)
	}
}

// contract satisfies the shim.Chaincode interface so that it is a valid Fabric contract.
type contract struct{}

// Init carries initialization logic for the chaincode. It is automatically invoked during chaincode instantiation.
func (c *contract) Init(stub shim.ChaincodeStubInterface) pp.Response {
	buyerBids = make(map[int]BidCollection)
	sellerBids = make(map[int]BidCollection)
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

	switch oc.action {
	case "buy", "sell", "markEnd":
		msg := fmt.Sprintf("[%s] Action '%s' @ slot '%s'", oc.txID, oc.action, oc.slot)
		fmt.Fprintln(os.Stdout, msg)
	}

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
	case "clock":
		return oc.clock()
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

	slotNum, _ := strconv.Atoi(oc.slot)

	// Add bid to bid collection
	switch oc.action {
	case "buy":
		bbMutex.Lock()
		buyerBids[slotNum] = append(buyerBids[slotNum], bid)
		bbMutex.Unlock()
	case "sell":
		sbMutex.Lock()
		sellerBids[slotNum] = append(sellerBids[slotNum], bid)
		sbMutex.Unlock()
	}

	if EnableEvents {
		// Notify listeners that the event has been executed
		err = oc.stub.SetEvent(oc.eventID, nil)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return shim.Error(msg)
		}
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

	slotNum, _ := strconv.Atoi(oc.slot)

	bbMutex.RLock()
	defer bbMutex.RUnlock()
	sbMutex.RLock()
	defer sbMutex.RUnlock()

	msg := fmt.Sprintf("[%s] Buyer bids for slot %d:", oc.txID, slotNum)
	fmt.Fprintln(os.Stdout, msg)
	if len(buyerBids[slotNum]) > 0 {
		for i, v := range buyerBids[slotNum] {
			fmt.Fprintf(os.Stdout, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(os.Stdout, "\tnone")
	}

	msg = fmt.Sprintf("[%s] Seller bids for slot %d:", oc.txID, slotNum)
	fmt.Fprintln(os.Stdout, msg)
	if len(sellerBids[slotNum]) > 0 {
		for i, v := range sellerBids[slotNum] {
			fmt.Fprintf(os.Stdout, "%2d: %s\n", i, v)
		}
	} else {
		fmt.Fprintln(os.Stdout, "\tnone")
	}

	// Settle the market for that slot
	if len(sellerBids[slotNum]) > 0 && len(buyerBids[slotNum]) > 0 {
		res, err := Settle(buyerBids[slotNum], sellerBids[slotNum])
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot find clearing price for slot %s: %s", oc.txID, oc.slot, err.Error())
			fmt.Fprintln(os.Stdout, msg)
		} else {
			writeVal.ppu = res.PricePerUnit
			writeVal.units = res.Units
			msg := fmt.Sprintf("[%s] %.6f kWh were cleared at %.2f ç/kWh in slot %s ✅", oc.txID, writeVal.units, writeVal.ppu, oc.slot)
			fmt.Fprintln(os.Stdout, msg)
			respPayload = []byte(msg)
		}
	} else {
		respPayload = []byte(fmt.Sprintf("No market for slot %d (buyer bids: %d, seller bids: %d)", slotNum, len(buyerBids[slotNum]), len(sellerBids[slotNum])))
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

	if EnableEvents {
		// Notify listeners that the event has been executed
		err = oc.stub.SetEvent(oc.eventID, nil)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return shim.Error(msg)
		}
	}

	return shim.Success(respPayload)
}

func (oc *opContext) clock() pp.Response {
	attrs := []string{oc.slot, oc.action, oc.txID} // The composite key is: slot_number + action + tx_id
	k, err := oc.stub.CreateCompositeKey("", attrs)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot create key %s: %s", oc.txID, attrs, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if err := oc.stub.PutState(k, oc.data); err != nil {
		msg := fmt.Sprintf("[%s] Cannot write value for key %s: %s", oc.txID, k, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	if EnableEvents {
		// Notify listeners that the event has been executed
		err = oc.stub.SetEvent(oc.eventID, nil)
		if err != nil {
			msg := fmt.Sprintf("[%s] Cannot create event: %s", oc.txID, err.Error())
			fmt.Fprintln(os.Stdout, msg)
			return shim.Error(msg)
		}
	}

	return shim.Success([]byte(k))
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
