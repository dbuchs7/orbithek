# ADR-0255: A role goes to someone who exists, and a grant that reaches nobody is shown as such

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26: a room's *Members and roles* listed `%40MitarbeiterBIT` for a group; the requester asked that the server refuse such grants and that the page mark grants that reach nobody.

## Context and problem statement

A room role is granted to a principal named in the path, `PUT /api/spaces/{space}/roles/{principal}` (ADR-0048): a user ID, or a group as `@name`.
The server took any string.
The permission check treats a principal that starts with `@` as a group and compares everything else with the caller's user ID (ADR-0022).

A grant to a name that is neither is therefore stored, listed and audited like any other, and opens the room to nobody.
One such grant was seen: `%40MitarbeiterBIT`, the `@` of `@MitarbeiterBIT` still percent-encoded when it reached the handler.
The browser encodes the name once and Go's router decodes it once, which a test shows to work; the second encoding presumably happened on the way, in a reverse proxy in front of that installation (an assumption, not verified).
Whatever its cause, the members of the group believed they had access and did not, and the page showed nothing wrong.

Restrictions of items already refuse people and groups that do not exist (ADR-0078).
Room roles had no such check.

## Decision drivers

- A grant that reaches nobody must not look like one that works.
- Fail closed: no change may widen access.
- Recovery replays every event as recorded; grants stored before a check must still load (ADR-0008).
- Whoever may not grant roles in a room learns nothing about which accounts and groups exist.

## Considered options

1. **Refuse at the API, mark on the page.** The handler accepts only an account's ID or a group with members, as for restrictions; the listing marks stored grants that reach nobody.
2. **Repair the name.** Decode the principal again when it starts with `%40`, so a doubly encoded `@` becomes a group.
3. **Refuse in the store.** `Store.GrantRole` validates the principal, so every path into the log is covered.
4. **Leave the server as it is** and fix the proxy where it happens.

## Decision outcome

Chosen option: **1**.

- `PUT /api/spaces/{space}/roles/{principal}` checks, when the caller administers the room, that the principal exists: a name without `@` must be the ID of an account, a name with `@` a group with at least one member.
  Anything else is refused with 400 and says which: `there is no user …` or `there is no group …`.
  Callers who do not administer the room are refused as before, whatever they name, and learn nothing about who exists.
- The check is the one restrictions use (ADR-0078), now one function for both.
- `GET /api/spaces/{space}/roles` marks every grant that reaches nobody with `"unknown": true`: no account has the ID, or the group has no members now.
- The page *Members and roles* shows such a grant with its stored name and a badge, *Unknown* or *No members*, with a hint; its role cannot be changed there, only taken back.
- `DELETE` takes back any stored grant, known or not, as before.
- Nothing changes in the event log, the codec or replay: existing grants load as they are.

### Consequences

- **Positive:** a misspelt or mangled name is refused at once, with the reason, instead of being stored as a grant to nobody.
- **Positive:** grants made before the check, and grants to groups that later emptied, are visible and can be removed.
- **Negative / trade-offs accepted:** a role can no longer be granted to a group before it has members; the group gets its first member first, as for restrictions.
  The import still grants to groups without members and reports them (ADR-0023); it writes through the trusted store, not this handler.
- **Negative / trade-offs accepted:** a group that loses its last member keeps its grants, marked; they reach its members again once it has some.
- **Follow-ups / risks to watch:** a proxy that encodes paths twice still breaks every name with a reserved character, including taking back such a grant from the page; that is fixed in the proxy's configuration, not here.

## Pros and cons of the options

### Option 2: repair the name

Strongest form: the administrator meant `@MitarbeiterBIT`, and the server could give them exactly that, without an error and without asking anyone to touch a proxy.

- Good: works behind a broken proxy.
- Bad: guesses. A server that decodes twice treats `%2540` differently from every other client and router, and fixes only `@`; any other reserved character stays broken.
- Bad: hides the misconfiguration that will break the next request, such as taking the grant back.

### Option 3: refuse in the store

Strongest form: every path into the log — API, import, restore — would be covered by one check, and no handler could forget it.

- Good: one place.
- Bad: the store does not know accounts; they live in the accounts journal (ADR-0027), so the check needs them passed in.
- Bad: the import deliberately grants to groups that have no members yet and reports them (ADR-0023); restore must bring back what was granted, whatever it was.
- Bad: if the check ran when events are applied, recovery could no longer load grants stored before it.

### Option 4: leave the server as it is

Strongest form: the server did nothing wrong; it stored exactly the name it was given, and the defect is in front of it.

- Good: no change to the server.
- Bad: the next typing error, or the next proxy, produces the same silent grant to nobody, and the page still shows it as working.

## Links

- amends ADR-0048 (roles are listed and taken back): a role is granted only to a principal that exists
- amended by ADR-0256 (a group exists when a member has an account)
- relates to ADR-0022 (permission model), ADR-0078 (restrictions for people and groups that exist), ADR-0023 (import), ADR-0008 (event log)
