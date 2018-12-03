package cmap

import "errors"

// ErrInvalidCapacity is the error returned when initializing a Map with an
// invalid capacity parameter.
var ErrInvalidCapacity = errors.New("Capacity should be a positive integer")
