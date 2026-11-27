package auth

import (
	"errors"
	"fmt"
)

// Suspending and deleting accounts
// (ADR-draft-suspending-and-deleting-accounts).
//
// A suspended account keeps everything it has, but no way in works: its
// sessions end, and its password, single sign-on, API tokens, OAuth tokens
// and password resets are refused until it is reactivated.
//
// A deleted account is gone from the accounts but for its ID, which stays
// known as deleted so what refers to it can say so, and is never reused.
// Its username is free again; its credentials, identities, tokens,
// approvals, saved views and feeds, contacts, profile picture and other
// personal settings are removed from the state. Earlier records of the
// journal and backups still hold what they held.

// ErrSuspended refuses a way in to a suspended account. A login answers it
// only after the password matched, so it tells nothing to whoever does not
// know the password.
var ErrSuspended = errors.New("the account is suspended")

const (
	// accountSuspended and accountReactivated close and open every way
	// into an account.
	accountSuspended   = "AccountSuspended"
	accountReactivated = "AccountReactivated"
	// accountDeleted removes an account, leaving its ID as deleted.
	accountDeleted = "AccountDeleted"
)

// applyLifecycle applies the events that suspend, reactivate and delete
// an account; it runs live and on load.
func (a *Accounts) applyLifecycle(e event) error {
	u, ok := a.users[e.User]
	if !ok {
		return fmt.Errorf("%s of unknown user %s", e.Kind, e.User)
	}
	switch e.Kind {
	case accountSuspended:
		u.Suspended = true
		a.endSessions(e.User)
		delete(a.resets, e.User)
	case accountReactivated:
		u.Suspended = false
	case accountDeleted:
		a.forget(u)
	}
	return nil
}

// forget removes an account and everything that opens it or belongs to
// it, keeping its ID as deleted; callers hold a.mu.
func (a *Accounts) forget(u *user) {
	id := u.ID
	if a.byName[fold(u.Username)] == id {
		delete(a.byName, fold(u.Username))
	}
	for h, t := range a.tokens {
		if t.User == id {
			delete(a.tokens, h)
			delete(a.tokenIDs, t.ID)
		}
	}
	for k, owner := range a.external {
		if owner == id {
			delete(a.external, k)
		}
	}
	for h, f := range a.feeds {
		if f.user == id {
			delete(a.feeds, h)
		}
	}
	s := a.oauth()
	for k, g := range s.grants {
		if g.User == id {
			delete(s.grants, k)
		}
	}
	for k, c := range s.codes {
		if c.user == id {
			delete(s.codes, k)
		}
	}
	for k, t := range s.access {
		if t.user == id {
			delete(s.access, k)
		}
	}
	delete(a.resets, id)
	a.endSessions(id)
	delete(a.users, id)
	a.deleted[id] = true
}

// endSessions ends every session of an account; callers hold a.mu.
func (a *Accounts) endSessions(id UserID) {
	for k, s := range a.sessions {
		if s.user == id {
			delete(a.sessions, k)
		}
	}
}

// active reports whether an account exists and is not suspended; callers
// hold a.mu.
func (a *Accounts) active(id UserID) bool {
	u, ok := a.users[id]
	return ok && !u.Suspended
}

// Suspend closes every way into an account and ends its sessions; the
// account and its data stay. Suspending a suspended account changes
// nothing.
func (a *Accounts) Suspend(id UserID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	if u.Suspended {
		return nil
	}
	return a.record(event{Kind: accountSuspended, User: id})
}

// Reactivate opens a suspended account again: its password, identities
// and tokens work as before. Reactivating an active account changes
// nothing.
func (a *Accounts) Reactivate(id UserID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	if !ok {
		return ErrNotFound
	}
	if !u.Suspended {
		return nil
	}
	return a.record(event{Kind: accountReactivated, User: id})
}

// Suspended reports whether an account is suspended.
func (a *Accounts) Suspended(id UserID) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[id]
	return ok && u.Suspended
}

// Delete removes an account for good. Only its ID stays, known as deleted
// (Deleted); the username may be taken again by a new account, which gets
// a new ID. What the item store grants the account is the caller's
// concern.
func (a *Accounts) Delete(id UserID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.users[id]; !ok {
		return ErrNotFound
	}
	return a.record(event{Kind: accountDeleted, User: id})
}

// Deleted answers which of the given IDs belonged to accounts that were
// deleted, so that what refers to them can say so.
func (a *Accounts) Deleted(ids []UserID) map[UserID]bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[UserID]bool{}
	for _, id := range ids {
		if a.deleted[id] {
			out[id] = true
		}
	}
	return out
}
