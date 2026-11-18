package item

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestHeadingNumbersFollowTheSharedVector holds the server's numbers to the
// cases the browser is tested with (ADR-0241).
func TestHeadingNumbersFollowTheSharedVector(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "web", "jstest", "heading-numbers-vector.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Cases []struct {
			Name    string
			Blocks  [][]any
			Numbers []string
		}
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Cases) == 0 {
		t.Fatal("the vector holds no case")
	}
	for _, c := range v.Cases {
		d := Doc{Numbered: true}
		for _, b := range c.Blocks {
			d.Blocks = append(d.Blocks, Block{Kind: BlockKind(b[0].(string)), Level: int(b[1].(float64)), Text: b[2].(string)})
		}
		if got := HeadingNumbers(d); !slices.Equal(got, c.Numbers) {
			t.Errorf("%s: got %q, want %q", c.Name, got, c.Numbers)
		}
	}
}

func TestAnUnnumberedBodyHasNoHeadingNumbers(t *testing.T) {
	d := Doc{Blocks: []Block{{Kind: Heading, Level: 1, Text: "A"}}}
	if got := HeadingNumbers(d); got != nil {
		t.Errorf("got %q, want none", got)
	}
}

// TestNumberingSurvivesCloneAndVeil: copying a body or veiling its
// protected blocks keeps whether it is numbered.
func TestNumberingSurvivesCloneAndVeil(t *testing.T) {
	d := Doc{Numbered: true, Blocks: []Block{{Kind: Heading, Level: 1, Text: "A"}, {Kind: Heading, Level: 1, Text: "Hidden", Secret: "s1"}, {Kind: Heading, Level: 1, Text: "B"}}}
	if !d.clone().Numbered {
		t.Error("a clone lost the numbering")
	}
	v := veil(d, map[ID]bool{"s1": true})
	if !v.Numbered {
		t.Fatal("a veiled body lost the numbering")
	}
	// The reader who may not read the protected heading does not count it.
	if got := HeadingNumbers(v); !slices.Equal(got, []string{"1", "", "2"}) {
		t.Errorf("got %q", got)
	}
}

// TestANumberedBodyIsStoredAndReadBack: the flag is part of the body's
// version, replayed from the log like the blocks.
func TestANumberedBodyIsStoredAndReadBack(t *testing.T) {
	s := setup(t)
	noErr(t, s.GrantRole("admin", "PLAT", "anna", Editor))
	anna := s.As("anna")
	page := mustItem(t)(anna.Create(NewItem{Space: "PLAT", Type: "page", Title: "Concept"}))
	noErr(t, anna.CommitBody(page.ID, Doc{Numbered: true, Blocks: []Block{{Kind: Heading, Level: 1, Text: "Goals"}}}))
	got, err := anna.Get(page.ID)
	noErr(t, err)
	if !got.Body.Numbered {
		t.Fatal("the numbering was not stored")
	}
	r, err := Replay(s.Log())
	noErr(t, err)
	if !mustItem(t)(r.As("anna").Get(page.ID)).Body.Numbered {
		t.Fatal("the replayed body is not numbered")
	}
	// Turning it off is a change of the body of its own.
	noErr(t, anna.CommitBody(page.ID, Doc{Blocks: got.Body.Blocks}))
	got, err = anna.Get(page.ID)
	noErr(t, err)
	if got.Body.Numbered {
		t.Fatal("the numbering was not turned off")
	}
	if r, err = Replay(s.Log()); err != nil {
		t.Fatal(err)
	}
	if mustItem(t)(r.As("anna").Get(page.ID)).Body.Numbered {
		t.Fatal("the replayed body is still numbered")
	}
}
