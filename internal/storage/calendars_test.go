package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dbuchs7/orbithek/internal/extcal"
	"github.com/dbuchs7/orbithek/internal/item"
)

// TestCalendarCredentialsAreSealedAndTheirKeysGoWithThem: the address of
// a calendar and an account's password are sealed with keys of their own,
// never on disk in plaintext; they survive a restart, removing them
// destroys their keys, a leftover key goes at the next start, and the
// journal is not backed up (ADR-0172).
func TestCalendarCredentialsAreSealedAndTheirKeysGoWithThem(t *testing.T) {
	const address = "https://calendar.example.org/private-0123456789abcdef/basic.ics"
	const password = "abcd-efgh-ijkl-mnop"
	dir := t.TempDir()
	db, _, err := Open(dir)
	noErr(t, err)
	src, err := db.Calendars.AddICS("bert", "Holidays", "#336699", address)
	noErr(t, err)
	acc, err := db.Calendars.Connect("bert", extcal.Account{Kind: extcal.KindCalDAV, Name: "bert@icloud.com", Server: "https://caldav.icloud.com"}, password)
	noErr(t, err)
	noErr(t, db.Close())
	entries, err := os.ReadDir(dir)
	noErr(t, err)
	for _, e := range entries {
		if raw, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil && (bytes.Contains(raw, []byte("private-0123456789abcdef")) || bytes.Contains(raw, []byte(password))) {
			t.Fatalf("%s holds a credential in plaintext", e.Name())
		}
	}

	db, rec, err := Open(dir)
	noErr(t, err)
	if rec.Shredded != 0 {
		t.Errorf("a restart destroyed %d keys of calendars that are there", rec.Shredded)
	}
	if got, err := db.Calendars.Secret("bert", src.ID); err != nil || got != address {
		t.Fatalf("after a restart: %q, %v", got, err)
	}
	keys := db.Calendars.Keys()
	noErr(t, db.Calendars.Remove("bert", src.ID))
	destroyed := 0
	for _, k := range keys {
		if _, err := db.keys.Open(item.ID(calendarKey+k), []byte("x"), nil); err != nil && bytes.Contains([]byte(err.Error()), []byte("shredded")) {
			destroyed++
		}
	}
	if destroyed != 1 {
		t.Errorf("%d keys destroyed, want the removed calendar's", destroyed)
	}
	if got, err := db.Calendars.Secret("bert", acc.ID); err != nil || got != password {
		t.Errorf("the account: %q, %v", got, err)
	}
	_, err = db.keys.Seal(item.ID(calendarKey+"orphan"), []byte("x"), nil)
	noErr(t, err)
	noErr(t, db.keys.persistPending())
	noErr(t, db.Close())
	db, rec, err = Open(dir)
	noErr(t, err)
	defer db.Close()
	if rec.Shredded != 1 {
		t.Errorf("destroyed %d keys at the start, want the orphan", rec.Shredded)
	}
	if _, ok := NotBackedUp[CalendarsFile]; !ok {
		t.Error("people's calendar credentials would be backed up")
	}
}
