package item

import (
	"errors"
	"testing"
)

func TestBlockKindsOfADR0042(t *testing.T) {
	s := setup(t)
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Runbook"}))
	good := []Block{
		{Kind: TOC},
		{Kind: TaskItem, Text: "Back up", Checked: true},
		{Kind: TaskItem, Text: ""},
		{Kind: Panel, Tone: "info", Text: "Read me", Marks: []Mark{{Type: MarkItalic, Start: 0, End: 4}}},
		{Kind: Table, Text: "a\tb\n1\t2\n\t"},
		{Kind: Table, Text: "single"},
		{Kind: Children},
	}
	noErr(t, s.CommitBody("anna", page.ID, Doc{Blocks: good}))
	bad := map[string]Block{
		"panel without tone":     {Kind: Panel, Text: "x"},
		"panel with a bad tone":  {Kind: Panel, Tone: "pink", Text: "x"},
		"tone on a paragraph":    {Kind: Paragraph, Tone: "info", Text: "x"},
		"checked paragraph":      {Kind: Paragraph, Checked: true, Text: "x"},
		"empty table":            {Kind: Table},
		"ragged table":           {Kind: Table, Text: "a\tb\n1"},
		"mark across two cells":  {Kind: Table, Text: "ab\tc", Marks: []Mark{{Type: MarkBold, Start: 1, End: 4}}},
		"text in a toc":          {Kind: TOC, Text: "x"},
		"text in children":       {Kind: Children, Text: "x"},
		"reference on a task":    {Kind: TaskItem, Text: "x", Ref: page.ID},
		"level on a panel":       {Kind: Panel, Tone: "note", Text: "x", Level: 2},
		"mentions in a children": {Kind: Children, Mentions: []ID{page.ID}},
	}
	for name, b := range bad {
		if err := s.CommitBody("anna", page.ID, Doc{Blocks: []Block{b}}); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
	// Table cells are text: search finds them.
	noErr(t, s.GrantSystemRole("root", "root", RoleAdmin))
	hits, err := s.As("root").Search("primary", Query{})
	noErr(t, err)
	if len(hits) != 0 {
		t.Fatalf("found %d hits for a word not in the body", len(hits))
	}
	noErr(t, s.CommitBody("anna", page.ID, Doc{Blocks: []Block{{Kind: Table, Text: "Host\tRole\ndb1\tprimary"}}}))
	if hits, err = s.As("root").Search("primary", Query{}); err != nil || len(hits) != 1 {
		t.Fatalf("table cell not found: %v, %v", hits, err)
	}
}

func TestTablesAreFormattedSizedAndColoured(t *testing.T) {
	s := setup(t)
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Hosts"}))
	table := Block{Kind: Table, Text: "Host\tRole\ndb1\tprimary",
		Marks:   []Mark{{Type: MarkBold, Start: 0, End: 4}, {Type: MarkItalic, Start: 14, End: 21}, {Type: MarkLink, Start: 10, End: 13, Href: "https://db1.example.org"}},
		Columns: []int{0, 240}, Rows: []int{40, 0},
		Cells: []Cell{{Row: 0, Col: 1, Fill: "blue"}, {Row: 1, Col: 0, Border: "red"}, {Row: 1, Col: 1, Fill: "gray", Border: "teal"}}}
	noErr(t, s.CommitBody("anna", page.ID, Doc{Blocks: []Block{table}}))
	got := mustItem(t)(s.Get(page.ID)).Body.Blocks[0]
	if len(got.Marks) != 3 || len(got.Cells) != 3 || got.Columns[1] != 240 || got.Rows[0] != 40 {
		t.Fatalf("table = %+v", got)
	}
	cells := TableCells(got)
	if cells[1][1].Text != "primary" || len(cells[1][1].Marks) != 1 || cells[1][1].Marks[0] != (Mark{Type: MarkItalic, Start: 0, End: 7}) {
		t.Fatalf("cell 1,1 = %+v", cells[1][1])
	}
	if len(cells[0][0].Marks) != 1 || len(cells[0][1].Marks) != 0 || cells[1][0].Marks[0].Href != "https://db1.example.org" {
		t.Fatalf("cells = %+v", cells)
	}

	with := func(f func(*Block)) Block {
		b := table
		b.Marks = nil
		b.Columns, b.Rows, b.Cells = nil, nil, nil
		f(&b)
		return b
	}
	bad := map[string]Block{
		"a mention in a cell": with(func(b *Block) {
			b.Mentions = []ID{page.ID}
			b.Marks = []Mark{{Type: MarkMention, Start: 0, End: 4, Ref: page.ID}}
		}),
		"a secret in a cell":        with(func(b *Block) { b.Marks = []Mark{{Type: MarkSecret, Start: 0, End: 4, Ref: "s1"}} }),
		"a mark over a line break":  with(func(b *Block) { b.Marks = []Mark{{Type: MarkBold, Start: 5, End: 12}} }),
		"too few widths":            with(func(b *Block) { b.Columns = []int{100} }),
		"too many heights":          with(func(b *Block) { b.Rows = []int{20, 20, 20} }),
		"every width 0":             with(func(b *Block) { b.Columns = []int{0, 0} }),
		"a width too small":         with(func(b *Block) { b.Columns = []int{10, 0} }),
		"a width too large":         with(func(b *Block) { b.Columns = []int{2001, 0} }),
		"a height too small":        with(func(b *Block) { b.Rows = []int{0, 8} }),
		"a negative height":         with(func(b *Block) { b.Rows = []int{-20, 0} }),
		"a cell outside":            with(func(b *Block) { b.Cells = []Cell{{Row: 2, Col: 0, Fill: "blue"}} }),
		"a cell twice":              with(func(b *Block) { b.Cells = []Cell{{Row: 0, Col: 0, Fill: "blue"}, {Row: 0, Col: 0, Border: "red"}} }),
		"cells out of order":        with(func(b *Block) { b.Cells = []Cell{{Row: 1, Col: 0, Fill: "blue"}, {Row: 0, Col: 1, Fill: "red"}} }),
		"a cell without colour":     with(func(b *Block) { b.Cells = []Cell{{Row: 0, Col: 0}} }),
		"a colour not a tone":       with(func(b *Block) { b.Cells = []Cell{{Row: 0, Col: 0, Fill: "#ff0000"}} }),
		"an empty list of cells":    with(func(b *Block) { b.Cells = []Cell{} }),
		"widths on a paragraph":     {Kind: Paragraph, Text: "x", Columns: []int{100}},
		"coloured cells on a panel": {Kind: Panel, Tone: "info", Text: "x", Cells: []Cell{{Fill: "blue"}}},
	}
	for name, b := range bad {
		if err := s.CommitBody("anna", page.ID, Doc{Blocks: []Block{b}}); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
}
