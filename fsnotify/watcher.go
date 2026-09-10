package fsnotify

import (
	"io/fs"
	"sync"
	"time"
)

var (
	// CheckInterval is how often Start rescans the watched paths.
	//
	// Set it once, during init or before the first Watcher is started. Start
	// reads it as it comes up, so writing it while any watcher goroutine is
	// live is a data race — it is a startup knob, not a runtime control.
	CheckInterval = 3 * time.Second
)

// Watcher polls a fs.FS on a fixed interval and reports file changes on
// Events, and walk failures on Errors.
//
// A Watcher is single-use. NewWatcher creates one, Add registers the paths to
// poll, Start runs the poll loop, and Stop terminates it permanently. A
// stopped Watcher cannot be restarted; create a new one instead.
type Watcher struct {
	mu         sync.Mutex
	fsys       fs.FS
	files      map[string]*File
	checkTimes uint64
	list       []string

	// done is closed by Stop, never sent to. Closing broadcasts, which is
	// what lets Start and an in-flight check both observe the shutdown —
	// a single-value send could only ever wake one of them.
	done      chan struct{}
	stopOnce  sync.Once
	closeOnce sync.Once

	Events chan Event
	Errors chan error
}

func NewWatcher(fsys fs.FS) *Watcher {
	return &Watcher{
		fsys:   fsys,
		files:  make(map[string]*File),
		Events: make(chan Event),
		Errors: make(chan error),
		done:   make(chan struct{}),
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

// Start runs the poll loop, checking every watched path once per
// CheckInterval and delivering changes on Events. It blocks until Stop is
// called, so it is normally run in its own goroutine.
//
// Start closes Events and Errors before returning. check, which Start calls,
// is the only sender on both channels, so closing them here — on the sending
// side, after the last possible send — is what keeps the close race-free.
// Stop deliberately does not close them.
//
// Start on an already-stopped Watcher returns immediately without polling.
// Calling Start more than once on the same Watcher is not supported.
func (w *Watcher) Start() {
	defer w.closeOnce.Do(func() {
		close(w.Events)
		close(w.Errors)
	})

	select {
	case <-w.done:
		return
	default:
	}

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

// Stop terminates the Watcher. It never blocks, and it is safe to call
// concurrently, from any goroutine, and more than once — every call after the
// first is a no-op.
//
// Stop is terminal: it is this package's Close. A stopped Watcher cannot be
// resumed, and a later Start returns immediately. Create a new Watcher with
// NewWatcher if you need to watch again.
//
// Stop only signals. Start is what closes Events and Errors on its way out,
// which is how a consumer ranging over those channels learns to shut down. If
// Start was never called the channels stay open, because Start owns them.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.done)
	})
}

// send delivers ev on Events, and reports whether it was delivered. It gives
// up if the Watcher has been stopped.
//
// Events is unbuffered and check holds w.mu across the whole scan, so a bare
// send here would park forever the moment the consumer goes away — taking
// Start's ability to ever observe done with it. Selecting on done is what
// keeps Stop non-blocking.
func (w *Watcher) send(ev Event) bool {
	select {
	case <-w.done:
		return false
	case w.Events <- ev:
		return true
	}
}

// sendErr delivers err on Errors, and reports whether it was delivered. It
// gives up if the Watcher has been stopped. See send.
func (w *Watcher) sendErr(err error) bool {
	select {
	case <-w.done:
		return false
	case w.Errors <- err:
		return true
	}
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

				if !w.send(Event{Name: path, Op: Write}) {
					return fs.SkipAll
				}
				return nil

			}

			w.files[path] = &File{
				ModTime:    fi.ModTime(),
				CheckTimes: w.checkTimes,
			}
			if !w.send(Event{Name: path, Op: Create}) {
				return fs.SkipAll
			}

			return nil
		})

		if err != nil {
			if !w.sendErr(err) {
				return
			}
		}
	}

	for n, t := range w.files {
		if t.CheckTimes < w.checkTimes {
			delete(w.files, n)
			if !w.send(Event{Name: n, Op: Remove}) {
				return
			}
		}
	}
}
