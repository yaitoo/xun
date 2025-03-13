package sse

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

type EventReader struct {
	r io.ReadCloser
}

func NewReader(r io.ReadCloser) *EventReader {
	return &EventReader{r: r}
}

func (r *EventReader) Next() (TextEvent, error) {

	reader := bufio.NewReader(r.r)

	var (
		buf  []byte
		line string
		err  error

		evt   TextEvent
		retry int
	)

	for {
		buf, err = reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				return evt, err
			}

			return evt, io.EOF
		}

		line = string(buf)
		if strings.HasPrefix(line, ":") {
			continue
		}

		if line == "\n" {
			return evt, nil
		}

		if strings.HasPrefix(line, "id:") {
			evt.ID = strings.TrimSpace(line[3:])
		} else if strings.HasPrefix(line, "event:") {
			evt.Name = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "retry:") {
			retry, err = strconv.Atoi(strings.TrimSpace(line[6:]))
			if err == nil {
				evt.Retry = retry
			}
		} else if strings.HasPrefix(line, "data:") {
			if evt.Data == "" {
				evt.Data = strings.TrimSpace(line[5:])
			} else {
				evt.Data += "\n" + strings.TrimSpace(line[5:])
			}
		}
	}
}

func (r *EventReader) Close() {
	r.r.Close()
}
