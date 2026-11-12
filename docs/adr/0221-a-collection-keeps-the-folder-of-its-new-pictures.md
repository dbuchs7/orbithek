# ADR-0221: A collection keeps the folder of its new pictures, for everyone

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 to keep the target folder per collection for everyone, settling the first open question of ADR-0187)
- **Open question:** whether uploads in a collection with a kept folder should skip the question; whether someone who may not edit the collection should be able to keep a folder for it
- **Question checked:** 2026-11

## Context and problem statement

Pictures uploaded in a collection are kept in a folder chosen each time (ADR-0187). The album proposed the folder chosen last, remembered in each browser only. When several people fill one collection, each proposes a different folder, and the files of a collection end up scattered.

Where does a collection's target folder live, who sets it, and what does a reader see who may not read that folder?

## Decision drivers

- **One model (D1, D4):** what a collection keeps is a property of its item, written through its type's fields (D3).
- **Unreadable is non-existent (D10):** a folder a reader may not read must not be proposed to them, nor named.
- **No change to the persisted format** (ADR-0025): no new event kind, and the golden logs stay as they are.
- **Asking stays:** the requester chose in ADR-0187 to be asked each time; the kept folder is the proposal.

## Considered options

1. **An optional item field `folder` of the collection's type, referring to a folder; set by whoever may edit the collection, when they upload into it.**
2. A relation from the collection to the folder.
3. A setting of the album per collection, outside the item.

## Decision outcome

Chosen option: **1**.

### The field

- The collection's type gains an optional field `folder` of the kind `item`, limited to folders (ADR-0202). It is added as the type's next version the first time a collection keeps a folder, so a type defined before, and the golden logs, need nothing.
- A type with the field is still the album's collection; a field of that name of another kind or target is not.

### Setting it

- `PUT /api/album/collections/{ref}/folder` with `folder`, a folder's ID, keeps it; an empty `folder` forgets it. Whoever may edit the collection may, naming a live folder they may read.
- The album sets it when someone who may edit the collection uploads into it and chooses a folder other than the one kept. The choice says so in one sentence.
- Someone who may not edit the collection is still asked; their choice is not kept for the others, only remembered in their browser.

### Reading it

- `GET /api/album/folders` gives each collection its `folder`, empty when none is kept or when the reader may not read it.
- The folder choice proposes the collection's folder first, then the folder chosen last in this browser, then the first folder one may add to.

### Consequences

- **Positive:** everyone who fills a collection is proposed the same folder; the files of a collection stay together.
- **Positive:** the folder is a property: it has the item's history and access, and it reaches no new event kind.
- **Negative / trade-offs accepted:** the collection's type has one more version once a folder is first kept, and the field shows on the collection's own page.
- **Negative / trade-offs accepted:** a collection kept with a folder of another room proposes nothing to readers of only its own room.
- **Follow-ups / risks to watch:** the open questions above; skipping the question is left out, as the requester chose to be asked (ADR-0187).

## Pros and cons of the options

### Option 1 — an item field (chosen)

Strongest form: validated by the type (D3), checked to name a readable live folder, kept in history, and invisible to readers who may not read the folder.

- Good: no new persistence; works across rooms.
- Bad: a new type version once.

### Option 2 — a relation

Strongest form: relations already link items, and show in impact and starmaps.

- Bad: a relation is many to many and has no "the one folder"; a second relation would need resolving.

### Option 3 — an album setting outside the item

Strongest form: no type change.

- Bad: a second store of what an item holds, with access of its own (against D1, D4).

## Links

- amends [ADR-0187](0187-pictures-are-added-where-one-is-in-the-album.md) (its first open question)
- relates to ADR-0175 (collections), ADR-0202 (item fields), ADR-0090 (folders)
