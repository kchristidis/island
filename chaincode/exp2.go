package main

import (
	"crypto/rsa"
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

// BufferLen ...
const BufferLen = 20

var (
	buyerBids, sellerBids map[int]BidCollection
	bbMutex, sbMutex      sync.RWMutex
)

// Keys
var keyPair *rsa.PrivateKey
var privBytes = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAyqvL/UJzq7uSETf+I8e+OMjams1ZYMvXF8PjzBhZJzrgJNTi
zJDAkFrnAPFTstflg8kU7wxkNOmH4steDIQ9hi+xAW8JRELKinIriQZJ+lSp6/ds
7Kjb0EfPzDF/nbJYjtdIVHIPfaLZno5O46CsmRt2KiwYdKDf08iHBzifugV8mtq2
ugxczA3PvgQ89Yyubcccprg/Zy54lOziy0Pn9mDePOI4oIOM0XO4VGbfbVZtAC9H
Lv4jiO4nB+ixem+3jGNR7IwsB+0xlcN1zvBic1WBNlJYqiEaIZTzqDHi8KdigQS7
bO0nMttS7Wr0WWyXHSPtrRmKvx04CY+IPHkWcQIDAQABAoIBABHvslXvk50XNI4h
jnRMMSGFZRNeKRLP93E6/OYLIZi/NScNUCUainA8G0WSFf417TIEkb22MwgbwtLn
fKNO8ML3ZYri8McBwjsOb5vo2pM0+vTPKOyo5QtBz7oah1jFd+DsXJJcpdJQn0HR
BlpO1feW3pZM4L0xn512mbyh3kDwIwzhzxnHzXnhD78bzrm7JoBBITcrjt228Z+3
mVNCftc48SGoVMnOMJr8B6tlOkL0tG3A593hZZfyCxrdsRmWRieQJACWqhfXJFPh
JXM8qxjtqCTauUKc3NEuWsqsmObN6Cpq8L1K+/bmjaNl6qeGigLHpWyN5ks/U9FP
7VDiECECgYEA5pWiGyQWYHYPntBpqlmfhb8zKUfBZahFnzFo9CKV7YDK7cUx3vcM
I5Q94btAQg6kD54dQMWzudmUb3XVU5+ZJ3yBgUTFqfSvcPopbrtrSji3u0pRUyRg
UgMfUN0orPLlAWLTlpy4SDsfcKeQHSfS+L8Eu0Kt+AJErs0I5eCIatsCgYEA4QKI
8IZ5BRbaGlJvPI98J9NRJjRhkxGh55JslNguD+w88Q8bDltMlEQRWfpkKHwMyVMB
Ll2VHg/Dg3KdQUkItv+27qMRP2eUFCj0XQo+5owmYHhpn5fYHJjYrsDSrNZDqSL/
kel3MuxSz9NK3EAGToIQtfAZhwjeOhzvNqB4N6MCgYEA1EkOZU5kC4ql9uCJZ3v7
kXbl8ytMsfqpnlYu+hSdU3svWJgjwdJQKrFgB2INVsOD55z58ZgSTxgxwCwLqmFU
7zWBRTG7iSzsGGc3neqObFarUJKrLJBg3SBixF/YAuHcU9pYUmEWh+lmmKCr3Su8
36V9Bant4Fa2RPgfKQP+k+ECgYEAk4ev9dSVoMqc8kk+efyyMQKS4HPTzjPvbgBJ
hUZA3VvNkViQKted3FDM96v+47SCRbZQve/KB83aKWOKy/Vw61u6u7jbZDErnBRG
NIK1P0CBIRuSVXufzRBCckInX/+UmV9DJo5nA1KD8ZPeL48jE3KgNkpY0nr0CjJS
fgS1DfUCgYEAmx3sAJqR/mksm8jQXiduGe2L9IwYoWvvVL6GzMik58iuI2OtmjF8
OSwjLClaBdvj45YrUN3/i5LiIvblup9QpqshyEbSuT5gvhuf3jVmb08TJszihVXJ
p2j5extRQ8Ua4T37+7UdkZMvJLEps9ic+IlJWYwKt1qM+ZIZuuitmwU=
-----END RSA PRIVATE KEY-----`)

func main() {
	if err := shim.Start(new(contract)); err != nil {
		println("Cannot establish handler with peer:", err)
	}
}

// contract satisfies the shim.Chaincode interface so that it is a valid Fabric contract.
type contract struct{}

// Init carries initialization logic for the chaincode. It is automatically invoked during chaincode instantiation.
func (c *contract) Init(stub shim.ChaincodeStubInterface) pp.Response {
	var err error

	buyerBids = make(map[int]BidCollection)
	sellerBids = make(map[int]BidCollection)

	keyPair, err = DeserializePrivate(privBytes)
	if err != nil {
		msg := fmt.Sprintf("[Cannot load keypair: %s", err.Error())
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
	var err error

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

	var decrData []byte
	decrData, err = Decrypt(oc.data, keyPair)
	if err != nil {
		msg := fmt.Sprintf("[%s] Cannot decrypt encoded payload: %s", oc.txID, err.Error())
		fmt.Fprintln(os.Stdout, msg)
		return shim.Error(msg)
	}

	var bidData []float64 // The expectation then is that `oc.data` is an encrypted pair of float64's.
	if err := json.Unmarshal(decrData, &bidData); err != nil {
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
		// This allows us to only keep the last 100 slots in memory.
		// We do not need the previous one as they have already been settled.
		if deleteKey := (slotNum - BufferLen); deleteKey >= 0 {
			if _, ok := buyerBids[deleteKey]; ok {
				delete(buyerBids, deleteKey)
			}
		}
		bbMutex.Unlock()
	case "sell":
		sbMutex.Lock()
		sellerBids[slotNum] = append(sellerBids[slotNum], bid)
		if deleteKey := (slotNum - BufferLen); deleteKey >= 0 {
			if _, ok := buyerBids[deleteKey]; ok {
				delete(buyerBids, deleteKey)
			}
		}
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

	msg = fmt.Sprintf("[%s] Buyer bids for slot %d:", oc.txID, slotNum)
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
