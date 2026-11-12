package item

import (
	"cmp"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
)

// The context of an item (ADR-0205): what refers to it, which items share
// its labels, and the daily notes that name it, as one reader may see
// them. It is derived on every request from the live state, like the
// starmap (ADR-0157), and holds nothing of its own. Nothing in it names an
// item the reader may not read (D10), and a sentence is taken from the
// text as the reader reads it, protected passages veiled (ADR-0088).

// The kinds of a reference besides the type of a relation made by hand.
const (
	// RefLink is a link in a text whose address is the item's address in
	// this wiki: a web address ending in #/item/ and its key or ID.
	RefLink = "link"
)

// sentenceSpan is the length, in characters, of the sentence a reference
// is shown in; a longer one is cut around the reference.
const sentenceSpan = 240

// ContextRef is one item that refers to the item, by one kind: a mention
// in its text, an embed, a link in its text, or a relation of a type made
// by hand. Sentence is the first sentence it stands in, where it stands
// in a text; Count how often the text refers by that kind.
type ContextRef struct {
	Item     Item
	Kind     string
	Derived  bool
	Sentence string
	Count    int
}

// ContextShared is an item that carries labels the item carries.
type ContextShared struct {
	Item   Item
	Labels []string
}

// ContextDaily is a daily note (ADR-0203) that mentions or links the item.
type ContextDaily struct {
	Item     Item
	Day      time.Time
	Sentence string
}

// ItemContext is the context of one item. Each list holds at most the
// limit asked for; More says that more were left out.
type ItemContext struct {
	References                            []ContextRef
	Shared                                []ContextShared
	Daily                                 []ContextDaily
	MoreReferences, MoreShared, MoreDaily bool
}

// Context returns the context of a readable item: the readable live items
// that refer to it, newest change first; those that share a label with
// it, the most labels shared first, the label of daily notes left out; and
// the daily notes that mention or link it, newest day first. Each list
// holds at most limit entries.
func (x *Session) Context(id ID, limit int) (ItemContext, error) {
	target, err := x.readable(id)
	if err != nil {
		return ItemContext{}, err
	}
	limit = max(limit, 1)
	var c ItemContext

	type key struct {
		from ID
		kind string
	}
	refs := map[key]*ContextRef{}
	var order []key
	daily := map[ID]*ContextDaily{}
	readable := func(from ID) (*Item, bool) {
		it := x.s.items[from]
		return it, it != nil && it.ID != target.ID && it.Presence == Live && x.s.canRead(x.who, it)
	}
	note := func(it *Item, kind string, derived bool, sentence string) {
		if d, ok := x.s.IsDailyNote(it); ok && (kind == RelMentions || kind == RefLink) {
			if daily[it.ID] == nil {
				daily[it.ID] = &ContextDaily{Item: it.summary(), Day: d, Sentence: sentence}
			} else if daily[it.ID].Sentence == "" {
				daily[it.ID].Sentence = sentence
			}
			return
		}
		k := key{it.ID, kind}
		if r := refs[k]; r != nil {
			r.Count++
			if r.Sentence == "" {
				r.Sentence = sentence
			}
			return
		}
		refs[k] = &ContextRef{Item: it.summary(), Kind: kind, Derived: derived, Sentence: sentence, Count: 1}
		order = append(order, k)
	}

	// Relations: those made by hand, and embeds; mentions are read from
	// the texts below, with their sentences, where the relation tells
	// that the reader may learn of them.
	mentions := map[ID]bool{}
	for _, r := range x.s.Relations(target.ID, Incoming) {
		it, ok := readable(r.From)
		if !ok || !x.seesRelation(&r) {
			continue
		}
		if r.Derived && r.Type == RelMentions {
			mentions[r.From] = true
			continue
		}
		note(it, r.Type, r.Derived, "")
	}
	// Texts: mentions, with their sentence, and links to the item's
	// address, read from the text as the reader reads it.
	for _, it := range x.s.items {
		if _, ok := readable(it.ID); !ok || len(it.Body.Blocks) == 0 {
			continue
		}
		seen := false
		for _, b := range x.s.veiled(x.who, it).Body.Blocks {
			for _, m := range b.Marks {
				switch {
				case m.Type == MarkMention && m.Ref == target.ID && mentions[it.ID]:
					note(it, RelMentions, true, sentenceAround(b.Text, m.Start, m.End))
					seen = true
				case m.Type == MarkLink && x.s.addresses(m.Href, target):
					note(it, RefLink, false, sentenceAround(b.Text, m.Start, m.End))
				}
			}
			// A mention without a mark, as an import may leave it, stands
			// in its whole block.
			if mentions[it.ID] && !seen && slices.Contains(b.Mentions, target.ID) && !slices.ContainsFunc(b.Marks, func(m Mark) bool { return m.Type == MarkMention && m.Ref == target.ID }) {
				note(it, RelMentions, true, sentenceAround(b.Text, 0, len([]rune(b.Text))))
				seen = true
			}
		}
	}

	changed := func(it Item) int { return x.s.lastChange(x.s.items[it.ID]).Seq }
	for _, k := range order {
		c.References = append(c.References, *refs[k])
	}
	slices.SortStableFunc(c.References, func(a, b ContextRef) int {
		return cmp.Or(cmp.Compare(changed(b.Item), changed(a.Item)), cmp.Compare(a.Kind, b.Kind))
	})
	c.References, c.MoreReferences = cut(c.References, limit)

	for _, d := range daily {
		c.Daily = append(c.Daily, *d)
	}
	slices.SortFunc(c.Daily, func(a, b ContextDaily) int {
		return cmp.Or(b.Day.Compare(a.Day), cmp.Compare(x.s.items[a.Item.ID].created, x.s.items[b.Item.ID].created))
	})
	c.Daily, c.MoreDaily = cut(c.Daily, limit)

	labels := slices.DeleteFunc(slices.Clone(target.Labels), func(l string) bool { return l == DailyLabel })
	if len(labels) > 0 {
		for _, it := range x.s.items {
			if _, ok := readable(it.ID); !ok {
				continue
			}
			var common []string
			for _, l := range it.Labels {
				if slices.Contains(labels, l) {
					common = append(common, l)
				}
			}
			if len(common) > 0 {
				c.Shared = append(c.Shared, ContextShared{Item: it.summary(), Labels: common})
			}
		}
		slices.SortFunc(c.Shared, func(a, b ContextShared) int {
			return cmp.Or(cmp.Compare(len(b.Labels), len(a.Labels)), cmp.Compare(changed(b.Item), changed(a.Item)), cmp.Compare(a.Item.ID, b.Item.ID))
		})
		c.Shared, c.MoreShared = cut(c.Shared, limit)
	}
	return c, nil
}

