package photosource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// drainBytes is how much of a refusal is read, only to be dropped, so the
// connection can be used again.
const drainBytes = 64 << 10

// errRedirect is a redirect the guarded client refused.
var errRedirect = errors.New("a redirect to an address that is not allowed")

// allowed reports whether u is an https address without a user name, on
// the default port, whose host ends with one of suffixes (each starting
// with a dot) and is more than the suffix.
func allowed(u *url.URL, suffixes ...string) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return false
	}
	if p := u.Port(); p != "" && p != "443" {
		return false
	}
	h := u.Hostname()
	if !hostName(h) {
		return false
	}
	h = strings.ToLower(h)
	for _, s := range suffixes {
		if strings.HasSuffix(h, s) && len(h) > len(s) {
			return true
		}
	}
	return false
}

// hostName reports whether h is a plain DNS name: letters, digits,
// hyphens and dots, no empty label, no address literal.
func hostName(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for _, r := range label {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			default:
				return false
			}
		}
	}
	return true
}

// guarded copies client with a redirect policy that follows only
// redirects ok accepts, judged against the first request, before the
// client's own policy (or Go's default of ten redirects) is asked.
func guarded(client *http.Client, ok func(next, first *url.URL) bool) *http.Client {
	c := *client
	own := client.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !ok(req.URL, via[0].URL) {
			return errRedirect
		}
		if own != nil {
			return own(req, via)
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	return &c
}

// readBounded reads body, at most limit bytes; a longer one is
// ErrTooLarge.
func readBounded(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: the answer is larger than %d bytes", ErrTooLarge, limit)
	}
	return data, nil
}

// drain reads the start of an answer that is dropped and closes it.
func drain(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, drainBytes))
	resp.Body.Close()
}

// bounded is a download that fails with ErrTooLarge once it is longer
// than its limit.
type bounded struct {
	rc    io.ReadCloser
	limit int64
	left  int64
}

func newBounded(rc io.ReadCloser, limit int64) *bounded {
	return &bounded{rc: rc, limit: limit, left: limit}
}

func (b *bounded) Read(p []byte) (int, error) {
	if b.left < 0 {
		return 0, b.tooLarge()
	}
	if int64(len(p)) > b.left+1 {
		p = p[:b.left+1]
	}
	n, err := b.rc.Read(p)
	if int64(n) > b.left {
		n, b.left = int(b.left), -1
		return n, b.tooLarge()
	}
	b.left -= int64(n)
	return n, err
}

func (b *bounded) tooLarge() error {
	return fmt.Errorf("%w: the download is larger than %d bytes", ErrTooLarge, b.limit)
}

func (b *bounded) Close() error { return b.rc.Close() }

// download checks a download's answer and hands its body out bounded by
// limit; a refusal is ErrNotFound for 404 and 410, ErrUnreadable with the
// status otherwise.
func download(resp *http.Response, limit int64) (io.ReadCloser, error) {
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		drain(resp)
		return nil, fmt.Errorf("%w: the service no longer has the photo (%d)", ErrNotFound, resp.StatusCode)
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		drain(resp)
		return nil, fmt.Errorf("%w: the service answers %d", ErrUnreadable, resp.StatusCode)
	case resp.ContentLength > limit:
		drain(resp)
		return nil, fmt.Errorf("%w: the download is larger than %d bytes", ErrTooLarge, limit)
	}
	return newBounded(resp.Body, limit), nil
}

// reason says why a request failed without the address it went to: an
// *url.Error names it, and addresses handed out by a service carry
// credentials of their own.
func reason(err error) string {
	if errors.Is(err, errRedirect) {
		return errRedirect.Error()
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "the service did not answer in time"
	}
	if errors.Is(err, context.Canceled) {
		return "the request was cancelled"
	}
	if msg := err.Error(); !strings.Contains(msg, "://") {
		return msg
	}
	return "the service cannot be reached"
}

// number is a count a service writes as a JSON number or as a string;
// anything else reads as zero.
type number int64

func (n *number) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*n = 0
	if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
		*n = number(v)
	} else if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 && f < 1<<53 {
		*n = number(f)
	}
	return nil
}

// int answers the count as an int, at most a billion, as no picture is
// larger.
func (n number) int() int {
	return int(min(n, 1e9))
}
