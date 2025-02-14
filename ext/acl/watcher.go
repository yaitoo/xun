package acl

import (
	"os"
	"sync/atomic"
	"time"
)

var ConfigCheckInterval = 1 * time.Minute

func watch(file string, v atomic.Value) {

	fi, _ := os.Stat(file)

	lastMod := fi.ModTime()

	ticker := time.NewTicker(ConfigCheckInterval)
	defer ticker.Stop()

	o := v.Load().(*Options)
	for {
		<-ticker.C

		fi, err := os.Stat(file)
		if err != nil {
			continue
		}

		if fi.ModTime().After(lastMod) {

			// n := &Options{}

			v.Store(&o)

			lastMod = fi.ModTime()
		}

	}

}
