package auth

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// identityUnlinked ends the tie between an account and an identity at an
// external provider (ADR-0229).
const identityUnlinked = "IdentityUnlinked"

var (
	// ErrIdentityTaken refuses to link an identity that is linked to an
	// account already, this one or another (ADR-0229).
	ErrIdentityTaken = errors.New("this identity is linked to an account already")
	// ErrProviderLinked refuses a second identity at the same provider:
	// an account has at most one per provider (ADR-0229).
	ErrProviderLinked = errors.New("the account is linked to an identity at this provider already")
	// ErrLastSignIn refuses to unlink the only way an account without a
	// password signs in (ADR-0229).
	ErrLastSignIn = errors.New("this is the account's only way to sign in")
)

// Identity is an identity at an external provider linked to an account:
// its issuer and subject (ADR-0030).
type Identity struct {
	Issuer  string
	Subject string
}

// Identities lists the identities linked to an account, ordered by issuer.
func (a *Accounts) Identities(id UserID) []Identity {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.identitiesOf(id)
}

// identitiesOf lists an account's identities; callers hold a.mu.
func (a *Accounts) identitiesOf(id UserID) []Identity {
	var out []Identity
	for key, owner := range a.external {
		if owner != id {
			continue
		}
		issuer, subject, _ := strings.Cut(key, "\x00")
		out = append(out, Identity{Issuer: issuer, Subject: subject})
	}
	slices.SortFunc(out, func(x, y Identity) int { return strings.Compare(x.Issuer, y.Issuer) })
	return out
}

// HasPassword says whether an account signs in with a password.
func (a *Accounts) HasPassword(id UserID) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	return ok && u.hash != ""
}

// ConfirmPassword checks the password of a signed-in account again before
// a change of how it is reached (ADR-0229). Failures are throttled as a
// login's are, per client address and per username; an account without a
// password never matches.
func (a *Accounts) ConfirmPassword(id UserID, password, addr string) error {
	now := a.now()
	a.mu.Lock()
	u, ok := a.users[id]
	if !ok {
		a.mu.Unlock()
		return ErrNotFound
	}
	name := "user:" + fold(u.Username)
	if wait := a.throttle.blocked(now, "addr:"+addr, name); wait > 0 {
		a.mu.Unlock()
		return &ThrottledError{Wait: wait}
	}
	hash := u.hash
	if hash == "" {
		hash = a.dummy
	}
	a.mu.Unlock()

	good := checkPassword(hash, password) && hash != a.dummy

	a.mu.Lock()
	defer a.mu.Unlock()
	if !good {
		a.throttle.fail(now, "addr:"+addr, name)
		return ErrUnauthorized
	}
	a.throttle.reset(name)
	return nil
}

// SessionStarted answers when the session with the given ID began.
func (a *Accounts) SessionStarted(sessionID string) (time.Time, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range a.sessions {
		if s.id == sessionID && sessionID != "" {
			return s.started, true
		}
	}
	return time.Time{}, false
}

// LinkIdentity ties an existing account to an identity at an external
// provider, after the account's holder proved they hold both (ADR-0229).
// An identity is linked to one account only, and an account to one
// identity per issuer.
func (a *Accounts) LinkIdentity(id UserID, issuer, subject string) error {
	if issuer == "" || subject == "" {
		return fmt.Errorf("%w: an external identity needs issuer and subject", ErrInvalid)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.users[id]; !ok {
		return ErrNotFound
	}
	if _, ok := a.external[issuer+"\x00"+subject]; ok {
		return ErrIdentityTaken
	}
	for _, x := range a.identitiesOf(id) {
		if x.Issuer == issuer {
			return ErrProviderLinked
		}
	}
	return a.record(event{Kind: identityLinked, User: id, Issuer: issuer, Subject: subject})
}

// UnlinkIdentity ends an account's tie to its identity at an issuer. The
// only identity of an account without a password stays, unless an
// administrator frees it on purpose (force), which leaves the account
// without a way to sign in (ADR-0229).
func (a *Accounts) UnlinkIdentity(id UserID, issuer string, force bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	ids := a.identitiesOf(id)
	i := slices.IndexFunc(ids, func(x Identity) bool { return x.Issuer == issuer })
	if i < 0 {
		return ErrNotFound
	}
	if !force && u.hash == "" && len(ids) == 1 {
		return ErrLastSignIn
	}
	return a.record(event{Kind: identityUnlinked, User: id, Issuer: issuer, Subject: ids[i].Subject})
}

// applyUnlink applies an IdentityUnlinked event.
func (a *Accounts) applyUnlink(e event) error {
	key := e.Issuer + "\x00" + e.Subject
	if owner, ok := a.external[key]; !ok || owner != e.User {
		return fmt.Errorf("unlinking an identity user %s does not hold", e.User)
	}
	delete(a.external, key)
	return nil
}
