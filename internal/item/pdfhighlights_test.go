package item

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/budget"
)

const (
	pdfV1 = ID("0000000000000000000000000000a001")
	pdfV2 = ID("0000000000000000000000000000a002")
	pngF  = ID("0000000000000000000000000000a003")
)

// pdfPage makes a page with a PDF in two versions and a picture.
func pdfPage(t *testing.T, s *Store) Item {
	t.Helper()
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Annual report"}))
	for _, a := range []Attachment{
		{ID: pdfV1, Name: "report.pdf", Type: "application/pdf", Size: 3, SHA256: strings.Repeat("ab", 32)},
		{ID: pdfV2, Name: "report.pdf", Type: "application/pdf", Size: 3, SHA256: strings.Repeat("cd", 32), Source: &Source{Item: page.ID, Attachment: pdfV1}},
		{ID: pngF, Name: "chart.png", Type: "image/png", Size: 3, SHA256: strings.Repeat("ef", 32)},
	} {
		if _, err := s.AddAttachment("anna", page.ID, a); err != nil {
			t.Fatal(err)
		}
	}
	return page
}

func TestAHighlightIsACommentOnAPassageOfAPDF(t *testing.T) {
	s := permFixture(t)
	page := pdfPage(t, s)
	a := Anchor{Quote: "revenue grew", Prefix: "In 2025 ", Suffix: " by 4 %", File: pdfV1, Version: pdfV2, Page: 3,
		Rects: []Rect{{X: 0.1, Y: 0.2, W: 0.5, H: 0.02}, {X: 0.1, Y: 0.22, W: 0.3, H: 0.02}}, Color: "green"}
	c := mustComment(t)(s.Comment("anna", page.ID, NewComment{Text: "Check against Q4", Anchor: &a}))
	if c.Anchor == nil || c.Anchor.File != pdfV1 || c.Anchor.Version != pdfV2 || c.Anchor.Page != 3 || c.Anchor.Prefix != "In 2025 " ||
		len(c.Anchor.Rects) != 2 || c.Anchor.Rects[1].Y != 0.22 || c.Anchor.Color != "green" {
		t.Fatalf("anchor = %+v", c.Anchor)
	}
	// A highlight is a thread like any other: answered, resolved, deleted.
	r := mustComment(t)(s.Comment("bert", page.ID, NewComment{Text: "Done.", Reply: c.ID}))
	if r.Reply != c.ID {
		t.Errorf("reply = %+v", r)
	}
	noErr(t, s.ResolveComment("anna", page.ID, c.ID, true))
	// An agent names the page and the quote only; the reader finds it.
	// The note is optional: a highlight may say nothing but what it marks.
	bare := mustComment(t)(s.Comment("anna", page.ID, NewComment{Text: "", Anchor: &Anchor{Quote: "outlook", File: pdfV1, Page: 1}}))
	if _, err := s.Comment("anna", page.ID, NewComment{Text: " ", Anchor: &Anchor{Quote: "outlook"}}); !errors.Is(err, ErrValidation) {
		t.Errorf("an empty comment on a passage of the body: %v", err)
	}
	if _, err := s.Comment("anna", page.ID, NewComment{Text: "", Reply: c.ID}); !errors.Is(err, ErrValidation) {
		t.Errorf("an empty reply: %v", err)
	}
	if bare.Anchor.Version != "" || bare.Anchor.Rects != nil || bare.Anchor.Color != "" {
		t.Errorf("bare anchor = %+v", bare.Anchor)
	}
	noErr(t, s.DeleteComment("anna", page.ID, bare.ID))
	// An item without a body may carry a PDF too.
	task := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "task", Title: "Review", Properties: Props{"priority": "low"}}))
	const taskPDF = ID("0000000000000000000000000000a004")
	if _, err := s.AddAttachment("anna", task.ID, Attachment{ID: taskPDF, Name: "contract", Type: "application/pdf", Size: 3, SHA256: strings.Repeat("aa", 32)}); err != nil {
		t.Fatal(err)
	}
	mustComment(t)(s.Comment("anna", task.ID, NewComment{Text: "Clause 4", Anchor: &Anchor{Quote: "liability", File: taskPDF, Page: 2, Color: "pink"}}))
	// The comments survive the journal (newTestStore replays it).
	got, err := s.As("anna").Comments(page.ID)
	noErr(t, err)
	if len(got) != 3 || !got[0].Resolved || !got[2].Deleted || got[2].Anchor != nil {
		t.Errorf("comments = %+v", got)
	}
}

