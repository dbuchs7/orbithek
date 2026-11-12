package connector

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/site"
)

// The connector of the kind highlights reads one's highlights from
// Readwise (ADR-0210) through its export API, version 2: GET
// <address>/api/v2/export/ with the account's token from
// OT_CONNECTOR_<ID>_TOKEN, sent as "Authorization: Token <token>", page
// by page by nextPageCursor, and, but for a full run, with updatedAfter
// for what changed since the last run. Each book or article becomes an
// item; its highlights are quotes with their notes and locations above a
// line, and what people write below the line stays.

// exportPath is where the service answers its export.
const exportPath = "/api/v2/export/"

// waits bounds how often a run waits when the service asks it to, and
// longestWait how long it waits each time: the export answers a few
// requests a minute.
const (
	waits       = 4
	longestWait = 60 * time.Second
)

// pause waits for d or until ctx ends; tests replace it.
var pause = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ErrNoToken is a connector of highlights without its account's token.
var ErrNoToken = errors.New("connector: the reading service needs the account's token")

type rwTag struct {
	Name string `json:"name"`
}

// rwHighlight is a highlight as the export gives it.
type rwHighlight struct {
	ID            int64           `json:"id"`
	Text          string          `json:"text"`
	Note          string          `json:"note,omitempty"`
	Location      json.RawMessage `json:"location,omitempty"`
	LocationType  string          `json:"location_type,omitempty"`
	HighlightedAt string          `json:"highlighted_at,omitempty"`
	URL           string          `json:"url,omitempty"`
	ReadwiseURL   string          `json:"readwise_url,omitempty"`
	Tags          []rwTag         `json:"tags,omitempty"`
	IsDeleted     bool            `json:"is_deleted,omitempty"`
	IsDiscard     bool            `json:"is_discard,omitempty"`
}

// rwBook is a book or article as the export gives it.
type rwBook struct {
	UserBookID    int64         `json:"user_book_id"`
	Title         string        `json:"title"`
	ReadableTitle string        `json:"readable_title"`
	Author        string        `json:"author"`
	Source        string        `json:"source"`
	CoverImageURL string        `json:"cover_image_url"`
	UniqueURL     string        `json:"unique_url"`
	Category      string        `json:"category"`
	DocumentNote  string        `json:"document_note"`
	Summary       string        `json:"summary"`
	ReadwiseURL   string        `json:"readwise_url"`
	SourceURL     string        `json:"source_url"`
	BookTags      []rwTag       `json:"book_tags"`
	Highlights    []rwHighlight `json:"highlights"`
}

type rwPage struct {
	Results        []rwBook        `json:"results"`
	NextPageCursor json.RawMessage `json:"nextPageCursor"`
}

// keptBook is what an item keeps of its book in its origin: the book
// without its highlights, and the highlights in their order, so the next
// run tells a change and merges what changed.
type keptBook struct {
	Book       rwBook        `json:"book"`
	Highlights []rwHighlight `json:"highlights"`
}

// highlightsReader reads the highlights of the account whose token the
// environment holds.
type highlightsReader struct{}

func (highlightsReader) Read(ctx context.Context, c site.Connector, env Env) (reading, error) {
	var rd reading
	if env.Token == "" {
		return rd, fmt.Errorf("%w in %s", ErrNoToken, TokenVar(c.ID))
	}
	q := url.Values{}
	if !env.Since.IsZero() {
		q.Set("updatedAfter", env.Since.UTC().Format(time.RFC3339))
		rd.partial = true
	}
	base := strings.TrimRight(c.Highlights.URL, "/") + exportPath
	w := wordsIn(language(c))
	seen := map[int64]bool{}
	for page := 0; page < maxPages; page++ {
		addr := base
		if len(q) > 0 {
			addr += "?" + q.Encode()
		}
		p, err := exportPage(ctx, env, addr, c.ID)
		if err != nil {
			return rd, err
		}
		for _, b := range p.Results {
			rd.datasets++
			if b.UserBookID == 0 || seen[b.UserBookID] {
				rd.problem("%s: a book without an ID of its own, or given twice, is left out", cmp.Or(b.ReadableTitle, b.Title))
				continue
			}
			seen[b.UserBookID] = true
			rd.entries = append(rd.entries, bookEntry(b, w, c.Highlights.URL, rd.partial))
		}
		cursor := strings.Trim(string(p.NextPageCursor), `"`)
		if cursor == "" || cursor == "null" {
			break
		}
		q.Set("pageCursor", cursor)
		if page == maxPages-1 {
			rd.problem("the export has more than %d pages; the rest is not read", maxPages)
		}
	}
	rd.apply = true
	return rd, nil
}

