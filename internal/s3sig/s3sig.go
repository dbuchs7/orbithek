// Package s3sig signs requests to S3-compatible object stores with AWS
// signature version 4 and sends them path-style. Backups (ADR-0073) and
// file shares of the album (ADR-0197) share it; it holds no knowledge of
// either.
package s3sig

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Client sends signed requests to one bucket, path-style.
type Client struct {
	Endpoint       *url.URL
	Bucket         string
	Region         string
	Access, Secret string
	HTTP           *http.Client
	// Now is the clock the signature is made with; time.Now when nil.
	Now func() time.Time
}

// Do sends a signed request for key (empty for the bucket itself). The
// body is held in memory so that its hash is signed too. Headers in
// header are sent unsigned, which signature version 4 allows for every
// header but host and the x-amz- ones.
func (c *Client) Do(ctx context.Context, method, key string, query url.Values, body []byte, header http.Header) (*http.Response, error) {
	u := *c.Endpoint
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + c.Bucket
	if key != "" {
		u.Path += "/" + key
	}
	u.RawPath = URIEncode(u.Path, false)
	u.RawQuery = CanonicalQuery(query)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.ContentLength = int64(len(body))
	sum := sha256.Sum256(body)
	req.Header.Set("x-amz-content-sha256", hex.EncodeToString(sum[:]))
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	Sign(req, c.Access, c.Secret, c.Region, "s3", now().UTC(), hex.EncodeToString(sum[:]), []string{"host", "x-amz-content-sha256", "x-amz-date"})
	return c.HTTP.Do(req)
}

// Sign adds an AWS signature version 4 to req, over the headers named
// (lower case, sorted) and the payload's hash.
func Sign(req *http.Request, access, secret, region, service string, t time.Time, payloadHash string, headers []string) {
	stamp, day := t.Format("20060102T150405Z"), t.Format("20060102")
	req.Header.Set("x-amz-date", stamp)
	var canonHeaders strings.Builder
	for _, h := range headers {
		v := req.Header.Get(h)
		if h == "host" {
			v = req.URL.Host
		}
		canonHeaders.WriteString(h + ":" + strings.Join(strings.Fields(v), " ") + "\n")
	}
	signed := strings.Join(headers, ";")
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := strings.Join([]string{req.Method, path, req.URL.RawQuery, canonHeaders.String(), signed, payloadHash}, "\n")
	scope := day + "/" + region + "/" + service + "/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	key := hmacSHA256([]byte("AWS4"+secret), day)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	key = hmacSHA256(key, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(key, toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+access+"/"+scope+", SignedHeaders="+signed+", Signature="+sig)
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// URIEncode encodes as signature version 4 requires: every byte but the
// unreserved ones, and the slash unless slash is set.
func URIEncode(s string, slash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-', c == '.', c == '_', c == '~', c == '/' && !slash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// CanonicalQuery is a query string sorted and encoded as signature
// version 4 requires; the same string is sent.
func CanonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vs := append([]string(nil), q[k]...)
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, URIEncode(k, true)+"="+URIEncode(v, true))
		}
	}
	return strings.Join(parts, "&")
}

// Presign answers the address of a GET of key signed in its query
// (signature version 4), valid for expires from now: whoever holds it may
// read the object until then, without the keys.
func (c *Client) Presign(key string, expires time.Duration) (string, error) {
	if expires <= 0 || expires > 7*24*time.Hour {
		return "", fmt.Errorf("s3sig: a signed address holds for up to seven days, not %v", expires)
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	return presign(c.Endpoint, c.Bucket, key, c.Access, c.Secret, c.Region, now().UTC(), expires), nil
}

func presign(endpoint *url.URL, bucket, key, access, secret, region string, t time.Time, expires time.Duration) string {
	u := *endpoint
	u.Path = strings.TrimSuffix(u.Path, "/")
	if bucket != "" {
		u.Path += "/" + bucket
	}
	u.Path += "/" + key
	stamp, day := t.Format("20060102T150405Z"), t.Format("20060102")
	scope := day + "/" + region + "/s3/aws4_request"
	q := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {access + "/" + scope},
		"X-Amz-Date":          {stamp},
		"X-Amz-Expires":       {fmt.Sprint(int64(expires / time.Second))},
		"X-Amz-SignedHeaders": {"host"},
	}
	path := URIEncode(u.Path, false)
	query := CanonicalQuery(q)
	canonical := strings.Join([]string{"GET", path, query, "host:" + u.Host + "\n", "host", "UNSIGNED-PAYLOAD"}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	k := hmacSHA256([]byte("AWS4"+secret), day)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(k, toSign))
	u.RawPath = path
	u.RawQuery = query + "&X-Amz-Signature=" + sig
	return u.String()
}
