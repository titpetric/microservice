package stats

import (
	"errors"
)

var (
	errFlusherDisabled = errors.New("Flusher is disabled, shutting down")
)
