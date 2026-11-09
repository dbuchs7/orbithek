package extcal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Kinds of calendars and accounts. An iCalendar address is a calendar by
// itself; a Google or a CalDAV account holds calendars, of which a person
// shows those they choose.
const (
	KindICS    = "ics"
	KindGoogle = "google"
	KindCalDAV = "caldav"
)

// Limits of what one person keeps.
const (
	maxSources  = 30
	maxAccounts = 5
	maxName     = 80
	maxSecret   = 4096
)

// Errors of keeping calendars.
var (
	ErrInvalid  = errors.New("extcal: invalid")
	ErrNotFound = errors.New("extcal: no such calendar or account")
	ErrTooMany  = errors.New("extcal: too many calendars or accounts")
)

// Account is a person's account at Google or at a CalDAV server, such as
// Apple iCloud, Nextcloud or Fastmail. Its credential — Google's refresh
// token, the CalDAV server's (app) password — is kept sealed, never
// listed.
type Account struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Name is who the person is there: the Google account's address or
	// the CalDAV user name.
	Name string `json:"name"`
	// Server is a CalDAV account's address, where its calendars are
	// found.
	Server    string    `json:"server,omitempty"`
	Connected time.Time `json:"connected"`
}

// Source is one calendar a person shows: an iCalendar address, or a
// calendar of one of their accounts. An iCalendar address is a credential
// too — Google's "secret address" reads the whole calendar — so it is kept
// sealed; only its host is listed.
type Source struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Colour string `json:"colour"`
	// Hidden is a calendar kept but not shown.
	Hidden bool `json:"hidden"`
	// Account and Calendar name an account's calendar: Google's calendar
	// ID, or the address of the CalDAV collection.
	Account  string `json:"account,omitempty"`
	Calendar string `json:"calendar,omitempty"`
	// Host is an iCalendar address's host, to tell calendars apart.
	Host  string    `json:"host,omitempty"`
	Added time.Time `json:"added"`
}

// Sealer seals credentials with a key per calendar or account and destroys
// the key when it is removed, so a removed credential cannot be read
// again, from the journal or from a copy of it.
type Sealer interface {
	Seal(key string, pt, aad []byte) ([]byte, error)
	Open(key string, ct, aad []byte) ([]byte, error)
	Shred(keys ...string) error
}

// Kinds of the journal's records.
const (
	sourceSet      = "source"
	sourceRemoved  = "source-removed"
	accountSet     = "account"
	accountRemoved = "account-removed"
)

// record is one entry of the journal: a calendar or account as it is now,
// with its credential sealed, or its removal.
type record struct {
	V       int       `json:"v"`
	Kind    string    `json:"kind"`
	At      time.Time `json:"at"`
	User    string    `json:"user"`
	ID      string    `json:"id"`
	Source  *Source   `json:"source,omitempty"`
	Account *Account  `json:"account,omitempty"`
	Key     string    `json:"key,omitempty"`
	Sealed  []byte    `json:"sealed,omitempty"`
}

type kept struct {
	key    string
	sealed []byte
}

type person struct {
	sources  []Source
	accounts []Account
	secrets  map[string]kept // by source or account ID
}

// Store keeps the calendars and accounts of every person. It is safe for
// concurrent use.
type Store struct {
	mu      sync.Mutex
	journal func([]byte) error
	sealer  Sealer
	newID   func() string
	now     func() time.Time
	people  map[string]*person
}

// The prefix binds a sealed credential to its purpose, its person and its
// calendar or account.
func aad(user, id, key string) []byte {
	return []byte("orbithek external calendar " + user + " " + id + " " + key)
}

// New rebuilds the calendars from the journal's records and journals later
// changes; newID makes the IDs of calendars, accounts and keys.
func New(records [][]byte, journal func([]byte) error, sealer Sealer, newID func() string) (*Store, error) {
	s := &Store{journal: journal, sealer: sealer, newID: newID, now: time.Now, people: map[string]*person{}}
	for i, data := range records {
		var r record
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("extcal: record %d: %w", i+1, err)
		}
		if err := s.apply(r); err != nil {
			return nil, fmt.Errorf("extcal: record %d: %w", i+1, err)
		}
	}
	return s, nil
}

func (s *Store) person(user string) *person {
	p := s.people[user]
	if p == nil {
		p = &person{secrets: map[string]kept{}}
		s.people[user] = p
	}
	return p
}

