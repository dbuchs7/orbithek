package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/sso"
)

// linkCookie binds a started link of an identity to the browser that
// started it (ADR-0229). It is SameSite=Lax because the provider's
// redirect back is a cross-site navigation; the session cookie is Strict
// and does not come with it, so the pending link names the account.
const linkCookie = "ot_oidc_link"

// freshSignIn is how recent a sign-in must be to stand in for the password
// of an account without one when it links an identity (ADR-0229). It is a
// security policy, not a resource budget.
const freshSignIn = 10 * time.Minute

// signedInRecently says whether a session that began at started is fresh
// enough at now to link an identity without a password.
func signedInRecently(started, now time.Time) bool {
	return !started.IsZero() && now.Sub(started) < freshSignIn
}

// linkFlow is a link begun by a signed-in account and not yet finished.
type linkFlow struct {
	user     auth.UserID
	provider string
	expires  time.Time
}

// linkFlows keeps the links begun, by the state the provider returns.
type linkFlows struct {
	mu      sync.Mutex
	now     func() time.Time
	pending map[string]linkFlow
}

func newLinkFlows() *linkFlows {
	return &linkFlows{now: time.Now, pending: map[string]linkFlow{}}
}

func (l *linkFlows) begin(state string, f linkFlow) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, x := range l.pending {
		if !now.Before(x.expires) {
			delete(l.pending, k)
		}
	}
	if len(l.pending) >= budget.Get().SSOPending {
		return false
	}
	f.expires = now.Add(sso.FlowLifetime)
	l.pending[state] = f
	return true
}

// take removes and answers the link begun with state, if it is still
// valid: a link is finished once.
func (l *linkFlows) take(state string) (linkFlow, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.pending[state]
	delete(l.pending, state)
	return f, ok && l.now().Before(f.expires)
}

// identityRoutes let a signed-in person link their account to an identity
// at a configured provider and unlink it, and administrators remove a
// link (ADR-0229). They are not offered over MCP: they change how an
// account is reached.
func identityRoutes(h oidcHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/me/identities", Summary: "The configured sign-in providers (id, name, kind) and whether one's account is linked to an identity at each; password says whether the account has a password", Handler: http.HandlerFunc(h.myIdentities)},
		{Method: http.MethodPost, Pattern: "/api/me/identities/{provider}", Summary: "Begin linking one's account to an identity at a provider: password is given again (an account without one needs a sign-in of the last ten minutes); answers url, the provider's address to open in this browser", Handler: http.HandlerFunc(h.beginLink)},
		{Method: http.MethodDelete, Pattern: "/api/me/identities/{provider}", Summary: "Unlink one's account from its identity at a provider; the only way an account without a password signs in stays", Handler: http.HandlerFunc(h.unlinkMine)},
		{Method: http.MethodDelete, Pattern: "/api/users/{id}/identities/{provider}", Summary: "Remove an account's link to its identity at a provider, also its only way to sign in (administrators)", Handler: http.HandlerFunc(h.unlinkUser)},
	}
}

func (h oidcHandlers) audit(e audit.Entry) { h.d.DB.Audit.Record(e) }

func (h oidcHandlers) providerByID(w http.ResponseWriter, r *http.Request) (*sso.Provider, bool) {
	id := r.PathValue("provider")
	for _, p := range h.d.SSO {
		if p.ID() == id {
			return p, true
		}
	}
	writeError(w, http.StatusNotFound, "no such sign-in provider")
	return nil, false
}

func (h oidcHandlers) linkedAt(user auth.UserID, p *sso.Provider) bool {
	for _, x := range h.d.DB.Accounts.Identities(user) {
		if x.Issuer == p.Issuer() {
			return true
		}
	}
	return false
}

// providerIDs names the configured providers an account is linked to.
func (h oidcHandlers) providerIDs(user auth.UserID) []string {
	out := []string{}
	for _, p := range h.d.SSO {
		if h.linkedAt(user, p) {
			out = append(out, p.ID())
		}
	}
	return out
}

