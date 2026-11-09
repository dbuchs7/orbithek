package extcal

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// fakeSealer keeps keys in memory; a shredded key opens nothing.
type fakeSealer struct{ keys map[string]bool }

func (f *fakeSealer) Seal(key string, pt, aad []byte) ([]byte, error) {
	f.keys[key] = true
	return append(append([]byte(key+"|"), aad...), append([]byte("|"), pt...)...), nil
}

func (f *fakeSealer) Open(key string, ct, aad []byte) ([]byte, error) {
	prefix := append(append([]byte(key+"|"), aad...), '|')
	if !f.keys[key] || !bytes.HasPrefix(ct, prefix) {
		return nil, errors.New("cannot open")
	}
	return ct[len(prefix):], nil
}

func (f *fakeSealer) Shred(keys ...string) error {
	for _, k := range keys {
		delete(f.keys, k)
	}
	return nil
}

func newStore(t *testing.T) (*Store, *[][]byte, *fakeSealer) {
	t.Helper()
	var records [][]byte
	n := 0
	sl := &fakeSealer{keys: map[string]bool{}}
	s, err := New(nil, func(b []byte) error { records = append(records, b); return nil }, sl, func() string { n++; return fmt.Sprintf("id%d", n) })
	if err != nil {
		t.Fatal(err)
	}
	return s, &records, sl
}

