package blockchain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/kchristidis/island/chaincode/schema"
)

// InvokeTimeout ...
const InvokeTimeout = 20 * time.Second

// Invoke ...
func (sc *SDKContext) Invoke(args schema.OpContextInput) ([]byte, error) {
	argsB, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	var reg fab.Registration
	var notifier <-chan *fab.CCEvent
	if schema.EnableEvents {
		reg, notifier, err = sc.EventClient.RegisterChaincodeEvent(sc.ChaincodeID, args.EventID)
		if err != nil {
			return nil, err
		}
		defer sc.EventClient.Unregister(reg)
	}

	// Create a request (proposal) and send it
	resp, err := sc.ChannelClient.Execute(channel.Request{
		ChaincodeID: sc.ChaincodeID,
		Fcn:         "invoke",
		Args:        [][]byte{argsB}})
	if err != nil {
		var msg string
		if strings.Contains(err.Error(), " MVCC_READ_CONFLICT") {
			msg = "failure: mvcc_read_conflict"
		} else {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	if schema.EnableEvents {
		// Wait for the result of the submission
		select {
		case <-notifier:
			// fmt.Fprintf(os.Stdout, "received update for event ID %s", ccEvent.EventName)
		case <-time.After(InvokeTimeout):
			return nil, fmt.Errorf("event_id:%s • did not hear back on event in time: %s", args.EventID, err.Error())
		}
	}

	return resp.Payload, nil
}
