package acl

import (
	"os"
	"sync/atomic"
	"time"
)

var (
	ReloadInterval = 1 * time.Minute
	zeroTime       time.Time
)

var getLastMod = func(file string) time.Time {
	fi, _ := os.Stat(file)
	if fi == nil {
		return zeroTime
	}

	return fi.ModTime()
}

func watch(file string, v atomic.Value) {
	lastMod := getLastMod(file)

	ticker := time.NewTicker(ReloadInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		modTime := getLastMod(file)

		if modTime.After(lastMod) {

			o := v.Load().(*Options)
			n := NewOptions()

			n.Config = file
			n.ViewerName = o.ViewerName
			n.LookupFunc = o.LookupFunc

			if loadOptions(file, n) {
				v.Store(n)
				lastMod = modTime
			}
		}
	}
}