func (s *Store) apply(r record) error {
	if r.V != 1 {
		return fmt.Errorf("format version %d is not supported", r.V)
	}
	p := s.person(r.User)
	switch r.Kind {
	case sourceSet:
		if r.Source == nil || r.Source.ID != r.ID {
			return errors.New("a calendar without its settings")
		}
		i := slices.IndexFunc(p.sources, func(x Source) bool { return x.ID == r.ID })
		if i < 0 {
			p.sources = append(p.sources, *r.Source)
		} else {
			p.sources[i] = *r.Source
		}
	case accountSet:
		if r.Account == nil || r.Account.ID != r.ID {
			return errors.New("an account without its settings")
		}
		i := slices.IndexFunc(p.accounts, func(x Account) bool { return x.ID == r.ID })
		if i < 0 {
			p.accounts = append(p.accounts, *r.Account)
		} else {
			p.accounts[i] = *r.Account
		}
	case sourceRemoved:
		p.sources = slices.DeleteFunc(p.sources, func(x Source) bool { return x.ID == r.ID })
		delete(p.secrets, r.ID)
	case accountRemoved:
		p.accounts = slices.DeleteFunc(p.accounts, func(x Account) bool { return x.ID == r.ID })
		p.sources = slices.DeleteFunc(p.sources, func(x Source) bool { return x.Account == r.ID })
		delete(p.secrets, r.ID)
	default:
		return fmt.Errorf("unknown kind %q", r.Kind)
	}
	if len(r.Sealed) > 0 && r.Key != "" {
		p.secrets[r.ID] = kept{key: r.Key, sealed: r.Sealed}
	}
	return nil
}

// write journals r and applies it.
func (s *Store) write(r record) error {
	r.V, r.At = 1, s.now().UTC()
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := s.journal(data); err != nil {
		return err
	}
	return s.apply(r)
}

// seal seals secret with a new key for the calendar or account id.
func (s *Store) seal(user, id, secret string) (string, []byte, error) {
	key := s.newID()
	ct, err := s.sealer.Seal(key, []byte(secret), aad(user, id, key))
	return key, ct, err
}

// Sources are a person's calendars, in the order they were added.
func (s *Store) Sources(user string) []Source {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.people[user]; p != nil {
		return slices.Clone(p.sources)
	}
	return nil
}

// Source is one of a person's calendars.
func (s *Store) Source(user, id string) (Source, bool) {
	for _, x := range s.Sources(user) {
		if x.ID == id {
			return x, true
		}
	}
	return Source{}, false
}

// Accounts are a person's accounts.
func (s *Store) Accounts(user string) []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.people[user]; p != nil {
		return slices.Clone(p.accounts)
	}
	return nil
}

// Account is one of a person's accounts.
func (s *Store) Account(user, id string) (Account, bool) {
	for _, a := range s.Accounts(user) {
		if a.ID == id {
			return a, true
		}
	}
	return Account{}, false
}

var colour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// checkSettings checks what a person names and colours a calendar.
func checkSettings(name, col string) error {
	switch {
	case strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > maxName || strings.ContainsAny(name, "\n\r<>"):
		return fmt.Errorf("%w: a calendar's name is one line of 1 to %d characters", ErrInvalid, maxName)
	case !colour.MatchString(col):
		return fmt.Errorf("%w: a calendar's colour is like #1a2b3c", ErrInvalid)
	}
	return nil
}

// ICSAddress checks an iCalendar address and writes it as it is read:
// webcal:// as https://. Only https is read, as the address is a
// credential that must not cross the network in the clear.
func ICSAddress(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || len(raw) > maxSecret {
		return "", fmt.Errorf("%w: an iCalendar address is like https://calendar.example.org/basic.ics", ErrInvalid)
	}
	switch strings.ToLower(u.Scheme) {
	case "webcal", "webcals", "https":
		u.Scheme = "https"
	default:
		return "", fmt.Errorf("%w: an iCalendar address begins with https:// or webcal://", ErrInvalid)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: an iCalendar address holds no user name or password", ErrInvalid)
	}
	u.Fragment = ""
	return u.String(), nil
}

// AddICS keeps an iCalendar address as a calendar of user's.
func (s *Store) AddICS(user, name, col, address string) (Source, error) {
	addr, err := ICSAddress(address)
	if err != nil {
		return Source{}, err
	}
	u, _ := url.Parse(addr)
	return s.add(user, Source{Kind: KindICS, Name: name, Colour: col, Host: u.Hostname()}, addr)
}

// AddCalendar keeps a calendar of one of user's accounts.
func (s *Store) AddCalendar(user, account, calendar, name, col string) (Source, error) {
	a, ok := s.Account(user, account)
	if !ok {
		return Source{}, ErrNotFound
	}
	if calendar == "" || len(calendar) > maxSecret {
		return Source{}, fmt.Errorf("%w: no calendar chosen", ErrInvalid)
	}
	for _, x := range s.Sources(user) {
		if x.Account == account && x.Calendar == calendar {
			return Source{}, fmt.Errorf("%w: this calendar is shown already", ErrInvalid)
		}
	}
	return s.add(user, Source{Kind: a.Kind, Name: name, Colour: col, Account: account, Calendar: calendar}, "")
}

