# ADR-DRAFT: Readers of protected passages, members of groups and holders of system roles exist

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26, after ADR-0255: check protected passages for the same gap; the requester then asked for all four proposed corrections — readers checked, group members and system roles checked, unknown readers marked, the test that relied on the gap adjusted.

## Context and problem statement

ADR-0255 refuses a room role for a principal that reaches nobody and marks such grants.
The same gap remained in three other places that take principals:

- **Readers of protected passages** (ADR-0088, ADR-0160): `POST /api/items/{ref}/secrets` and `PUT /api/items/{ref}/secrets/{id}` checked readers only for their form (`checkReaders`: not empty, no spaces, at most the limit).
  `%40ops`, `0123456789abcdef` and `@nobody` were stored as readers; the passage was closed to them without anyone noticing.
  The interface's picker offers only people and groups its search finds, so the gap is open to callers of the API, such as scripts with tokens.
- **Members of groups** (ADR-0048): `PUT /api/groups/{group}/members/{principal}` took any string.
  A member no account answers to makes a group count as having members, so the check of ADR-0255 (a group exists when it has members) accepted a group that reaches nobody.
- **Holders of system roles** (ADR-0048): `PUT /api/system-roles/{role}/{principal}` took any string.

## Decision drivers

- A principal that reaches nobody must not look like one that works (ADR-0255).
- One rule for every place that names people and groups: restrictions (ADR-0078), room roles (ADR-0255), readers, members, system roles.
- Fail closed; never widen access.
- Recovery replays every event as recorded; nothing stored before the check may stop loading (ADR-0008).
- Whoever may not make a change learns nothing about which accounts and groups exist.

## Considered options

1. **Check at the API, mark what is stored**, with the function ADR-0255 introduced; a group exists when one of its members has an account.
2. **Check in the item model** (`checkReaders`, `AddToGroup`, `GrantSystemRole`).
3. **Check readers only**, leaving members and system roles as they were.

## Decision outcome

Chosen option: **1**.

- **Readers:** protecting a passage or blocks and changing a passage's readers refuse a reader that is neither the ID of an account nor a group as below (400, `there is no user …` / `there is no group …`).
  The check runs only for whoever may protect (edits the item) or decides who reads the passage; everyone else is refused by the item model as before and learns nothing about who exists.
- **Groups:** a group exists when at least one of its members is the ID of an account.
  This now holds for restrictions, room roles and readers alike, so members stored before this check cannot make an empty group pass.
- **Members and system roles:** adding a member to a group and granting a system role take only the ID of an account; a group cannot be a member.
  Callers who are not administrators are refused as before.
- **Marking:** a protected passage, as returned by `GET /api/items/{ref}` and by every change of the item, lists in `unknownReaders` its readers that reach nobody, for those who see its readers.
  The item's list of protections names them as stored with *reaches nobody*; **Change readers** marks them, and saving them again is refused until they are taken out.
- **Unchanged:** the event log, the codec, replay, restore from a backup and copying an item; the item model's form check of readers stays as it is.

### Consequences

- **Positive:** a mistyped or mangled reader, member or holder is refused at once with the reason.
- **Positive:** one rule for every principal; a group of stored members without accounts no longer counts.
- **Negative / trade-offs accepted:** a passage can no longer be protected for a group before it has members; the group gets its first member first.
  A test that protected a passage for a group without members is adjusted.
- **Negative / trade-offs accepted:** saving the readers of a passage that holds an unknown reader is refused until it is taken out; the mark says which.
- **Follow-ups / risks to watch:** the administration's list of groups and of system roles does not mark members or holders without an account; restrictions stored before ADR-0078, and groups that empty after a restriction, are not marked either.

## Pros and cons of the options

### Option 2: check in the item model

Strongest form: every path that writes readers, members or holders — API, import, restore, future callers — would be covered by one check, and no handler could forget it.

- Good: one place for each command.
- Bad: the item model does not know accounts; they live in the accounts journal (ADR-0027), and passing them in couples the store to authentication.
- Bad: restore must bring back what was stored, and the import grants to groups before they have members (ADR-0023).
- Bad: a check when events are applied would stop recovery from loading what was stored before it.

### Option 3: check readers only

Strongest form: readers are what was found; members and system roles are set by administrators from lists of existing accounts, and leaving them alone keeps the change small.

- Good: smaller change.
- Bad: a member without an account keeps making an empty group pass every check that relies on members, so the readers' check would still accept a group that reaches nobody.
- Bad: two rules for the same kind of name.

## Links

- amends ADR-0088 (protected passages): readers exist
- amends ADR-0048 (roles are listed and taken back): members and holders of system roles are accounts
- amends ADR-0255 (a role goes to someone who exists): a group exists when a member has an account
- relates to ADR-0078 (restrictions for people and groups that exist), ADR-0160 (protected blocks), ADR-0008 (event log), ADR-0027 (accounts)
