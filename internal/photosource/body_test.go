package photosource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAllowed(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://a.icloud.com/x":                               true,
		"https://A.ICLOUD.COM/x":                               true,
		"https://a.icloud.com:443/x":                           true,
		"https://icloud.com/x":                                 false,
		"https://a.icloud.com:8443/x":                          false,
		"http://a.icloud.com/x":                                false,
		"https://u:p@a.icloud.com/x":                           false,
		"https://a.icloud.com.evil/x":                          false,
		"https://a..icloud.com/x":                              false,
		"https://a_b.icloud.com/x":                             false,
		"https://[::1]/x":                                      false,
		"https://" + strings.Repeat("a", 64) + ".icloud.com/":  false,
		"https://" + strings.Repeat("a.", 130) + "icloud.com/": false,
		"https:opaque.icloud.com":                              false,
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := allowed(u, ".icloud.com"); got != want {
			t.Errorf("allowed(%q) = %v", raw, got)
		}
	}
	if allowed(nil, ".icloud.com") {
		t.Error("nil allowed")
	}
}

type chunks struct {
	parts  []string
	closed bool
}

func (c *chunks) Read(p []byte) (int, error) {
	if len(c.parts) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.parts[0])
	c.parts[0] = c.parts[0][n:]
	if c.parts[0] == "" {
		c.parts = c.parts[1:]
	}
	return n, nil
}

func (c *chunks) Close() error { c.closed = true; return nil }

func TestBounded(t *testing.T) {
	for _, tc := range []struct {
		parts []string
		limit int64
		want  string
		large bool
	}{
		{[]string{"abc", "def"}, 6, "abcdef", false},
		{[]string{"abc", "def"}, 7, "abcdef", false},
		{[]string{"abc", "defg"}, 6, "abcdef", true},
		{[]string{"abcdefg"}, 2, "ab", true},
		{[]string{"a"}, 0, "", true},
		{nil, 0, "", false},
	} {
		c := &chunks{parts: append([]string(nil), tc.parts...)}
		b := newBounded(c, tc.limit)
		data, err := io.ReadAll(b)
		if string(data) != tc.want || errors.Is(err, ErrTooLarge) != tc.large || (!tc.large && err != nil) {
			t.Errorf("%v/%d: %q, %v", tc.parts, tc.limit, data, err)
		}
		if tc.large {
			if _, err := b.Read(make([]byte, 4)); !errors.Is(err, ErrTooLarge) {
				t.Errorf("read after too large: %v", err)
			}
		}
		b.Close()
		if !c.closed {
			t.Error("not closed")
		}
	}
}

func TestDownloadKnownLength(t *testing.T) {
	resp := &http.Response{StatusCode: 200, ContentLength: 11, Body: io.NopCloser(strings.NewReader("hello world"))}
	_, err := download(resp, 10)
	wantErr(t, err, ErrTooLarge)
	resp = &http.Response{StatusCode: 200, ContentLength: 11, Body: io.NopCloser(strings.NewReader("hello world"))}
	rc, err := download(resp, 11)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := io.ReadAll(rc); err != nil || string(data) != "hello world" {
		t.Fatalf("%q, %v", data, err)
	}
	resp = &http.Response{StatusCode: http.StatusGone, Body: io.NopCloser(strings.NewReader(""))}
	_, err = download(resp, 11)
	wantErr(t, err, ErrNotFound)
}

type failing struct{}

func (failing) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestReadBoundedFailure(t *testing.T) {
	_, err := readBounded(failing{}, 10)
	wantErr(t, err, ErrUnreadable)
	if !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("message %v", err)
	}
}

func TestReason(t *testing.T) {
	for err, want := range map[error]string{
		&url.Error{Op: "Get", URL: "https://secret/x", Err: context.DeadlineExceeded}:          "the service did not answer in time",
		&url.Error{Op: "Get", URL: "https://secret/x", Err: context.Canceled}:                  "the request was cancelled",
		&url.Error{Op: "Get", URL: "https://secret/x", Err: errors.New("dial tcp: boom")}:      "dial tcp: boom",
		&url.Error{Op: "Get", URL: "https://secret/x", Err: fmt.Errorf("at https://secret/y")}: "the service cannot be reached",
		&url.Error{Op: "Get", URL: "https://secret/x", Err: errRedirect}:                       errRedirect.Error(),
	} {
		if got := reason(err); got != want {
			t.Errorf("reason(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestNumber(t *testing.T) {
	var v struct{ A, B, C, D, E, F number }
	if err := json.Unmarshal([]byte(`{"A":12,"B":"34","C":"x","D":-5,"E":7.9,"F":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != 12 || v.B != 34 || v.C != 0 || v.D != 0 || v.E != 7 || v.F != 0 {
		t.Fatalf("%+v", v)
	}
	if number(5e9).int() != 1e9 {
		t.Fatal("not capped")
	}
}

func TestGuardedTooManyRedirects(t *testing.T) {
	n := 0
	client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		http.Redirect(w, r, fmt.Sprintf("/r%d", n), http.StatusFound)
	}))
	c := guarded(client, func(next, first *url.URL) bool { return true })
	resp, err := c.Get("https://a.example/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("followed redirects for ever")
	}
	if n != 10 {
		t.Fatalf("%d requests", n)
	}
}
