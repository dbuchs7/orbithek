package s3sig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The examples of AWS's documentation of signature version 4.
func TestSignatureVersion4(t *testing.T) {
	// "GET Object" of the S3 documentation.
	req, _ := http.NewRequest("GET", "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	empty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	req.Header.Set("x-amz-content-sha256", empty)
	at := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	Sign(req, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3", at, empty,
		[]string{"host", "range", "x-amz-content-sha256", "x-amz-date"})
	if got := req.Header.Get("Authorization"); !strings.HasSuffix(got, "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41") {
		t.Errorf("GET Object: %s", got)
	}
	// "get-vanilla" of the signature version 4 test suite.
	req, _ = http.NewRequest("GET", "https://example.amazonaws.com/", nil)
	Sign(req, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "us-east-1", "service", time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC), empty,
		[]string{"host", "x-amz-date"})
	if got := req.Header.Get("Authorization"); got != "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31" {
		t.Errorf("get-vanilla: %s", got)
	}
	if got := CanonicalQuery(map[string][]string{"prefix": {"a b/c"}, "list-type": {"2"}, "uploads": {""}}); got != "list-type=2&prefix=a%20b%2Fc&uploads=" {
		t.Errorf("query %s", got)
	}
}

func TestClientDo(t *testing.T) {
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
	}))
	defer srv.Close()
	ep, _ := url.Parse(srv.URL + "/base/")
	c := Client{Endpoint: ep, Bucket: "photos", Region: "eu-central-1", Access: "AKID", Secret: "SECRET", HTTP: srv.Client(),
		Now: func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }}
	resp, err := c.Do(context.Background(), http.MethodGet, "2024/a b.jpg", url.Values{"x": {"1"}}, nil, http.Header{"Range": {"bytes=0-9"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got.URL.EscapedPath() != "/base/photos/2024/a%20b.jpg" || got.URL.RawQuery != "x=1" || got.Header.Get("Range") != "bytes=0-9" || got.Header.Get("x-amz-date") != "20261111T120000Z" {
		t.Errorf("request %s %s %v", got.URL.EscapedPath(), got.URL.RawQuery, got.Header)
	}
	if !strings.HasPrefix(got.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AKID/20261111/eu-central-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=") {
		t.Errorf("authorization %s", got.Header.Get("Authorization"))
	}
	c.Now = nil
	c.Endpoint = &url.URL{Scheme: "https", Host: "bad host"}
	if _, err := c.Do(context.Background(), http.MethodGet, "", nil, nil, nil); err == nil {
		t.Error("a bad endpoint was sent")
	}
}

// The example of a presigned URL in AWS's documentation of signature
// version 4 (query string authentication), virtual-hosted style.
func TestPresign(t *testing.T) {
	ep, _ := url.Parse("https://examplebucket.s3.amazonaws.com")
	got := presign(ep, "", "test.txt", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1",
		time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC), 86400*time.Second)
	if !strings.HasSuffix(got, "X-Amz-Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404") {
		t.Fatalf("presigned %s", got)
	}
	c := Client{Endpoint: ep, Bucket: "photos", Region: "eu-central-1", Access: "AKID", Secret: "SECRET", Now: func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) }}
	link, err := c.Presign("2024/a b.jpg", 15*time.Minute)
	if err != nil || !strings.HasPrefix(link, "https://examplebucket.s3.amazonaws.com/photos/2024/a%20b.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKID%2F20261112%2Feu-central-1%2Fs3%2Faws4_request&X-Amz-Date=20261112T120000Z&X-Amz-Expires=900&") {
		t.Fatalf("link %s %v", link, err)
	}
	for _, bad := range []time.Duration{0, 8 * 24 * time.Hour} {
		if _, err := c.Presign("a.jpg", bad); err == nil {
			t.Errorf("expires %v accepted", bad)
		}
	}
}
