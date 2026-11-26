# ADR-0258: The start warns when no administrator has an account, and repairs only for the account the operator names

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26: carry out the follow-up ADR-0257 names, the start that creates the first administrator. Asked how the start should react when admin is held only by IDs without an account, the requester chose: warn, repair nothing by itself, and give the role only when the operator names an account with `OT_ADMIN_USERNAME`.

## Context and problem statement

The start makes sure the installation has an administrator (ADR-0013, `EnsureAdmin`).
If anyone holds the system role admin, it does nothing; if nobody does, it creates the account `OT_ADMIN_USERNAME` (default `admin`) on a first start, or gives the role to that existing account after a crash between the two steps.

Since ADR-0256 a system role goes only to an account, and since ADR-0257 the last administrator with an account cannot give the role up.
A role of admin stored before may still be held by IDs no account has.
If those are its only holders, nobody can sign in to administer the installation, and the start counted them as administrators and changed nothing, saying nothing.

## Decision drivers

- An installation without anyone who can administer it must be told, not hidden.
- No account gets the most powerful role without an operator's decision.
- An update must not stop a running installation from starting.
- Nothing changes in the log or on replay (ADR-0008).

## Considered options

1. **Warn; give the role only to the account `OT_ADMIN_USERNAME` names, when it is set.**
2. **Repair as for a missing administrator:** give the role to the account `OT_ADMIN_USERNAME` names, `admin` by default, and stop the start when there is no such account.
3. **Repair, else warn:** give the role to that account, `admin` by default, and start with a warning when there is none.

## Decision outcome

Chosen option: **1**, the requester's choice.

- `EnsureAdmin` counts only holders of admin that are accounts.
  When admin is held, but by none of them, it changes nothing and names the holders in `Bootstrap.WithoutAccount`.
- When `OT_ADMIN_USERNAME` is not set, the start logs a warning naming those holders and how to name an account.
- When it is set, the start gives that account the role through `RestoreAdmin`, recorded in the log and the audit trail as the server's own change (`system`), and logs it as a warning naming the account and the holders.
  When there is no such account it logs a warning and starts.
- Unchanged: the first start of an empty installation, the repair after a crash between creating the account and granting its role, the log and replay.
  `Store.SystemRoleHeld`, used only here, gives way to `Store.SystemRoleHolders`.

### Consequences

- **Positive:** an installation without an administrator who can sign in says so at every start.
- **Positive:** no account named `admin` by chance is raised to administrator by an update.
- **Positive:** the server starts in every case; nothing about the start can lock its users out.
- **Negative / trade-offs accepted:** where `OT_ADMIN_USERNAME` stays set from the first installation, as in many container setups, the repair happens at the next start without a further step; the account is still the one the operator chose as administrator.
- **Negative / trade-offs accepted:** the holders without an account keep the role until an administrator takes it back (ADR-0257).

## Pros and cons of the options

### Option 2: repair, else stop the start

Strongest form: it is exactly what the start does when nobody holds admin, so there is one rule; and an installation nobody can administer is broken, which an operator should notice before users do.

- Good: one rule; the state cannot persist unnoticed.
- Bad: after an update, an installation that served its users stops starting until an operator acts.
- Bad: by default it raises any existing account named `admin`, which may belong to someone who was never meant to administer.

### Option 3: repair, else warn

Strongest form: the installation is repaired without an operator in most cases, and never stops starting.

- Good: no outage; often no manual step.
- Bad: raises an account named `admin` by default without anyone choosing it, silently except for a log line.

## Links

- amends ADR-0257 (the administration marks principals without an account): its follow-up on the start
- amends ADR-0013 (authentication and the first administrator): the start counts administrators with an account
- relates to ADR-0256 (readers and members exist), ADR-0029 (audit trail), ADR-0008 (event log)
