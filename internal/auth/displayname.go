package auth

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxDisplayName is the length of a display name, in characters.
const MaxDisplayName = 64

// DisplayName returns the name a user chose to be called by, or empty when
// they have not chosen one (ADR-draft-a-display-name-each-person-chooses).
// The username stays what identifies the account.
func (a *Accounts) DisplayName(id UserID) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u, ok := a.users[id]; ok {
		return u.displayName
	}
	return ""
}

// SetDisplayName records the name a user chose to be called by, trimmed;
// empty clears it. Setting the same name again changes nothing.
func (a *Accounts) SetDisplayName(id UserID, name string) error {
	name = strings.TrimSpace(name)
	if err := checkDisplayName(name); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	if u.displayName == name {
		return nil
	}
	return a.record(event{Kind: displayNameSet, User: id, DisplayName: name})
}

// checkDisplayName accepts a name of at most MaxDisplayName characters of
// valid UTF-8 without control or formatting characters: those could break
// a line, hide a character or turn the text around where the name is
// shown. The zero-width non-joiner and joiner stay allowed, since some
// scripts spell names with them.
func checkDisplayName(name string) error {
	if !utf8.ValidString(name) {
		return fmt.Errorf("%w: a display name is text", ErrInvalid)
	}
	if utf8.RuneCountInString(name) > MaxDisplayName {
		return fmt.Errorf("%w: a display name has at most %d characters", ErrInvalid, MaxDisplayName)
	}
	for _, r := range name {
		if r == '‌' || r == '‍' {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return fmt.Errorf("%w: a display name holds no control or formatting characters", ErrInvalid)
		}
	}
	return nil
}
