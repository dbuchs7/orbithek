// Package share reads the files of a file share an album folder shows
// (ADR-0196): a directory of the server, such as a Samba share the
// operator mounted, or a bucket of an S3-compatible object store. It only
// lists and reads; nothing here writes to a share.
package share

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Kind is how a share is reached.
type Kind string

const (
	// Dir is a directory of the server: a mounted Samba share or any
	// other file system the host mounts.
	Dir Kind = "dir"
	// S3 is a bucket of an S3-compatible object store.
	S3 Kind = "s3"
)

// Environment variables the operator sets.
const (
	// RootsVar lists the directories administrators may bind, separated
	// by the system's list separator.
	RootsVar = "OT_SHARE_ROOTS"
	// AllowPrivateVar, set to "on", lets an S3 share reach addresses of
	// the server's own network.
	AllowPrivateVar = "OT_SHARE_ALLOW_PRIVATE"
)

// MaxPath bounds the length of a path below a share's address.
const MaxPath = 1024

// Object is one file of a share.
type Object struct {
	// Path is below the share's address, with "/" between its segments.
	Path string
	Size int64
	// Tag changes when the content does: an S3 object's ETag, a file's
	// modification time in nanoseconds.
	Tag     string
	ModTime time.Time
}

// Source lists and reads the files of one share.
type Source interface {
	// List answers the share's files in path order, at most max of them;
	// more reports whether there were further files.
	List(ctx context.Context, max int) (objs []Object, more bool, err error)
	// Open reads a whole file.
	Open(ctx context.Context, p string) (io.ReadCloser, error)
	// OpenRange reads n bytes of a file from off on, fewer at its end.
	OpenRange(ctx context.Context, p string, off, n int64) (io.ReadCloser, error)
	// Close releases what the source holds.
	Close() error
}

// Env reads the server's environment.
type Env func(string) string

// ErrInvalid is an address or path that cannot name a share or a file of
// one.
var ErrInvalid = errors.New("share: invalid")

// ErrUnreadable is a share that cannot be read at all: a missing mount,
// another device than the one bound, a bucket that answers an error. A run
// that meets it withdraws nothing.
var ErrUnreadable = errors.New("share: unreadable")

// Address is a share's address, parsed.
type Address struct {
	Kind Kind
	// Dir is the directory of a Dir share, absolute and clean.
	Dir string
	// Endpoint, Bucket, Prefix and Region name an S3 share.
	Endpoint *url.URL
	Bucket   string
	Prefix   string
	Region   string
}

var bucketName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// ParseAddress parses an absolute path as a Dir share and
// s3://bucket/prefix?endpoint=https://…&region=… as an S3 share.
func ParseAddress(s string) (Address, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return Address{}, fmt.Errorf("%w: the address is empty", ErrInvalid)
	case strings.HasPrefix(s, "s3://"):
		return parseS3(s)
	case filepath.IsAbs(s):
		if strings.ContainsRune(s, 0) {
			return Address{}, fmt.Errorf("%w: the path holds a NUL", ErrInvalid)
		}
		return Address{Kind: Dir, Dir: filepath.Clean(s)}, nil
	}
	return Address{}, fmt.Errorf("%w: an address is an absolute directory of the server or s3://bucket/folder?endpoint=https://…&region=…", ErrInvalid)
}

