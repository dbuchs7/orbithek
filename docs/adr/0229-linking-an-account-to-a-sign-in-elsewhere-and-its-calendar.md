# ADR-0229: Linking an existing account to a sign-in at Google, Microsoft or Apple, and showing that account's calendar

- **Status:** Proposed
- **Implementation:** Not started
- **Date:** 2026-11-16
- **Deciders:** Project initiators (request of 2026-11-16: people shall sign in with Google, and an existing account must be linked to the Google or Microsoft account for that; once one's Google or Apple account is linked, one shall be able to show its calendar)
- **Open question:** whether two accounts of one person, one of them made at a first sign-in, can ever be merged; whether a sign-in that matches no account should become an account request (ADR-0061) instead of being refused; and whether Apple accepts a sign-in without a form post when only `openid` is asked for, which is assumed below and must be verified against Apple's documentation before Apple is built
- **Question checked:** 2026-11

## Context and problem statement

Single sign-on (ADR-0030, ADR-0066) knows an identity only as its issuer and subject.
A first sign-in makes a new account without roles (just-in-time), and an identity is never tied to an existing account.
ADR-0030 leaves open whether an existing account can be linked, and by which proof.

This has three effects that the request runs into:

- A person who has worked under a local account and then signs in with Google gets a second, empty account; their roles, authorship and history stay with the first one.
- Whoever has an account at an allowed provider gets an account here.
  With Google and no `OT_GOOGLE_DOMAINS`, that is any Google account.
- Calendars of other services (ADR-0174) connect Google through the client of single sign-on.
  An operator who sets `OT_GOOGLE_CLIENT_ID` for calendars turns on sign-in with Google, and with it new accounts at a first sign-in.

The request adds Apple, for which there is no provider yet, and asks that a linked account lead to its calendar.
Signing in and reading a calendar are different grants: Google grants them by separate scopes, and Apple's sign-in grants no access to iCloud data at all; iCloud calendars are read over CalDAV with an app-specific password (ADR-0174).

How does a person tie an identity at Google, Microsoft or Apple to the account they already have, so that only they can do it, and how does that lead to the calendar of the same service?

## Decision drivers

- An identity is issuer and subject, never an e-mail address (ADR-0013, ADR-0030, ADR-0066).
  Whoever controls a provider or a mail domain must not thereby control an account here.
- Linking is a security-relevant change of how an account is reached: it must prove that the same person holds both, it is audited, and it can be undone.
- Nobody is locked out by undoing a link, and an installation keeps a way in when a provider fails.
- The installation decides whether a sign-in may create accounts at all, as it decides whether it takes account requests (ADR-0061).
- A calendar grant is the person's own decision and stays revocable on its own (ADR-0174).
  A sign-in must not depend on whether calendars are allowed, nor ask for more than a sign-in needs.
- No new dependency (AGENTS.md); `go-oidc` and `golang.org/x/oauth2` are already used, and an ES256 signature is in the standard library.
- One callback address for every provider stays (ADR-0066), so linking needs no new redirect registration.

## Considered options

1. **Self-service linking from one's account page, after the password is given again; one identity per provider; an administrator's setting whether a sign-in may create accounts; the calendar offered after linking as a separate grant for the same account.**
2. **Automatic linking by a verified e-mail address.**
3. **An administrator binds an identity to an account.**
4. **One consent for sign-in and calendar.**

## Decision outcome

Chosen option: **1**, because it is the only one in which the person who holds the account also proves they hold the identity (options 2 and 3 each trust a third party for that), and because it keeps the sign-in independent of the calendar, which option 4 does not.

- **Linking:** the account page lists the configured providers and, for each, whether the account is linked.
  *Link* starts at `POST /api/me/identities/{provider}` with the current password; an account without a password gives none, but its session must have begun with a sign-in no older than ten minutes.
  The server keeps the pending link in memory, bound to the user and the state, for the flow's lifetime (`sso.FlowLifetime`), and sets the state cookie with the purpose `link` beside the provider.
  The provider returns to the one callback `/api/oidc/callback` (ADR-0066).
  The callback finishes a link only when the purpose is `link`, the state matches the pending link, and the session cookie belongs to the same user; otherwise it fails as a sign-in fails.
  A link records `IdentityLinked` in the accounts journal, the event a first sign-in already writes.
- **Refusals:** an identity already linked to any account, an account made at a first sign-in included, is refused and named as such.
  An account has at most one identity per provider; linking a second one at the same provider is refused until the first is removed.
  Both are policies, not budgets, and are classified in `internal/budget/registry_test.go`.
- **Removing a link:** the person removes their own link (`DELETE /api/me/identities/{provider}`); an administrator removes any account's link (`DELETE /api/users/{id}/identities/{provider}`), for a lost provider account or to free an identity held by an account made at a first sign-in.
  Removal records a new event `IdentityUnlinked`.
  Removing the last way to sign in, the only identity of an account without a password, is refused.
  An administrator never adds a link.
- **New accounts at sign-in:** administrators choose under *Administration → Sign-in* whether a sign-in that matches no linked identity creates an account (`SignInSet` in `site.wal`, `PUT /api/settings/sign-in`).
  The setting exists whether a provider is configured or not, so it can be turned off before an operator sets a client.
  Off, such a sign-in is refused with a message that says to sign in with the password and link the identity on the account page; it says nothing about whether an account exists.
  Its default is *on*, so installations that rely on accounts made at a first sign-in (ADR-0030) keep working.
- **Apple as a provider:** `OT_APPLE_CLIENT_ID` (a Services ID), `OT_APPLE_TEAM_ID`, `OT_APPLE_KEY_ID` and `OT_APPLE_PRIVATE_KEY` (the PEM of the key Apple issues) add a provider with the issuer `https://appleid.apple.com`.
  Apple's client secret is a JSON Web Token signed with ES256; the server signs a new one for each token request, valid for five minutes.
  Only `openid` is asked for, so that Apple returns to the callback by a redirect, as the others do, and the state cookie stays `SameSite=Lax`.
  This is the assumption named under *Open question*.
  An account made at a first sign-in with Apple therefore gets a username without a name in it.
  Apple requires a paid developer membership for a Services ID; that is the operator's choice.
- **Microsoft** links as Google does, through the Entra ID provider of ADR-0066, which signs in only accounts of one tenant.
  Its calendars are not read through the link: Microsoft Graph is left open by ADR-0174.
- **The calendar after linking:** when calendars of other services are allowed (`site.Calendars`, ADR-0174), the account page offers next to a linked identity what its service allows.
  - Google: *Show the calendars of this Google account* starts the existing calendar connection (`GET /api/me/calendar-accounts/google/connect`) with `login_hint` set to the identity's subject, so that Google proposes the same account.
    The calendar flow also asks for `openid`, so the connected calendar account records the Google subject it belongs to; the account page shows identity and calendar together.
    If the person chooses another Google account there, that account's calendars are connected as before; the hint is a proposal, not a constraint.
  - Apple: *Show the calendars of this iCloud account* opens the CalDAV form of ADR-0174 with the server `https://caldav.icloud.com` filled in and a link to where Apple issues app-specific passwords.
    The Apple ID and the app-specific password are entered there; the sign-in gives neither.
  - Removing a link asks whether a calendar account recorded for the same subject is disconnected too; by default it stays, as the two grants are separate.
- **Audit:** `identity.link`, `identity.link-refused`, `identity.unlink` (with the actor, the account, the provider's ID and the issuer, never the subject's tokens) and `signin.change`.
- **API and MCP:** the routes above are authenticated; the identity and setting routes change how an account is reached and are not offered over MCP (ADR-0012, `internal/mcp/tools.go`).
- **UI:** catalogue keys in `en.js` and `de.js`, the account page and the administration page in both manuals, and the linking flow in `e2e/smoke.cjs` against the fake provider of `sso/ssotest`.

### Consequences

- **Positive:** a person keeps one account and adds the sign-ins they want; roles, authorship and history stay where they are.
- **Positive:** an installation can allow sign-in with Google for its known people only, also with a Google account outside any domain, and can set the Google client for calendars without opening sign-up.
- **Positive:** the calendar of a linked Google account is one click away and still a grant of its own, revocable without losing the sign-in.
- **Positive:** the data model is unchanged; an account already maps several identities, and `IdentityLinked` is replayed today.
- **Negative / trade-offs accepted:** `IdentityUnlinked` is a new event kind; an older binary refuses to open an accounts journal that holds one (`unknown event`), so going back to an older release after a link was removed needs a restore.
- **Negative / trade-offs accepted:** an account made at a first sign-in before linking existed keeps its identity until an administrator removes the link; it is then an account with no way to sign in.
  Merging the two is not decided (*Open question*).
- **Negative / trade-offs accepted:** the default *on* keeps today's exposure for installations that do not change the setting; the administration page and the start-up warning of ADR-0066 say so.
- **Negative / trade-offs accepted:** an Apple account's calendar still needs an app-specific password; the link only fills in the server.
- **Negative / trade-offs accepted:** asking Google for `calendar.readonly` remains a sensitive scope; until Google verifies the client, the calendar consent shows Google's warning about an unverified app.
  The sign-in does not, because it never asks for that scope.
- **Follow-ups / risks to watch:** turning a refused sign-in into an account request (ADR-0061); merging accounts; Microsoft calendars through Graph (ADR-0174); revoking a provider session at sign-out (ADR-0030).

## Pros and cons of the options

### Option 1: self-service linking after the password, separate calendar grant

- Good: the person proves both sides at once: the password (or a fresh sign-in) for the account, the provider's sign-in for the identity.
- Good: no administrator needs to know a subject, which is an opaque number.
- Good: the sign-in stays minimal, and the calendar stays the person's own, revocable decision.
- Bad: a person without access to their password and without a fresh sign-in must first reset the password (ADR-0061 delivery) or ask an administrator.
- Bad: one more step between linking and seeing the calendar.

### Option 2: automatic linking by a verified e-mail address

Strongest form: nothing for anyone to do; Google marks addresses as verified, and for addresses at `gmail.com` that mark is reliable.
Most people would find their account linked at their first sign-in.

- Bad: a domain's administrator at Google Workspace or Entra ID can create or reassign any address in the domain, and so take any account here whose address lies in it.
- Bad: an address given to someone else after its owner left opens the former owner's account.
- Bad: the mail address of an account is not verified here (ADR-0061), so the match compares a verified address with an unverified one.
- Bad: it contradicts ADR-0013, ADR-0030 and ADR-0066, which rule matching by e-mail out for these reasons.

### Option 3: an administrator binds an identity to an account

Strongest form: the administrator already decides who has an account and which roles it holds (ADR-0013); binding the way in is the same kind of decision, and no person needs to act.

- Bad: the administrator must know the subject, an opaque number the provider shows to nobody but the relying party; in practice it can only be learned from a sign-in, which is option 1 with an extra step.
- Bad: a mistyped or misread subject gives a stranger an account with roles, and nothing in the flow catches it.
- Good: remains useful for removing a link, which this decision allows administrators.

### Option 4: one consent for sign-in and calendar

Strongest form: one click and one consent screen give both; one refresh token serves sign-in and calendar; Google's incremental authorization (`include_granted_scopes`) is built for this.

- Bad: every sign-in with Google would ask for a sensitive scope, and until Google verifies the client every sign-in would show the warning about an unverified app.
- Bad: the sign-in would depend on whether an administrator allows calendars, and would ask for more than it needs where they are not allowed.
- Bad: the provider path of single sign-on would keep a long-lived credential that it does not keep today; withdrawing the calendar would have to keep the sign-in working, and the other way round.
- Bad: Apple's sign-in grants no calendar access, so it would work for Google only.

## Links

- answers the open question of ADR-0030 (linking an existing account), which stays accepted
- relates to ADR-0013 (no account controlled from outside), ADR-0066 (one callback, several providers), ADR-0061 (account requests and the site setting beside them), ADR-0174 (calendars of other services), ADR-0029 (audit trail), ADR-0012 (what MCP offers)
