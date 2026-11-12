package share

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/s3sig"
)

// fakeBucket answers ListObjectsV2 in pages of two and GET with ranges,
// for requests signed with the keys AKID and SECRET.
type fakeBucket struct {
	bucket  string
	objects map[string]string
	deny    bool
}

func (f *fakeBucket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sum := sha256.Sum256(nil)
	at, _ := time.Parse("20060102T150405Z", r.Header.Get("x-amz-date"))
	check, _ := http.NewRequest(r.Method, "https://"+r.Host+r.URL.RequestURI(), nil)
	check.Header.Set("x-amz-content-sha256", r.Header.Get("x-amz-content-sha256"))
	s3sig.Sign(check, "AKID", "SECRET", "eu-central-1", "s3", at, hex.EncodeToString(sum[:]), []string{"host", "x-amz-content-sha256", "x-amz-date"})
	if f.deny || check.Header.Get("Authorization") != r.Header.Get("Authorization") {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "<Error><Code>AccessDenied</Code><Message>no</Message></Error>")
		return
	}
	key, ok := strings.CutPrefix(r.URL.Path, "/"+f.bucket)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		f.list(w, r.URL.Query())
		return
	}
	body, ok := f.objects[key]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
		return
	}
	if rg := r.Header.Get("Range"); rg != "" {
		var from, to int
		fmt.Sscanf(rg, "bytes=%d-%d", &from, &to)
		if to >= len(body) {
			to = len(body) - 1
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, body[from:to+1])
		return
	}
	io.WriteString(w, body)
}

func (f *fakeBucket) list(w http.ResponseWriter, q url.Values) {
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, q.Get("prefix")) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	start, _ := strconv.Atoi(q.Get("continuation-token"))
	end := min(start+2, len(keys))
	var b strings.Builder
	b.WriteString("<ListBucketResult>")
	for _, k := range keys[start:end] {
		fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>%d</Size><ETag>"%x"</ETag><LastModified>2024-07-01T10:00:00.000Z</LastModified></Contents>`, k, len(f.objects[k]), sha256.Sum256([]byte(f.objects[k])))
	}
	if end < len(keys) {
		fmt.Fprintf(&b, "<IsTruncated>true</IsTruncated><NextContinuationToken>%d</NextContinuationToken>", end)
	}
	b.WriteString("</ListBucketResult>")
	io.WriteString(w, b.String())
}

func TestS3Source(t *testing.T) {
	f := &fakeBucket{bucket: "photos", objects: map[string]string{
		"family/2024/a.jpg":         "0123456789",
		"family/2024/b.jpg":         "bb",
		"family/2024/":              "",
		"family/@eaDir/x.jpg":       "x",
		"family/.trash/y.jpg":       "y",
		"family/c.jpg":              "c",
		"other/d.jpg":               "d",
		"family/bad//double.jpg":    "z",
		"family/z/last picture.jpg": "zz",
	}}
	srv := httptest.NewTLSServer(f)
	defer srv.Close()
	a, err := ParseAddress("s3://photos/family?endpoint=" + url.QueryEscape(srv.URL) + "&region=eu-central-1")
	if err != nil {
		t.Fatal(err)
	}
	src := newS3Source(a, "AKID", "SECRET", srv.Client())
	defer src.Close()
	ctx := context.Background()
	objs, more, err := src.List(ctx, 100)
	if err != nil || more {
		t.Fatalf("list: %v %v", more, err)
	}
	var paths []string
	for _, o := range objs {
		paths = append(paths, o.Path)
	}
	if strings.Join(paths, ",") != "2024/a.jpg,2024/b.jpg,c.jpg,z/last picture.jpg" {
		t.Fatalf("paths %v", paths)
	}
	if objs[0].Size != 10 || len(objs[0].Tag) != 64 || objs[0].ModTime.Year() != 2024 {
		t.Errorf("object %+v", objs[0])
	}
	if objs, more, _ := src.List(ctx, 3); len(objs) != 3 || !more {
		t.Errorf("max: %d %v", len(objs), more)
	}

	r, err := src.Open(ctx, "z/last picture.jpg")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "zz" {
		t.Errorf("open %q", b)
	}
	r, err = src.OpenRange(ctx, "2024/a.jpg", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(r)
	r.Close()
	if string(b) != "234" {
		t.Errorf("range %q", b)
	}
	if _, err := src.Open(ctx, "gone.jpg"); err == nil || errors.Is(err, ErrUnreadable) || !strings.Contains(err.Error(), "NoSuchKey") {
		t.Errorf("missing object: %v", err)
	}
	if _, err := src.Open(ctx, "../other/d.jpg"); !errors.Is(err, ErrInvalid) {
		t.Errorf("escape: %v", err)
	}
	if _, err := src.OpenRange(ctx, "2024/a.jpg", 0, 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty range: %v", err)
	}

	f.deny = true
	if _, _, err := src.List(ctx, 10); !errors.Is(err, ErrUnreadable) || !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("denied list: %v", err)
	}
	if _, err := src.Open(ctx, "c.jpg"); !errors.Is(err, ErrUnreadable) {
		t.Errorf("denied read: %v", err)
	}
	if _, err := src.OpenRange(ctx, "c.jpg", 0, 1); !errors.Is(err, ErrUnreadable) {
		t.Errorf("denied range: %v", err)
	}
	f.deny = false

	wrong := newS3Source(a, "AKID", "WRONG", srv.Client())
	if _, _, err := wrong.List(ctx, 10); !errors.Is(err, ErrUnreadable) {
		t.Errorf("wrong key: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := src.List(cancelled, 10); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
	srv.Close()
	if _, _, err := src.List(ctx, 10); !errors.Is(err, ErrUnreadable) {
		t.Errorf("gone: %v", err)
	}
	if _, err := src.Open(ctx, "c.jpg"); err == nil {
		t.Error("read from a closed server")
	}
}

func TestOpenS3(t *testing.T) {
	srv := httptest.NewTLSServer(&fakeBucket{bucket: "photos", objects: map[string]string{"a.jpg": "a"}})
	defer srv.Close()
	a, _ := ParseAddress("s3://photos?endpoint=" + url.QueryEscape(srv.URL) + "&region=eu-central-1")
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	if _, err := Open(a, "nas", 0, get); !errors.Is(err, ErrUnreadable) || !strings.Contains(err.Error(), "OT_SHARE_NAS_ACCESS_KEY") {
		t.Fatalf("no keys: %v", err)
	}
	env["OT_SHARE_NAS_ACCESS_KEY"], env["OT_SHARE_NAS_SECRET_KEY"] = "AKID", "SECRET"
	src, err := Open(a, "nas", 0, get)
	if err != nil {
		t.Fatal(err)
	}
	// The test server listens on the loopback address, which only an
	// operator's OT_SHARE_ALLOW_PRIVATE=on admits.
	if _, _, err := src.List(context.Background(), 10); !errors.Is(err, ErrUnreadable) {
		t.Errorf("loopback reached: %v", err)
	}
	src.Close()
	env[AllowPrivateVar] = "on"
	src, err = Open(a, "nas", 0, get)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	// Admitted now, the connection fails only on the test server's own
	// certificate, which the system does not trust.
	if _, _, err := src.List(context.Background(), 10); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Errorf("private allowed: %v", err)
	}
}
