package item

import "testing"

// A starmap in a text names the map it shows and how it is narrowed at
// first; it holds no item (ADR-0252).
func TestAStarmapBlockNamesItsRoomLabelSearchHiddenKindsAndDepth(t *testing.T) {
	good := []Block{
		{Kind: StarmapBlock},
		{Kind: StarmapBlock, Space: "PLAT"},
		{Kind: StarmapBlock, Space: StarmapAllRooms, Label: "onboarding"},
		{Kind: StarmapBlock, Search: "release plan", HideTypes: []string{"decision", "task"}, HideKinds: []string{"done", "label", "outside", "picture"}},
		{Kind: StarmapBlock, Depth: 1},
		{Kind: StarmapBlock, Depth: MaxStarmapDepth, HideKinds: []string{"picture"}},
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
		"text":                {Block{Kind: StarmapBlock, Text: "map"}, "no text"},
		"mentions":            {Block{Kind: StarmapBlock, Mentions: []ID{"x"}}, "no text"},
		"bad room":            {Block{Kind: StarmapBlock, Space: "plat"}, "room"},
		"two stars":           {Block{Kind: StarmapBlock, Space: "**"}, "room"},
		"label not as stored": {Block{Kind: StarmapBlock, Label: " Onboarding "}, "label"},
		"search spaced":       {Block{Kind: StarmapBlock, Search: " plan"}, "search"},
		"search control":      {Block{Kind: StarmapBlock, Search: "a\nb"}, "search"},
		"depth too far":       {Block{Kind: StarmapBlock, Depth: MaxStarmapDepth + 1}, "depth"},
		"depth negative":      {Block{Kind: StarmapBlock, Depth: -1}, "depth"},
		"no kind named":       {Block{Kind: StarmapBlock, HideKinds: []string{}}, "leaves out"},
		"unknown kind":        {Block{Kind: StarmapBlock, HideKinds: []string{"item"}}, "kind"},
		"kinds unsorted":      {Block{Kind: StarmapBlock, HideKinds: []string{"picture", "done"}}, "kind"},
		"kind twice":          {Block{Kind: StarmapBlock, HideKinds: []string{"done", "done"}}, "kind"},
		"types unsorted":      {Block{Kind: StarmapBlock, HideTypes: []string{"task", "decision"}}, "sorted"},
		"no type named":       {Block{Kind: StarmapBlock, HideTypes: []string{}}, "types"},
		"label elsewhere":     {Block{Kind: Paragraph, Label: "x"}, "only a starmap"},
		"search elsewhere":    {Block{Kind: ItemsView, Layout: ViewTable, Search: "x"}, "only a starmap"},
		"kinds elsewhere":     {Block{Kind: Children, HideKinds: []string{"done"}}, "only a starmap"},
		"room elsewhere":      {Block{Kind: Paragraph, Space: "PLAT"}, "room"},
		"layout on a starmap": {Block{Kind: StarmapBlock, Layout: ViewTable}, "only a view"},
		"query on a starmap":  {Block{Kind: StarmapBlock, Query: "state = open"}, "query"},
		"depth on a toc":      {Block{Kind: TOC, Depth: 1}, "depth"},
	}
	for name, c := range bad {
		isInvalid(t, name, checkBlock(c.b, "self"), c.want)
	}
}

// A starmap in a text is part of the body: it is kept and replayed as
// written.
func TestAStarmapBlockIsKeptInTheBody(t *testing.T) {
	s := permFixture(t)
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Overview"}))
	m := Block{Kind: StarmapBlock, Space: StarmapAllRooms, Label: "onboarding", Search: "plan", HideTypes: []string{"task"}, HideKinds: []string{"done", "picture"}, Depth: 2}
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: []Block{{Kind: Paragraph, Text: "How it hangs together"}, m}}))
	got := mustItem(t)(anna.Get(page.ID)).Body.Blocks[1]
	if got.Space != StarmapAllRooms || got.Label != "onboarding" || got.Search != "plan" || len(got.HideTypes) != 1 || len(got.HideKinds) != 2 || got.Depth != 2 {
		t.Errorf("kept %+v", got)
	}
	re, err := Replay(s.Log())
	noErr(t, err)
	if back := mustItem(t)(re.As("anna").Get(page.ID)).Body.Blocks[1]; back.HideKinds[1] != "picture" || back.Label != "onboarding" {
		t.Errorf("replayed %+v", back)
	}
}
