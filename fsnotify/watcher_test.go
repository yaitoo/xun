package fsnotify

import (
	"testing"
	"testing/fstest"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests run inside a synctest bubble, which buys three things:
//
//   - A fake clock, so CheckInterval elapses instantly. That is why none of
//     these tests touch the global — the default 3s is already free here.
//   - synctest.Test waits for every goroutine in the bubble to exit and fails
//     the test on deadlock, so "Start leaked" and "Stop blocked forever" are
//     caught by the harness rather than by a timeout we invent.
//   - synctest.Wait, which returns once every other goroutine is durably
//     blocked — the precise state these tests want to assert on, instead of
//     sleeping and hoping.

// TestCheckDetectsCreateWriteRemove covers the change-detection logic that
// used to be exercised only indirectly, through the App-level watch tests.
func TestCheckDetectsCreateWriteRemove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		base := time.Now()
		fsys := fstest.MapFS{
			"keep.txt":   {Data: []byte("keep"), ModTime: base},
			"change.txt": {Data: []byte("v1"), ModTime: base},
			"gone.txt":   {Data: []byte("bye"), ModTime: base},
		}

		w := NewWatcher(fsys)
		require.NoError(t, w.Add("."))

		// Mutate before Start so the poll loop never races these map writes.
		fsys["change.txt"] = &fstest.MapFile{Data: []byte("v2"), ModTime: base.Add(time.Second)}
		fsys["added.txt"] = &fstest.MapFile{Data: []byte("new"), ModTime: base.Add(time.Second)}
		delete(fsys, "gone.txt")

		go w.Start()
		defer w.Stop()

		// Outlives several ticks of the fake clock, so a missing event fails
		// here with a useful message rather than spinning the bubble forever.
		timeout := time.After(5 * CheckInterval)

		got := make(map[string]Op)
		for len(got) < 3 {
			select {
			case ev := <-w.Events:
				got[ev.Name] = ev.Op
			case <-timeout:
				t.Fatalf("timed out waiting for events, got %v", got)
			}
		}

		require.Equal(t, Write, got["change.txt"])
		require.Equal(t, Create, got["added.txt"])
		require.Equal(t, Remove, got["gone.txt"])
		require.NotContains(t, got, "keep.txt", "unchanged file must not emit an event")
	})
}

func TestStopIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := NewWatcher(fstest.MapFS{})

		// Before #132 Stop was an unbuffered send, so the second call had no
		// receiver and parked forever. That now shows up as a bubble deadlock.
		w.Stop()
		w.Stop()
		w.Stop()
	})
}

// TestStopBeforeStart pins that stopping a Watcher that was never started is
// safe, and that a later Start is a no-op rather than a fresh poll loop.
func TestStopBeforeStart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := NewWatcher(fstest.MapFS{
			"a.txt": {Data: []byte("a"), ModTime: time.Now()},
		})
		require.NoError(t, w.Add("."))

		w.Stop()

		// Running in the root goroutine on purpose: if Start polls instead of
		// returning, the bubble never drains and the test fails.
		w.Start()
	})
}

func TestStopClosesEventsAndErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := NewWatcher(fstest.MapFS{
			"a.txt": {Data: []byte("a"), ModTime: time.Now()},
		})
		require.NoError(t, w.Add("."))

		go w.Start()

		w.Stop()
		synctest.Wait()

		// Start closes both channels on its way out; that is what lets a
		// consumer ranging over them shut down.
		_, ok := <-w.Events
		require.False(t, ok, "Events must be closed once Start returns")

		_, ok = <-w.Errors
		require.False(t, ok, "Errors must be closed once Start returns")
	})
}

// TestStopUnblocksPendingSend is the deadlock regression test. Events is
// unbuffered and check holds w.mu across the whole scan, so a check that is
// mid-send with no consumer used to park forever — taking Start's ability to
// ever observe done with it.
func TestStopUnblocksPendingSend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		base := time.Now()
		fsys := fstest.MapFS{"a.txt": {Data: []byte("a"), ModTime: base}}

		w := NewWatcher(fsys)
		require.NoError(t, w.Add("."))

		// Queue a change before Start, then never drain Events.
		fsys["a.txt"] = &fstest.MapFile{Data: []byte("b"), ModTime: base.Add(time.Second)}

		go w.Start()

		// Let the fake clock reach the first tick, then wait for the poll
		// goroutine to go durably blocked. With nobody draining Events, the
		// only place it can be blocked is the send inside check — exactly the
		// state that used to be unrecoverable.
		time.Sleep(2 * CheckInterval)
		synctest.Wait()

		w.Stop()
	})
}
