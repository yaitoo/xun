package acl

import (
	"bufio"
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

	for {
		<-ticker.C

		fi, err := os.Stat(file)
		if err != nil {
			continue
		}

		if fi.ModTime().After(lastMod) {
			f, err := os.OpenFile(file, os.O_RDONLY, 0644)
			if err == nil {
				n := loadOptions(bufio.NewScanner(f))
				v.Store(n)

				f.Close()

				lastMod = fi.ModTime()
			}
		}
	}
}
