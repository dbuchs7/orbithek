package share

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
)

// dirSource reads a directory of the server through os.Root, so no
// symbolic link and no ".." leads out of it.
type dirSource struct {
	root *os.Root
}

func openDir(dir string, dev uint64) (Source, error) {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory the server can read", ErrUnreadable, dir)
	}
	if dev != 0 {
		if got := Device(fi); got != 0 && got != dev {
			return nil, fmt.Errorf("%w: %s is no longer on the device it was bound on; is the share mounted?", ErrUnreadable, dir)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return &dirSource{root: root}, nil
}

// DeviceOf answers the device a directory lies on, 0 where the system does
// not tell; a share remembers it when bound, so that an unmounted share,
// whose mount point lies on the server's own disk, is not read as empty.
func DeviceOf(dir string) (uint64, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	return Device(fi), nil
}

func (d *dirSource) List(ctx context.Context, max int) ([]Object, bool, error) {
	var out []Object
	more := false
	err := fs.WalkDir(d.root.FS(), ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if p == "." {
				return err
			}
			return nil // an unreadable subdirectory is left out
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if p == "." {
			return nil
		}
		if Skipped(p) || CheckPath(p) != nil {
			if e.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if e.IsDir() || !e.Type().IsRegular() {
			return nil
		}
		if len(out) >= max {
			more = true
			return fs.SkipAll
		}
		fi, err := e.Info()
		if err != nil {
			return nil
		}
		out = append(out, Object{Path: p, Size: fi.Size(), Tag: strconv.FormatInt(fi.ModTime().UnixNano(), 10), ModTime: fi.ModTime()})
		return nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false, err
		}
		return nil, false, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	// WalkDir visits each directory in lexical order, which differs from
	// path order where a name holds a character below "/"; sort, so both
	// kinds of share answer the same order.
	slices.SortFunc(out, func(a, b Object) int { return strings.Compare(a.Path, b.Path) })
	return out, more, nil
}

func (d *dirSource) open(p string) (*os.File, error) {
	if err := CheckPath(p); err != nil {
		return nil, err
	}
	f, err := d.root.Open(p)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("%w: %s is not a file", ErrInvalid, p)
	}
	return f, nil
}

func (d *dirSource) Open(_ context.Context, p string) (io.ReadCloser, error) {
	return d.open(p)
}

func (d *dirSource) OpenRange(_ context.Context, p string, off, n int64) (io.ReadCloser, error) {
	if off < 0 || n < 0 {
		return nil, fmt.Errorf("%w: range", ErrInvalid)
	}
	f, err := d.open(p)
	if err != nil {
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{io.NewSectionReader(f, off, n), f}, nil
}

func (d *dirSource) Close() error { return d.root.Close() }
