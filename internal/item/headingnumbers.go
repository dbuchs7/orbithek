package item

import (
	"strconv"
	"strings"
)

// HeadingNumbers returns the number of every heading of a numbered body,
// by block index: "1", "1.1", "1.1.1" (ADR-0241). A
// block that is no heading, or a heading without text, has "". The
// shallowest level the body uses counts as the first, so a body that
// starts at level 2 numbers its level 2 headings 1, 2, 3; a level the body
// skips below another counts as 0, as in "1.0.1". An unnumbered body has
// no numbers: the result is nil.
//
// The numbers are a function of the blocks as the reader reads them:
// headings inside a protected section the reader may not read are not
// there and not counted (ADR-0160). assets/numbering.js computes the same,
// and internal/web/jstest/heading-numbers-vector.json holds both to it.
func HeadingNumbers(d Doc) []string {
	if !d.Numbered {
		return nil
	}
	numbered := func(b Block) bool { return b.Kind == Heading && strings.TrimSpace(b.Text) != "" }
	top := 0
	for _, b := range d.Blocks {
		if numbered(b) && (top == 0 || headingLevel(b) < top) {
			top = headingLevel(b)
		}
	}
	out := make([]string, len(d.Blocks))
	var count [3]int
	for i, b := range d.Blocks {
		if !numbered(b) {
			continue
		}
		depth := headingLevel(b) - top
		count[depth]++
		for j := depth + 1; j < len(count); j++ {
			count[j] = 0
		}
		parts := make([]string, depth+1)
		for j := range parts {
			parts[j] = strconv.Itoa(count[j])
		}
		out[i] = strings.Join(parts, ".")
	}
	return out
}

// headingLevel is a heading's level, 1 to 3; 0 reads as 1 (ADR-0032).
func headingLevel(b Block) int {
	if b.Level < 1 {
		return 1
	}
	return min(b.Level, 3)
}