func (h oidcHandlers) myIdentities(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	type providerJSON struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Linked bool   `json:"linked"`
		// Calendar says that the calendars of the linked account can be
		// shown: a Google identity, while calendars of other services are
		// allowed and Google is set up for them (ADR-0229, ADR-0174).
		Calendar bool `json:"calendar"`
	}
	list := []providerJSON{}
	for _, prov := range h.d.SSO {
		linked := h.linkedAt(p.User, prov)
		list = append(list, providerJSON{ID: prov.ID(), Name: prov.Name(), Kind: prov.Kind(), Linked: linked,
			Calendar: linked && prov.Kind() == "google" && h.googleCalendars()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": list, "password": h.d.DB.Accounts.HasPassword(p.User)})
}

func refuseCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

func (h oidcHandlers) beginLink(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if p.Via != "session" {
		writeError(w, http.StatusForbidden, "linking needs a signed-in browser, not a token")
		return
	}
	prov, ok := h.providerByID(w, r)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	accounts := h.d.DB.Accounts
	if accounts.HasPassword(p.User) {
		err := accounts.ConfirmPassword(p.User, req.Password, clientAddr(r))
		var throttled *auth.ThrottledError
		switch {
		case errors.As(err, &throttled):
			w.Header().Set("Retry-After", strconv.Itoa(int(throttled.Wait.Seconds())+1))
			refuseCode(w, http.StatusTooManyRequests, "throttled", "too many failed attempts")
			return
		case err != nil:
			h.audit(audit.Entry{Event: audit.IdentityLinkRefused, Actor: string(p.User), Addr: clientAddr(r), Detail: map[string]string{"provider": prov.ID(), "reason": "password"}})
			refuseCode(w, http.StatusForbidden, "password", "the password does not match")
			return
		}
	} else if started, ok := accounts.SessionStarted(p.Session); !ok || !signedInRecently(started, time.Now()) {
		refuseCode(w, http.StatusForbidden, "reauth", "sign in again first: linking without a password needs a sign-in of the last ten minutes")
		return
	}
	if h.linkedAt(p.User, prov) {
		refuseCode(w, http.StatusConflict, "linked", "the account is linked to an identity at this provider already")
		return
	}
	authURL, state, err := prov.Begin(r.Context())
	if err != nil {
		h.log.Error("linking cannot start", "provider", prov.ID(), "err", err)
		writeError(w, http.StatusBadGateway, "the identity provider cannot be reached")
		return
	}
	if !h.links.begin(state, linkFlow{user: p.User, provider: prov.ID()}) {
		writeError(w, http.StatusServiceUnavailable, "too many sign-ins in progress")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: linkCookie, Value: prov.ID() + "." + state, Path: "/api/oidc/", MaxAge: int(sso.FlowLifetime.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	writeJSON(w, http.StatusOK, map[string]string{"url": authURL})
}

// linkState answers the state of a link begun in this browser, when the
// callback's state is that one.
func linkState(r *http.Request) (provider, state string, ok bool) {
	c, err := r.Cookie(linkCookie)
	if err != nil {
		return "", "", false
	}
	provider, state, ok = strings.Cut(c.Value, ".")
	if !ok || provider == "" || state == "" || state != r.URL.Query().Get("state") {
		return "", "", false
	}
	return provider, state, true
}

// finishLink completes a link the provider returned to the callback with.
func (h oidcHandlers) finishLink(w http.ResponseWriter, r *http.Request, provider, state string) {
	http.SetCookie(w, &http.Cookie{Name: linkCookie, Value: "", Path: "/api/oidc/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	f, ok := h.links.take(state)
	refuse := func(user auth.UserID, code, reason string, err error) {
		h.audit(audit.Entry{Event: audit.IdentityLinkRefused, Actor: string(user), Addr: clientAddr(r), Detail: map[string]string{"provider": provider, "reason": reason}})
		if err != nil {
			h.log.Warn("linking an identity failed", "reason", reason, "err", err)
		}
		http.Redirect(w, r, "/#/account?link_error="+code, http.StatusFound)
	}
	if !ok || f.provider != provider {
		refuse("", "expired", "link not begun in this browser or expired", nil)
		return
	}
	prov := h.provider(provider)
	if prov == nil || prov.ID() != provider {
		refuse(f.user, "expired", "provider no longer configured", nil)
		return
	}
	if e := r.URL.Query().Get("error"); e != "" {
		refuse(f.user, "provider", "provider: "+e, nil)
		return
	}
	id, err := prov.Finish(r.Context(), state, r.URL.Query().Get("code"))
	switch {
	case errors.Is(err, sso.ErrDomain):
		refuse(f.user, "domain", "domain not allowed", err)
		return
	case errors.Is(err, sso.ErrUnknownState):
		refuse(f.user, "expired", "verification", err)
		return
	case err != nil:
		refuse(f.user, "provider", "verification", err)
		return
	}
	switch err := h.d.DB.Accounts.LinkIdentity(f.user, id.Issuer, id.Subject); {
	case errors.Is(err, auth.ErrIdentityTaken):
		refuse(f.user, "taken", "identity linked to an account already", nil)
		return
	case errors.Is(err, auth.ErrProviderLinked):
		refuse(f.user, "linked", "account linked at this provider already", nil)
		return
	case err != nil:
		refuse(f.user, "provider", "recording the link", err)
		return
	}
	h.audit(audit.Entry{Event: audit.IdentityLinked, Actor: string(f.user), Subject: string(f.user), Addr: clientAddr(r),
		Detail: map[string]string{"provider": provider, "issuer": id.Issuer}})
	http.Redirect(w, r, "/#/account?linked="+provider, http.StatusFound)
}

func (h oidcHandlers) unlink(w http.ResponseWriter, r *http.Request, actor, user auth.UserID, force bool) {
	prov, ok := h.providerByID(w, r)
	if !ok {
		return
	}
	switch err := h.d.DB.Accounts.UnlinkIdentity(user, prov.Issuer(), force); {
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, "the account is not linked to an identity at this provider")
		return
	case errors.Is(err, auth.ErrLastSignIn):
		refuseCode(w, http.StatusConflict, "last", "this is the account's only way to sign in")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "the link was not removed")
		return
	}
	h.audit(audit.Entry{Event: audit.IdentityUnlinked, Actor: string(actor), Subject: string(user), Addr: clientAddr(r),
		Detail: map[string]string{"provider": prov.ID(), "issuer": prov.Issuer()}})
	writeJSON(w, http.StatusOK, map[string]any{"identities": h.providerIDs(user)})
}

func (h oidcHandlers) unlinkMine(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	h.unlink(w, r, p.User, p.User, false)
}

func (h oidcHandlers) unlinkUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := authHandlers{d: h.d}.adminOnly(w, r)
	if !ok {
		return
	}
	h.unlink(w, r, auth.UserID(actor), auth.UserID(r.PathValue("id")), true)
}

// googleCalendars says whether people may connect the calendars of a
// Google account: calendars of other services are allowed and Google is set
// up for them (ADR-0174).
func (h oidcHandlers) googleCalendars() bool {
	return h.d.Calendars != nil && h.d.Calendars.Google() != nil && h.d.DB.Site.Calendars().External
}

// linkedGoogleSubject answers the subject of the Google identity an
// account is linked to, through the provider of the kind google, whose
// client also connects calendars (ADR-0229).
func linkedGoogleSubject(d Deps, user auth.UserID) string {
	for _, p := range d.SSO {
		if p.Kind() != "google" {
			continue
		}
		for _, x := range d.DB.Accounts.Identities(user) {
			if x.Issuer == p.Issuer() {
				return x.Subject
			}
		}
	}
	return ""
}
