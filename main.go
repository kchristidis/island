package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/kchristidis/exp2/blockchain"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	sc := blockchain.SDKContext{
		SDKConfigFile: "config.yaml",

		OrgName:  "clark",
		OrgAdmin: "Admin",
		UserName: "User1",

		OrdererID:   "joe.example.com",
		ChannelID:   "clark-channel",
		ChaincodeID: "exp2",

		ChannelConfigPath:   os.Getenv("GOPATH") + "/src/github.com/kchristidis/exp2/fixtures/artifacts/clark-channel.tx",
		ChaincodeGoPath:     os.Getenv("GOPATH"),
		ChaincodeSourcePath: "github.com/kchristidis/exp2/chaincode/",
	}

	if err := sc.Setup(); err != nil {
		return err
	}
	defer sc.SDK.Close()

	if err := sc.Install(); err != nil {
		return err
	}

	// Get notified when a new block is added to the peer's chain

	heightChan := make(chan uint64)
	doneChan := make(chan struct{})

	go func() {
		var lastHeight uint64
		var resp *fab.BlockchainInfoResponse
		var err error
		for {
			resp, err = sc.LedgerClient.QueryInfo()
			if err != nil {
				msg := fmt.Sprintf("Unable to query ledger: %s", err.Error())
				fmt.Fprintln(os.Stdout, msg)
				close(doneChan)
				return
			}
			if tempHeight := resp.BCI.GetHeight(); tempHeight != lastHeight {
				lastHeight = tempHeight
				heightChan <- lastHeight
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		for {
			select {
			case val := <-heightChan:
				fmt.Fprintf(os.Stdout, ">>> current blockchain height: %d\n", val)
			case <-doneChan:
				fmt.Fprintln(os.Stdout, "Goroutine exiting")
				return
			}
		}
	}()

	defer close(doneChan)

	// Start the simulation

	println()

	var bid []byte

	bid, _ = json.Marshal([]float64{6.5, 2})
	_, err := sc.Invoke(2, "buy", bid)
	if err != nil {
		return err
	}

	bid, _ = json.Marshal([]float64{10, 2})
	sc.Invoke(2, "buy", bid)

	bid, _ = json.Marshal([]float64{6.5, 2})
	sc.Invoke(2, "sell", bid)

	bid, _ = json.Marshal([]float64{11, 2})
	sc.Invoke(2, "sell", bid)

	resp, _ := sc.Invoke(2, "markEnd", []byte("prvKey"))
	fmt.Fprintf(os.Stdout, "%s\n", resp)

	bid, _ = json.Marshal([]float64{6.5, 2})
	sc.Invoke(2, "sell", bid)

	/* if resp, err := sc.Query(2, "bid"); err != nil {
		return err
	} else {
		fmt.Fprintf(os.Stdout, "Response from chaincode query for '2/bid': %s\n", resp)
	} */

	fmt.Fprintf(os.Stdout, "Run completed successfully 😎")

	println()

	return nil
}
