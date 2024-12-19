package fsnotify

import "time"

type File struct {
	ModTime    time.Time
	CheckTimes uint64
}
