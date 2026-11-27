package auth

import (
	"errors"
	"testing"
	"time"
)

// TestASuspendedAccountCannotSignInUntilReactivated: suspending an account
// ends its sessions and refuses every way in — password, single sign-on,
// API tokens, a password reset — while keeping the account, its tokens and
// its data; reactivating it opens them again, and a reload of the journal
// gives the same state (ADR-draft-suspending-and-deleting-accounts).
func TestASuspendedAccountCannotSignInUntilReactivated(t *testing.T) {
	a, j, c := newAccounts(t)
	u, err := a.CreateUser("anna", pw)
	noErr(t, err)
	noErr(t, a.SetEmail(u.ID, "anna@example.org"))
	secret, _, err := a.Login("anna", pw, "10.0.0.1")
	noErr(t, err)
	tok, _, err := a.CreateToken(u.ID, "script", ScopeFull, time.Hour)
	noErr(t, err)
	_, _, err = a.HandReset(u.ID)
	noErr(t, err)

	noErr(t, a.Suspend(u.ID))
	noErr(t, a.Suspend(u.ID)) // already: no error
	if !a.Suspended(u.ID) {
		t.Fatal("not suspended")
	}
	if _, ok := a.Session(secret); ok {
		t.Error("a session outlived the suspension")
	}
	if _, ok := a.TokenPrincipal(tok); ok {
		t.Error("an API token works while suspended")
	}
	if _, _, err := a.Login("anna", pw, "10.0.0.1"); !errors.Is(err, ErrSuspended) {
		t.Errorf("login with the right password: %v", err)
	}
	if _, _, err := a.Login("anna", "a wrong password", "10.0.0.1"); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("login with a wrong password: %v", err)
	}
	if _, _, err := a.StartSession(u.ID); !errors.Is(err, ErrSuspended) {
		t.Errorf("single sign-on: %v", err)
	}
	if _, _, _, err := a.RequestReset("anna", "10.0.0.2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("reset by mail: %v", err)
	}
	if _, _, err := a.HandReset(u.ID); !errors.Is(err, ErrSuspended) {
		t.Errorf("handed reset: %v", err)
	}
	users := a.Users()
	if len(users) != 1 || !users[0].Suspended {
		t.Fatalf("users %+v", users)
	}
	if _, ok := a.Usernames([]UserID{u.ID})[u.ID]; !ok {
		t.Error("a suspended account lost its name")
	}

	b, err := New(j.records, j.append, WithClock(c.now), WithIterations(1000))
	noErr(t, err)
	if !b.Suspended(u.ID) {
		t.Fatal("the suspension did not survive a reload")
	}
	noErr(t, b.Reactivate(u.ID))
	noErr(t, b.Reactivate(u.ID)) // already: no error
	if _, ok := b.TokenPrincipal(tok); !ok {
		t.Error("the token does not work again")
	}
	if _, _, err := b.Login("anna", pw, "10.0.0.1"); err != nil {
		t.Errorf("login after reactivation: %v", err)
	}
	if err := b.Suspend("nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown account: %v", err)
	}
}

// TestASuspendedAccountsOAuthTokensStop: an assistant acting for a
// suspended person neither uses its access token nor gets new ones.
func TestASuspendedAccountsOAuthTokensStop(t *testing.T) {
	a, _, _ := newAccounts(t)
	u, err := a.CreateUser("olga", pw)
	noErr(t, err)
	cl, secret, err := a.CreateOAuthClient("Assistant", []string{callback}, "", false)
	noErr(t, err)
	code, err := a.IssueCode(u.ID, cl.ID, callback, challengeOf(verifier), resource, ScopeFull)
	noErr(t, err)
	tok, err := a.ExchangeCode(cl.ID, secret, code, callback, verifier, resource)
	noErr(t, err)
	noErr(t, a.Suspend(u.ID))
	if _, _, ok := a.OAuthPrincipal(tok.Access); ok {
		t.Error("an access token works while suspended")
	}
	if _, err := a.Refresh(cl.ID, secret, tok.Refresh); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("refresh while suspended: %v", err)
	}
	if _, err := a.IssueCode(u.ID, cl.ID, callback, challengeOf(verifier), resource, ScopeFull); !errors.Is(err, ErrSuspended) {
		t.Errorf("approval while suspended: %v", err)
	}
}

