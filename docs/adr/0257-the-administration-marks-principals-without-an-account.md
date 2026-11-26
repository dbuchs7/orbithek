# ADR-0257: The administration marks group members and system role holders without an account

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26: carry out the follow-up ADR-0256 names, marking members and holders without an account in the administration's lists.

## Context and problem statement

Since ADR-0256, group members and system role holders are accounts.
What was stored before stays: a group member or a system role holder may be an ID no account has, from a script, a typing error or a proxy that encoded a path twice (ADR-0255).

Such entries were invisible or looked valid:

- **Groups** listed every member through the name lookup, which shows an unknown ID as it is, without saying it is no account.
  A group of none but such members looked filled although it reaches nobody (ADR-0256 already stops it counting as a group).
- **System roles** were shown only as checkboxes beside the accounts; a holder without an account appeared nowhere and could not be taken back from the interface.
- **The last administrator** check counted every holder of `admin`: a holder without an account let the last administrator with an account give the role up, leaving the installation without anyone to administer it.

## Decision drivers

- What reaches nobody is shown as such (ADR-0255, ADR-0256).
- What is shown as wrong can be taken out where it is shown.
- The installation always keeps an administrator who can sign in.
- Nothing changes in the log or on replay (ADR-0008).

## Considered options

1. **Mark in the listings and the interface; guard the last administrator at the API.**
2. **Remove such entries automatically**, at start or on a schedule.
3. **Mark only**, leaving the last administrator check as it is.

## Decision outcome

Chosen option: **1**.

- `GET /api/groups` names, per group, its members without an account in `unknown`, and says `reachesNobody` when no member has one.
  The administration shows such a member with its ID and the badge *No account*, and such a group with *Reaches nobody*; × takes the member out as before.
- `GET /api/system-roles` marks a holder without an account with `unknown`.
  The administration lists them under *System roles without an account*, with a button per role that takes it back.
- `DELETE /api/system-roles/admin/{principal}` refuses to take the role from an account when no other holder of `admin` has an account (400, `… is the last administrator`).
  Taking it from a holder without an account is allowed whenever the item model allows it.
- Unchanged: the event log, replay, the item model's own check of the last administrator, and the start that creates the first administrator.

### Consequences

- **Positive:** members and holders stored before ADR-0256 are visible and removable from the administration.
- **Positive:** a holder of `admin` without an account no longer lets the last real administrator give the role up.
- **Negative / trade-offs accepted:** nothing is cleaned up by itself; an administrator takes each entry out.
- **Follow-ups / risks to watch:** the start that creates the first administrator (ADR-0013) still treats any holder of `admin` as an administrator, so an installation whose only holders of `admin` have no account is not repaired by a restart.

## Pros and cons of the options

### Option 2: remove such entries automatically

Strongest form: an entry no account answers to does nothing, and removing it at start leaves no dead data and nothing for an administrator to do.

- Good: no manual work.
- Bad: writes to the log without a person's decision, which every audit then has to explain.
- Bad: an account restored from a backup of the accounts journal later would find its memberships and roles gone.

### Option 3: mark only

Strongest form: the last administrator check belongs to the item model, which does not know accounts; marking makes a holder without an account visible, and an administrator can see the risk.

- Good: smaller change; the rule stays in one place.
- Bad: the risk is to lock everyone out, which seeing a badge does not prevent; the check is cheap where accounts are known.

## Links

- amends ADR-0256 (readers and members exist): its follow-up on the administration's lists
- amends ADR-0048 (roles are listed and taken back): the last administrator is one with an account
- relates to ADR-0255 (a role goes to someone who exists), ADR-0013 (the first administrator), ADR-0008 (event log), ADR-0027 (accounts)