func parseS3(s string) (Address, error) {
	u, err := url.Parse(s)
	if err != nil {
		return Address{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if u.User != nil {
		return Address{}, fmt.Errorf("%w: the keys come from the environment, never from the address", ErrInvalid)
	}
	if !bucketName.MatchString(u.Host) {
		return Address{}, fmt.Errorf("%w: %q is not a bucket name", ErrInvalid, u.Host)
	}
	q := u.Query()
	ep, err := url.Parse(q.Get("endpoint"))
	if err != nil || ep.Scheme != "https" || ep.Host == "" || ep.User != nil || ep.RawQuery != "" || ep.Fragment != "" {
		return Address{}, fmt.Errorf("%w: an s3 address names its endpoint over HTTPS, s3://bucket/folder?endpoint=https://s3.example.com&region=eu-central-1", ErrInvalid)
	}
	prefix := strings.Trim(u.Path, "/")
	if prefix != "" {
		if err := CheckPath(prefix); err != nil {
			return Address{}, err
		}
	}
	region := q.Get("region")
	if region == "" {
		region = "us-east-1"
	}
	return Address{Kind: S3, Endpoint: ep, Bucket: u.Host, Prefix: prefix, Region: region}, nil
}

// CheckPath checks a path below a share's address: segments separated by
// "/", none empty, "." or "..", valid UTF-8 without control characters,
// at most MaxPath bytes.
func CheckPath(p string) error {
	if p == "" || len(p) > MaxPath || !utf8.ValidString(p) {
		return fmt.Errorf("%w: path %q", ErrInvalid, p)
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return fmt.Errorf("%w: path %q", ErrInvalid, p)
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("%w: path %q", ErrInvalid, p)
		}
	}
	return nil
}

// Skipped reports whether a path is left out of a share: a segment
// starting with a dot, or a housekeeping directory of common NAS systems.
func Skipped(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") || seg == "@eaDir" || seg == "#recycle" || seg == "#snapshot" {
			return true
		}
	}
	return false
}

// Roots answers the directories administrators may bind, from RootsVar.
func Roots(env Env) []string {
	var out []string
	for _, r := range filepath.SplitList(env(RootsVar)) {
		r = strings.TrimSpace(r)
		if filepath.IsAbs(r) {
			out = append(out, filepath.Clean(r))
		}
	}
	return out
}

// CheckDir checks that a Dir share's directory lies in one of roots and
// neither inside the data directory nor containing it.
func CheckDir(dir string, roots []string, dataDir string) error {
	dir = filepath.Clean(dir)
	if len(roots) == 0 {
		return fmt.Errorf("%w: no directory may be bound; the operator lists them in %s", ErrInvalid, RootsVar)
	}
	if dataDir != "" {
		data := filepath.Clean(dataDir)
		if within(dir, data) || within(data, dir) {
			return fmt.Errorf("%w: a share must lie outside the data directory and not contain it", ErrInvalid)
		}
	}
	for _, r := range roots {
		if within(dir, r) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s is not in a directory the operator allows (%s)", ErrInvalid, dir, RootsVar)
}

// within reports whether p is base or below it.
func within(p, base string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// KeyVars names the environment variables that hold an S3 share's keys:
// OT_SHARE_<ID>_ACCESS_KEY and OT_SHARE_<ID>_SECRET_KEY, the ID in
// capitals with every other character as "_".
func KeyVars(id string) (access, secret string) {
	var b strings.Builder
	for _, r := range strings.ToUpper(id) {
		if ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	base := "OT_SHARE_" + b.String()
	return base + "_ACCESS_KEY", base + "_SECRET_KEY"
}

// Link fills a link template's {path} with p, each segment escaped. Only
// https: and smb: templates make a link; others answer "".
func Link(template, p string) string {
	if !LinkTemplateOK(template) || CheckPath(p) != nil {
		return ""
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.ReplaceAll(template, "{path}", strings.Join(segs, "/"))
}

// LinkTemplateOK reports whether a link template may be set: an https: or
// smb: address naming {path}, without spaces or control characters.
func LinkTemplateOK(t string) bool {
	if len(t) > 512 || !strings.Contains(t, "{path}") {
		return false
	}
	for _, r := range t {
		if r <= 0x20 || r == 0x7f {
			return false
		}
	}
	u, err := url.Parse(strings.ReplaceAll(t, "{path}", "p"))
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "smb"
}

// Open opens the source an address names. dev is the device a Dir share
// was bound on, 0 when unknown; env holds an S3 share's keys under the
// names KeyVars gives for id.
func Open(a Address, id string, dev uint64, env Env) (Source, error) {
	switch a.Kind {
	case Dir:
		return openDir(a.Dir, dev)
	case S3:
		return openS3(a, id, env)
	}
	return nil, fmt.Errorf("%w: kind %q", ErrInvalid, a.Kind)
}

// join is a key below a prefix.
func join(prefix, p string) string {
	if prefix == "" {
		return p
	}
	return path.Join(prefix, p)
}
