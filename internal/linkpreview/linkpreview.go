// Package linkpreview reads what a web page says about itself (ADR-0208):
// its title, description, site name, picture, canonical address, author
// and date of publication, from its HTML head, and, when asked, its main
// text as blocks of a body.
//
// It is the server's own outgoing call, so it never reaches an address
// that is not public unless told to: every connection is checked where it
// is opened, after name resolution and for every redirect, as the link
// checker's are (linkcheck.Transport, ADR-0039). A page and a picture are
// read within the budgets LinkPreviewPageBytes and LinkPreviewImageBytes,
// each request within LinkPreviewSeconds, and no more than
// LinkPreviewsAtOnce at once (ADR-0015). Nothing a page holds is run:
// the HTML is parsed into a tree and only text and addresses are taken
// from it; a picture is kept only when its bytes are one of the formats
// a browser shows without running anything (item.ImageTypes).
package linkpreview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/linkcheck"
)

// Errors of a reading. Use errors.Is.
var (
	// ErrAddress is an address that is not an http or https link.
	ErrAddress = errors.New("not a web link")
	// ErrInternal is a page or picture on an address that is not public.
	ErrInternal = errors.New("refused: the address is not public")
	// ErrNotPage is an answer that is not an HTML page.
	ErrNotPage = errors.New("the address is not a web page")
	// ErrNotPicture is a picture of a format a browser cannot show
	// without running something, or no picture at all.
	ErrNotPicture = errors.New("the picture is not a PNG, JPEG, GIF or WebP image")
	// ErrTooLarge is a picture larger than the budget.
	ErrTooLarge = errors.New("the picture is too large")
	// ErrBusy refuses a reading while LinkPreviewsAtOnce run.
	ErrBusy = errors.New("too many pages are being read at once; try again shortly")
	// ErrUnreachable is a page that did not answer.
	ErrUnreachable = errors.New("the page did not answer")
)

// StatusError is an answer with an HTTP error status.
type StatusError struct{ Code int }

func (e StatusError) Error() string {
	return fmt.Sprintf("the page answered %d %s", e.Code, http.StatusText(e.Code))
}

