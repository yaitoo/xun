package htmx

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestIsHxRequest(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		c := &xun.Context{
			Request: httptest.NewRequest("GET", "/", nil),
		}

		require.False(t, IsHxRequest(c))
		require.False(t, IsBoosted(c))
		require.False(t, IsHistoryRestore(c))
	})

	t.Run("present", func(t *testing.T) {
		c := &xun.Context{
			Request: httptest.NewRequest("GET", "/", nil),
		}

		c.Request.Header.Set(HxRequest, "true")
		c.Request.Header.Set(HxBoosted, "true")
		c.Request.Header.Set(HxHistoryRestoreRequest, "true")
		c.Request.Header.Set(HxTarget, "main")
		c.Request.Header.Set(HxTrigger, "btn-1")
		c.Request.Header.Set(HxTriggerName, "submit")
		c.Request.Header.Set(HxPrompt, "yes")
		c.Request.Header.Set(HxCurrentUrl, "/home")

		require.True(t, IsHxRequest(c))
		require.True(t, IsBoosted(c))
		require.True(t, IsHistoryRestore(c))
		require.Equal(t, "main", Target(c))
		require.Equal(t, "btn-1", Trigger(c))
		require.Equal(t, "submit", TriggerName(c))
		require.Equal(t, "yes", Prompt(c))
		require.Equal(t, "/home", CurrentURL(c))
	})

	t.Run("non-true", func(t *testing.T) {
		c := &xun.Context{
			Request: httptest.NewRequest("GET", "/", nil),
		}

		c.Request.Header.Set(HxRequest, "false")
		c.Request.Header.Set(HxBoosted, "1")

		require.False(t, IsHxRequest(c))
		require.False(t, IsBoosted(c))
	})
}