// exportPage reads one page of the export, waiting as long as the
// service asks when it is asked too often.
func exportPage(ctx context.Context, env Env, addr, id string) (rwPage, error) {
	var p rwPage
	for try := 0; ; try++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
		if err != nil {
			return p, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Token "+env.Token)
		resp, err := env.HTTP.Do(req)
		if err != nil {
			return p, err
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
		resp.Body.Close()
		switch {
		case err != nil:
			return p, err
		case resp.StatusCode == http.StatusTooManyRequests && try < waits:
			secs, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
			if err != nil || secs < 0 {
				secs = 10
			}
			if err := pause(ctx, min(time.Duration(secs)*time.Second, longestWait)); err != nil {
				return p, err
			}
			continue
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return p, fmt.Errorf("connector: the reading service refused the token in %s (%s)", TokenVar(id), resp.Status)
		case resp.StatusCode != http.StatusOK:
			return p, fmt.Errorf("connector: %s answered %s", addr, resp.Status)
		case len(data) > maxAnswer:
			return p, fmt.Errorf("connector: the answer of %s is larger than %d MB", addr, maxAnswer>>20)
		}
		if err := json.Unmarshal(data, &p); err != nil {
			return p, fmt.Errorf("connector: the answer of %s is not the export: %w", addr, err)
		}
		return p, nil
	}
}

// bookEntry is a book's entry: its title, values, labels and the page
// written from its highlights, merged with those it had when only what
// changed was read.
func bookEntry(b rwBook, w labels, service string, partial bool) Entry {
	title := oneLine(cmp.Or(b.ReadableTitle, b.Title, strconv.FormatInt(b.UserBookID, 10)))
	link := cmp.Or(webOnly(b.SourceURL), webOnly(b.UniqueURL), webOnly(b.ReadwiseURL))
	var tags []string
	for _, t := range b.BookTags {
		if s := strings.TrimSpace(t.Name); s != "" {
			tags = append(tags, s)
		}
	}
	values := map[string]string{"author": b.Author, "category": b.Category, "cover": webOnly(b.CoverImageURL), "source": b.Source,
		"url": cmp.Or(webOnly(b.SourceURL), webOnly(b.UniqueURL)), "link": webOnly(b.ReadwiseURL), "tags": strings.Join(tags, ", ")}
	return Entry{SourceID: strconv.FormatInt(b.UserBookID, 10), Dataset: title, Title: title, DatasetURL: link, Values: values, Labels: tags,
		Below: isBelowLine,
		Update: func(kept string) (Content, error) {
			k := keptBook{Book: b, Highlights: mergeHighlights(kept, b.Highlights, partial)}
			k.Book.Highlights = nil
			raw, err := json.Marshal(k)
			if err != nil {
				return Content{}, err
			}
			return Content{Title: title, Body: highlightsPage(k, w, service), Raw: string(raw)}, nil
		}}
}

// webOnly is an address if it is a web address a page may link.
func webOnly(s string) string {
	s = strings.TrimSpace(s)
	if webLink(s) && item.SafeHref(s) {
		return s
	}
	return ""
}

// mergeHighlights are a book's highlights: those read, and, when only
// what changed was read, those kept before that were not changed, less
// those deleted or discarded, in the order of their places in the book.
func mergeHighlights(kept string, read []rwHighlight, partial bool) []rwHighlight {
	byID := map[int64]rwHighlight{}
	var k keptBook
	if partial && kept != "" && json.Unmarshal([]byte(kept), &k) == nil {
		for _, h := range k.Highlights {
			byID[h.ID] = h
		}
	}
	for _, h := range read {
		if h.IsDeleted || h.IsDiscard {
			delete(byID, h.ID)
			continue
		}
		byID[h.ID] = h
	}
	out := make([]rwHighlight, 0, len(byID))
	for _, h := range byID {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b rwHighlight) int {
		return cmp.Or(cmp.Compare(place(a), place(b)), strings.Compare(a.HighlightedAt, b.HighlightedAt), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// place is a highlight's place in its book as a number, 0 when unknown.
func place(h rwHighlight) float64 {
	n, _ := strconv.ParseFloat(strings.Trim(string(h.Location), `"`), 64)
	return n
}

// isBelowLine finds the line under which people write what a run keeps,
// in any language of the pages (ADR-0210).
func isBelowLine(b item.Block) bool {
	if b.Kind != item.Panel || b.Tone != "note" {
		return false
	}
	for _, w := range pageLabels {
		if b.Text == w.below {
			return true
		}
	}
	return false
}

// highlightsPage writes a book's page: where it comes from, its address,
// cover, summary and note, each highlight as a quote with its note and
// place, and the line under which people write.
func highlightsPage(k keptBook, w labels, service string) []item.Block {
	var d doc
	d.blocks = append(d.blocks, item.Block{Kind: item.Panel, Tone: "info", Text: fmt.Sprintf(w.fromHighlights, service)})
	d.links([][2]string{{w.source, cmp.Or(webOnly(k.Book.SourceURL), webOnly(k.Book.UniqueURL))}, {w.cover, webOnly(k.Book.CoverImageURL)}})
	d.paragraphs(k.Book.Summary)
	if note := strings.TrimSpace(k.Book.DocumentNote); note != "" {
		d.blocks = append(d.blocks, italic(item.Paragraph, clip(note)))
	}
	for _, h := range k.Highlights {
		if text := strings.TrimSpace(h.Text); text != "" {
			d.add(item.Quote, clip(text))
		}
		if note := strings.TrimSpace(h.Note); note != "" {
			d.blocks = append(d.blocks, italic(item.Paragraph, clip(note)))
		}
		if where := whereIs(h, w); where != "" {
			b := item.Block{Kind: item.Paragraph, Text: where}
			if href := cmp.Or(webOnly(h.URL), webOnly(h.ReadwiseURL)); href != "" {
				b.Marks = []item.Mark{{Type: item.MarkLink, Start: 0, End: utf8.RuneCountInString(where), Href: href}}
			}
			d.blocks = append(d.blocks, b)
		}
	}
	d.blocks = append(d.blocks, item.Block{Kind: item.Divider}, item.Block{Kind: item.Panel, Tone: "note", Text: w.below})
	return d.blocks
}

// italic is a block of text set in italics, as a note is.
func italic(kind item.BlockKind, text string) item.Block {
	return item.Block{Kind: kind, Text: text, Marks: []item.Mark{{Type: item.MarkItalic, Start: 0, End: utf8.RuneCountInString(text)}}}
}

// whereIs tells a highlight's place, e.g. "Page 12" or "At 03:20", and
// its tags.
func whereIs(h rwHighlight, w labels) string {
	var parts []string
	if n := place(h); n > 0 {
		switch h.LocationType {
		case "page":
			parts = append(parts, fmt.Sprintf("%s %g", w.onPage, n))
		case "location":
			parts = append(parts, fmt.Sprintf("%s %g", w.location, n))
		case "time_offset":
			s := int(n)
			parts = append(parts, fmt.Sprintf("%s %02d:%02d", w.at, s/60, s%60))
		default:
			parts = append(parts, fmt.Sprintf("%s %g", w.position, n))
		}
	}
	var tags []string
	for _, t := range h.Tags {
		if s := strings.TrimSpace(t.Name); s != "" {
			tags = append(tags, "#"+s)
		}
	}
	if len(tags) > 0 {
		parts = append(parts, strings.Join(tags, " "))
	}
	return oneLine(strings.Join(parts, " · "))
}
