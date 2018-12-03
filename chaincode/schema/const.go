package schema

const (
	EnableEvents  = false     // Used to enable/disable emission of events
	ExpNum        = 1         // Identifies the experiment this chaincode is running [EDITME]
	PostKeySuffix = "privkey" // The suffix we use for the write-key in `postKey` calls. Separate with the prefix using a dash.
	TraceLength   = 35036     // Used to size the metrics variable
)
