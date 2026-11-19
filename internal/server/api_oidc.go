package server

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/site"
	"github.com/dbuchs7/orbithek/internal/sso"
)

// stateCookie binds a started single sign-on to the browser that started
// it, so a callback cannot be replayed into another browser (login CSRF).
const stateCookie = "ot_oidc_state"

// oidcRoutes are the single sign-on routes (ADR-0030). They are public:
// the caller has no principal yet.
func oidcRoutes(d Deps) []Route {
	h := oidcHandlers{d: d, log: d.Log, links: newLinkFlows()}
	if h.log == nil {
		h.log = slog.New(slog.DiscardHandler)
	}
	return append(identityRoutes(h),
		Route{Method: http.MethodGet, Pattern: "/api/oidc", Summary: "Whether single sign-on is configured, and its providers (id, name, kind)", Access: Public, Handler: http.HandlerFunc(h.info)},
		Route{Method: http.MethodGet, Pattern: "/api/oidc/login", Summary: "Start single sign-on with the provider given as provider (the first by default); redirects to it", Access: Public, Handler: http.HandlerFunc(h.login)},
		Route{Method: http.MethodGet, Pattern: "/api/oidc/callback", Summary: "The identity provider's redirect back; starts a session, or finishes linking an account begun in this browser (ADR-0229)", Access: Public, Handler: http.HandlerFunc(h.callback)},
	)
}

type oidcHandlers struct {
	d   Deps
	log *slog.Logger
	// links are the links of identities begun and not finished (ADR-0229).
	links *linkFlows
}

// provider finds a configured provider by its ID; an empty ID is the
// first.
func (h oidcHandlers) provider(id string) *sso.Provider {
	for _, p := range h.d.SSO {
		if id == "" || p.ID() == id {
			return p
		}
	}
	return nil
}

func (h oidcHandlers) info(w http.ResponseWriter, _ *http.Request) {
	if len(h.d.SSO) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "providers": []any{}})
		return
	}
	type providerJSON struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Kind string `json:"kind,omitempty"`
	}
	list := []providerJSON{}
	for _, p := range h.d.SSO {
		list = append(list, providerJSON{ID: p.ID(), Name: p.Name(), Kind: p.Kind()})
	}
	// name is the first provider's, as before several were possible.
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "name": list[0].Name, "providers": list})
}

