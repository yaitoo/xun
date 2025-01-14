package xun

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTextTemplate(t *testing.T) {
	fsys := &fstest.MapFS{
		"invalid.txt": {Data: []byte(`{{ define "home" }} {{ define "home" }}{{ end }}`), ModTime: time.Now()},
	}

	t.Run("fails_cannot_read_file", func(t *testing.T) {

		tmpl := &TextTemplate{
			name: "foo.txt",
		}

		err := tmpl.Load(fsys)
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("fails_parse_invalid_template", func(t *testing.T) {
		tmpl := &TextTemplate{
			name: "invalid.txt",
		}

		err := tmpl.Load(fsys)
		require.NotNil(t, err)
	})

}
