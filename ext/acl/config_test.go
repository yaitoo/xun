package acl

import (
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConfig exercises the file-driven config loader across three phases:
// invalid path, valid path, and reload after mutation. The whole test runs
// inside a synctest bubble so the 1-second reload interval is driven by
// fake time and the test completes in milliseconds of real wall-clock time.
//
// synctest forbids t.Run inside the bubble (subtests would spawn goroutines
// outside the bubble's control), so the three phases are sequential blocks
// separated by `// phase:` comments rather than t.Run subtests.
//
// We bypass New() and invoke loadOptions + watch directly so the watcher
// can use a bubble-local stop channel; the package-level stop variable is
// created outside any bubble and would block fake-clock progression.
func TestConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		lastMod := time.Now()

		fsys := fstest.MapFS{
			"acl.ini": &fstest.MapFile{
				Data: []byte(`
[allow_hosts]
abc.com

[host_whitelist]
/allow
/admin

[host_redirect]
url=http://abc.com
status_code=301

[allow_ipnets]
172.0.0.0/24
192.0.0.1
[deny_ipnets]
192.0.0.1/24
169
[allow_countries]
cn
[deny_countries]
us
*
`),
				ModTime: lastMod,
			},
		}

		rawGetLastMod := getLastMod
		rawOpenFile := openFile

		getLastMod = func(file string) time.Time {
			mu.Lock()
			defer mu.Unlock()

			if file == "not_found" {
				return rawGetLastMod(file)
			}

			return lastMod
		}

		openFile = func(file string) (fs.File, error) {
			mu.Lock()
			defer mu.Unlock()

			if file == "not_found" {
				return rawOpenFile(file)
			}

			return fsys.Open(file)
		}
		ReloadInterval = 1 * time.Second

		// phase: invalid — config path doesn't exist; loader returns
		// defaults; no watcher needed since there's nothing to reload.
		o := NewOptions()
		loadOptions("not_found", o)
		v.Store(o)

		require.Len(t, o.AllowHosts, 0)
		require.Empty(t, o.HostRedirectURL)
		require.Equal(t, 302, o.HostRedirectStatusCode)
		require.Len(t, o.HostWhitelist, 0)

		require.Len(t, o.AllowIPNets, 0)
		require.Len(t, o.DenyIPNets, 0)

		require.Len(t, o.AllowCountries.Items, 0)
		require.True(t, !o.AllowCountries.HasAny)

		require.Len(t, o.DenyCountries.Items, 0)
		require.True(t, !o.DenyCountries.HasAny)

		// phase: load — switch to a real config; v reflects the parsed
		// ini immediately because loadOptions runs synchronously.
		o = NewOptions()
		loadOptions("acl.ini", o)
		v.Store(o)

		_, ok := o.AllowHosts["abc.com"]
		require.True(t, ok)
		require.Len(t, o.AllowHosts, 1)
		require.Equal(t, "http://abc.com", o.HostRedirectURL)
		require.Equal(t, 301, o.HostRedirectStatusCode)
		require.Len(t, o.HostWhitelist, 2)
		require.Equal(t, "/allow", o.HostWhitelist[0])
		require.Equal(t, "/admin", o.HostWhitelist[1])

		require.Len(t, o.AllowIPNets, 2)
		require.Equal(t, ParseIPNet("172.0.0.0/24"), o.AllowIPNets[0])
		require.Equal(t, ParseIPNet("192.0.0.1"), o.AllowIPNets[1])

		require.Len(t, o.DenyIPNets, 1)
		require.Equal(t, ParseIPNet("192.0.0.1/24"), o.DenyIPNets[0])

		require.Len(t, o.AllowCountries.Items, 1)
		_, ok = o.AllowCountries.Items["cn"]
		require.True(t, ok)
		require.True(t, !o.AllowCountries.HasAny)

		require.Len(t, o.DenyCountries.Items, 2)
		_, ok = o.DenyCountries.Items["us"]
		require.True(t, ok)
		require.True(t, o.DenyCountries.HasAny)

		// phase: reload — start the watcher with a bubble-local stop
		// channel, mutate the file, and let fake time advance. The
		// watcher captured lastMod at fake t=0 when it started; in
		// synctest fake time does not advance while the test is busy
		// with assertions, so without first waiting for the watcher to
		// be durably blocked and bumping fake time forward, the new
		// lastMod would equal the captured one and reload would never
		// fire.
		stopCh := make(chan struct{})
		go watch("acl.ini", &v, stopCh)

		synctest.Wait()
		time.Sleep(time.Microsecond)

		fsys["acl.ini"].Data = []byte(`
[allow_hosts]
123.com

[allow_ipnets]
; 172.0.0.1
[deny_ipnets]
192.0.0.1/24
172.0.0.1
[allow_countries]
cn
# us
[host_redirect]
url=http://123.com
status_code=302

[host_whitelist]
/status
`)

		mu.Lock()
		lastMod = time.Now()
		mu.Unlock()

		// Inside the bubble, fake time advances when every goroutine
		// is durably blocked. The watcher sits on a 1s ticker in a
		// select, so this 2-second Sleep advances fake time by 2s
		// (the ticker fires once at fake t=1s, the watcher reloads,
		// returns to the select, and fake time continues), then the
		// Sleep returns with v holding the new config — all without
		// a single real wall-clock second of waiting.
		time.Sleep(2 * time.Second)

		o = v.Load().(*Options)

		_, ok = o.AllowHosts["123.com"]
		require.True(t, ok)
		require.Len(t, o.AllowHosts, 1)
		require.Equal(t, "http://123.com", o.HostRedirectURL)
		require.Equal(t, 302, o.HostRedirectStatusCode)
		require.Len(t, o.HostWhitelist, 1)
		require.Equal(t, "/status", o.HostWhitelist[0])

		require.Len(t, o.AllowIPNets, 0)

		require.Len(t, o.DenyIPNets, 2)
		require.Equal(t, ParseIPNet("192.0.0.1/24"), o.DenyIPNets[0])
		require.Equal(t, ParseIPNet("172.0.0.1"), o.DenyIPNets[1])

		require.Len(t, o.AllowCountries.Items, 1)
		_, ok = o.AllowCountries.Items["cn"]
		require.True(t, ok)
		require.True(t, !o.AllowCountries.HasAny)

		require.Len(t, o.DenyCountries.Items, 0)
		require.True(t, !o.DenyCountries.HasAny)

		close(stopCh)
	})
}
