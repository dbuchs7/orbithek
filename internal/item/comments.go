package item

import (
	"math"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
)

// Comment is one comment on an item (ADR-0035). It is part of its item:
// readable when the item is, erased with it.
type Comment struct {
	ID      ID
	Author  string
	Created time.Time
	// Edited is when the text last changed, or zero.
	Edited time.Time
	// Text is plain text; it is empty once the comment is deleted.
	Text    string
	Deleted bool
	// Anchor is the text of the body an inline comment is on; nil for a
	// comment on the whole item (ADR-0047).
	Anchor *Anchor
	// Reply is the comment this one answers; empty for the first comment
	// of a thread. A thread is one level deep.
	Reply ID
	// Resolved is whether the thread was marked done, by ResolvedBy at
	// ResolvedAt; only the first comment of a thread is resolved.
	Resolved   bool
	ResolvedBy string
	ResolvedAt time.Time
	// file is the file the comment is on (a PDF or a drawing), kept when
	// the comment is deleted, so its thread stays with the file's readers
	// (commentsOnHiddenFiles).
	file ID
}

// Anchor names a passage of a body by its text (ADR-0047): the quoted
// text and a little of what precedes and follows it, which tells apart
// passages of the same text. Its position is found again when the body is
// shown; when the quoted text is gone, the comment is no longer on any
// passage, and it says so.
//
// An anchor may name an element of a drawing instead (ADR-0171): Drawing
// is the drawing's first version, Element the element's ID in its scene,
// and Quote the element's text, which may be empty.
//
// Or it names a highlight in a PDF file of the item (ADR-0208): File is
// the PDF's first version, which outlives every new version; Version the
// version the highlight was made on, whose Rects place it on Page (from
// 1); Quote, Prefix and Suffix the text it marks, by which a reader finds
// it again on another version; Color its colour, one of
// HighlightColors, or empty for the first. Agents name a page and a
// quote only.
type Anchor struct {
	Quote   string
	Prefix  string `json:",omitempty"`
	Suffix  string `json:",omitempty"`
	Drawing ID     `json:",omitempty"`
	Element string `json:",omitempty"`
	File    ID     `json:",omitempty"`
	Version ID     `json:",omitempty"`
	Page    int    `json:",omitempty"`
	Rects   []Rect `json:",omitempty"`
	Color   string `json:",omitempty"`
}

// Rect is a rectangle on a page of a PDF, in fractions of the page's width
// and height from its top left corner, the page shown as its own rotation
// turns it (ADR-0208).
type Rect struct {
	X, Y, W, H float64
}

// HighlightColors are the colours a highlight in a PDF may have; the
// first is the one an anchor without a colour has (ADR-0208).
var HighlightColors = []string{"yellow", "green", "blue", "pink", "purple"}

// IsPDF reports whether a file is a PDF: told so by its bytes when it was
// stored, or named so.
func IsPDF(a Attachment) bool {
	return a.Type == "application/pdf" || strings.EqualFold(path.Ext(a.Name), ".pdf")
}

// checkHighlight checks a highlight in a PDF of an item (ADR-0208).
func checkHighlight(it *Item, a Anchor) error {
	if a.Drawing != "" || a.Element != "" {
		return invalid("a comment is on a PDF or on a drawing, not on both")
	}
	d, ok := drawing(it, a.File)
	if !ok || d.First != a.File {
		return invalid("item %s has no file %s; a highlight names a PDF by its first version", it.Key, a.File)
	}
	v := a.Version
	if v == "" {
		v = a.File
	}
	f, ok := it.attachment(v)
	if !ok || f.Deleted || !slices.Contains(d.Versions, v) {
		return invalid("%s is no version of the file %s", v, a.File)
	}
	if !IsPDF(*f) {
		return invalid("the file %s is no PDF", f.Name)
	}
	if a.Page < 1 {
		return invalid("a highlight is on a page, counted from 1")
	}
	if n := budget.Get().HighlightRects; len(a.Rects) > n { // ADR-0015
		return invalid("a highlight draws at most %d rectangles", n)
	}
	const slack = 1e-6
	for _, r := range a.Rects {
		sum := r.X + r.Y + r.W + r.H
		if math.IsNaN(sum) || math.IsInf(sum, 0) || r.X < 0 || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > 1+slack || r.Y+r.H > 1+slack {
			return invalid("a rectangle of a highlight lies on its page, in fractions of it")
		}
	}
	if a.Color != "" && !slices.Contains(HighlightColors, a.Color) {
		return invalid("colour %q: a highlight is %s", a.Color, strings.Join(HighlightColors, ", "))
	}
	if strings.TrimSpace(a.Quote) == "" && len(a.Rects) == 0 {
		return invalid("a highlight needs the text it marks or its place on the page")
	}
	return nil
}

