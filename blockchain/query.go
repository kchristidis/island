package blockchain

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
)

// Query ...
func (sc *SDKContext) Query(slot int, action string) ([]string, error) {
	slotB := []byte(strconv.Itoa(slot))
	actionB := []byte(action)

	resp, err := sc.ChannelClient.Query(channel.Request{
		ChaincodeID: sc.ChaincodeID,
		Fcn:         "query",
		Args:        [][]byte{nil, actionB, slotB, nil},
	})
	if err != nil {
		return []string{}, fmt.Errorf("query for %s @ %d failed: %s", action, slot, err.Error())
	}

	var respS []string
	if err := json.Unmarshal(resp.Payload, &respS); err != nil {
		return []string{}, fmt.Errorf("cannot unmarshal returned response for %s @ %d: %s", action, slot, err.Error())
	}

	return respS, nil
}
