# ADR-0261: A query field for who made or changed an item

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26: the start page's list *Recently created or changed* gets a filter *By me*. Asked what *by me* means, the requester chose the items one created or changed at some point (option 1 below), over the items one created (option 2) and the items one changed last (option 3), and a switch *All* | *By me* whose choice the browser keeps. Built in #407 and accepted by the requester on 2026-11-27.
- **Open question:** Should the list *By me* be ordered by one's own last change rather than by anyone's, and should `contributor` be offered as an order (ORDER BY), for which an item has many values?
- **Question checked:** 2026-11

## Context and problem statement

The query language (ADR-0136) knows who created an item (`creator`) and when it last changed (`updated`, ADR-0203), but not who changed it.
The start page lists what was created or changed last in one's rooms by `ORDER BY updated DESC`; a reader who wants their own work back among everyone's has no condition to ask for it.
Which field should a query read to find the items a person made or changed, and what counts as a change?

## Decision drivers

- One query language for all content (ADR-0001, ADR-0136): the start page, views, the query view and agents over MCP ask in the same words.
- The same notion of a change as `updated`, so that an item listed by its last change and an item listed as changed by someone agree.
- D10: nothing a caller receives reveals an item they may not read, nor which accounts exist.
- No new persisted data and no change to the event format: the event log already records the actor of every event.

## Considered options

1. A built-in field `contributor` (also `contributors`): every account that created the item or made an event that changed the item itself, as `updated` counts changes.
2. Only the existing `creator`, used by the start page as *By me*.
3. A built-in field for the account of the last change only (for example `updater`).

## Decision outcome

Chosen option: **1, a built-in field `contributor`**, because it answers the question the requester asked — what did I create or work on — in the query language every view and agent already uses, from data the event log already holds.

- `contributor` holds the creator and the actor of every event in `changesItem`: content, state, labels, place, comments and files; relations, access, holds and watches are no change of the item, as for `updated`.
- It takes `=` and `IN`, and so `!=` and `NOT IN`; a value is a username, a user ID or `currentUser()`, as in `creator` and user fields; an unknown name stands for nobody, without an error.
- It is not an order: `ORDER BY contributor` is refused, as an item has many contributors.
- The start page's list *Recently created or changed* asks for `contributor = currentUser() ORDER BY updated DESC` under *By me*; the choice *All* | *By me* is each reader's own, kept in the browser as the layout of the rooms is.

### Consequences

- **Positive:** "my work" is one condition, in every place a query is written; agents learn it from the MCP description of the query language.
- **Positive:** nothing is stored anew: the field reads the event log the store keeps in memory, through the caller's session, so only readable items are candidates (D10).
- **Negative / trade-offs accepted:** a condition on `contributor` walks the events that touch each candidate item; on a large installation that is more work per item than `creator`. The listing's budget (ADR-0015) still bounds what a page answers.
- **Negative / trade-offs accepted:** a built-in field shadows a property of the same name unless the name is quoted (ADR-0136); an installation with a property called `contributor` reaches it as `"contributor"`.
- **Negative / trade-offs accepted:** *By me* orders by anyone's last change, so an item one worked on long ago comes first when someone else changed it today.
- **Follow-ups / risks to watch:** measure a query on `contributor` in the scale test (ADR-0081) before it is used in a view many people open at once.

## Pros and cons of the options

### Option 1: `contributor`, everyone who created or changed the item

Strongest form: the set of people who worked on an item is what "mine" means for pages that several people write; it uses exactly the events `updated` already counts, so the two fields never disagree, and it needs no new data.

- Good: finds the pages one edited but did not start, which is most of the work in a shared wiki.
- Good: one notion of a change across `updated` and `contributor`.
- Bad: costs a walk over an item's events per condition.
- Bad: one more built-in name that shadows a property.

### Option 2: only `creator`

Strongest form: it exists, it is cheap (one event per item), and for tasks and decisions the creator is usually the person who cares; the start page would need no server change at all.

- Good: no change to the query language, no ADR beyond this one.
- Bad: misses every page one edited but someone else created, which the requester named as the point of the filter.

### Option 3: the account of the last change

Strongest form: it is as cheap as `updated` (the last change is found already) and answers "where did I leave off", which matches a list ordered by the last change.

- Good: cheap and consistent with the list's order.
- Bad: an item drops out of *By me* as soon as anyone else touches it, so one's own work disappears in a busy room.

## Links

- extends [ADR-0136](0136-a-query-language-selects-items-by-their-fields.md) (the query language and its built-in fields)
- relates to [ADR-0203](0203-views-of-the-items-of-a-type-as-a-table-a-gallery-or-a-list.md) (the field `updated` and what counts as a change), [ADR-0022](0022-permission-model.md) (D10), [ADR-0015](0015-resource-budgets-and-paged-listings.md) (budgets), [ADR-0081](0081-the-server-stays-fast-with-tens-of-thousands-of-items.md) (scale test), [ADR-0012](0012-mcp-adapter-over-http-api.md) (MCP)
