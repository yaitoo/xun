package fsnotify

import (
	"io/fs"
	"sync"
	"time"
)

const (
	CheckInterval = 3 * time.Second
)

type Watcher struct {
	mu         sync.Mutex
	fsys       fs.FS
	files      map[string]*File
	checkTimes uint64
	list       []string
	done       chan struct{}
	Events     chan Event
	Errors     chan error
}

func NewWatcher(fsys fs.FS) *Watcher {
	return &Watcher{
		fsys:   fsys,
		files:  make(map[string]*File),
		Events: make(chan Event),
		Errors: make(chan error),
	}
}

func (w *Watcher) Add(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.list = append(w.list, path)

	return fs.WalkDir(w.fsys, path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		w.files[path] = &File{
			ModTime:    info.ModTime(),
			CheckTimes: w.checkTimes,
		}

		return nil
	})
}

func (w *Watcher) Start() {
	t := time.NewTicker(CheckInterval)
	defer t.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-t.C:
			w.check()
		}
	}
}

func (w *Watcher) Stop() {
	w.done <- struct{}{}
}

func (w *Watcher) check() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.checkTimes++

	for _, path := range w.list {
		err := fs.WalkDir(w.fsys, path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			fi, err := d.Info()
			if err != nil {
				return err
			}

			f, ok := w.files[path]
			if ok {
				f.CheckTimes = w.checkTimes

				mt := fi.ModTime()

				if f.ModTime.Equal(mt) {

					return nil
				}

				f.ModTime = mt

				w.Events <- Event{
					Name: path,
					Op:   Write,
				}
				return nil

			}

			w.files[path] = &File{
				ModTime:    fi.ModTime(),
				CheckTimes: w.checkTimes,
			}
			w.Events <- Event{
				Name: path,
				Op:   Create,
			}

			return nil
		})

		if err != nil {
			w.Errors <- err
		}
	}

	for n, t := range w.files {
		if t.CheckTimes < w.checkTimes {
			delete(w.files, n)
			w.Events <- Event{
				Name: n,
				Op:   Remove,
			}
		}
	}
}