func (h oidcHandlers) login(w http.ResponseWriter, r *http.Request) {
	p := h.provider(r.URL.Query().Get("provider"))
	if p == nil {
		writeError(w, http.StatusNotFound, "single sign-on is not configured")
		return
	}
	authURL, state, err := p.Begin(r.Context())
	if err != nil {
		h.log.Error("single sign-on cannot start", "provider", p.ID(), "err", err)
		writeError(w, http.StatusBadGateway, "the identity provider cannot be reached")
		return
	}
	// The cookie names the provider beside the state, so the one callback
	// address serves every provider. SameSite=Lax, not Strict: the
	// provider's redirect back is a cross-site navigation, and a Strict
	// cookie would not be sent with it.
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: p.ID() + "." + state, Path: "/api/oidc/", MaxAge: int(sso.FlowLifetime.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	http.Redirect(w, r, authURL, http.StatusFound)
}

// fail ends a sign-on attempt: audited, and the browser goes back to the UI
// with a generic error, never the provider's details. code tells the UI
// which message to show.
func (h oidcHandlers) fail(w http.ResponseWriter, r *http.Request, reason string, err error, code ...string) {
	h.d.DB.Audit.Record(audit.Entry{Event: audit.LoginFailed, Addr: clientAddr(r), Detail: map[string]string{"method": "oidc", "reason": reason}})
	if err != nil {
		h.log.Warn("single sign-on failed", "reason", reason, "err", err)
	}
	c := "sso"
	if len(code) > 0 {
		c = code[0]
	}
	http.Redirect(w, r, "/?login_error="+c, http.StatusFound)
}

func (h oidcHandlers) callback(w http.ResponseWriter, r *http.Request) {
	if len(h.d.SSO) == 0 {
		writeError(w, http.StatusNotFound, "single sign-on is not configured")
		return
	}
	// A link begun in this browser comes back to the same address; its
	// own cookie tells it from a sign-in (ADR-0229).
	if provider, state, ok := linkState(r); ok {
		h.finishLink(w, r, provider, state)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/api/oidc/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isSecure(r)})
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		h.fail(w, r, "provider: "+e, nil)
		return
	}
	state := q.Get("state")
	c, err := r.Cookie(stateCookie)
	var prov *sso.Provider
	var bound string
	if err == nil {
		id, s, ok := strings.Cut(c.Value, ".")
		if ok && id != "" {
			prov, bound = h.provider(id), s
		}
	}
	if prov == nil || state == "" || subtle.ConstantTimeCompare([]byte(bound), []byte(state)) != 1 {
		h.fail(w, r, "state does not belong to this browser", err)
		return
	}
	id, err := prov.Finish(r.Context(), state, q.Get("code"))
	if errors.Is(err, sso.ErrDomain) {
		h.fail(w, r, "domain not allowed", err, "domain")
		return
	}
	if err != nil {
		h.fail(w, r, "verification", err)
		return
	}
	accounts := h.d.DB.Accounts
	u, ok := accounts.UserByIdentity(id.Issuer, id.Subject)
	if !ok && h.d.DB.Site.SignIn().NoNewAccounts {
		// Administrators allow only identities linked to an account
		// (ADR-0229). The answer says nothing about whether an account
		// with the provider's name or address exists.
		h.fail(w, r, "no linked account", nil, "unlinked")
		return
	}
	if !ok {
		// Just-in-time provisioning (ADR-0013): a new account without roles.
		// It is never linked to an existing account by e-mail address,
		// which would let whoever controls the provider take that account.
		if u, err = accounts.CreateExternalUser(id.Username, id.Issuer, id.Subject); err != nil {
			h.fail(w, r, "provisioning", err)
			return
		}
		h.d.DB.Audit.Record(audit.Entry{Event: audit.UserCreated, Addr: clientAddr(r), Subject: string(u.ID),
			Detail: map[string]string{"username": u.Username, "method": "oidc", "provider": prov.ID(), "issuer": id.Issuer}})
	}
	// The provider's name becomes the display name of an account that never
	// had one (ADR-draft-the-provider-names-a-first-display-name). A name the
	// account page would refuse is left out; the sign-in goes on either way.
	if _, err := accounts.SetFirstDisplayName(u.ID, id.Name); err != nil && !errors.Is(err, auth.ErrInvalid) {
		h.log.Warn("display name from single sign-on not saved", "err", err)
	}
	secret, p, err := accounts.StartSession(u.ID)
	if err != nil {
		h.fail(w, r, "session", err)
		return
	}
	h.d.DB.Audit.Record(audit.Entry{Event: audit.Login, Actor: string(p.User), Addr: clientAddr(r), Detail: map[string]string{"method": "oidc", "provider": prov.ID()}})
	setSessionCookie(w, r, secret)
	http.Redirect(w, r, "/", http.StatusFound)
}

// signInRoutes let administrators decide whether a first sign-in through
// single sign-on creates an account (ADR-0229). The setting exists
// whether a provider is configured or not, so it can be turned off before
// an operator sets a client.
func signInRoutes(d Deps) []Route {
	h := signInHandlers{authHandlers: authHandlers{d: d}}
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/settings/sign-in", Summary: "Whether a first sign-in through single sign-on creates an account without roles, and whether a provider is configured (administrators)", Handler: http.HandlerFunc(h.settings)},
		{Method: http.MethodPut, Pattern: "/api/settings/sign-in", Summary: "Let a first sign-in through single sign-on create an account, or allow only identities linked to an account: newAccounts (administrators)", Handler: http.HandlerFunc(h.setSettings)},
	}
}

type signInHandlers struct{ authHandlers }

// signInSettings is the setting as the API names it: positively, as the
// administration page asks it.
type signInSettings struct {
	NewAccounts *bool `json:"newAccounts"`
}

func (h signInHandlers) settings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminOnly(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"newAccounts": !h.d.DB.Site.SignIn().NoNewAccounts, "sso": len(h.d.SSO) > 0})
}

func (h signInHandlers) setSettings(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminOnly(w, r)
	if !ok {
		return
	}
	var req signInSettings
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.NewAccounts == nil {
		writeError(w, http.StatusBadRequest, "newAccounts is required")
		return
	}
	before := h.d.DB.Site.SignIn()
	next := site.SignIn{NoNewAccounts: !*req.NewAccounts}
	if err := h.d.DB.Site.SetSignIn(actor, next); err != nil {
		writeError(w, http.StatusInternalServerError, "setting not saved")
		return
	}
	if before != next {
		state := "on"
		if next.NoNewAccounts {
			state = "off"
		}
		h.record(audit.Entry{Event: audit.SignInChanged, Actor: actor, Addr: clientAddr(r), Detail: map[string]string{"newAccounts": state}})
	}
	h.settings(w, r)
}
