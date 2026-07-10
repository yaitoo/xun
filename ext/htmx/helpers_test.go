package htmx

import (
	"encoding/json"
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

func TestWriteHelpers(t *testing.T) {
	t.Run("trigger string", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{
			Response: xun.NewResponseWriter(w),
		}

		WriteTrigger(c, HxTrigger, "item-added")
		require.Equal(t, "item-added", w.Header().Get(HxTrigger))
	})

	t.Run("trigger detail", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{
			Response: xun.NewResponseWriter(w),
		}

		WriteTrigger(c, HxTriggerAfterSettle, HxHeader[string]{"item-added": "abc"})
		value := w.Header().Get(HxTriggerAfterSettle)

		var got HxHeader[string]
		require.NoError(t, json.Unmarshal([]byte(value), &got))
		require.Equal(t, "abc", got["item-added"])
	})

	t.Run("redirect", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{
			Response: xun.NewResponseWriter(w),
		}

		WriteRedirect(c, "/dashboard")
		require.Equal(t, "/dashboard", w.Header().Get(HxRedirect))
		require.Equal(t, 200, w.Code)
	})

	t.Run("refresh", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{
			Response: xun.NewResponseWriter(w),
		}

		WriteRefresh(c)
		require.Equal(t, "true", w.Header().Get(HxRefresh))
	})

	t.Run("location", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{
			Response: xun.NewResponseWriter(w),
		}

		WriteLocation(c, HxHeader[string]{"path": "/home", "source": "xun"})
		value := w.Header().Get(HxLocation)

		var got HxHeader[string]
		require.NoError(t, json.Unmarshal([]byte(value), &got))
		require.Equal(t, "/home", got["path"])
		require.Equal(t, "xun", got["source"])
	})
}

func TestWriteRetarget(t *testing.T) {
	w := httptest.NewRecorder()
	c := &xun.Context{
		Response: xun.NewResponseWriter(w),
	}

	WriteRetarget(c, "#errors")
	require.Equal(t, "#errors", w.Header().Get(HxRetarget))
}

func TestWriteSwapOverrides(t *testing.T) {
	t.Run("push url current", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{Response: xun.NewResponseWriter(w)}

		WritePushUrl(c)
		require.Equal(t, "true", w.Header().Get(HxPushUrl))
	})

	t.Run("push url to", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{Response: xun.NewResponseWriter(w)}

		WritePushUrlTo(c, "/items/42")
		require.Equal(t, "/items/42", w.Header().Get(HxPushUrl))
	})

	t.Run("replace url current", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{Response: xun.NewResponseWriter(w)}

		WriteReplaceUrl(c)
		require.Equal(t, "true", w.Header().Get(HxReplaceUrl))
	})

	t.Run("replace url to", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{Response: xun.NewResponseWriter(w)}

		WriteReplaceUrlTo(c, "/items/42")
		require.Equal(t, "/items/42", w.Header().Get(HxReplaceUrl))
	})

	t.Run("reswap", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{Response: xun.NewResponseWriter(w)}

		WriteReswap(c, "outerHTML swap:200ms")
		require.Equal(t, "outerHTML swap:200ms", w.Header().Get(HxReswap))
	})

	t.Run("reselect", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := &xun.Context{Response: xun.NewResponseWriter(w)}

		WriteReselect(c, "#content")
		require.Equal(t, "#content", w.Header().Get(HxReselect))
	})
}
