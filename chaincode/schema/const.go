package schema

const (
	BlocksPerSlot = 30        // How many blocks constitute a slot?
	BlockOffset   = 20        // How many blocks into a slot should the 'PostKey' notification come up?
	EnableEvents  = false     // Used to enable/disable emission of events
	ExpNum        = 1         // Identifies the experiment this chaincode is running [EDITME]
	PostKeySuffix = "privkey" // The suffix we use for the write-key in `postKey` calls. Separate with the prefix using a dash.
	RetryCount    = 2         // The maximum number of times an agent should repeat a failed chaincode invocation.
	TraceLength   = 35036     // Used to size the metrics variable
)
