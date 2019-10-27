package schema

import "time"

// Experiments parameters
const (
	ExpNum = 3 // Identifies the experiment this chaincode is running.

	Alpha      = 10 // The factor by which we multiply the binary exponential backoff calculation result.
	RetryCount = 2  // The maximum number of times an agent should repeat a failed chaincode invocation.

	BatchTimeout  = 200 * time.Millisecond // The BatchTimeout value for this channel. ATTN: The value here should be synced with the one in configtx.yaml.
	BlocksPerSlot = 140                    // How many blocks constitute a slot?
	BlockOffset   = 70                     // How many blocks into a slot should the 'PostKey' notification come up?
	ClockPeriod   = 100 * time.Millisecond // How often do we invoke the clock method to help with the creation of new blocks?
	SleepDuration = 100 * time.Millisecond // How often do we check for new blocks?

	PostKeySuffix = "privkey" // The suffix we use for the write-key in `postKey` calls. Separate with the prefix using a dash.
	TraceLength   = 35036     // Used to size the metrics variable.
	EnableEvents  = false     // Used to enable/disable the emission of chaincode events.

	StagingLevel = Debug // Identifies the staging level for the experiment.
)

// Level identifies a staging level.
type Level int

// Supported staging levels.
const (
	Debug Level = iota
	Prod
)