// TestADeletedAccountLeavesOnlyItsID: deleting an account ends every way in
// and removes its credentials and personal data from the accounts; its ID
// stays known as deleted and is never reused, its username is free, and a
// reload of the journal gives the same state
// (ADR-draft-suspending-and-deleting-accounts).
func TestADeletedAccountLeavesOnlyItsID(t *testing.T) {
	a, j, _ := newAccounts(t)
	u, err := a.CreateUser("bruno", pw)
	noErr(t, err)
	other, err := a.CreateUser("carla", pw)
	noErr(t, err)
	noErr(t, a.SetEmail(u.ID, "bruno@example.org"))
	noErr(t, a.SetDisplayName(u.ID, "Bruno B."))
	noErr(t, a.SetAvatar(u.ID, []byte("png"), "image/png"))
	noErr(t, a.SetFavourite(u.ID, "TEAM", true))
	noErr(t, a.SetLanguage(u.ID, "de"))
	noErr(t, a.LinkIdentity(u.ID, "https://idp.example", "sub-1"))
	_, err = a.SaveCalendarView(u.ID, CalendarView{Name: "Mine", Params: "view=month"})
	noErr(t, err)
	views := a.CalendarViews(u.ID)
	feed, err := a.NewCalendarFeed(u.ID, views[0].ID)
	noErr(t, err)
	secret, _, err := a.Login("bruno", pw, "10.0.0.1")
	noErr(t, err)
	tok, _, err := a.CreateToken(u.ID, "script", ScopeFull, time.Hour)
	noErr(t, err)
	cl, csecret, err := a.CreateOAuthClient("Assistant", []string{callback}, "", false)
	noErr(t, err)
	code, err := a.IssueCode(u.ID, cl.ID, callback, challengeOf(verifier), resource, ScopeFull)
	noErr(t, err)
	otok, err := a.ExchangeCode(cl.ID, csecret, code, callback, verifier, resource)
	noErr(t, err)

	noErr(t, a.Delete(u.ID))
	check := func(name string, a *Accounts) {
		t.Helper()
		if _, ok := a.Session(secret); ok {
			t.Errorf("%s: a session outlived the deletion", name)
		}
		if _, ok := a.TokenPrincipal(tok); ok {
			t.Errorf("%s: an API token outlived the deletion", name)
		}
		if _, _, ok := a.OAuthPrincipal(otok.Access); ok && name == "live" {
			t.Errorf("%s: an access token outlived the deletion", name)
		}
		if _, err := a.Refresh(cl.ID, csecret, otok.Refresh); !errors.Is(err, ErrInvalidGrant) {
			t.Errorf("%s: refresh after deletion: %v", name, err)
		}
		if _, _, err := a.Login("bruno", pw, "10.0.0.9"); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s: login after deletion: %v", name, err)
		}
		if _, ok := a.UserByIdentity("https://idp.example", "sub-1"); ok {
			t.Errorf("%s: the identity is still linked", name)
		}
		if _, _, ok := a.CalendarFeed(feed); ok {
			t.Errorf("%s: the calendar feed still opens", name)
		}
		if _, ok := a.Usernames([]UserID{u.ID})[u.ID]; ok {
			t.Errorf("%s: a deleted account has a name", name)
		}
		if got := a.Deleted([]UserID{u.ID, other.ID, "nobody"}); len(got) != 1 || !got[u.ID] {
			t.Errorf("%s: deleted %v", name, got)
		}
		if users := a.Users(); len(users) != 1 || users[0].ID != other.ID {
			t.Errorf("%s: users %+v", name, users)
		}
		if a.DisplayName(u.ID) != "" || a.Language(u.ID) != "" || a.Favourites(u.ID) != nil ||
			a.Contacts(u.ID) != (Contacts{}) || len(a.CalendarViews(u.ID)) != 0 || len(a.Grants(u.ID)) != 0 {
			t.Errorf("%s: personal data kept", name)
		}
		if data, _ := a.Avatar(u.ID); data != nil {
			t.Errorf("%s: profile picture kept", name)
		}
		if _, _, err := a.StartSession(u.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: single sign-on: %v", name, err)
		}
		for what, err := range map[string]error{
			"delete":     a.Delete(u.ID),
			"suspend":    a.Suspend(u.ID),
			"reactivate": a.Reactivate(u.ID),
			"email":      a.SetEmail(u.ID, "x@example.org"),
			"link":       a.LinkIdentity(u.ID, "https://idp.example", "sub-2"),
		} {
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("%s: %s after deletion: %v", name, what, err)
			}
		}
		if _, _, err := a.HandReset(u.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: handed reset after deletion: %v", name, err)
		}
	}
	check("live", a)
	b, err := New(j.records, j.append, WithIterations(1000))
	noErr(t, err)
	check("reloaded", b)

	again, err := b.CreateUser("bruno", pw)
	noErr(t, err)
	if again.ID == u.ID {
		t.Fatal("a deleted account's ID was reused")
	}
	if b.Deleted([]UserID{again.ID})[again.ID] {
		t.Error("the new account counts as deleted")
	}
}
