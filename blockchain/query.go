package blockchain

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/kchristidis/island/chaincode/schema"
)

// Query ...
func (sc *SDKContext) Query(args schema.OpContextInput) ([]byte, error) {
	argsB, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	resp, err := sc.ChannelClient.Query(channel.Request{
		ChaincodeID: sc.ChaincodeID,
		Fcn:         "query",
		Args:        [][]byte{argsB},
	})
	if err != nil {
		return nil, fmt.Errorf("[%s] query failed: %s", args.EventID, err.Error())
	}

	return resp.Payload, nil
}