// anchorFile is the file an anchor is on, or empty for a passage of the
// body.
func anchorFile(a *Anchor) ID {
	switch {
	case a == nil:
		return ""
	case a.File != "":
		return a.File
	}
	return a.Drawing
}

// elementIDRE is the form of an element's ID in a drawing's scene
// (internal/whiteboard).
var elementIDRE = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

// NewComment is a comment to add: on the whole item, on a passage of its
// body (Anchor) or in answer to another comment (Reply).
type NewComment struct {
	Text   string
	Anchor *Anchor
	Reply  ID
}

// AddComment adds a comment to a live item and returns it.
func (s *Store) AddComment(actor string, id ID, text string) (Comment, error) {
	return s.Comment(actor, id, NewComment{Text: text})
}

// Comment adds a comment, an inline comment or a reply to a live item and
// returns it.
func (s *Store) Comment(actor string, id ID, n NewComment) (Comment, error) {
	it, err := s.live(id)
	if err != nil {
		return Comment{}, err
	}
	// A highlight in a PDF needs no note (ADR-0208); every other comment
	// says something.
	if strings.TrimSpace(n.Text) == "" && (n.Anchor == nil || n.Anchor.File == "") {
		return Comment{}, invalid("comment is empty")
	}
	if n.Anchor != nil && n.Reply != "" {
		return Comment{}, invalid("a reply is on its thread's passage, not on its own")
	}
	if n.Anchor != nil {
		a := *n.Anchor
		if a.File == "" && (a.Version != "" || a.Page != 0 || len(a.Rects) > 0 || a.Color != "") {
			return Comment{}, invalid("only a highlight in a PDF has a version, a page, rectangles and a colour")
		}
		if td, _ := s.latest(it.Type); !td.Body && a.File == "" {
			return Comment{}, invalid("type %q has no body to comment on", it.Type)
		}
		switch {
		case a.File != "":
			// A PDF is a file of any item, with a body or without.
			if err := checkHighlight(it, a); err != nil {
				return Comment{}, err
			}
			a.Rects = slices.Clone(a.Rects)
		case a.Element != "" || a.Drawing != "":
			if !elementIDRE.MatchString(a.Element) {
				return Comment{}, invalid("element %q of a drawing", a.Element)
			}
			if _, ok := it.attachment(a.Drawing); !ok {
				return Comment{}, invalid("item %s has no drawing %s", it.Key, a.Drawing)
			}
			a.Prefix, a.Suffix = "", ""
		case strings.TrimSpace(a.Quote) == "":
			return Comment{}, invalid("an inline comment needs the text it is on")
		}
		n.Anchor = &a
	}
	if n.Reply != "" {
		root, err := s.liveComment(id, n.Reply)
		if err != nil {
			return Comment{}, err
		}
		if root.Reply != "" {
			// Answering a reply answers its thread.
			if _, err := s.liveComment(id, root.Reply); err != nil {
				return Comment{}, err
			}
			n.Reply = root.Reply
		}
	}
	cid := s.newID()
	if err := s.record(actor, Event{Kind: CommentAdded, Item: id, Comment: cid, Text: n.Text, Anchor: n.Anchor, Reply: n.Reply}); err != nil {
		return Comment{}, err
	}
	c, _ := s.items[id].comment(cid)
	return *c, nil
}

// ResolveComment marks a thread done, or open again.
func (s *Store) ResolveComment(actor string, id, comment ID, resolved bool) error {
	c, err := s.liveComment(id, comment)
	if err != nil {
		return err
	}
	if c.Reply != "" {
		return invalid("only a thread is resolved, not a reply in it")
	}
	if c.Resolved == resolved {
		return nil
	}
	kind := CommentResolved
	if !resolved {
		kind = CommentReopened
	}
	return s.record(actor, Event{Kind: kind, Item: id, Comment: comment})
}

// EditComment replaces the text of a comment that is not deleted.
func (s *Store) EditComment(actor string, id, comment ID, text string) error {
	if _, err := s.liveComment(id, comment); err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return invalid("comment is empty")
	}
	return s.record(actor, Event{Kind: CommentEdited, Item: id, Comment: comment, Text: text})
}

// DeleteComment hides a comment's text from every reader. The text stays in
// the encrypted log until the item is erased (ADR-0035).
func (s *Store) DeleteComment(actor string, id, comment ID) error {
	if _, err := s.liveComment(id, comment); err != nil {
		return err
	}
	return s.record(actor, Event{Kind: CommentDeleted, Item: id, Comment: comment})
}

// liveComment returns a comment that is not deleted, on a live item.
func (s *Store) liveComment(id, comment ID) (*Comment, error) {
	it, err := s.live(id)
	if err != nil {
		return nil, err
	}
	c, ok := it.comment(comment)
	if !ok {
		return nil, notFound("comment %s", comment)
	}
	if c.Deleted {
		return nil, invalid("comment %s is deleted", comment)
	}
	return c, nil
}

