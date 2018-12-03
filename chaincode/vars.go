package main

import (
	"io"

	"github.com/kchristidis/island/chaincode/schema"
)

var (
	metricsOutputVal schema.MetricsOutput // A singleton that gets populated with metrics during the lifecycle of the chaincode
	w                io.Writer            // Write all messages to this file
)
