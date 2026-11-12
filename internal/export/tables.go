package export

import (
	"strings"
	"unicode/utf8"

	"github.com/dbuchs7/orbithek/internal/item"
)

// tone is a tone of item.TableTones as a document writes it (ADR-0214):
// strong for a border, soft for a fill, so that text on a filled cell
// stays readable. The values match the tones of the web UI.
type tone struct{ strong, soft string }

var tableTones = map[string]tone{
	"blue":   {"2F6FDE", "D5E2F8"},
	"green":  {"1F9D55", "D2EDDD"},
	"yellow": {"D4A106", "F6ECCD"},
	"orange": {"E8710A", "FADFC8"},
	"red":    {"D93636", "F7D6D6"},
	"purple": {"8A4FD8", "E7DCF7"},
	"teal":   {"0F9B9B", "CFEBEB"},
	"gray":   {"7A8394", "E4E6EA"},
}

// tableCell is one cell of a table as an export writes it: its text
// trimmed of surrounding spaces with its marks, and its colours.
type tableCell struct {
	item.TableCell
	Fill, Border string
}

// styledRows are a table block's rows of cells with their marks and
// colours, every row as wide as the first (ADR-0042, ADR-0214).
func styledRows(bl item.Block) [][]tableCell {
	if strings.TrimSpace(bl.Text) == "" {
		return nil
	}
	colours := map[[2]int]item.Cell{}
	for _, c := range bl.Cells {
		colours[[2]int{c.Row, c.Col}] = c
	}
	var out [][]tableCell
	for i, row := range item.TableCells(bl) {
		cells := make([]tableCell, len(row))
		for j, c := range row {
			col := colours[[2]int{i, j}]
			cells[j] = tableCell{TableCell: trimCell(c), Fill: col.Fill, Border: col.Border}
		}
		out = append(out, cells)
	}
	return out
}

// trimCell trims a cell's text of surrounding spaces and keeps its marks
// on the text that remains.
func trimCell(c item.TableCell) item.TableCell {
	lead := utf8.RuneCountInString(c.Text) - utf8.RuneCountInString(strings.TrimLeft(c.Text, " "))
	text := strings.TrimSpace(c.Text)
	n := utf8.RuneCountInString(text)
	out := item.TableCell{Text: text}
	for _, m := range c.Marks {
		m.Start, m.End = max(m.Start-lead, 0), min(m.End-lead, n)
		if m.End > m.Start {
			out.Marks = append(out.Marks, m)
		}
	}
	return out
}

// sizeAt is the size at index i of a table's column widths or row
// heights, 0 where none is given.
func sizeAt(sizes []int, i int) int {
	if i < len(sizes) {
		return sizes[i]
	}
	return 0
}
