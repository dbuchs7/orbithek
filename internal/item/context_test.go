package item

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func mentionIn(before string, ref ID, label, after string) Block {
	return MentionParagraph(before, ref, label, after)
}

// contextFixture: anna edits PLAT; ben only views it. target is the item
// whose context is read.
func contextFixture(t *testing.T) (*Store, Item) {
	t.Helper()
	s := dailyFixture(t)
	target := mustItem(t)(s.As("anna").Create(NewItem{Space: "PLAT", Type: "page", Title: "Runbook"}))
	noErr(t, s.As("anna").SetLabels(target.ID, []string{"ops", "db"}))
	return s, target
}

func TestTheContextNamesWhatRefersToAnItemWithItsSentence(t *testing.T) {
	s, target := contextFixture(t)
	anna := s.As("anna")
	mention := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Incident", Body: &Doc{Blocks: []Block{
		{Kind: Paragraph, Text: "It was late."},
		mentionIn("The disk filled up. We followed the ", target.ID, "Runbook", " to the letter! Then we slept."),
	}}}))
	link := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Links", Body: &Doc{Blocks: []Block{
		{Kind: Paragraph, Text: "See the steps here.", Marks: []Mark{{Type: MarkLink, Start: 4, End: 9, Href: "https://wiki.example/#/item/" + target.Key}}},
		{Kind: Paragraph, Text: "And by its ID.", Marks: []Mark{{Type: MarkLink, Start: 4, End: 6, Href: "https://wiki.example/#/item/" + string(target.ID) + "?edit"}}},
		{Kind: Paragraph, Text: "Elsewhere.", Marks: []Mark{{Type: MarkLink, Start: 0, End: 4, Href: "https://example.org/runbook"}}},
	}}}))
	embed := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Host", Body: &Doc{Blocks: []Block{{Kind: Embed, Ref: target.ID}}}}))
	noErr(t, s.AddRelation("anna", "implements", embed.ID, target.ID))
	// What it refers to itself is no reference to it.
	noErr(t, anna.CommitBody(target.ID, Doc{Blocks: []Block{mentionIn("Back to ", mention.ID, "Incident", ".")}}))

	c, err := anna.Context(target.ID, 50)
	noErr(t, err)
	got := map[string]ContextRef{}
	for _, r := range c.References {
		got[r.Item.Title+"/"+r.Kind] = r
	}
	if r, ok := got["Incident/mentions"]; !ok || r.Sentence != "We followed the Runbook to the letter!" {
		t.Errorf("mention: %+v", r)
	}
	if r, ok := got["Links/link"]; !ok || r.Sentence != "See the steps here." || r.Count != 2 {
		t.Errorf("link: %+v", r)
	}
	if _, ok := got["Host/embeds"]; !ok {
		t.Error("the embed is missing")
	}
	if r, ok := got["Host/implements"]; !ok || r.Sentence != "" {
		t.Errorf("relation: %+v", r)
	}
	if len(c.References) != 4 {
		t.Errorf("%d references: %+v", len(c.References), c.References)
	}
	if slices.ContainsFunc(c.References, func(r ContextRef) bool { return r.Item.ID == target.ID }) {
		t.Error("the item refers to itself")
	}
	_ = link
	// The newest change comes first.
	if c.References[0].Item.ID != embed.ID {
		t.Errorf("first %s", c.References[0].Item.Title)
	}
}

func TestTheContextNamesOnlyWhatTheReaderReads(t *testing.T) {
	s, target := contextFixture(t)
	anna := s.As("anna")
	hidden := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Secret plan", Body: &Doc{Blocks: []Block{
		mentionIn("Use the ", target.ID, "Runbook", "."),
	}}}))
	noErr(t, s.Restrict("anna", hidden.ID, Restriction{Read: []string{"anna"}}))
	noErr(t, anna.SetLabels(hidden.ID, []string{"ops"}))
	// A sentence with a protected passage shows it veiled.
	open := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Open", Body: &Doc{Blocks: []Block{
		mentionIn("Code 1234 opens the ", target.ID, "Runbook", "."),
		mentionIn("Only for anna: ", target.ID, "Runbook", "."),
	}}}))
	sec, err := anna.Protect(open.ID, 0, 5, 9, nil)
	noErr(t, err)
	_, err = anna.ProtectBlocks(open.ID, 1, 1, nil)
	noErr(t, err)
	_ = sec

	c, err := s.As("ben").Context(target.ID, 50)
	noErr(t, err)
	for _, r := range c.References {
		if r.Item.ID == hidden.ID {
			t.Error("ben learns of an item he may not read")
		}
		if strings.Contains(r.Sentence, "1234") || strings.Contains(r.Sentence, "anna") {
			t.Errorf("ben reads a protected passage: %q", r.Sentence)
		}
	}
	if !slices.ContainsFunc(c.References, func(r ContextRef) bool {
		return r.Item.ID == open.ID && strings.Contains(r.Sentence, SecretPlaceholder)
	}) {
		t.Errorf("the open page's sentence is missing or not veiled: %+v", c.References)
	}
	for _, sh := range c.Shared {
		if sh.Item.ID == hidden.ID {
			t.Error("ben learns of a shared label of an item he may not read")
		}
	}
	// anna reads both.
	c, err = anna.Context(target.ID, 50)
	noErr(t, err)
	if !slices.ContainsFunc(c.References, func(r ContextRef) bool { return r.Item.ID == hidden.ID }) {
		t.Error("anna misses her own page")
	}
	// An unreadable item has no context.
	if _, err := s.As("nobody").Context(target.ID, 50); !errors.Is(err, ErrNotFound) {
		t.Fatalf("nobody: %v", err)
	}
}

