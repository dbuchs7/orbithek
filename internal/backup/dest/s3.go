package dest

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/dbuchs7/orbithek/internal/s3sig"
)

// s3 is a bucket of an S3-compatible object store (Amazon S3, MinIO,
// Wasabi, Synology C2, …), addressed as
// s3://bucket/folder?endpoint=https://host&region=… and reached path-style
// with signature version 4. The keys are in AccessKeyEnv and SecretKeyEnv.
type s3 struct {
	c      s3sig.Client
	bucket string
	prefix string
}

// partSize is the size of each part of a multipart upload: a backup is
// streamed without knowing its length, so it goes up in parts of this
// size, each held in memory once. S3 allows 10,000 parts, so up to 160 GB.
// A variable for tests.
var partSize = 16 << 20

func newS3(a address, env Env) (Target, error) {
	q := a.url.Query()
	ep, _ := url.Parse(q.Get("endpoint"))
	region := q.Get("region")
	if region == "" {
		region = "us-east-1"
	}
	access, secret := strings.TrimSpace(env(AccessKeyEnv)), strings.TrimSpace(env(SecretKeyEnv))
	if access == "" || secret == "" {
		return nil, fmt.Errorf("s3: set %s and %s in the server's environment", AccessKeyEnv, SecretKeyEnv)
	}
	cfg, err := tlsConfig(env, ep.Hostname())
	if err != nil {
		return nil, err
	}
	bucket := a.url.Hostname()
	return &s3{c: s3sig.Client{Endpoint: ep, Bucket: bucket, Region: region, Access: access, Secret: secret, HTTP: httpClient(cfg)}, bucket: bucket, prefix: a.dir}, nil
}

func (s *s3) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

// do sends a signed request; body is held in memory, so its hash is
// signed too.
func (s *s3) do(ctx context.Context, method, key string, query url.Values, body []byte) (*http.Response, error) {
	return s.c.Do(ctx, method, key, query, body, nil)
}

func (s *s3) Put(ctx context.Context, name string, r io.Reader) error {
	if err := checkName(name); err != nil {
		return err
	}
	key := s.key(name)
	buf := make([]byte, partSize)
	n, err := io.ReadFull(readerCtx{ctx, r}, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return err
	}
	if n < partSize {
		resp, err := s.do(ctx, http.MethodPut, key, nil, buf[:n])
		if err != nil {
			return fmt.Errorf("s3: storing %s: %w", name, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return statusError("s3", "PUT", key, resp)
		}
		return nil
	}
	// A multipart upload: the object appears only when it is completed,
	// and an aborted one leaves nothing.
	upload, err := s.startUpload(ctx, key)
	if err != nil {
		return err
	}
	var parts []completedPart
	err = func() error {
		for num := 1; n > 0; num++ {
			etag, err := s.uploadPart(ctx, key, upload, num, buf[:n])
			if err != nil {
				return err
			}
			parts = append(parts, completedPart{Number: num, ETag: etag})
			n, err = io.ReadFull(readerCtx{ctx, r}, buf)
			if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
				return err
			}
		}
		return s.completeUpload(ctx, key, upload, parts)
	}()
	if err != nil {
		q := url.Values{"uploadId": {upload}}
		if resp, aerr := s.do(context.WithoutCancel(ctx), http.MethodDelete, key, q, nil); aerr == nil {
			resp.Body.Close()
		}
	}
	return err
}

type completedPart struct {
	Number int    `xml:"PartNumber"`
	ETag   string `xml:"ETag"`
}

func (s *s3) startUpload(ctx context.Context, key string) (string, error) {
	resp, err := s.do(ctx, http.MethodPost, key, url.Values{"uploads": {""}}, nil)
	if err != nil {
		return "", fmt.Errorf("s3: starting an upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", statusError("s3", "POST", key+"?uploads", resp)
	}
	var out struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxListing)).Decode(&out); err != nil || out.UploadID == "" {
		return "", fmt.Errorf("s3: the answer to starting an upload names no upload: %v", err)
	}
	return out.UploadID, nil
}

func (s *s3) uploadPart(ctx context.Context, key, upload string, num int, data []byte) (string, error) {
	resp, err := s.do(ctx, http.MethodPut, key, url.Values{"partNumber": {strconv.Itoa(num)}, "uploadId": {upload}}, data)
	if err != nil {
		return "", fmt.Errorf("s3: storing part %d: %w", num, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", statusError("s3", "PUT", fmt.Sprintf("%s part %d", key, num), resp)
	}
	return resp.Header.Get("ETag"), nil
}

func (s *s3) completeUpload(ctx context.Context, key, upload string, parts []completedPart) error {
	body, err := xml.Marshal(struct {
		XMLName xml.Name        `xml:"CompleteMultipartUpload"`
		Parts   []completedPart `xml:"Part"`
	}{Parts: parts})
	if err != nil {
		return err
	}
	resp, err := s.do(ctx, http.MethodPost, key, url.Values{"uploadId": {upload}}, body)
	if err != nil {
		return fmt.Errorf("s3: completing the upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusError("s3", "POST", key+"?uploadId", resp)
	}
	// S3 may answer 200 and still fail, with an Error document.
	var out struct {
		XMLName xml.Name
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxListing)).Decode(&out); err == nil && out.XMLName.Local == "Error" {
		return fmt.Errorf("s3: completing the upload: %s: %s", out.Code, out.Message)
	}
	return nil
}

func (s *s3) List(ctx context.Context) ([]string, error) {
	prefix := ""
	if s.prefix != "" {
		prefix = s.prefix + "/"
	}
	var out []string
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		resp, err := s.do(ctx, http.MethodGet, "", q, nil)
		if err != nil {
			return nil, fmt.Errorf("s3: listing the bucket: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return nil, statusError("s3", "GET", s.bucket+"/"+prefix, resp)
		}
		var page struct {
			Contents []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(io.LimitReader(resp.Body, maxListing)).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("s3: reading the listing: %w", err)
		}
		for _, c := range page.Contents {
			name := strings.TrimPrefix(c.Key, prefix)
			if checkName(name) == nil {
				out = append(out, name)
			}
		}
		if !page.IsTruncated || page.NextContinuationToken == "" {
			return out, nil
		}
		token = page.NextContinuationToken
	}
}

func (s *s3) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	resp, err := s.do(ctx, http.MethodGet, s.key(name), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("s3: reading %s: %w", name, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, statusError("s3", "GET", s.key(name), resp)
	}
	return resp.Body, nil
}

func (s *s3) Delete(ctx context.Context, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	resp, err := s.do(ctx, http.MethodDelete, s.key(name), nil, nil)
	if err != nil {
		return fmt.Errorf("s3: deleting %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return statusError("s3", "DELETE", s.key(name), resp)
	}
	return nil
}
