package linkpreview

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// loopback lets the tests reach their own server, which the public
// check refuses.
func loopback(a netip.Addr) bool { return a.IsLoopback() }

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

const article = `<!doctype html><html><head>
<meta charset="utf-8">
<title>  Fallback   title </title>
<meta property="og:title" content="The page's title">
<meta name="description" content="Short description">
<meta property="og:description" content="What the page is about">
<meta property="og:site_name" content="Example News">
<meta property="og:image" content="/pictures/card.png">
<meta name="author" content="Ada Muster">
<meta property="article:published_time" content="2026-10-30T08:15:00+01:00">
<link rel="canonical" href="https://example.org/story">
<script>alert("never")</script>
</head><body>
<header><a href="/">Home</a></header>
<nav><ul><li>Menu</li></ul></nav>
<article>
<h1>Headline</h1>
<p>First <b>bold</b> and <a href="/more">a link</a>, then <em>more</em>.</p>
<h4>Small heading</h4>
<ul><li>one</li><li>two</li></ul>
<ol><li>first</li></ol>
<blockquote><p>Quoted words</p></blockquote>
<pre>code
  indented</pre>
<table><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table>
<hr>
<script>document.write("no")</script>
<form><input value="no"></form>
<p onclick="x()">Last <img src="x.png" alt="picture"> line<br>broken</p>
</article>
<footer>Copyright</footer>
</body></html>`

func newServer(t *testing.T, mux *http.ServeMux) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAPageNamesItself(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/story", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "orbithek-linkpreview" {
			t.Errorf("user agent %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(article))
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/story", http.StatusMovedPermanently)
	})
	srv := newServer(t, mux)
	r := New(Options{Allowed: loopback})
	p, err := r.Read(context.Background(), srv.URL+"/old", false)
	if err != nil {
		t.Fatal(err)
	}
	want := Page{URL: srv.URL + "/story", Canonical: "https://example.org/story", Title: "The page's title", Description: "What the page is about",
		Site: "Example News", Image: srv.URL + "/pictures/card.png", Author: "Ada Muster", Published: "2026-10-30"}
	if p.URL != want.URL || p.Canonical != want.Canonical || p.Title != want.Title || p.Description != want.Description || p.Site != want.Site ||
		p.Image != want.Image || p.Author != want.Author || p.Published != want.Published || p.Text != nil {
		t.Fatalf("got  %+v\nwant %+v", p, want)
	}
}