// cut keeps the first n entries and says whether any were left out.
func cut[T any](list []T, n int) ([]T, bool) {
	if len(list) <= n {
		return list, false
	}
	return list[:n], true
}

// addresses reports whether a link's address is the item's address in
// this wiki: #/item/ followed by its ID or one of its keys, current or
// former (D2), with or without the site before it and a query after it.
// Whether a full address names this site cannot be told from the link
// alone; one that shows another site's item of the same key counts too.
func (s *Store) addresses(href string, it *Item) bool {
	_, rest, ok := strings.Cut(href, "#/item/")
	if !ok {
		return false
	}
	ref, _, _ := strings.Cut(rest, "?")
	ref, _, _ = strings.Cut(ref, "/")
	if r, err := url.PathUnescape(ref); err == nil {
		ref = r
	}
	return ref != "" && (ID(ref) == it.ID || s.keys[ref] == it.ID)
}

// sentenceAround is the sentence of text in which the characters from
// start to end stand, counted in code points: from after the end of the
// sentence before (a full stop, question or exclamation mark followed by a
// space) to the end of its own. A sentence longer than sentenceSpan is cut
// around the reference, marked by an ellipsis.
func sentenceAround(text string, start, end int) string {
	r := []rune(text)
	start, end = min(max(start, 0), len(r)), min(max(end, 0), len(r))
	stop := func(i int) bool {
		return i >= 0 && i < len(r) && strings.ContainsRune(".!?", r[i]) && (i+1 == len(r) || unicode.IsSpace(r[i+1]))
	}
	from := 0
	for i := start - 1; i >= 0; i-- {
		if stop(i) {
			from = i + 1
			break
		}
	}
	to := len(r)
	for i := max(end, from); i < len(r); i++ {
		if stop(i) {
			to = i + 1
			break
		}
	}
	for from < to && unicode.IsSpace(r[from]) {
		from++
	}
	for to > from && unicode.IsSpace(r[to-1]) {
		to--
	}
	if to-from <= sentenceSpan {
		return string(r[from:to])
	}
	mid := (start + end) / 2
	a := max(from, mid-sentenceSpan/2)
	b := min(to, a+sentenceSpan)
	a = max(from, b-sentenceSpan)
	out := strings.TrimSpace(string(r[a:b]))
	if a > from {
		out = "…" + out
	}
	if b < to {
		out += "…"
	}
	return out
}
