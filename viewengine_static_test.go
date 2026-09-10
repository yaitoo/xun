package xun

import (
	"embed"
	"io/fs"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/embed_helper.txt
var staticEmbedFS embed.FS

// closeCountingFS wraps an fs.FS so the file returned from Open(".")
// records whether Close was called. Other Open paths pass through untouched.
type closeCountingFS struct {
	fsys   fs.FS
	closes atomic.Int32
}

func (f *closeCountingFS) Open(name string) (fs.File, error) {
	file, err := f.fsys.Open(name)
	if err != nil || name != "." {
		return file, err
	}
	return &closeCountingFile{File: file, closes: &f.closes}, nil
}

type closeCountingFile struct {
	fs.File
	closes *atomic.Int32
}

func (f *closeCountingFile) Close() error {
	f.closes.Add(1)
	return f.File.Close()
}

// TestStaticViewEngine_ProbeFileClosed is the regression test for the
// "root.Close() is missing" leak in StaticViewEngine.Load: the probe handle
// obtained from fsys.Open(".") must be closed before Load returns.
//
// https://github.com/yaitoo/xun/issues/135
func TestStaticViewEngine_ProbeFileClosed(t *testing.T) {
	fsys := &closeCountingFS{fsys: fstest.MapFS{}}

	ve := &StaticViewEngine{}
	ve.Load(fsys, nil)

	require.Equal(t, int32(1), fsys.closes.Load(),
		"expected the probe fs.File from fsys.Open(\".\") to be closed exactly once")
}

// TestStaticViewEngine_DetectsEmbedViaSubFS guards the canonical production
// wiring shown in README.md (fs.Sub over embed.FS). The current probe
// (fsys.Open(".")) happens to walk past fs.subFS because subFS.Open
// delegates to the parent embed.FS, returning *embed.openFile whose
// PkgPath is "embed". Any future "reflect on fsys instead" refactor must
// preserve that behaviour or it will silently disable ETags for the
// documented production wiring.
//
// https://github.com/yaitoo/xun/issues/135
func TestStaticViewEngine_DetectsEmbedViaSubFS(t *testing.T) {
	sub, err := fs.Sub(staticEmbedFS, "testdata")
	require.NoError(t, err)

	ve := &StaticViewEngine{}
	ve.Load(sub, nil)

	require.True(t, ve.isEmbedFsys,
		"expected isEmbedFsys=true for fs.Sub(embed.FS, ...) (canonical production wiring)")
}

// TestStaticViewEngine_NonEmbedNotDetected is the symmetric guard: a
// non-embed fs.FS (fstest.MapFS here) must not be misclassified as embed.
// A regression here would also leak fd-relative issues (every non-embed
// load would now eagerly hash every public file at registration time).
func TestStaticViewEngine_NonEmbedNotDetected(t *testing.T) {
	fsys := fstest.MapFS{}

	ve := &StaticViewEngine{}
	ve.Load(fsys, nil)

	require.False(t, ve.isEmbedFsys,
		"expected isEmbedFsys=false for fstest.MapFS")
}
