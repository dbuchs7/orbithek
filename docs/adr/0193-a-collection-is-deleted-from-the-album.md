# ADR-0193: A collection is deleted from the album, its pictures staying where they are

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: the open question of ADR-0190, whether collections are deleted from the album the same way as folders, is to be built. That it moves the collection alone to the trash is a default proposed with the implementation and awaits the requester's confirmation.
- **Open question:** whether the album should list what is in the trash
- **Question checked:** 2026-11

## Context and problem statement

ADR-0190 lets an open folder be deleted from the album, into the trash with what it holds. A collection (ADR-0175) is an item too, but holds nothing: it gathers, by labels, pictures that are files of other items. It was deleted only on its page.

How is a collection deleted from the album without touching the pictures it shows?

## Decision drivers

- **One way of deleting** (D7, D8, ADR-0021): a collection is an item; its deletion is the item's move to the trash, restorable.
- **No copies, no side effects** (ADR-0175): the pictures a collection shows belong to other items and keep their labels.
- **Same place as folders** (ADR-0190).

## Considered options

1. **"Delete collection" in an open collection moves the collection alone to the trash.**
2. **Deleting a collection also deletes the pictures it shows.**
3. **Collections stay deleted on their page only.**

## Decision outcome

Chosen option: **1**.

- An open collection offers **Delete collection** to whoever may edit it.
- The album asks to confirm, saying that its pictures and videos stay where they are, with their labels.
- It then calls the item's move to the trash (`POST /api/items/{ref}/trash`). The server decides as for any item.
- The album returns to the folder above and links the collection's page, where **Restore** brings it back with its rule.
- No route, event or format is new.

### Consequences

- **Positive:** collections are made, changed and deleted where they are used.
- **Positive:** nothing a collection shows is lost with it.
- **Negative / trade-offs accepted:** the labels that made a picture part of the collection stay on the picture; whoever wants them gone removes them.

## Pros and cons of the options

### Option 2: with the pictures

Strongest form: a collection made to gather pictures for a purpose can be cleared away with them in one step.

- Bad: the pictures belong to other items, rooms and people (ADR-0175); one person's collection would delete another's photos.

### Option 3: on the page only

Strongest form: no second place to delete an item.

- Bad: it is the step ADR-0190 removed for folders; the album is where collections are made and used.

## Links

- amends [ADR-0175](0175-a-collection-in-the-album-gathers-labelled-pictures-of-every-room.md) (a collection is deleted on its page)
- relates to ADR-0021 (deletion), ADR-0190 (a folder deleted from the album)