func TestTheContextNamesItemsSharingLabelsAndDailyNotes(t *testing.T) {
	s, target := contextFixture(t)
	anna := s.As("anna")
	both := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Both"}))
	noErr(t, anna.SetLabels(both.ID, []string{"db", "ops", "x"}))
	one := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "One"}))
	noErr(t, anna.SetLabels(one.ID, []string{"ops"}))
	none := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "None"}))
	noErr(t, anna.SetLabels(none.ID, []string{"hr"}))

	note, _, err := anna.AppendToDailyNote("PLAT", onDay(2026, 11, 12), "", "x")
	noErr(t, err)
	noErr(t, anna.CommitBody(note.ID, Doc{Blocks: []Block{mentionIn("Read the ", target.ID, "Runbook", " again.")}}))
	other, _, err := anna.OpenDailyNote("PLAT", onDay(2026, 11, 13), "")
	noErr(t, err)

	c, err := anna.Context(target.ID, 50)
	noErr(t, err)
	if len(c.Shared) != 2 || c.Shared[0].Item.ID != both.ID || !slices.Equal(c.Shared[0].Labels, []string{"db", "ops"}) || c.Shared[1].Item.ID != one.ID {
		t.Fatalf("shared %+v", c.Shared)
	}
	if len(c.Daily) != 1 || c.Daily[0].Item.ID != note.ID || !c.Daily[0].Day.Equal(onDay(2026, 11, 12)) || c.Daily[0].Sentence != "Read the Runbook again." {
		t.Fatalf("daily %+v", c.Daily)
	}
	if slices.ContainsFunc(c.References, func(r ContextRef) bool { return r.Item.ID == note.ID }) {
		t.Error("a daily note is named twice")
	}
	// The label of daily notes is not shared: two notes are not related
	// by being notes.
	c, err = anna.Context(note.ID, 50)
	noErr(t, err)
	if slices.ContainsFunc(c.Shared, func(sh ContextShared) bool { return sh.Item.ID == other.ID }) {
		t.Error("daily notes share their label")
	}
}

func TestTheContextIsBounded(t *testing.T) {
	s, target := contextFixture(t)
	anna := s.As("anna")
	for i := range 5 {
		p := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "P" + string(rune('a'+i)), Body: &Doc{Blocks: []Block{mentionIn("", target.ID, "R", "")}}}))
		noErr(t, anna.SetLabels(p.ID, []string{"ops"}))
	}
	c, err := anna.Context(target.ID, 3)
	noErr(t, err)
	if len(c.References) != 3 || !c.MoreReferences || len(c.Shared) != 3 || !c.MoreShared || c.MoreDaily {
		t.Fatalf("%d refs (%v), %d shared (%v)", len(c.References), c.MoreReferences, len(c.Shared), c.MoreShared)
	}
}

func TestSentenceAround(t *testing.T) {
	for _, tc := range []struct {
		text       string
		start, end int
		want       string
	}{
		{"One. Two X here. Three.", 9, 10, "Two X here."},
		{"X at the start", 0, 1, "X at the start"},
		{"No stop before X? Yes.", 15, 16, "No stop before X?"},
		{"Version 1.2 of X is out. Next.", 15, 16, "Version 1.2 of X is out."},
		{"  padded X  ", 9, 10, "padded X"},
		{strings.Repeat("a ", 200) + "X" + strings.Repeat(" b", 200), 400, 401, ""},
	} {
		got := sentenceAround(tc.text, tc.start, tc.end)
		if tc.want == "" {
			if n := len([]rune(got)); n > sentenceSpan+2 || !strings.Contains(got, "X") || !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
				t.Errorf("long: %d runes %q", n, got)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("%q [%d,%d]: %q, want %q", tc.text, tc.start, tc.end, got, tc.want)
		}
	}
}
