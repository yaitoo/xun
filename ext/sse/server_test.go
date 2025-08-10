package sse

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/xun"
)

func TestServer(t *testing.T) {
	t.Run("join", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		c1, id, ok, err := srv.Join("join", nil)
		require.Nil(t, c1)
		require.Equal(t, 0, id)
		require.False(t, ok)
		require.ErrorIs(t, err, ErrNotStreamer)

		c1, id, ok, err = srv.Join("join", &notFlusher{})
		require.Nil(t, c1)
		require.Equal(t, 0, id)
		require.False(t, ok)
		require.ErrorIs(t, err, ErrNotStreamer)

		c1, id, ok, err = srv.Join("join", rw)
		require.NotNil(t, c1)
		require.NoError(t, err)
		require.Equal(t, 1, id)
		require.Equal(t, 1, c1.connID)
		require.True(t, ok)
		require.Nil(t, err)

		c2, id2, ok, err := srv.Join("join", rw)
		require.NotNil(t, c2)
		require.Equal(t, 2, id2)
		require.Equal(t, 2, c2.connID)
		require.Equal(t, "join", c2.ID)
		require.False(t, ok)
		require.NoError(t, err)
		require.Equal(t, c1, c2)

		c3 := srv.Get("join")
		require.Equal(t, c1, c3)

		ok = srv.Leave(c1.ID, id)
		require.False(t, ok)

		c3 = srv.Get("join")
		require.Equal(t, c1, c3)

		ok = srv.Leave(c1.ID, id2)
		require.True(t, ok)

		c3 = srv.Get("join")
		require.Nil(t, c3)

		ok = srv.Leave(c1.ID, id2)
		require.True(t, ok)
	})

	t.Run("send", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		c, _, _, err := srv.Join("send", rw)
		require.NoError(t, err)

		r := NewReader(&readCloser{Buffer: rw.Body})
		defer r.Close()

		err = c.Send(&TextEvent{Data: "data1"})
		require.NoError(t, err)
		evt, err := r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "data1", evt.Data)

		err = c.Send(&TextEvent{ID: "id1", Data: "data1"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "id1", evt.ID)
		require.Equal(t, "", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "data1", evt.Data)

		err = c.Send(&TextEvent{ID: "id1", Name: "event1", Data: "data1"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "id1", evt.ID)
		require.Equal(t, "event1", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "data1", evt.Data)

		err = c.Send(&TextEvent{ID: "id1", Name: "event1", Retry: 1000, Data: "data1"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "id1", evt.ID)
		require.Equal(t, "event1", evt.Name)
		require.Equal(t, 1000, evt.Retry)
		require.Equal(t, "data1", evt.Data)

		err = c.Send(&PingEvent{})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "", evt.Data)

		err = c.Send(&JsonEvent{Data: "data2"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "\"data2\"", evt.Data)

		err = c.Send(&JsonEvent{ID: "id2", Data: "data2"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "id2", evt.ID)
		require.Equal(t, "", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "\"data2\"", evt.Data)

		err = c.Send(&JsonEvent{ID: "id2", Name: "event2", Data: "data2"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "id2", evt.ID)
		require.Equal(t, "event2", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "\"data2\"", evt.Data)

		err = c.Send(&JsonEvent{ID: "id2", Name: "event2", Retry: 1000, Data: "data2"})
		require.NoError(t, err)
		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "id2", evt.ID)
		require.Equal(t, "event2", evt.Name)
		require.Equal(t, 1000, evt.Retry)
		require.Equal(t, "\"data2\"", evt.Data)
	})

	t.Run("broadcast", func(t *testing.T) {
		srv := New()

		rw1 := httptest.NewRecorder()
		rw2 := httptest.NewRecorder()

		c1, _, _, err := srv.Join("c1", rw1)
		require.NotNil(t, c1)
		require.NoError(t, err)

		c2, _, _, err := srv.Join("c2", rw2)
		require.NotNil(t, c2)
		require.NoError(t, err)

		errs, err := srv.Broadcast(context.TODO(), &TextEvent{Name: "event1", Data: "data1"})
		require.NoError(t, err)
		require.Nil(t, errs)

		r1 := NewReader(&readCloser{Buffer: rw1.Body})
		defer r1.Close()
		r2 := NewReader(&readCloser{Buffer: rw2.Body})
		defer r2.Close()

		evt1, err1 := r1.Next()
		evt2, err2 := r2.Next()

		require.NoError(t, err1)
		require.NoError(t, err2)

		require.Equal(t, "event1", evt1.Name)
		require.Equal(t, "data1", evt1.Data)
		require.Equal(t, "event1", evt2.Name)
		require.Equal(t, "data1", evt2.Data)

		ctx, cancel := context.WithCancel(context.TODO())
		cancel()

		_, err = srv.Broadcast(ctx, &TextEvent{Name: "event1", Data: "data1"})
		require.ErrorIs(t, err, context.Canceled)

		evt1, err1 = r1.Next()
		evt2, err2 = r2.Next()

		require.ErrorIs(t, err1, io.EOF)
		require.ErrorIs(t, err2, io.EOF)

	})

	t.Run("fail_to_send", func(t *testing.T) {
		srv := New()

		rw := &streamerMock{
			ResponseWriter: httptest.NewRecorder(),
		}

		c, id, _, err := srv.Join("invalid", rw)
		require.NoError(t, err)

		err = c.Send(&JsonEvent{Name: "event1", Data: make(chan int)})
		require.Error(t, err)

		err = c.Send(&TextEvent{Name: "event1"})
		require.Error(t, err)

		srv.Leave(c.ID, id)

		err = c.Send(&TextEvent{Name: "event1"})
		require.ErrorIs(t, err, ErrServerClosed)

		c, id, _, err = srv.Join("invalid", rw)
		require.NoError(t, err)
		require.NotNil(t, c)
		require.Equal(t, 1, id)

		ctx, cf := context.WithCancelCause(context.Background())
		cf(context.Canceled)

		_, err = srv.Broadcast(ctx, &TextEvent{Name: "event1", Data: "data1"})
		require.ErrorIs(t, err, context.Canceled)

	})

	t.Run("shutdown", func(t *testing.T) {
		srv := New()

		c1, _, _, err := srv.Join("c1", httptest.NewRecorder())
		require.NoError(t, err)
		require.NotNil(t, c1)

		srv.Shutdown()

		err = context.Cause(c1.Context())
		require.ErrorIs(t, err, ErrServerClosed)

		require.Len(t, srv.clients, 0)
	})

	t.Run("client_wait", func(t *testing.T) {
		srv := New()

		rw := httptest.NewRecorder()

		c, _, _, err := srv.Join("closed", rw)
		require.NoError(t, err)
		ctx, cf := context.WithCancel(context.Background())

		c.Close()

		err = c.Wait(ctx)
		require.ErrorIs(t, err, ErrServerClosed)

		c2, _, _, err := srv.Join("cancelled", rw)
		require.NoError(t, err)
		cf()

		err = c2.Wait(ctx)
		require.ErrorIs(t, err, context.Canceled)

	})

	t.Run("send_with_gzip", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		gc := &xun.GzipCompressor{}

		c, _, _, err := srv.Join("send", gc.New(rw))
		require.NoError(t, err)

		err = c.Send(&TextEvent{Name: "event1", Data: "data1"})
		require.NoError(t, err)

		gr, _ := gzip.NewReader(rw.Body)

		r := NewReader(gr)

		evt, err := r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "event1", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "data1", evt.Data)

		err = c.Send(&JsonEvent{Name: "event2", Data: "data2"})
		require.NoError(t, err)

		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "event2", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "\"data2\"", evt.Data)
	})

	t.Run("send_with_deflate", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		gc := &xun.DeflateCompressor{}

		c, _, _, err := srv.Join("send", gc.New(rw))
		require.NoError(t, err)

		err = c.Send(&TextEvent{Name: "event1", Data: "data1"})
		require.NoError(t, err)

		fr := flate.NewReader(rw.Body)

		r := NewReader(fr)

		evt, err := r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "event1", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "data1", evt.Data)

		err = c.Send(&JsonEvent{Name: "event2", Data: "data2"})
		require.NoError(t, err)

		evt, err = r.Next()
		require.NoError(t, err)
		require.Equal(t, "", evt.ID)
		require.Equal(t, "event2", evt.Name)
		require.Equal(t, 0, evt.Retry)
		require.Equal(t, "\"data2\"", evt.Data)
	})

	t.Run("timeout", func(t *testing.T) {
		srv := New(WithClientTimeout(3 * time.Second))
		rw := &stdWriter{ResponseRecorder: httptest.NewRecorder()}

		_, _, _, err := srv.Join("keepalive", rw)
		require.NoError(t, err)

		sm := &streamerMock{
			isWorking: true,
		}
		c, _, _, err := srv.Join("timeout", sm)
		require.NoError(t, err)
		sm.mu.Lock()
		sm.isWorking = false
		sm.mu.Unlock()

		time.Sleep(2 * time.Second)
		body := rw.GetBody()
		require.Equal(t, body, ": ping\n\n\n")
		time.Sleep(2 * time.Second)

		err = context.Cause(c.Context())

		require.ErrorIs(t, err, ErrClientTimeout)

	})
}

type notFlusher struct {
}

func (*notFlusher) Header() http.Header {
	return http.Header{}
}

func (*notFlusher) Write([]byte) (int, error) {
	return 0, errors.New("mock: invalid")
}

func (*notFlusher) WriteHeader(int) {}

type streamerMock struct {
	mu        sync.Mutex
	isWorking bool
	http.ResponseWriter
}

func (*streamerMock) Header() http.Header {
	return http.Header{}
}

func (s *streamerMock) Write(buf []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isWorking {
		return len(buf), nil
	}

	return 0, errors.New("mock: invalid")
}

func (*streamerMock) Flush() {}

type readCloser struct {
	*bytes.Buffer
}

func (*readCloser) Close() error {
	return nil
}

type stdWriter struct {
	mu sync.Mutex
	*httptest.ResponseRecorder
}

func (s *stdWriter) Write(buf []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Body.Write(buf)
}

func (s *stdWriter) GetBody() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Body.String()
}
