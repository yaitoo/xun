package sse

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/async"
	"github.com/yaitoo/xun"
)

func TestServer(t *testing.T) {
	t.Run("join", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		c1, err := srv.Join(context.TODO(), "join", nil)
		require.Nil(t, c1)
		require.ErrorIs(t, err, ErrNotStreamer)

		c1, err = srv.Join(context.TODO(), "join", &notStreamer{})
		require.Nil(t, c1)
		require.ErrorIs(t, err, ErrNotStreamer)

		c1, err = srv.Join(context.TODO(), "join", rw)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c2, err := srv.Join(ctx, "join", rw)
		require.NoError(t, err)

		require.Equal(t, c1, c2)
		require.Equal(t, "join", c1.ID)

		c3 := srv.Get("join")

		require.Equal(t, c1, c3)

		go func() {
			time.Sleep(1 * time.Second)
			c3.Close()
		}()

		c2.Wait()
		c3.Wait()

		srv.Leave("join")

		c4 := srv.Get("join")
		require.Nil(t, c4)

	})

	t.Run("send", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		c, err := srv.Join(context.TODO(), "send", rw)
		require.NoError(t, err)

		r := NewReader(rw.Body)

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

		c1, err := srv.Join(context.TODO(), "c1", rw1)
		require.NotNil(t, c1)
		require.NoError(t, err)

		c2, err := srv.Join(context.TODO(), "c2", rw2)
		require.NotNil(t, c2)
		require.NoError(t, err)

		errs, err := srv.Broadcast(context.TODO(), &TextEvent{Name: "event1", Data: "data1"})
		require.NoError(t, err)
		require.Nil(t, errs)

		buf1 := rw1.Body.Bytes()
		buf2 := rw2.Body.Bytes()

		require.Equal(t, buf1, buf2)
		require.Equal(t, "event: event1\ndata: data1\n\n\n", string(buf1))

		ctx, cancel := context.WithCancel(context.TODO())
		cancel()

		_, err = srv.Broadcast(ctx, &TextEvent{Name: "event1", Data: "data1"})
		require.ErrorIs(t, err, context.Canceled)

	})

	t.Run("invalid", func(t *testing.T) {
		srv := New()

		rw := &streamerMock{
			ResponseWriter: httptest.NewRecorder(),
		}

		ctx, cancel := context.WithCancel(context.TODO())

		c, err := srv.Join(ctx, "invalid", rw)
		require.NoError(t, err)

		err = c.Send(&JsonEvent{Name: "event1", Data: make(chan int)})
		require.Error(t, err)

		err = c.Send(&TextEvent{Name: "event1"})
		require.Error(t, err)

		cancel()

		err = c.Send(&TextEvent{Name: "event1"})
		require.ErrorIs(t, err, ErrClientClosed)

		errs, err := srv.Broadcast(context.TODO(), &TextEvent{Name: "event1", Data: "data1"})
		require.ErrorIs(t, err, async.ErrTooLessDone)
		require.Len(t, errs, 1)

	})

	t.Run("shutdown", func(t *testing.T) {
		srv := New()

		c1, err := srv.Join(context.TODO(), "c1", httptest.NewRecorder())
		require.NoError(t, err)
		require.NotNil(t, c1)

		c2, err := srv.Join(context.TODO(), "c1", httptest.NewRecorder())
		require.NoError(t, err)
		require.NotNil(t, c2)
		srv.Shutdown()
		c1.Wait()
		c2.Wait()

		require.Len(t, srv.conns, 0)
	})

	t.Run("send_with_gzip", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		gc := &xun.GzipCompressor{}

		c, err := srv.Join(context.TODO(), "send", gc.New(rw))
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

		c, err := srv.Join(context.TODO(), "send", gc.New(rw))
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
}

type notStreamer struct {
}

func (s *notStreamer) Header() http.Header {
	return http.Header{}
}

func (s *notStreamer) Write([]byte) (int, error) {
	return 0, errors.New("mock: invalid")
}

func (s *notStreamer) WriteHeader(int) {}

type streamerMock struct {
	http.ResponseWriter
}

func (*streamerMock) Write([]byte) (int, error) {
	return 0, errors.New("mock: invalid")
}

func (*streamerMock) Flush() {}
