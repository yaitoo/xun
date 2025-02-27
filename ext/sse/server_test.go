package sse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaitoo/async"
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

		c2, err := srv.Join(context.TODO(), "join", rw)
		require.NoError(t, err)

		require.Equal(t, c1, c2)
		require.Equal(t, "join", c1.ID)

		c3 := srv.Get("join")

		require.Equal(t, c1, c3)

		srv.Leave("join")

		c4 := srv.Get("join")
		require.Nil(t, c4)

	})

	t.Run("send", func(t *testing.T) {
		srv := New()
		rw := httptest.NewRecorder()

		c, err := srv.Join(context.TODO(), "send", rw)
		require.NoError(t, err)

		err = c.Send(Event{Name: "event1", Data: "data1"})
		require.NoError(t, err)
		buf := rw.Body.Bytes()
		require.Equal(t, "event: event1\ndata: \"data1\"\n\n", string(buf))

		err = c.Send(Event{Name: "event2", Data: "data2"})
		require.NoError(t, err)
		buf = rw.Body.Bytes()
		require.Equal(t, "event: event1\ndata: \"data1\"\n\nevent: event2\ndata: \"data2\"\n\n", string(buf))
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

		errs, err := srv.Broadcast(context.TODO(), Event{Name: "event1", Data: "data1"})
		require.NoError(t, err)
		require.Nil(t, errs)

		buf1 := rw1.Body.Bytes()
		buf2 := rw2.Body.Bytes()

		require.Equal(t, buf1, buf2)
		require.Equal(t, "event: event1\ndata: \"data1\"\n\n", string(buf1))

		ctx, cancel := context.WithCancel(context.TODO())
		cancel()

		_, err = srv.Broadcast(ctx, Event{Name: "event1", Data: "data1"})
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

		err = c.Send(Event{Name: "event1", Data: make(chan int)})
		require.Error(t, err)

		err = c.Send(Event{Name: "event1"})
		require.Error(t, err)

		cancel()

		err = c.Send(Event{Name: "event1"})
		require.ErrorIs(t, err, ErrClientClosed)

		errs, err := srv.Broadcast(context.TODO(), Event{Name: "event1", Data: "data1"})
		require.ErrorIs(t, err, async.ErrTooLessDone)
		require.Len(t, errs, 1)

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
