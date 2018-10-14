package blockchain

import (
	"fmt"
	"os"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/event"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/common/cauthdsl"
)

// SDKContext ...
type SDKContext struct {
	SDKConfigFile string

	OrgName  string
	OrgAdmin string
	UserName string

	OrdererID   string
	ChannelID   string
	ChaincodeID string

	ChannelConfigPath   string
	ChaincodeGoPath     string
	ChaincodeSourcePath string

	SDK *fabsdk.FabricSDK

	RMClient      *resmgmt.Client
	ChannelClient *channel.Client
	EventClient   *event.Client
	LedgerClient  *ledger.Client
}

// Setup ...
func (sc *SDKContext) Setup() error {
	sdk, err := fabsdk.New(config.FromFile(sc.SDKConfigFile))
	if err != nil {
		return fmt.Errorf("Failed to initialize SDK: %s", err)
	}
	sc.SDK = sdk
	fmt.Fprintln(os.Stdout, "SDK initialized")

	// The resource management client is responsible for managing channels
	// (create/update channel).
	rmcc := sc.SDK.Context(fabsdk.WithUser(sc.OrgAdmin), fabsdk.WithOrg(sc.OrgName))
	rmClient, err := resmgmt.New(rmcc)
	if err != nil {
		return fmt.Errorf("Failed to create resource management client: %s", err)
	}
	sc.RMClient = rmClient
	fmt.Fprintln(os.Stdout, "Resource management client created")

	// The MSP client allow us to retrieve user information from their
	// identity, like its signing identity which we will need to save
	// the channel.
	mspClient, err := mspclient.New(sdk.Context(), mspclient.WithOrg(sc.OrgName))
	if err != nil {
		return fmt.Errorf("Failed to create MSP client: %s", err)
	}
	fmt.Fprintln(os.Stdout, "MSP client created")

	adminID, err := mspClient.GetSigningIdentity(sc.OrgAdmin)
	if err != nil {
		return fmt.Errorf("Failed to get admin signing identity: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Admin signing identity created")

	req := resmgmt.SaveChannelRequest{
		ChannelID:         sc.ChannelID,
		ChannelConfigPath: sc.ChannelConfigPath,
		SigningIdentities: []msp.SigningIdentity{adminID},
	}

	txID, err := sc.RMClient.SaveChannel(req, resmgmt.WithOrdererEndpoint(sc.OrdererID))
	if err != nil || txID.TransactionID == "" {
		return fmt.Errorf("Failed to save channel: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Channel created")

	// Make admin user join the previously created channel
	if err = sc.RMClient.JoinChannel(
		sc.ChannelID,
		resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(sc.OrdererID),
	); err != nil {
		return fmt.Errorf("Failed to make admin join channel: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Channel joined")

	fmt.Fprintln(os.Stdout, "Setup completed")

	return nil
}

// Install ...
func (sc *SDKContext) Install() error {
	pkg, err := packager.NewCCPackage(sc.ChaincodeSourcePath, sc.ChaincodeGoPath)
	if err != nil {
		return fmt.Errorf("Failed to create chaincode package: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Chaincode package created")

	req := resmgmt.InstallCCRequest{
		Name:    sc.ChaincodeID,
		Path:    sc.ChaincodeSourcePath,
		Version: "0",
		Package: pkg,
	}

	if _, err := sc.RMClient.InstallCC(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts)); err != nil {
		return fmt.Errorf("Failed to install chaincode: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Chaincode installed")

	// Set up chaincode policy
	pol := cauthdsl.SignedByAnyMember([]string{"clark.example.com"})

	resp, err := sc.RMClient.InstantiateCC(sc.ChannelID, resmgmt.InstantiateCCRequest{
		Name:    sc.ChaincodeID,
		Path:    sc.ChaincodeGoPath,
		Version: "0",
		Args:    [][]byte{[]byte("init")},
		Policy:  pol,
	})
	if err != nil || resp.TransactionID == "" {
		return fmt.Errorf("Failed to instantiate the chaincode: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Chaincode instantiated")

	cc := sc.SDK.ChannelContext(sc.ChannelID, fabsdk.WithUser(sc.UserName))
	ccl, err := channel.New(cc)
	if err != nil {
		return fmt.Errorf("Failed to create channel client: %s", err)
	}
	sc.ChannelClient = ccl
	fmt.Fprintln(os.Stdout, "Channel client created")

	ec, err := event.New(cc)
	if err != nil {
		return fmt.Errorf("Failed to create event client: %s", err)
	}
	sc.EventClient = ec
	fmt.Fprintln(os.Stdout, "Event client created")

	lc, err := ledger.New(cc)
	if err != nil {
		return fmt.Errorf("Failed to create ledger client: %s", err)
	}
	sc.LedgerClient = lc
	fmt.Fprintln(os.Stdout, "Ledger client created")

	fmt.Fprintln(os.Stdout, "Chaincode installation & instantiation completed")
	return nil
}