func TestAHighlightNamesAPDFOfItsItemAndAPlaceOnAPage(t *testing.T) {
	s := setup(t)
	page := pdfPage(t, s)
	other := pdfPage(t, s)
	_ = other
	ok := func() Anchor {
		return Anchor{Quote: "q", File: pdfV1, Version: pdfV1, Page: 1, Rects: []Rect{{X: 0, Y: 0, W: 1, H: 1}}, Color: "yellow"}
	}
	many := make([]Rect, budget.Get().HighlightRects+1)
	for i := range many {
		many[i] = Rect{X: 0.1, Y: 0.001 * float64(i), W: 0.1, H: 0.001}
	}
	for name, change := range map[string]func(*Anchor){
		"a later version":       func(a *Anchor) { a.File = pdfV2 },
		"no PDF":                func(a *Anchor) { a.File, a.Version = pngF, "" },
		"no file of the item":   func(a *Anchor) { a.File = "0000000000000000000000000000ffff" },
		"a version of another":  func(a *Anchor) { a.Version = pngF },
		"no page":               func(a *Anchor) { a.Page = 0 },
		"a negative page":       func(a *Anchor) { a.Page = -2 },
		"off the page":          func(a *Anchor) { a.Rects = []Rect{{X: 0.6, Y: 0, W: 0.5, H: 0.1}} },
		"above the page":        func(a *Anchor) { a.Rects = []Rect{{X: 0, Y: -0.1, W: 0.5, H: 0.1}} },
		"no width":              func(a *Anchor) { a.Rects = []Rect{{X: 0, Y: 0, W: 0, H: 0.1}} },
		"not a number":          func(a *Anchor) { a.Rects = []Rect{{X: math.NaN(), Y: 0, W: 0.1, H: 0.1}} },
		"too many rectangles":   func(a *Anchor) { a.Rects = many },
		"an unknown colour":     func(a *Anchor) { a.Color = "#ff0000" },
		"neither text nor area": func(a *Anchor) { a.Quote, a.Rects = " ", nil },
		"also a drawing":        func(a *Anchor) { a.Drawing = pdfV1 },
		"also an element":       func(a *Anchor) { a.Element = "el" },
	} {
		a := ok()
		change(&a)
		if _, err := s.Comment("anna", page.ID, NewComment{Text: "?", Anchor: &a}); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// What only a highlight has is refused on a passage of the body.
	for name, a := range map[string]Anchor{
		"page":    {Quote: "x", Page: 1},
		"rects":   {Quote: "x", Rects: []Rect{{W: 0.1, H: 0.1}}},
		"colour":  {Quote: "x", Color: "yellow"},
		"version": {Quote: "x", Version: pdfV1},
	} {
		if _, err := s.Comment("anna", page.ID, NewComment{Text: "?", Anchor: &a}); !errors.Is(err, ErrValidation) {
			t.Errorf("a passage with a %s: %v", name, err)
		}
	}
	// A deleted PDF takes no new highlights.
	noErr(t, s.DeleteAttachment("anna", page.ID, pdfV1))
	noErr(t, s.DeleteAttachment("anna", page.ID, pdfV2))
	a := ok()
	if _, err := s.Comment("anna", page.ID, NewComment{Text: "?", Anchor: &a}); !errors.Is(err, ErrValidation) {
		t.Errorf("a deleted PDF: %v", err)
	}
}

// protectedPDF protects a block that shows the PDF of a page for anna and
// @hr; ben views PLAT, hanna is in @hr (blockFixture's people).
func protectedPDF(t *testing.T) (*Store, Item) {
	t.Helper()
	s, _, _, _, _ := blockFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Contracts"}))
	for _, a := range []Attachment{
		{ID: pdfV1, Name: "contract.pdf", Type: "application/pdf", Size: 3, SHA256: strings.Repeat("ab", 32)},
		{ID: pdfV2, Name: "contract.pdf", Type: "application/pdf", Size: 3, SHA256: strings.Repeat("cd", 32), Source: &Source{Item: page.ID, Attachment: pdfV1}},
	} {
		_, err := anna.AddAttachment(page.ID, a)
		noErr(t, err)
	}
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{
		{Kind: Image, Ref: pdfV2, Text: "The contract"},
		{Kind: Paragraph, Text: "Open to all."},
	}}))
	_, err := anna.ProtectBlocks(page.ID, 0, 0, []string{"@hr"})
	noErr(t, err)
	return s, page
}