func TestCalendarsAndAccountsAreKeptPerPerson(t *testing.T) {
	s, records, sl := newStore(t)
	ics, err := s.AddICS("anna", "Holidays", "#336699", "webcal://calendar.example.org/secret/basic.ics#x")
	if err != nil || ics.Host != "calendar.example.org" || ics.Kind != KindICS {
		t.Fatalf("%+v %v", ics, err)
	}
	if addr, err := s.Secret("anna", ics.ID); err != nil || addr != "https://calendar.example.org/secret/basic.ics" {
		t.Fatalf("address %q %v", addr, err)
	}
	for _, raw := range []string{"http://example.org/a.ics", "ftp://x/a.ics", "https://user:pw@example.org/a.ics", "nonsense", "https://" + strings.Repeat("a", maxSecret)} {
		if _, err := s.AddICS("anna", "x", "#000000", raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", raw, err)
		}
	}
	for _, bad := range [][2]string{{"", "#000000"}, {"a\nb", "#000000"}, {strings.Repeat("x", maxName+1), "#000000"}, {"x", "red"}} {
		if _, err := s.AddICS("anna", bad[0], bad[1], "https://example.org/a.ics"); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}

	// A Google account, connected twice: the same account, a new token.
	g, err := s.Connect("anna", Account{Kind: KindGoogle, Name: "anna@example.org"}, "refresh-1")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := s.Connect("anna", Account{Kind: KindGoogle, Name: "anna@example.org"}, "refresh-2")
	if err != nil || g2.ID != g.ID || len(s.Accounts("anna")) != 1 {
		t.Fatalf("connected again: %+v %v", g2, err)
	}
	if tok, _ := s.Secret("anna", g.ID); tok != "refresh-2" {
		t.Fatalf("token %q", tok)
	}
	if err := s.Renew("anna", g.ID, "refresh-3"); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.Secret("anna", g.ID); tok != "refresh-3" || len(sl.keys) != 2 {
		t.Fatalf("renewed: %q, %d keys", tok, len(sl.keys))
	}
	work, err := s.AddCalendar("anna", g.ID, "work@group.calendar.google.com", "Work", "#aa0000")
	if err != nil || work.Kind != KindGoogle {
		t.Fatal(err)
	}
	if _, err := s.AddCalendar("anna", g.ID, "work@group.calendar.google.com", "Work", "#aa0000"); !errors.Is(err, ErrInvalid) {
		t.Errorf("the same calendar twice: %v", err)
	}
	if _, err := s.AddCalendar("anna", "nope", "x", "Work", "#aa0000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown account: %v", err)
	}
	if _, err := s.AddCalendar("anna", g.ID, "", "Work", "#aa0000"); !errors.Is(err, ErrInvalid) {
		t.Errorf("no calendar: %v", err)
	}
	if _, err := s.Connect("anna", Account{Kind: KindICS, Name: "x"}, "y"); !errors.Is(err, ErrInvalid) {
		t.Errorf("an iCalendar account: %v", err)
	}
	dav, err := s.Connect("anna", Account{Kind: KindCalDAV, Name: "anna@icloud.com", Server: "https://caldav.icloud.com"}, "app-password")
	if err != nil {
		t.Fatal(err)
	}
	home, err := s.AddCalendar("anna", dav.ID, "/123/calendars/home/", "Home", "#00aa00")
	if err != nil || home.Kind != KindCalDAV {
		t.Fatal(err)
	}

	// Others see nothing of anna's.
	if len(s.Sources("bert")) != 0 || len(s.Accounts("bert")) != 0 {
		t.Fatal("bert sees anna's calendars")
	}
	if _, err := s.Secret("bert", ics.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("bert reads anna's address")
	}
	if _, ok := s.Source("bert", ics.ID); ok {
		t.Fatal("bert finds anna's calendar")
	}
	if _, err := s.Update("bert", ics.ID, "x", "#000000", false); !errors.Is(err, ErrNotFound) {
		t.Fatal("bert changes anna's calendar")
	}
	if err := s.Remove("bert", ics.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("bert removes anna's calendar")
	}
	if err := s.Disconnect("bert", g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("bert disconnects anna's account")
	}
	if _, ok := s.Account("bert", g.ID); ok {
		t.Fatal("bert finds anna's account")
	}

	// Renamed and hidden; then the journal is read again.
	if u, err := s.Update("anna", ics.ID, "Feiertage", "#112233", true); err != nil || !u.Hidden || u.Name != "Feiertage" {
		t.Fatalf("%+v %v", u, err)
	}
	if _, err := s.Update("anna", ics.ID, "", "#112233", true); !errors.Is(err, ErrInvalid) {
		t.Fatal("an empty name")
	}
	again, err := New(*records, func([]byte) error { return nil }, sl, func() string { return "x" })
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Sources("anna"); len(got) != 3 || got[0].Name != "Feiertage" || !got[0].Hidden {
		t.Fatalf("read again: %+v", got)
	}
	if tok, _ := again.Secret("anna", g.ID); tok != "refresh-3" {
		t.Fatalf("read again: token %q", tok)
	}
	keys := again.Keys()
	slices.Sort(keys)
	if len(keys) != 3 {
		t.Fatalf("keys %v", keys)
	}

	// Disconnecting an account removes its calendars and its key; removing
	// a calendar its address's key.
	if err := s.Disconnect("anna", g.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Source("anna", work.ID); ok || len(s.Accounts("anna")) != 1 {
		t.Fatal("the account's calendar stays")
	}
	if err := s.Remove("anna", ics.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("anna", home.ID); err != nil {
		t.Fatal(err)
	}
	if len(sl.keys) != 1 || len(s.Keys()) != 1 {
		t.Fatalf("left keys %v", sl.keys)
	}
}

func TestLimitsAndBrokenJournals(t *testing.T) {
	s, _, _ := newStore(t)
	for i := range maxSources {
		if _, err := s.AddICS("anna", fmt.Sprint("c", i), "#000000", "https://example.org/a.ics"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddICS("anna", "one more", "#000000", "https://example.org/a.ics"); !errors.Is(err, ErrTooMany) {
		t.Fatalf("one calendar too many: %v", err)
	}
	for i := range maxAccounts {
		if _, err := s.Connect("anna", Account{Kind: KindCalDAV, Name: fmt.Sprint("u", i), Server: "https://dav.example.org"}, "pw"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Connect("anna", Account{Kind: KindCalDAV, Name: "more", Server: "https://dav.example.org"}, "pw"); !errors.Is(err, ErrTooMany) {
		t.Fatalf("one account too many: %v", err)
	}
	if _, err := s.Secret("anna", "nothing"); !errors.Is(err, ErrNotFound) {
		t.Fatal("no secret")
	}
	if err := s.Renew("anna", "nothing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("renew nothing")
	}
	for _, bad := range []string{`nonsense`, `{"v":2}`, `{"v":1,"kind":"source","id":"a"}`, `{"v":1,"kind":"account","id":"a"}`, `{"v":1,"kind":"other"}`} {
		if _, err := New([][]byte{[]byte(bad)}, nil, nil, nil); err == nil {
			t.Errorf("%s is refused", bad)
		}
	}
	failing := errors.New("disk full")
	f, _ := New(nil, func([]byte) error { return failing }, &fakeSealer{keys: map[string]bool{}}, func() string { return "k" })
	if _, err := f.AddICS("anna", "x", "#000000", "https://example.org/a.ics"); !errors.Is(err, failing) {
		t.Fatalf("a journal that fails: %v", err)
	}
}
