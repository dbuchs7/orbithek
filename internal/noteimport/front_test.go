package noteimport

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
)

// A value of a note becomes a field's value where it fits the field's
// kind, and is refused with the reason where it does not.
func TestValuesFitFieldsByKind(t *testing.T) {
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		kind item.FieldKind
		in   any
		want any
		bad  bool
	}{
		{item.KindText, []any{"a", 1}, "a, 1", false},
		{item.KindText, "", nil, false},
		{item.KindNumber, 3, 3.0, false},
		{item.KindNumber, 2.5, 2.5, false},
		{item.KindNumber, "1'000", 1000.0, false},
		{item.KindNumber, "many", nil, true},
		{item.KindNumber, "NaN", nil, true},
		{item.KindBool, true, true, false},
		{item.KindBool, "Yes", true, false},
		{item.KindBool, "No", false, false},
		{item.KindBool, "perhaps", nil, true},
		{item.KindDate, day, day, false},
		{item.KindDate, "January 2, 2026", day, false},
		{item.KindDate, "January 2, 2026 → January 5, 2026", day, false},
		{item.KindDate, "02.01.2026", day, false},
		{item.KindDate, "soon", nil, true},
		{item.KindDateTime, "2026-01-02T10:00:00+01:00", time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC), false},
		{item.KindDateTime, time.Date(2026, 1, 2, 9, 0, 0, 5, time.UTC), time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC), false},
		{item.KindDateTime, "January 2, 2026", nil, true},
		{item.KindEnum, "HIGH", "high", false},
		{item.KindEnum, "urgent", nil, true},
		{item.KindURL, "https://example.org/a", "https://example.org/a", false},
		{item.KindURL, "javascript:alert(1)", nil, true},
		{item.KindUser, "anna", nil, true},
	} {
		got, err := fieldValue(item.FieldDef{Name: "f", Kind: c.kind, Values: []string{"low", "high"}}, c.in)
		if (err != nil) != c.bad {
			t.Errorf("%s %v: err %v", c.kind, c.in, err)
			continue
		}
		if g, ok := got.(time.Time); ok {
			if !g.Equal(c.want.(time.Time)) {
				t.Errorf("%s %v: %v, want %v", c.kind, c.in, got, c.want)
			}
			continue
		}
		if got != c.want {
			t.Errorf("%s %v: %v, want %v", c.kind, c.in, got, c.want)
		}
	}
	for in, want := range map[any]string{
		nil: "", 2.5: "2.5", true: "true", time.Date(2026, 1, 2, 10, 30, 0, 0, time.UTC): "2026-01-02T10:30:00Z",
	} {
		if got := text(in); got != want {
			t.Errorf("text(%v) = %q, want %q", in, got, want)
		}
	}
	if got := text(map[string]any{"a": 1}); got != `{"a":1}` {
		t.Errorf("a mapping as text: %q", got)
	}
	if got := tagsOf("a b,c", false); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("tags %v", got)
	}
	if got := tagsOf("two words, b", true); !slices.Equal(got, []string{"two words", " b"}) {
		t.Errorf("tags of a table %v", got)
	}
}

// A front matter that is no YAML is cut off and reported; one that never
// ends is text.
func TestFrontMatterThatIsNoYAML(t *testing.T) {
	var failed error
	m, order, rest := frontMatter("---\n: [\n---\nText", func(err error) { failed = err })
	if m != nil || order != nil || rest != "Text" || failed == nil {
		t.Errorf("%v %v %q %v", m, order, rest, failed)
	}
	if m, _, rest := frontMatter("---\na: 1\nText", nil); m != nil || !strings.HasPrefix(rest, "---") {
		t.Errorf("unclosed: %v %q", m, rest)
	}
	if m, order, _ := frontMatter("---\nb: 1\na: [x]\n...\n", nil); len(m) != 2 || !slices.Equal(order, []string{"b", "a"}) {
		t.Errorf("closed by dots: %v %v", m, order)
	}
}
