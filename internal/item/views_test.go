package item

import (
	"strings"
	"testing"
)

// A view block names what it shows and how; it holds no items (ADR-0201).
func TestAViewBlockNamesItsLayoutTypeRoomQueryGroupAndColumns(t *testing.T) {
	good := []Block{
		{Kind: ItemsView, Layout: ViewTable},
		{Kind: ItemsView, Layout: ViewGallery, ItemType: "task", Space: "PLAT", Query: `state = open ORDER BY updated DESC`,
			Group: "field:priority", Columns: []string{"key", "title", "state", "field:assignee", "labels", "updated"}, Sort: "-updated"},
		{Kind: ItemsView, Layout: ViewList, Group: "state", Sort: "field:due date"},
		{Kind: ItemsView, Layout: ViewTable, Group: "labels", Query: "label = onboarding"},
	}
	for i, b := range good {
		if err := checkBlock(b, "self"); err != nil {
			t.Errorf("good %d: %v", i, err)
		}
	}
	bad := map[string]struct {
		b    Block
		want string
	}{
		"no layout":         {Block{Kind: ItemsView}, "layout"},
		"unknown layout":    {Block{Kind: ItemsView, Layout: "board"}, "layout"},
		"bad room":          {Block{Kind: ItemsView, Layout: ViewTable, Space: "plat"}, "room"},
		"bad type":          {Block{Kind: ItemsView, Layout: ViewTable, ItemType: " task"}, "type"},
		"bad query":         {Block{Kind: ItemsView, Layout: ViewTable, Query: "state = "}, "query"},
		"bad group":         {Block{Kind: ItemsView, Layout: ViewTable, Group: "title"}, "group"},
		"empty field group": {Block{Kind: ItemsView, Layout: ViewTable, Group: "field:"}, "group"},
		"bad column":        {Block{Kind: ItemsView, Layout: ViewTable, Columns: []string{"key", "colour"}}, "column"},
		"column twice":      {Block{Kind: ItemsView, Layout: ViewTable, Columns: []string{"key", "key"}}, "column"},
		"empty columns":     {Block{Kind: ItemsView, Layout: ViewTable, Columns: []string{}}, "column"},
		"too many columns":  {Block{Kind: ItemsView, Layout: ViewTable, Columns: manyColumns(MaxViewColumns + 1)}, "column"},
		"bad sort":          {Block{Kind: ItemsView, Layout: ViewTable, Sort: "--title"}, "sort"},
		"control in field":  {Block{Kind: ItemsView, Layout: ViewTable, Sort: "field:a\nb"}, "sort"},
		"text":              {Block{Kind: ItemsView, Layout: ViewTable, Text: "Onboarding"}, "no text"},
		"mentions":          {Block{Kind: ItemsView, Layout: ViewTable, Mentions: []ID{"x"}}, "no text"},
		"layout elsewhere":  {Block{Kind: Paragraph, Layout: ViewTable}, "only a view"},
		"type elsewhere":    {Block{Kind: Paragraph, ItemType: "task"}, "only a view"},
		"columns elsewhere": {Block{Kind: Paragraph, Columns: []string{"key"}}, "only a view"},
		"query elsewhere":   {Block{Kind: Paragraph, Query: "state = open"}, "query"},
	}
	for name, c := range bad {
		isInvalid(t, name, checkBlock(c.b, "self"), c.want)
	}
	if err := checkBlock(Block{Kind: TaskList, Query: "state = open"}, "self"); err != nil {
		t.Errorf("a task list's query: %v", err)
	}
}

func manyColumns(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "field:f" + strings.Repeat("x", i)
	}
	return out
}

// A view block is part of the body: it is kept and replayed as written.
func TestAViewBlockIsKeptInTheBody(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Overview"}))
	view := Block{Kind: ItemsView, Layout: ViewGallery, ItemType: "task", Space: "PLAT", Group: "state", Columns: []string{"title", "field:priority"}, Sort: "title"}
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Paragraph, Text: "Open work"}, view}}))
	got := mustItem(t)(anna.Get(page.ID)).Body.Blocks[1]
	if got.Layout != ViewGallery || got.ItemType != "task" || got.Space != "PLAT" || got.Group != "state" || len(got.Columns) != 2 || got.Sort != "title" {
		t.Errorf("kept %+v", got)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if back := mustItem(t)(re.As("anna").Get(page.ID)).Body.Blocks[1]; back.Columns[1] != "field:priority" {
		t.Errorf("replayed %+v", back)
	}
}