// Page is what a page says about itself. Every address is absolute, http
// or https; a value the page does not give is empty.
type Page struct {
	// URL is the address read, after redirects.
	URL         string `json:"url"`
	Canonical   string `json:"canonical,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Site        string `json:"site,omitempty"`
	Image       string `json:"image,omitempty"`
	Author      string `json:"author,omitempty"`
	// Published is the day the page names as its publication, YYYY-MM-DD.
	Published string `json:"published,omitempty"`
	// Text is the page's main text as blocks, when asked for; Cut says
	// that the page or the budget LinkPreviewBlocks cut it short.
	Text []item.Block `json:"-"`
	Cut  bool         `json:"-"`
}

// Options configure a Reader. Zero values take the defaults.
type Options struct {
	// Allowed decides which addresses may be connected to;
	// linkcheck.Public by default. OT_LINK_PREVIEWS_ALLOW_PRIVATE and the
	// tests widen it.
	Allowed func(netip.Addr) bool
}

// Reader reads pages and pictures. It is safe for concurrent use.
type Reader struct {
	client  *http.Client
	running chan struct{}
}

// redirectLimit is the number of redirects followed for one address.
const redirectLimit = 5

// maxField bounds the length of one value taken from a page, in
// characters.
const maxField = 1000

// New returns a Reader. Its budgets are read when it is made.
func New(o Options) *Reader {
	if o.Allowed == nil {
		o.Allowed = linkcheck.Public
	}
	b := budget.Get()
	timeout := time.Duration(b.LinkPreviewSeconds) * time.Second
	client := &http.Client{
		Transport: linkcheck.Transport(o.Allowed, timeout),
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= redirectLimit {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrAddress
			}
			return nil
		},
	}
	return &Reader{client: client, running: make(chan struct{}, b.LinkPreviewsAtOnce)}
}

// WebAddress checks that s is an absolute http or https address and
// returns it parsed.
func WebAddress(s string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, ErrAddress
	}
	return u, nil
}

// get sends one GET and returns the answer, its body bounded by limit+1
// bytes. The caller closes the body.
func (r *Reader) get(ctx context.Context, address, accept string) (*http.Response, error) {
	u, err := WebAddress(address)
	if err != nil {
		return nil, err
	}
	select {
	case r.running <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		<-r.running
		return nil, ErrAddress
	}
	req.Header.Set("User-Agent", "orbithek-linkpreview")
	req.Header.Set("Accept", accept)
	resp, err := r.client.Do(req)
	if err != nil {
		<-r.running
		return nil, classify(err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		<-r.running
		return nil, StatusError{resp.StatusCode}
	}
	resp.Body = released{resp.Body, r.running}
	return resp, nil
}

// released gives the reading's place back when its body is closed.
type released struct {
	io.ReadCloser
	running chan struct{}
}

func (b released) Close() error {
	err := b.ReadCloser.Close()
	<-b.running
	return err
}

// classify names why a request failed, without the address.
func classify(err error) error {
	var dns *net.DNSError
	switch {
	case errors.Is(err, ErrAddress):
		return ErrAddress
	case strings.Contains(err.Error(), "internal address"):
		return ErrInternal
	case errors.As(err, &dns):
		return fmt.Errorf("%w: the host name does not resolve", ErrUnreachable)
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Timeout"):
		return fmt.Errorf("%w in time", ErrUnreachable)
	case strings.Contains(err.Error(), "too many redirects"):
		return fmt.Errorf("%w: too many redirects", ErrUnreachable)
	}
	return ErrUnreachable
}

// Read reads the page at address and what it says about itself; with
// text, also its main text (Page.Text).
func (r *Reader) Read(ctx context.Context, address string, text bool) (Page, error) {
	resp, err := r.get(ctx, address, "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()
	mt, params, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt != "text/html" && mt != "application/xhtml+xml" {
		return Page{}, ErrNotPage
	}
	limit := budget.Get().LinkPreviewPageBytes
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return Page{}, classify(err)
	}
	// A page larger than the budget is read as far as it goes: its head,
	// where a page names itself, comes first.
	cut := int64(len(data)) > limit
	if cut {
		data = data[:limit]
	}
	return parse(resp.Request.URL, utf8Text(data, params["charset"]), text, cut)
}

// Picture reads the picture at address and returns its bytes, which are
// one of item.ImageTypes.
func (r *Reader) Picture(ctx context.Context, address string) ([]byte, error) {
	resp, err := r.get(ctx, address, "image/png,image/jpeg,image/gif,image/webp")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limit := min(budget.Get().LinkPreviewImageBytes, budget.Get().AttachmentBytes)
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	switch {
	case err != nil:
		return nil, classify(err)
	case int64(len(data)) > limit:
		return nil, ErrTooLarge
	case !slices.Contains(item.ImageTypes, http.DetectContentType(data)):
		return nil, ErrNotPicture
	}
	return data, nil
}

// --- the head -----------------------------------------------------------------

// parse reads a page's metadata and, with text, its main text.
func parse(base *url.URL, doc []byte, text, cut bool) (Page, error) {
	root, err := html.Parse(bytes.NewReader(doc))
	if err != nil {
		return Page{}, ErrNotPage
	}
	p := Page{URL: base.String()}
	meta := map[string]string{}
	var title, canonical, imageLink string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if title == "" {
					title = textOf(n)
				}
			case atom.Meta:
				key := strings.ToLower(attr(n, "property"))
				if key == "" {
					key = strings.ToLower(attr(n, "name"))
				}
				if key == "" {
					key = strings.ToLower(attr(n, "itemprop"))
				}
				if v := clean(attr(n, "content")); key != "" && v != "" {
					if _, seen := meta[key]; !seen {
						meta[key] = v
					}
				}
			case atom.Link:
				rels := strings.Fields(strings.ToLower(attr(n, "rel")))
				if slices.Contains(rels, "canonical") && canonical == "" {
					canonical = attr(n, "href")
				}
				if slices.Contains(rels, "image_src") && imageLink == "" {
					imageLink = attr(n, "href")
				}
			case atom.Script, atom.Style, atom.Template, atom.Noscript:
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := meta[k]; v != "" {
				return v
			}
		}
		return ""
	}
	p.Title = short(first("og:title", "twitter:title"))
	if p.Title == "" {
		p.Title = short(clean(title))
	}
	p.Description = short(first("og:description", "twitter:description", "description"))
	p.Site = short(first("og:site_name", "application-name"))
	if p.Site == "" {
		p.Site = strings.TrimPrefix(base.Hostname(), "www.")
	}
	if author := first("author", "article:author", "twitter:creator", "dc.creator"); author != "" && !strings.Contains(author, "://") {
		p.Author = short(author)
	}
	p.Published = day(first("article:published_time", "og:published_time", "datepublished", "date", "dc.date", "dc.date.issued", "dcterms.created"))
	p.Image = absolute(base, first("og:image:secure_url", "og:image", "og:image:url", "twitter:image", "twitter:image:src"))
	if p.Image == "" {
		p.Image = absolute(base, imageLink)
	}
	p.Canonical = absolute(base, canonical)
	if p.Canonical == "" {
		p.Canonical = absolute(base, first("og:url"))
	}
	if text {
		p.Text, p.Cut = mainText(root, base)
		p.Cut = p.Cut || cut
	}
	return p, nil
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Namespace == "" && strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

// textOf is the text below a node, scripts and styles left out.
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && (n.DataAtom == atom.Script || n.DataAtom == atom.Style):
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// clean collapses white space and drops control characters.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// short cuts a value to maxField characters.
func short(s string) string {
	if utf8.RuneCountInString(s) <= maxField {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:maxField-1])) + "…"
}

// day takes the day of a date or point in time a page names.
func day(s string) string {
	if len(s) < 10 {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s[:10]); err != nil {
		return ""
	}
	return s[:10]
}

// absolute resolves an address against the page's and keeps it only when
// it is http or https.
func absolute(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

// --- character sets -------------------------------------------------------------

// utf8Text returns a page's bytes as UTF-8: pages in ISO 8859-1 or
// Windows-1252, as named by the answer or the page, are converted; every
// other page is taken as UTF-8, invalid bytes replaced.
func utf8Text(data []byte, charset string) []byte {
	if charset == "" {
		charset = sniffCharset(data)
	}
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "iso-8859-1", "latin1", "latin-1", "windows-1252", "cp1252", "us-ascii", "ascii":
		var b strings.Builder
		for _, c := range data {
			if c >= 0x80 && c < 0xa0 {
				b.WriteRune(cp1252[c-0x80])
			} else {
				b.WriteRune(rune(c))
			}
		}
		return []byte(b.String())
	}
	return bytes.ToValidUTF8(data, []byte("�"))
}

// sniffCharset finds the character set a page names in its first bytes.
func sniffCharset(data []byte) string {
	head := strings.ToLower(string(data[:min(len(data), 2048)]))
	i := strings.Index(head, "charset=")
	if i < 0 {
		return ""
	}
	v := strings.TrimLeft(head[i+len("charset="):], `"' `)
	end := strings.IndexAny(v, `"'; />`)
	if end < 0 {
		return ""
	}
	return v[:end]
}

// cp1252 maps the bytes 0x80 to 0x9f of Windows-1252, which ISO 8859-1
// leaves to control characters and pages use for quotes and dashes.
var cp1252 = [32]rune{
	'€', '�', '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', '�', 'Ž', '�',
	'�', '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', '�', 'ž', 'Ÿ',
}
