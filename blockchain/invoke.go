package blockchain

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
)

// InvokeTimeout ...
const InvokeTimeout = 20 * time.Second

// Invoke ...
func (sc *SDKContext) Invoke(slot int, action string, dataB []byte) ([]byte, error) {
	eventB := []byte(strconv.Itoa(rand.Intn(1E6)))
	actionB := []byte(action)
	slotB := []byte(strconv.Itoa(slot))

	// fmt.Fprintf(os.Stdout, "[%s] %s @ %d\n", eventB, action, slot)

	reg, notifier, err := sc.EventClient.RegisterChaincodeEvent(sc.ChaincodeID, string(eventB))
	if err != nil {
		return nil, err
	}
	defer sc.EventClient.Unregister(reg)

	// Create a request (proposal) and send it
	resp, err := sc.ChannelClient.Execute(channel.Request{
		ChaincodeID: sc.ChaincodeID,
		Fcn:         "invoke",
		Args:        [][]byte{eventB, actionB, slotB, dataB}})
	if err != nil {
		return nil, fmt.Errorf("[%s] cannot execute request", eventB)
	}

	// Wait for the result of the submission
	select {
	case <-notifier:
		// fmt.Fprintf(os.Stdout, "Received update for event ID %s\n", ccEvent.EventName)
	case <-time.After(InvokeTimeout):
		return nil, fmt.Errorf("[%s] did not hear back on event in time: %s", eventB, err.Error())
	}

	return resp.Payload, nil
}
