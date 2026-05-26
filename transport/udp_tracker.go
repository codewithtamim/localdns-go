package transport

import (
	"sync/atomic"
	"time"
)

type tracker struct {
	start    time.Time
	upload   atomic.Int64
	download atomic.Int64
}

func makeTracker() *tracker {
	return &tracker{
		start: time.Now(),
	}
}
