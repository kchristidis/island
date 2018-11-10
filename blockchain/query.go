package blockchain

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
)

// Query ...
func (sc *SDKContext) Query(slot int, action string) ([]byte, error) {
	slotB := []byte(strconv.Itoa(slot))
	actionB := []byte(action)

	resp, err := sc.ChannelClient.Query(channel.Request{
		ChaincodeID: sc.ChaincodeID,
		Fcn:         "query",
		Args:        [][]byte{nil, actionB, slotB, nil},
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %s", err.Error())
	}

	return resp.Payload, nil
}
