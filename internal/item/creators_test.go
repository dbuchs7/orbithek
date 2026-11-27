package item

import (
	"slices"
	"testing"
)

// names stands for the accounts: a user ID is found by its username.
func names(user string) string {
	return map[string]string{"anna": "anna.keller", "pat": "pat.meier"}[user]
}

func TestSearchFindsItemsByTheirCreator(t *testing.T) {
	s, items := searchFixture(t)
	pat := s.As("pat")
	mine := mustItem(t)(pat.Create(NewItem{Space: "PLAT", Type: "page", Title: "Release notes"}))
	ben := s.As("ben")

	hits, err := ben.Search("keller", Query{Names: names})
	noErr(t, err)
	// anna made every item of the fixture; ben does not read the secret one
	// (D10), so it is not found by its creator either.
	if got, want := hitKeys(hits), []string{"PLAT-1", "PLAT-2", "PLAT-3"}; !slices.Equal(sorted(got), want) {
		t.Fatalf("keller = %v, want %v", got, want)
	}
	for _, h := range hits {
		if h.Item.ID == items["secret"].ID {
			t.Fatal("an unreadable item is found by its creator")
		}
	}

	hits, err = ben.Search("pat.meier", Query{Names: names})
	noErr(t, err)
	if got := hitKeys(hits); !slices.Equal(got, []string{mine.Key}) {
		t.Fatalf("pat.meier = %v, want %v", got, mine.Key)
	}

	// Without Names nobody is found by who made it.
	hits, err = ben.Search("keller", Query{})
	noErr(t, err)
	if len(hits) != 0 {
		t.Fatalf("keller without names = %v, want none", hitKeys(hits))
	}
}

func TestSearchRanksTheCreatorBelowTheTitle(t *testing.T) {
	s, _ := searchFixture(t)
	pat := s.As("pat")
	titled := mustItem(t)(pat.Create(NewItem{Space: "PLAT", Type: "page", Title: "Keller inventory"}))
	hits, err := s.As("ben").Search("keller", Query{Names: names})
	noErr(t, err)
	if len(hits) == 0 || hits[0].Item.ID != titled.ID {
		t.Fatalf("first hit = %v, want %s, the word in its title", hitKeys(hits), titled.Key)
	}
}

func TestCreatorsCountsOnlyReadableItems(t *testing.T) {
	s, items := searchFixture(t)
	pat := s.As("pat")
	mustItem(t)(pat.Create(NewItem{Space: "PLAT", Type: "page", Title: "Release notes"}))
	got := s.As("ben").Creators()
	if got["anna"] != 3 || got["pat"] != 1 || len(got) != 2 {
		t.Fatalf("creators = %v, want anna 3 (not the secret one) and pat 1", got)
	}
	if got := s.As("carl").Creators(); len(got) != 0 {
		t.Fatalf("an outsider learns of creators: %v", got)
	}
	noErr(t, s.As("anna").Trash(items["runbook"].ID))
	if got := s.As("ben").Creators(); got["anna"] != 2 {
		t.Fatalf("a trashed item counts: %v", got)
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}