func (s *Store) add(user string, src Source, secret string) (Source, error) {
	if err := checkSettings(src.Name, src.Colour); err != nil {
		return Source{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.people[user]; p != nil && len(p.sources) >= maxSources {
		return Source{}, fmt.Errorf("%w: at most %d calendars", ErrTooMany, maxSources)
	}
	src.ID, src.Added = s.newID(), s.now().UTC()
	r := record{Kind: sourceSet, User: user, ID: src.ID, Source: &src}
	if secret != "" {
		var err error
		if r.Key, r.Sealed, err = s.seal(user, src.ID, secret); err != nil {
			return Source{}, err
		}
	}
	return src, s.write(r)
}

// Update changes how a calendar is named, coloured and whether it is
// shown.
func (s *Store) Update(user, id, name, col string, hidden bool) (Source, error) {
	if err := checkSettings(name, col); err != nil {
		return Source{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.people[user]
	var src Source
	if p != nil {
		if i := slices.IndexFunc(p.sources, func(x Source) bool { return x.ID == id }); i >= 0 {
			src = p.sources[i]
		}
	}
	if src.ID == "" {
		return Source{}, ErrNotFound
	}
	src.Name, src.Colour, src.Hidden = name, col, hidden
	return src, s.write(record{Kind: sourceSet, User: user, ID: id, Source: &src})
}

// Remove removes a calendar and destroys the key of its address.
func (s *Store) Remove(user, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.people[user]
	if p == nil || !slices.ContainsFunc(p.sources, func(x Source) bool { return x.ID == id }) {
		return ErrNotFound
	}
	k, sealed := p.secrets[id]
	if err := s.write(record{Kind: sourceRemoved, User: user, ID: id}); err != nil {
		return err
	}
	if sealed {
		return s.sealer.Shred(k.key)
	}
	return nil
}

// Connect keeps an account of user's with its credential; connecting the
// same account (kind, name and server) again replaces its credential and
// keeps its calendars.
func (s *Store) Connect(user string, a Account, secret string) (Account, error) {
	if a.Kind != KindGoogle && a.Kind != KindCalDAV || a.Name == "" || len(a.Name) > 320 || secret == "" || len(secret) > maxSecret {
		return Account{}, fmt.Errorf("%w: an account needs its kind, name and credential", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.person(user)
	var old kept
	if i := slices.IndexFunc(p.accounts, func(x Account) bool { return x.Kind == a.Kind && x.Name == a.Name && x.Server == a.Server }); i >= 0 {
		a.ID, old = p.accounts[i].ID, p.secrets[p.accounts[i].ID]
	} else if len(p.accounts) >= maxAccounts {
		return Account{}, fmt.Errorf("%w: at most %d accounts", ErrTooMany, maxAccounts)
	} else {
		a.ID = s.newID()
	}
	a.Connected = s.now().UTC()
	r := record{Kind: accountSet, User: user, ID: a.ID, Account: &a}
	var err error
	if r.Key, r.Sealed, err = s.seal(user, a.ID, secret); err != nil {
		return Account{}, err
	}
	if err := s.write(r); err != nil {
		return Account{}, err
	}
	if old.key != "" {
		return a, s.sealer.Shred(old.key)
	}
	return a, nil
}

// Renew replaces an account's credential, as Google may hand out a new
// refresh token.
func (s *Store) Renew(user, id, secret string) error {
	a, ok := s.Account(user, id)
	if !ok {
		return ErrNotFound
	}
	_, err := s.Connect(user, a, secret)
	return err
}

// Disconnect removes an account and its calendars and destroys the key of
// its credential.
func (s *Store) Disconnect(user, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.people[user]
	if p == nil || !slices.ContainsFunc(p.accounts, func(x Account) bool { return x.ID == id }) {
		return ErrNotFound
	}
	k := p.secrets[id]
	if err := s.write(record{Kind: accountRemoved, User: user, ID: id}); err != nil {
		return err
	}
	if k.key != "" {
		return s.sealer.Shred(k.key)
	}
	return nil
}

// Secret is the credential kept for a calendar or an account of user's:
// an iCalendar address, a refresh token or a password.
func (s *Store) Secret(user, id string) (string, error) {
	s.mu.Lock()
	k, ok := kept{}, false
	if p := s.people[user]; p != nil {
		k, ok = p.secrets[id]
	}
	s.mu.Unlock()
	if !ok {
		return "", ErrNotFound
	}
	pt, err := s.sealer.Open(k.key, k.sealed, aad(user, id, k.key))
	return string(pt), err
}

// Keys are the keys of the kept credentials, which must not be destroyed
// as leftovers.
func (s *Store) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, p := range s.people {
		for _, k := range p.secrets {
			out = append(out, k.key)
		}
	}
	return out
}
