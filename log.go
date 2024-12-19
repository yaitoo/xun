package htmx

import (
	"strconv"
	"sync/atomic"
	"time"
)

var (
	logPrefix = strconv.FormatInt(time.Now().UnixMilli(), 36) + "-"
	logID     int64
)

func nextLogID() string {
	return logPrefix + strconv.FormatInt(atomic.AddInt64(&logID, 1), 36)
}