func TestFallbacksOfAPlainPage(t *testing.T) {
	base, _ := url.Parse("https://www.example.org/a/b")
	p, err := parse(base, []byte(`<html><head><title> Plain
	page </title><meta name="description" content="  spaced   out "><meta name="twitter:image" content="javascript:alert(1)">
	<link rel="image_src" href="pic.jpg"><meta name="author" content="https://example.org/ada"><meta name="date" content="soon"></head></html>`), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Plain page" || p.Description != "spaced out" || p.Site != "example.org" || p.Image != "https://www.example.org/a/pic.jpg" ||
		p.Author != "" || p.Published != "" || p.Canonical != "" {
		t.Fatalf("%+v", p)
	}
	long := strings.Repeat("x", maxField+50)
	p, _ = parse(base, []byte(`<title>`+long+`</title>`), false, false)
	if n := len([]rune(p.Title)); n != maxField || !strings.HasSuffix(p.Title, "…") {
		t.Fatalf("a long title is cut to %d: %d", maxField, n)
	}
}

func TestTheMainTextBecomesBlocks(t *testing.T) {
	base, _ := url.Parse("https://example.org/story")
	p, err := parse(base, []byte(article), true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []item.Block{
		{Kind: item.Heading, Level: 1, Text: "Headline"},
		{Kind: item.Paragraph, Text: "First bold and a link, then more.", Marks: []item.Mark{
			{Type: item.MarkBold, Start: 6, End: 10}, {Type: item.MarkLink, Start: 15, End: 21, Href: "https://example.org/more"}, {Type: item.MarkItalic, Start: 28, End: 32}}},
		{Kind: item.Heading, Level: 3, Text: "Small heading"},
		{Kind: item.BulletItem, Text: "one"},
		{Kind: item.BulletItem, Text: "two"},
		{Kind: item.OrderedItem, Text: "first"},
		{Kind: item.Quote, Text: "Quoted words"},
		{Kind: item.CodeBlock, Text: "code\n  indented"},
		{Kind: item.Table, Text: "A\tB\n1\t2"},
		{Kind: item.Divider},
		{Kind: item.Paragraph, Text: "Last picture line broken"},
	}
	if len(p.Text) != len(want) {
		t.Fatalf("%d blocks, want %d: %+v", len(p.Text), len(want), p.Text)
	}
	for i := range want {
		g, w := p.Text[i], want[i]
		if g.Kind != w.Kind || g.Text != w.Text || g.Level != w.Level || len(g.Marks) != len(w.Marks) {
			t.Fatalf("block %d: got %+v, want %+v", i, g, w)
		}
		for j := range w.Marks {
			if g.Marks[j] != w.Marks[j] {
				t.Errorf("block %d mark %d: got %+v, want %+v", i, j, g.Marks[j], w.Marks[j])
			}
		}
	}
	if p.Cut {
		t.Error("the text is whole")
	}
	// Every block is one a body takes.
	s := item.NewStore()
	noErr(t, s.DefineType("a", item.TypeDef{Name: "page", Body: true}))
	noErr(t, s.CreateSpace("a", "S", "S"))
	if _, err := s.Create("a", item.NewItem{Space: "S", Type: "page", Title: "t", Body: &item.Doc{Blocks: p.Text}}); err != nil {
		t.Fatalf("a body refuses the text: %v", err)
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestTheTextFallsBackToMainAndBody(t *testing.T) {
	base, _ := url.Parse("https://example.org/")
	p, _ := parse(base, []byte(`<body><p>outside</p><div role="main"><div>loose <i>text</i></div><p hidden="hidden">gone</p></div></body>`), true, false)
	if len(p.Text) != 1 || p.Text[0].Text != "loose text" || p.Text[0].Marks[0].Type != item.MarkItalic {
		t.Fatalf("main: %+v", p.Text)
	}
	p, _ = parse(base, []byte(`<body><p>only</p><table><tr><td>a</td></tr><tr><td>b</td><td>c</td></tr></table></body>`), true, false)
	if len(p.Text) != 3 || p.Text[0].Text != "only" || p.Text[1].Text != "a" || p.Text[2].Text != "b · c" {
		t.Fatalf("body: %+v", p.Text)
	}
	b := budget.Default()
	b.LinkPreviewBlocks = 2
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	p, _ = parse(base, []byte(`<p>1</p><p>2</p><p>3</p>`), true, false)
	if len(p.Text) != 2 || !p.Cut {
		t.Fatalf("the budget cuts the text: %+v", p)
	}
}

func TestOldCharacterSetsAreRead(t *testing.T) {
	base, _ := url.Parse("https://example.org/")
	latin := append([]byte(`<meta charset="iso-8859-1"><title>Z`), 0xfc, 'r', 'i', 'c', 'h', ' ', 0x93, 'x', 0x94, '<', '/', 't', 'i', 't', 'l', 'e', '>')
	p, _ := parse(base, utf8Text(latin, ""), false, false)
	if p.Title != "Zürich “x”" {
		t.Fatalf("%q", p.Title)
	}
	if got := string(utf8Text([]byte{'a', 0xff, 'b'}, "utf-8")); got != "a�b" {
		t.Fatalf("invalid UTF-8: %q", got)
	}
	if sniffCharset([]byte(`<meta charset=`)) != "" {
		t.Error("an unterminated charset names none")
	}
}

func TestOnlyPublicAddressesAndWebPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/ftp", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "ftp://example.org/x", http.StatusFound)
	})
	srv := newServer(t, mux)
	ctx := context.Background()
	// The default reader refuses the test's own, local server.
	if _, err := New(Options{}).Read(ctx, srv.URL+"/json", false); !errors.Is(err, ErrInternal) {
		t.Fatalf("a local address: %v", err)
	}
	r := New(Options{Allowed: loopback})
	for path, want := range map[string]error{"/json": ErrNotPage, "/loop": ErrUnreachable, "/ftp": ErrAddress} {
		if _, err := r.Read(ctx, srv.URL+path, false); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", path, err, want)
		}
	}
	var se StatusError
	if _, err := r.Read(ctx, srv.URL+"/gone", false); !errors.As(err, &se) || se.Code != 404 || !strings.Contains(err.Error(), "404") {
		t.Errorf("a missing page: %v", err)
	}
	for _, bad := range []string{"", "javascript:alert(1)", "/relative", "https://user:pw@example.org/", "ftp://example.org/"} {
		if _, err := r.Read(ctx, bad, false); !errors.Is(err, ErrAddress) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, err := r.Read(ctx, "http://no-such-host.invalid/", false); !errors.Is(err, ErrUnreachable) {
		t.Errorf("an unknown host: %v", err)
	}
	// A redirect from a public page to an internal address is caught as
	// the connection is opened: here the test's server is the internal one.
	public := New(Options{Allowed: func(a netip.Addr) bool { return !a.IsLoopback() }})
	if _, err := public.Read(ctx, srv.URL+"/json", false); !errors.Is(err, ErrInternal) {
		t.Errorf("an internal address: %v", err)
	}
}

func TestAPictureIsCheckedByItsBytes(t *testing.T) {
	picture := pngBytes(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/card.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml") // the bytes decide, not the header
		w.Write(picture)
	})
	mux.HandleFunc("/card.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	})
	mux.HandleFunc("/big.png", func(w http.ResponseWriter, r *http.Request) { w.Write(append(picture, make([]byte, 2000)...)) })
	srv := newServer(t, mux)
	r := New(Options{Allowed: loopback})
	got, err := r.Picture(context.Background(), srv.URL+"/card.png")
	if err != nil || !bytes.Equal(got, picture) {
		t.Fatalf("picture: %v", err)
	}
	if _, err := r.Picture(context.Background(), srv.URL+"/card.svg"); !errors.Is(err, ErrNotPicture) {
		t.Fatalf("an SVG: %v", err)
	}
	b := budget.Default()
	b.LinkPreviewImageBytes = int64(len(picture) + 100)
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	if _, err := r.Picture(context.Background(), srv.URL+"/big.png"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("a large picture: %v", err)
	}
}

func TestALargePageIsReadAsFarAsTheBudget(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<title>Big</title><article><p>start</p>` + strings.Repeat("<p>filler</p>", 1000) + `</article>`))
	})
	srv := newServer(t, mux)
	b := budget.Default()
	b.LinkPreviewPageBytes = 200
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	p, err := New(Options{Allowed: loopback}).Read(context.Background(), srv.URL+"/", true)
	if err != nil || p.Title != "Big" || !p.Cut || len(p.Text) == 0 || p.Text[0].Text != "start" {
		t.Fatalf("%v %+v", err, p)
	}
}

func TestReadingsAtOnceAreBounded(t *testing.T) {
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.(http.Flusher).Flush()
		<-release
	})
	srv := newServer(t, mux)
	b := budget.Default()
	b.LinkPreviewsAtOnce = 1
	budget.Set(b)
	t.Cleanup(func() { budget.Set(budget.Default()) })
	r := New(Options{Allowed: loopback})
	done := make(chan error)
	go func() {
		_, err := r.Read(context.Background(), srv.URL+"/", false)
		done <- err
	}()
	// Wait until the first reading holds its place.
	for len(r.running) == 0 {
		select {
		case err := <-done:
			t.Fatalf("the first reading ended: %v", err)
		default:
		}
	}
	if _, err := r.Read(context.Background(), srv.URL+"/", false); !errors.Is(err, ErrBusy) {
		t.Fatalf("a second reading at once: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(r.running) != 0 {
		t.Fatal("the place was not given back")
	}
}
