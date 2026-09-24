package export

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// sink is where the files go: a directory or an archive, built beside out
// under a temporary name and moved into place only once the export succeeds,
// so a failed run never leaves half a site where the previous one stood.
type sink interface {
	add(name string, b []byte) error
	// abort removes what was written so far. Its error is what is left
	// behind under the temporary name.
	abort() error
}

// epoch stamps every archive entry, so two exports of the same config on the
// same day are the same bytes (security.txt carries the date) and a deploy
// step can tell that nothing changed. Zip cannot
// represent a date before 1980.
var epoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

func open(out string) (sink, func() error, error) {
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(out)), 0o755); err != nil {
		return nil, nil, fmt.Errorf("export: %w", err)
	}
	lower := strings.ToLower(out)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return openArchive(out, func(w io.Writer) archive { return zipArchive{zip.NewWriter(w)} })
	case strings.HasSuffix(lower, ".tar"):
		return openArchive(out, func(w io.Writer) archive { return tarArchive{tw: tar.NewWriter(w)} })
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return openArchive(out, func(w io.Writer) archive {
			gz := gzip.NewWriter(w)
			return tarArchive{tw: tar.NewWriter(gz), gz: gz}
		})
	}
	return openDir(out)
}

type dirSink struct{ tmp string }

func openDir(out string) (sink, func() error, error) {
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		return nil, nil, fmt.Errorf("export: %s is not empty; give a new or empty directory, or an archive name", out)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("export: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(filepath.Clean(out)), ".cairn-export-*")
	if err != nil {
		return nil, nil, fmt.Errorf("export: %w", err)
	}
	s := dirSink{tmp}
	return s, func() error {
		// MkdirTemp makes the directory private; a web server reading it as
		// another user needs it open like any published tree.
		if err := os.Chmod(tmp, 0o755); err != nil {
			return fmt.Errorf("export: %w", errors.Join(err, s.abort()))
		}
		// Only an empty directory can be standing there by now, and a rename
		// onto a directory fails on Windows even when it is empty.
		if err := os.Remove(out); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("export: %w", errors.Join(err, s.abort()))
		}
		if err := os.Rename(tmp, out); err != nil {
			return fmt.Errorf("export: %w", errors.Join(err, s.abort()))
		}
		return nil
	}, nil
}

func (s dirSink) add(name string, b []byte) error {
	p := filepath.Join(s.tmp, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func (s dirSink) abort() error { return os.RemoveAll(s.tmp) }

type archive interface {
	add(name string, b []byte) error
	io.Closer
}

type zipArchive struct{ zw *zip.Writer }

func (a zipArchive) add(name string, b []byte) error {
	w, err := a.zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: epoch})
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func (a zipArchive) Close() error { return a.zw.Close() }

type tarArchive struct {
	tw *tar.Writer
	gz *gzip.Writer
}

func (a tarArchive) add(name string, b []byte) error {
	if err := a.tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b)), ModTime: epoch, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := a.tw.Write(b)
	return err
}

func (a tarArchive) Close() error {
	err := a.tw.Close()
	if a.gz != nil {
		err = errors.Join(err, a.gz.Close())
	}
	return err
}

type fileSink struct {
	f *os.File
	archive
}

func openArchive(out string, wrap func(io.Writer) archive) (sink, func() error, error) {
	f, err := os.CreateTemp(filepath.Dir(out), ".cairn-export-*")
	if err != nil {
		return nil, nil, fmt.Errorf("export: %w", err)
	}
	s := fileSink{f, wrap(f)}
	return s, func() error {
		err := errors.Join(s.Close(), f.Close())
		if err == nil {
			err = os.Chmod(f.Name(), 0o644)
		}
		if err == nil {
			err = os.Rename(f.Name(), out)
		}
		if err != nil {
			return fmt.Errorf("export: %w", errors.Join(err, os.Remove(f.Name())))
		}
		return nil
	}, nil
}

func (s fileSink) abort() error {
	return errors.Join(s.f.Close(), os.Remove(s.f.Name()))
}
