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

func TestContributorsCountOnlyReadableItems(t *testing.T) {
	s, items := searchFixture(t)
	pat := s.As("pat")
	mustItem(t)(pat.Create(NewItem{Space: "PLAT", Type: "page", Title: "Release notes"}))
	noErr(t, pat.SetTitle(items["backup"].ID, "Backup policy, revised"))
	noErr(t, pat.SetTitle(items["backup"].ID, "Backup policy, revised again"))
	// pat also changes the secret page, which ben does not read.
	noErr(t, s.As("anna").Restrict(items["secret"].ID, Restriction{Read: []string{"anna", "pat"}}))
	noErr(t, pat.SetTitle(items["secret"].ID, "Credentials"))
	got := s.As("ben").Contributors()
	if got["anna"] != 3 || got["pat"] != 2 || len(got) != 2 {
		t.Fatalf("contributors = %v, want anna 3 and pat 2 (made one, changed one twice, not the secret one)", got)
	}
	if got := s.As("carl").Contributors(); len(got) != 0 {
		t.Fatalf("an outsider learns of contributors: %v", got)
	}
	noErr(t, s.As("anna").Trash(items["runbook"].ID))
	if got := s.As("ben").Contributors(); got["anna"] != 2 {
		t.Fatalf("a trashed item counts: %v", got)
	}
}

func TestSearchFindsItemsByWhoChangedThem(t *testing.T) {
	s, items := searchFixture(t)
	pat := s.As("pat")
	noErr(t, pat.SetTitle(items["runbook"].ID, "Database runbook, checked"))
	// A relation is no change of the item itself (changesItem), so it
	// makes nobody a contributor.
	noErr(t, s.AddRelation("pat", "relates", items["decision"].ID, items["backup"].ID))
	hits, err := s.As("ben").Search("meier", Query{Names: names})
	noErr(t, err)
	if got := hitKeys(hits); !slices.Equal(got, []string{items["runbook"].Key}) {
		t.Fatalf("meier = %v, want %s, which pat changed", got, items["runbook"].Key)
	}
	// The creator is still found.
	hits, err = s.As("ben").Search("keller", Query{Names: names})
	noErr(t, err)
	if len(hits) != 3 {
		t.Fatalf("keller = %v, want anna's three readable items", hitKeys(hits))
	}
}

func TestStarmapNodesNameWhoChangedThem(t *testing.T) {
	s, items := searchFixture(t)
	noErr(t, s.As("pat").SetTitle(items["runbook"].ID, "Database runbook, checked"))
	m, err := s.As("ben").Starmap("PLAT", 100)
	noErr(t, err)
	for _, n := range m.Nodes {
		if n.ID != string(items["runbook"].ID) {
			continue
		}
		if n.Creator != "anna" || !slices.Equal(n.Contributors, []string{"pat"}) {
			t.Fatalf("runbook: creator %q, contributors %v; want anna and [pat]", n.Creator, n.Contributors)
		}
		return
	}
	t.Fatal("the runbook is not on the map")
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}
