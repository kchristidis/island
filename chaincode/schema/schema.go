package schema

// BidInput is the type that we expect the `oc.args.Data`
// JSON-encoded argument to a `bid` call to decode to.
type BidInput struct {
	PricePerUnitInCents float64
	QuantityInKWh       float64
}

// BidOutput is the type that we encapsulate `bid`'s successful response in.
// It is encoded as a JSON object and returned to the user via the `shim.Success` method.
type BidOutput struct {
	WriteKeyAttrs []string
}

// ClockOutput is the type that we encapsulate `clock`'s successful response in.
// It is encoded as a JSON object and returned to the user via the `shim.Success` method.
type ClockOutput struct {
	WriteKeyAttrs []string
}

// OpContextInput is the type that we encapsulate the invocation/query arguments into.
// It is encoded as a JSON objected during the `Invoke`/`Query` call.
type OpContextInput struct {
	EventID string
	Action  string
	Slot    int    // Not needed for `metrics` query, or `clock`
	Data    []byte // Not needed for `metrics` query, or `clock`
}

// MarkEndInput is the type that we expect the `oc.args.Data`
// JSON-encoded argument to a `markEnd` call to decode to.
type MarkEndInput struct {
	PrivKey []byte // Not needed for experiments 1, 3
}

// MarkEndOutput is the type that we encapsulate `markEnd`'s successful response in.
// It is encoded as a JSON object and:
// - persisted in the chaincode's write-key
// - returned to the user via the `shim.Success` method
type MarkEndOutput struct {
	WriteKeyAttrs       []string
	PrivKey             []byte // Not needed for experiments 1, 3
	PricePerUnitInCents float64
	QuantityInKWh       float64
	Slot                int
	Message             string
}

// MetricsOutput is the type that we encapsulate `metrics`'s successful response in.
// It is encoded as a JSON object and returned to the user via the `shim.Success` method.
// It is populated during the bidding process.
type MetricsOutput struct {
	LateTXsCount, LateBuysCount, LateSellsCount                             [TraceLength]int
	LateDecryptsCount                                                       [TraceLength]int
	ProblematicIterCount, ProblematicMarshalCount                           [TraceLength]int
	ProblematicDecryptCount, ProblematicBidCalcCount                        [TraceLength]int
	ProblematicKeyCount, ProblematicGetStateCount, ProblematicPutStateCount [TraceLength]int
	// DuplTXsCount, DuplBuysCount, DuplSellsCount [TraceLength]int
}

// PostKeyInput is the type that we expect the `oc.args.Data`
// JSON-encoded argument to a `postKey` call to decode to.
type PostKeyInput struct {
	ReadKeyAttrs []string
	PrivKey      []byte
	BidEventID   string // Used in exp 1
}

// PostKeyOutput is the type that we encapsulate `postKey`'s successful response in.
// It is encoded as a JSON object and returned to the user via the `shim.Success` method.
type PostKeyOutput struct {
	WriteKeyAttrs []string
	PrivKey       []byte
}

// SlotOutput is the type that we encapsulate `slot`'s successful response in.
// It is encoded as a JSON object and returned to the user via the `shim.Success` method.
type SlotOutput struct {
	Values [][]byte
}
