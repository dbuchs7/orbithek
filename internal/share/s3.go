package share

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/linkcheck"
	"github.com/dbuchs7/orbithek/internal/s3sig"
)

// maxListingPage bounds one page of a bucket's listing as read.
const maxListingPage = 8 << 20

// maxErrorAnswer bounds an error answer of a store as read.
const maxErrorAnswer = 64 << 10

// requestTimeout bounds connecting and the answer's headers.
const requestTimeout = 30 * time.Second

type s3Source struct {
	c      s3sig.Client
	prefix string
}

func openS3(a Address, id string, env Env) (Source, error) {
	av, sv := KeyVars(id)
	access, secret := strings.TrimSpace(env(av)), strings.TrimSpace(env(sv))
	if access == "" || secret == "" {
		return nil, fmt.Errorf("%w: set %s and %s in the server's environment", ErrUnreadable, av, sv)
	}
	allowed := linkcheck.Public
	if env(AllowPrivateVar) == "on" {
		allowed = func(netip.Addr) bool { return true }
	}
	client := &http.Client{
		Transport: linkcheck.Transport(allowed, requestTimeout),
		// A redirect would leave the signed bucket; S3 does not send one
		// for path-style requests to the right region.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return newS3Source(a, access, secret, client), nil
}

func newS3Source(a Address, access, secret string, client *http.Client) *s3Source {
	return &s3Source{c: s3sig.Client{Endpoint: a.Endpoint, Bucket: a.Bucket, Region: a.Region, Access: access, Secret: secret, HTTP: client}, prefix: a.Prefix}
}

func (s *s3Source) List(ctx context.Context, max int) ([]Object, bool, error) {
	prefix := ""
	if s.prefix != "" {
		prefix = s.prefix + "/"
	}
	var out []Object
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		resp, err := s.c.Do(ctx, http.MethodGet, "", q, nil, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil, false, ctx.Err()
			}
			return nil, false, fmt.Errorf("%w: listing the bucket: %v", ErrUnreadable, err)
		}
		if resp.StatusCode != http.StatusOK {
			err := statusError(resp, "listing the bucket")
			resp.Body.Close()
			return nil, false, err
		}
		var page struct {
			Contents []struct {
				Key          string `xml:"Key"`
				Size         int64  `xml:"Size"`
				ETag         string `xml:"ETag"`
				LastModified string `xml:"LastModified"`
			} `xml:"Contents"`
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(io.LimitReader(resp.Body, maxListingPage)).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, false, fmt.Errorf("%w: reading the listing: %v", ErrUnreadable, err)
		}
		for _, c := range page.Contents {
			p, ok := strings.CutPrefix(c.Key, prefix)
			if !ok || strings.HasSuffix(p, "/") || CheckPath(p) != nil || Skipped(p) || c.Size < 0 {
				continue
			}
			if len(out) >= max {
				return out, true, nil
			}
			mod, _ := time.Parse(time.RFC3339, c.LastModified)
			out = append(out, Object{Path: p, Size: c.Size, Tag: strings.Trim(c.ETag, `"`), ModTime: mod})
		}
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return out, false, nil
		}
		token = page.NextContinuationToken
	}
}

func (s *s3Source) get(ctx context.Context, p string, header http.Header, want int) (io.ReadCloser, error) {
	if err := CheckPath(p); err != nil {
		return nil, err
	}
	resp, err := s.c.Do(ctx, http.MethodGet, join(s.prefix, p), nil, nil, header)
	if err != nil {
		return nil, fmt.Errorf("share: reading %s: %w", p, err)
	}
	if resp.StatusCode != want {
		defer resp.Body.Close()
		return nil, statusError(resp, "reading "+p)
	}
	return resp.Body, nil
}

func (s *s3Source) Open(ctx context.Context, p string) (io.ReadCloser, error) {
	return s.get(ctx, p, nil, http.StatusOK)
}

func (s *s3Source) OpenRange(ctx context.Context, p string, off, n int64) (io.ReadCloser, error) {
	if off < 0 || n <= 0 {
		return nil, fmt.Errorf("%w: range", ErrInvalid)
	}
	h := http.Header{"Range": {"bytes=" + strconv.FormatInt(off, 10) + "-" + strconv.FormatInt(off+n-1, 10)}}
	body, err := s.get(ctx, p, h, http.StatusPartialContent)
	if err != nil {
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(body, n), body}, nil
}

func (s *s3Source) Close() error {
	s.c.HTTP.CloseIdleConnections()
	return nil
}

// statusError describes an unexpected answer with the store's own error
// code, read with bounds; it never repeats the answer's body beyond that.
func statusError(resp *http.Response, what string) error {
	var e struct {
		Code string `xml:"Code"`
	}
	_ = xml.NewDecoder(io.LimitReader(resp.Body, maxErrorAnswer)).Decode(&e)
	code := e.Code
	if len(code) > 64 {
		code = code[:64]
	}
	msg := fmt.Sprintf("%s: %s", what, resp.Status)
	if code != "" {
		msg += " (" + code + ")"
	}
	if resp.StatusCode == http.StatusNotFound && what != "listing the bucket" {
		return fmt.Errorf("share: %s", msg)
	}
	return fmt.Errorf("%w: %s", ErrUnreadable, msg)
}