func TestHighlightsInAProtectedPDFAreSeenByItsReadersOnly(t *testing.T) {
	s, page := protectedPDF(t)
	anna := s.As("anna")
	noErr(t, s.As("ben").Watch(page.ID, true))
	hl := mustComment(t)(anna.Comment(page.ID, NewComment{Text: "Penalty zebracrossing", Anchor: &Anchor{Quote: "penalty", File: pdfV1, Page: 2}}))
	mustComment(t)(anna.Comment(page.ID, NewComment{Text: "Agreed zebracrossing @ben", Reply: hl.ID}))
	open := mustComment(t)(anna.Comment(page.ID, NewComment{Text: "On the whole page"}))
	for _, who := range []string{"anna", "hanna"} {
		got, err := s.As(who).Comments(page.ID)
		noErr(t, err)
		if len(got) != 3 {
			t.Errorf("%s reads %d comments", who, len(got))
		}
	}
	ben := s.As("ben")
	got, err := ben.Comments(page.ID)
	noErr(t, err)
	if len(got) != 1 || got[0].ID != open.ID {
		t.Errorf("ben reads %+v", got)
	}
	if it := mustItem(t)(ben.Get(page.ID)); len(it.Comments) != 1 {
		t.Errorf("ben's item has %d comments", len(it.Comments))
	}
	feed, _ := ben.CommentFeed("ben", "", 0, 100)
	for _, e := range feed {
		if e.Comment.ID != open.ID {
			t.Errorf("ben's feed shows %+v", e.Comment)
		}
	}
	if hits, _ := ben.Search("zebracrossing", Query{}); len(hits) != 0 {
		t.Errorf("ben finds the note: %+v", hits)
	}
	if hits, _ := s.As("hanna").Search("zebracrossing", Query{}); len(hits) != 1 {
		t.Errorf("hanna finds %d", len(hits))
	}
	// Deleting the highlight leaves its reply hidden all the same.
	noErr(t, anna.DeleteComment(page.ID, hl.ID))
	if got, _ := ben.Comments(page.ID); len(got) != 1 {
		t.Errorf("after the deletion ben reads %+v", got)
	}
	// The mention in the reply is not ben's to be told of.
	for _, e := range s.Log() {
		if e.Kind == CommentAdded && ben.Named("ben").mentioned(e) {
			t.Errorf("ben is told of a mention in %s", e.Comment)
		}
	}
}

func TestCommentsOnAProtectedDrawingAreSeenByItsReadersOnly(t *testing.T) {
	s, page, _, _, _ := blockFixture(t)
	c := mustComment(t)(s.As("anna").Comment(page.ID, NewComment{Text: "Too high", Anchor: &Anchor{Drawing: planV1, Element: "n1"}}))
	if got, _ := s.As("ben").Comments(page.ID); slices.ContainsFunc(got, func(x Comment) bool { return x.ID == c.ID }) {
		t.Errorf("ben reads a comment on a protected drawing")
	}
	if got, _ := s.As("hanna").Comments(page.ID); !slices.ContainsFunc(got, func(x Comment) bool { return x.ID == c.ID }) {
		t.Errorf("hanna does not read the comment")
	}
}

func TestAPDFWithHighlightsStaysWithThem(t *testing.T) {
	s := permFixture(t)
	page := pdfPage(t, s)
	dst := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Archive"}))
	mustComment(t)(s.Comment("anna", page.ID, NewComment{Text: "?", Anchor: &Anchor{Quote: "x", File: pdfV1, Page: 1}}))
	if _, _, _, err := s.As("anna").planMove(page.ID, pdfV2, dst.ID); !errors.Is(err, ErrReferenced) {
		t.Errorf("a PDF with highlights moves: %v", err)
	}
}

func TestIsPDF(t *testing.T) {
	for _, c := range []struct {
		a    Attachment
		want bool
	}{
		{Attachment{Name: "a.pdf", Type: "application/octet-stream"}, true},
		{Attachment{Name: "a.PDF"}, true},
		{Attachment{Name: "contract", Type: "application/pdf"}, true},
		{Attachment{Name: "a.pdf.png", Type: "image/png"}, false},
		{Attachment{Name: "a.txt", Type: "text/plain"}, false},
	} {
		if IsPDF(c.a) != c.want {
			t.Errorf("IsPDF(%+v) = %v", c.a, !c.want)
		}
	}
}
