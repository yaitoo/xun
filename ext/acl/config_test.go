package acl

import (
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {

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
			ModTime: lastMod},
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

	t.Run("invalid", func(t *testing.T) {
		New(WithConfig("not_found"))

		o := v.Load().(*Options)

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
	})

	stop <- struct{}{}

	New(WithConfig("acl.ini"))
	t.Run("load", func(t *testing.T) {
		o := v.Load().(*Options)

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
	})

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

	t.Run("reload", func(t *testing.T) {
		time.Sleep(2 * time.Second)

		o := v.Load().(*Options)

		_, ok := o.AllowHosts["123.com"]
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
	})

	stop <- struct{}{}
}
