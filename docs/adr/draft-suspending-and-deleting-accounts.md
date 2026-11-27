# ADR-DRAFT: Administrators suspend accounts and delete them, leaving the account's ID as deleted

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-27
- **Deciders:** Project initiators. Request of 2026-11-26: administrators can neither delete nor suspend accounts; build it. Asked what deleting means and what happens to an account's roles, the requester chose: suspending is reversible, deleting is final and leaves the ID as a tombstone, and roles, memberships and system roles are taken back automatically.
- **Open question:** The accounts journal is append-only, so the records written before a deletion still hold the account's username, mail address, profile picture and password hash on disk and in backups. Should a deletion also remove them — by compacting `accounts.wal`, or by sealing personal data with a key per account that a deletion destroys (as ADR-0026 does for items) — and how do backups taken before it follow?
- **Question checked:** 2026-11

## Context and problem statement

ADR-0027 left "no account deactivation or deletion" open, and named deactivating and erasing accounts as a follow-up.
An administrator who must stop a person from signing in — someone who left, a compromised account, a mistake — has no way to do it: removing roles leaves sessions, API tokens and approvals of assistants working, and nothing frees an account that should not exist.

An account's ID is the principal of the item store (ADR-0022): it is the actor of events, the author of comments, a value of user fields, a reader of protected passages, a member of groups and a holder of roles.
The account itself is kept in `accounts.wal`, event-sourced, apart from the item store.

## Decision drivers

- A person can be shut out at once, by every way in: password, single sign-on, API tokens, OAuth tokens, password resets, sessions already open.
- Shutting out is reversible; deleting is final and says so.
- Identities are permanent (D7, ADR-0013): an ID is never reused, and what refers to it keeps resolving.
- Nothing grants access through an ID no account answers to (ADR-0255 to ADR-0257).
- The installation keeps an administrator who can sign in (ADR-0257).
- Replay of both journals gives the same state as the live commands (ADR-0008).

## Considered options

1. **Suspend, and delete as a tombstone in the accounts:** a reversible suspension; a deletion that removes the account from the accounts' state but for its ID, takes back its roles in the item store and ends what it keeps elsewhere.
2. **Suspend only**, and leave deleting for later.
3. **Delete by removing the account's data from disk as well**, compacting the accounts journal or shredding a key per account.

## Decision outcome

Chosen option: **1**, the requester's choice.

**Suspending** (`PUT /api/users/{id}/suspended`, `{"suspended": true|false}`, administrators):

- The accounts journal records `AccountSuspended` and `AccountReactivated`.
- A suspension ends the account's sessions and pending reset; while it lasts, sessions, API tokens and OAuth access tokens do not resolve, a refresh or a new approval of an assistant is refused, a login with the right password answers 403 *this account is suspended*, a wrong password answers as before, single sign-on returns to the sign-in page with `login_error=suspended`, and no reset link is made or used.
- The account keeps its tokens, roles, memberships, identities and settings; reactivating it opens them again.
- The administrators' list of accounts marks it `suspended`.

**Deleting** (`DELETE /api/users/{id}`, administrators):

- First, the item store takes back in one batch every room role, group membership and system role granted to the ID (`Store.RevokeAll`; events `RoleRevoked`, `GroupMemberRemoved`, `SystemRoleRevoked`, no new kind and no codec change).
- Then the account's connections to process engines end and its calendars and accounts of other services are removed, their keys destroyed (ADR-0151, ADR-0174).
- Last, the accounts journal records `AccountDeleted`: the account leaves the accounts' state with its username, password hash, identities, API tokens, OAuth approvals and codes, reset, sessions, saved calendar views and their feeds, contacts, profile picture, language, favourites, daily room and display name.
  Its ID stays in a set of deleted IDs and is never issued again (IDs are random); its username is free for a new account with a new ID.
- Each step can be repeated, so a request that failed half-way is completed by asking again.
- The ID is no account any more: no role, membership, system role or reader goes to it (ADR-0255, ADR-0256), and where it is still stored — restrictions, readers of protected passages, user fields, history — it is marked as reaching nobody as before.
- `GET /api/users/names` names deleted IDs in `deleted`, and the interface shows them as *Deleted account*.
- A later sign-in through single sign-on with the deleted account's identity finds no account: where first sign-ins create accounts (ADR-0229), it makes a new one without roles.

**Both:**

- Only administrators decide; nobody suspends or deletes their own account, nor the holder of `admin` when no other holder of `admin` is an account that is not suspended.
  The same rule now guards taking back the role `admin` (ADR-0257): a suspended administrator does not count.
- The checks run under the item store's lock, so two administrators cannot shut each other out at once.
- The audit trail records `user.suspend`, `user.reactivate` and `user.delete` (with the username), and a login refused for suspension as `login.failed` with the reason.
- The MCP adapter does not offer these routes (ADR-0012).
- Under *Administration → Users and system roles* each account but one's own offers *Suspend* or *Reactivate*, and *Delete*, which asks for the username to be typed again.

### Consequences

- **Positive:** an account is shut out by every way in with one action, and reversibly.
- **Positive:** a deleted account leaves no credential, role or personal setting in the running server, and what it wrote keeps naming it, as a deleted account.
- **Negative / trade-offs accepted:** what earlier records of `accounts.wal` and backups hold stays on disk until they are replaced (the open question).
- **Negative / trade-offs accepted:** a deletion spans two journals and two other stores without one transaction; a failure between the steps leaves the account with its roles taken back, which asking again completes.
- **Negative / trade-offs accepted:** tokens of a suspended account stay valid when it is reactivated; an administrator who suspects a leak deletes the account or the person revokes them.
- **Follow-ups / risks to watch:** the start that repairs a missing administrator (ADR-0258) still counts a suspended holder of `admin` as an administrator; restrictions and readers that name a deleted account are marked but not removed.

## Pros and cons of the options

### Option 2: suspend only

Strongest form: suspension covers the urgent need — shutting a person out — completely and reversibly; deleting raises the question of personal data on disk that this record cannot answer, and doing half of it may suggest an erasure that did not happen.

- Good: smaller; nothing irreversible; no claim about erasure.
- Bad: a username cannot be freed, and an account that should not exist stays listed with its credentials and settings.
- Bad: a person who asks for their account to be removed cannot be served at all.

### Option 3: remove the data from disk as well

Strongest form: only this is a deletion in the sense of data protection law; a deletion that leaves the password hash and the mail address in the journal is a suspension with a different name.

- Good: the account's personal data leaves the data directory.
- Bad: compacting `accounts.wal` rewrites a journal that backups and incremental backups rely on (ADR-0034, ADR-0073); a key per account changes the journal's format and every reader of it.
- Bad: backups taken before keep the data anyway, so the guarantee is weaker than it sounds unless backups are handled too.
- Bad: much larger; it deserves a decision of its own, which this record leaves open.

## Links

- answers the follow-up of ADR-0027 (local accounts): deactivating and erasing accounts
- amends ADR-0257 (the last administrator): a suspended account does not count
- relates to ADR-0013 (identities are permanent), ADR-0022 (principals), ADR-0026 (erasure of items), ADR-0029 (audit trail), ADR-0151 (engine connections), ADR-0174 (calendars of other services), ADR-0229 (sign-in links), ADR-0255 and ADR-0256 (principals that exist), ADR-0258 (the start)