func (it *Item) comment(id ID) (*Comment, bool) {
	for i := range it.Comments {
		if it.Comments[i].ID == id {
			return &it.Comments[i], true
		}
	}
	return nil, false
}

// applyComment applies a comment event. Comments leave the item's Version
// alone: they are not a change of the item itself.
func (s *Store) applyComment(e Event, touch func(...ID)) {
	it := s.items[e.Item]
	touch(it.ID)
	switch e.Kind {
	case CommentAdded:
		it.Comments = append(it.Comments, Comment{ID: e.Comment, Author: e.Actor, Created: e.At, Text: e.Text, Anchor: e.Anchor, Reply: e.Reply, file: anchorFile(e.Anchor)})
	case CommentEdited:
		if c, ok := it.comment(e.Comment); ok {
			c.Text, c.Edited = e.Text, e.At
		}
	case CommentDeleted:
		if c, ok := it.comment(e.Comment); ok {
			c.Text, c.Anchor, c.Deleted = "", nil, true
		}
	case CommentResolved:
		if c, ok := it.comment(e.Comment); ok {
			c.Resolved, c.ResolvedBy, c.ResolvedAt = true, e.Actor, e.At
		}
	case CommentReopened:
		if c, ok := it.comment(e.Comment); ok {
			c.Resolved, c.ResolvedBy, c.ResolvedAt = false, "", time.Time{}
		}
	}
}

// --- sessions ---------------------------------------------------------------

// Comments returns the comments of a readable item.
func (x *Session) Comments(id ID) ([]Comment, error) {
	it, err := x.readable(id)
	if err != nil {
		return nil, err
	}
	out := x.s.visibleComments(x.who, it)
	for i := range out {
		out[i].Anchor = x.veilAnchor(it, out[i].Anchor)
	}
	return out, nil
}

// commentsOnHiddenFiles returns the comments of an item that are on a
// file the principal may not have — a highlight in a protected PDF
// (ADR-0208), a comment on an element of a protected drawing (ADR-0171) —
// with the replies in their threads: the file's protection keeps them to
// its readers, and from everyone else that they exist (ADR-0160). It is
// nil when there are none.
func (s *Store) commentsOnHiddenFiles(principal string, it *Item) map[ID]bool {
	if len(it.secretFiles) == 0 {
		return nil
	}
	var out map[ID]bool
	for _, c := range it.Comments {
		if c.file != "" && c.Reply == "" && !s.readsFile(principal, it, c.file) {
			if out == nil {
				out = map[ID]bool{}
			}
			out[c.ID] = true
		}
	}
	for _, c := range it.Comments {
		if out != nil && c.Reply != "" && out[c.Reply] {
			out[c.ID] = true
		}
	}
	return out
}

// visibleComments returns a copy of the comments of an item the principal
// sees: all but those on files they may not have.
func (s *Store) visibleComments(principal string, it *Item) []Comment {
	hide := s.commentsOnHiddenFiles(principal, it)
	return slices.DeleteFunc(slices.Clone(it.Comments), func(c Comment) bool { return hide[c.ID] })
}

// AddComment comments on an item the principal may edit.
func (x *Session) AddComment(id ID, text string) (Comment, error) {
	return x.Comment(id, NewComment{Text: text})
}

// Comment adds a comment, an inline comment or a reply to an item the
// principal may edit.
func (x *Session) Comment(id ID, n NewComment) (Comment, error) {
	if _, err := x.editable(id); err != nil {
		return Comment{}, err
	}
	return x.s.Comment(x.who, id, n)
}

// ResolveComment marks a thread on an item the principal may edit done, or
// open again.
func (x *Session) ResolveComment(id, comment ID, resolved bool) error {
	if _, err := x.editable(id); err != nil {
		return err
	}
	return x.s.ResolveComment(x.who, id, comment, resolved)
}

// EditComment edits one's own comment on an item one may still edit.
func (x *Session) EditComment(id, comment ID, text string) error {
	if _, err := x.editable(id); err != nil {
		return err
	}
	c, err := x.s.liveComment(id, comment)
	if err != nil {
		return err
	}
	if c.Author != x.who {
		return forbidden("%s may only edit their own comments", x.who)
	}
	return x.s.EditComment(x.who, id, comment, text)
}

// DeleteComment deletes one's own comment on an item one may still edit;
// a space admin deletes any comment.
func (x *Session) DeleteComment(id, comment ID) error {
	it, err := x.readable(id)
	if err != nil {
		return err
	}
	if x.s.spaceRole(x.who, it.Space) < SpaceAdmin {
		if _, err := x.editable(id); err != nil {
			return err
		}
		c, err := x.s.liveComment(id, comment)
		if err != nil {
			return err
		}
		if c.Author != x.who {
			return forbidden("%s may only delete their own comments", x.who)
		}
	}
	return x.s.DeleteComment(x.who, id, comment)
}
