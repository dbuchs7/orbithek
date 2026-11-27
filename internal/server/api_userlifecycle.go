package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/dbuchs7/orbithek/internal/audit"
	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/item"
)

// userLifecycleRoutes let administrators suspend, reactivate and delete
// accounts (ADR-draft-suspending-and-deleting-accounts).
func userLifecycleRoutes(d Deps) []Route {
	h := authHandlers{d: d}
	return []Route{
		{Method: http.MethodPut, Pattern: "/api/users/{id}/suspended", Summary: "Suspend an account, ending its sessions and refusing every way in, or reactivate it: suspended (administrators; not one's own account, not the last administrator)", Handler: http.HandlerFunc(h.setSuspended)},
		{Method: http.MethodDelete, Pattern: "/api/users/{id}", Summary: "Delete an account for good: its roles, memberships, credentials and personal data go, its ID stays as deleted (administrators; not one's own account, not the last administrator)", Handler: http.HandlerFunc(h.deleteUser)},
	}
}

// refusal refuses a suspension or deletion with a status and a reason the
// caller can act on.
type refusal struct {
	status int
	reason string
}

func (e *refusal) Error() string { return e.reason }

// refused answers a refusal and reports whether err was one.
func refused(w http.ResponseWriter, err error) bool {
	var no *refusal
	if !errors.As(err, &no) {
		return false
	}
	writeError(w, no.status, no.reason)
	return true
}

// mayEnd checks, under the item store's lock, that the caller administers
// the installation and that the account may be suspended or deleted: it
// exists, it is not the caller's own, and the installation keeps an
// administrator who can sign in.
func (h authHandlers) mayEnd(s *item.Store, caller, target auth.UserID) error {
	if !s.HasSystemRole(string(caller), item.RoleAdmin) {
		return &refusal{http.StatusForbidden, "administrators only"}
	}
	if _, ok := h.d.DB.Accounts.Usernames([]auth.UserID{target})[target]; !ok {
		return &refusal{http.StatusNotFound, "no such account"}
	}
	if caller == target {
		return &refusal{http.StatusBadRequest, "one does not suspend or delete one's own account"}
	}
	if s.HasSystemRole(string(target), item.RoleAdmin) && !h.otherAdminSignsIn(s, target) {
		return &refusal{http.StatusBadRequest, fmt.Sprintf("%s is the last administrator who can sign in", target)}
	}
	return nil
}

// otherAdminSignsIn reports whether someone besides principal holds the
// system role admin and can sign in: an account that is not suspended
// (ADR-0257, ADR-draft-suspending-and-deleting-accounts).
func (h authHandlers) otherAdminSignsIn(s *item.Store, principal auth.UserID) bool {
	var others []auth.UserID
	for _, p := range s.SystemRoleHolders(item.RoleAdmin) {
		if auth.UserID(p) != principal {
			others = append(others, auth.UserID(p))
		}
	}
	for id := range h.d.DB.Accounts.Usernames(others) {
		if !h.d.DB.Accounts.Suspended(id) {
			return true
		}
	}
	return false
}

func (h authHandlers) setSuspended(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	target := auth.UserID(r.PathValue("id"))
	var req struct {
		Suspended *bool `json:"suspended"`
	}
	if err := readJSON(w, r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Suspended == nil {
		writeError(w, http.StatusBadRequest, "suspended must be true or false")
		return
	}
	err := h.d.DB.Write(func(s *item.Store) error {
		if err := h.mayEnd(s, p.User, target); err != nil {
			return err
		}
		if *req.Suspended {
			return h.d.DB.Accounts.Suspend(target)
		}
		return h.d.DB.Accounts.Reactivate(target)
	})
	switch {
	case refused(w, err):
		return
	case err != nil:
		h.d.Log.Error("account not suspended or reactivated", "err", err)
		writeError(w, http.StatusInternalServerError, "the change was not made")
		return
	}
	event := audit.UserReactivated
	if *req.Suspended {
		event = audit.UserSuspended
	}
	h.record(audit.Entry{Event: event, Actor: string(p.User), Addr: clientAddr(r), Subject: string(target)})
	w.WriteHeader(http.StatusNoContent)
}

// deleteUser deletes an account. Its roles in rooms, memberships of groups
// and system roles are taken back in one batch of the item store first,
// then its connections to process engines and its calendars of other
// services end, and last the account goes. Each step can be repeated, so
// a request that fails half-way is completed by asking again.
func (h authHandlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	target := auth.UserID(r.PathValue("id"))
	username := h.d.DB.Accounts.Usernames([]auth.UserID{target})[target]
	err := h.d.DB.Write(func(s *item.Store) error {
		if err := h.mayEnd(s, p.User, target); err != nil {
			return err
		}
		return s.RevokeAll(string(p.User), string(target))
	})
	if err == nil {
		err = h.endOutsideTheAccounts(target)
	}
	if err == nil {
		err = h.d.DB.Accounts.Delete(target)
	}
	switch {
	case refused(w, err):
		return
	case errors.Is(err, item.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		h.d.Log.Error("account not deleted", "err", err)
		writeError(w, http.StatusInternalServerError, "the account was not deleted; ask again to complete it")
		return
	}
	h.record(audit.Entry{Event: audit.UserDeleted, Actor: string(p.User), Addr: clientAddr(r), Subject: string(target),
		Detail: map[string]string{"username": username}})
	w.WriteHeader(http.StatusNoContent)
}

// endOutsideTheAccounts ends what a deleted account keeps outside the
// accounts journal: its connections to process engines (ADR-0151) and its
// calendars and accounts of other services (ADR-0174), destroying their
// keys.
func (h authHandlers) endOutsideTheAccounts(id auth.UserID) error {
	user := string(id)
	if c := h.d.DB.Connections; c != nil {
		for _, cn := range c.Of(user, "") {
			if err := c.End(user, cn.Engine, "account deleted"); err != nil {
				return err
			}
		}
	}
	if c := h.d.DB.Calendars; c != nil {
		// An account's calendars go with it; those of addresses after.
		for _, acc := range c.Accounts(user) {
			if err := c.Disconnect(user, acc.ID); err != nil {
				return err
			}
		}
		for _, src := range c.Sources(user) {
			if err := c.Remove(user, src.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
