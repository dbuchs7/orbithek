# ADR-0175: A collection in the album gathers the labelled pictures of every room

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 for a sub-album that, steered by labels, gathers every drawing and picture of the instance; the defaults below, proposed with the implementation, were confirmed by the requester; accepted 2026-11-10)
- **Open question:** whether a collection should also gather by kind (only drawings), by room or by a free query (ADR-0136) rather than by labels alone, and whether its rule should be offered to agents as an MCP tool; all are left out here
- **Question checked:** 2026-11

## Context and problem statement

The album (ADR-0054) lists the pictures of every item one may read. Folders (ADR-0090) hold the pictures uploaded into them, and a picture carries labels of its own that narrow the album. A filter by label is set by hand each time and is not kept; a folder holds only its own files, so a picture that belongs to a page in another room cannot be in it without a copy.

People want a sub-album that collects, by labels, every drawing and picture of the instance in one place, keeps doing so as pictures are labelled, and is found where folders are. How is such a collection kept, and what does it show, without a second model of access and without copying pictures?

## Decision drivers

- **One model (D1, D4):** whatever is kept and queried is an item with properties; a rule that decides what a collection shows is a property, not text in a body.
- **Validated changes (D3):** the rule is written through the type's fields.
- **Unreadable is non-existent (D10):** a collection shows each reader only the pictures they may read; it grants nothing, and two readers of one collection may see different pictures.
- **No copies:** a picture stays a file of its own item (ADR-0045); its versions stay one stack (ADR-0103).
- **No change to the persisted format** (ADR-0025).
- Found and used like a folder.

## Considered options

1. **A collection is an item of the type `collection` placed like a folder; its rule (labels, any or all, and whether the labels of a picture's item count) is three fields; the album gathers at read time.**
2. **A saved filter of the album per person**, kept in the accounts journal as saved calendar views are (ADR-0168).
3. **A folder whose own item labels are its rule.**
4. **A collection that copies matching pictures into itself** when they are labelled.

## Decision outcome

Chosen option: **1**.

### The collection

- **Type:** an item of the type `collection`, without a body or workflow, with three required fields:
  - `rule` (text): the labels, normalised as labels are (ADR-0044) and separated by commas;
  - `match` (enum `any`, `all`): a picture carries any one of the labels, or every one;
  - `source` (enum `pictures`, `pictures and items`): only a picture's own labels count (ADR-0090), or the labels of its item too.

  The album defines the type when its first collection is made. If a type of that name exists in another shape, the album refuses to use it, as for folders.
- **Placement:** in a room, or in a folder of the album, by whoever may create there; not in another collection. Its access, history, trash and erasure are those of any item of the room.
- **Rule:** at least one and at most 20 labels, the bound of a picture's labels. A rule typed on the collection's own page is read leniently: what is not a label is passed over, and a rule without labels gathers nothing.

### What it shows

- The pictures of **every room** whose labels match the rule, read with the reader's rights. Each reader sees only pictures of items they may read.
- A stack of versions (ADR-0103) shows when any of its versions matches, with its newest version on top, as for every filter.
- The album's filters (text, kind, label, room) narrow what a collection gathers.
- A collection holds no pictures of its own: nothing is uploaded into it, and files attached to it on its page show in it only when its rule matches them.

### API

- `GET /api/album?collection=` lists what a collection gathers.
- `GET /api/album/folders` adds `collections`, each with its `rule` and whether the reader may change it (`editable`).
- `POST /api/album/collections` makes one: name, space or parent, labels, match and items.
- `PUT /api/album/collections/{ref}` changes the rule and answers with the rule as kept.

### The album

- *New collection* beside *New folder*: room (at the top), name, labels, *Any of the labels* or *All of the labels*, and *Labels of the page or task count too*.
- A collection is listed after the folders with a mark of its own and its labels. Opened, it says in one sentence which pictures it gathers; one who may edit it changes its rule there.

### Consequences

- **Positive:** pictures of every room are found in one place by their labels, and the collection stays current without anyone filing pictures.
- **Positive:** nothing new is persisted; codec, encryption, backup and erasure are untouched.
- **Negative / trade-offs accepted:** collections are items, so they show in the room's page tree, in search and in the type menu, as folders do (ADR-0090).
- **Negative / trade-offs accepted:** a collection in a room shows pictures of other rooms. A reader of the room sees only what they may read elsewhere, but the collection's name and labels are visible to every reader of its room.
- **Negative / trade-offs accepted:** each request scans every item in memory, like the album and search (ADR-0033).
- **Negative / trade-offs accepted:** a collection is renamed, moved and deleted on its page, as a folder is.

## Pros and cons of the options

### Option 2: a saved filter per person

Strongest form: no item and no type; nothing appears in a room's page tree; it costs a few bytes in the accounts journal and follows a pattern already in place (ADR-0168).

- Bad: a sub-album that only its maker sees cannot be shared with a team, and it is not found where folders are. A collection that a room's readers open together is what was asked for.

### Option 3: a folder whose item labels are its rule

Strongest form: no new type; a folder both holds uploads and gathers.

- Bad: existing folders that carry labels would begin to gather pictures without anyone asking. Item labels reach kanbans and label boards (ADR-0060, ADR-0087), so the folder would appear there under its rule's labels. "Any or all" and whether item labels count have no place to be kept.

### Option 4: copies into the collection

Strongest form: the collection is an ordinary folder afterwards; it works offline from the rule, and each copy takes on the collection's access.

- Bad: copies diverge from their originals (ADR-0054), every label set writes files across items, a picture from a restricted item could be copied into an open room by the act of labelling, and storage grows with each collection.

## Links

- amends [ADR-0090](0090-album-folders-uploads-and-picture-labels.md) (collections beside folders)
- amended by [ADR-0187](0187-pictures-are-added-where-one-is-in-the-album.md) (a collection is filled by giving its labels)
- amended by [ADR-0193](0193-a-collection-is-deleted-from-the-album.md) (a collection deleted from the album)
- relates to ADR-0054 (the album), ADR-0103 (stacks of versions), ADR-0044 (labels), ADR-0022 (permissions), ADR-0033 (search in memory), ADR-0136 (query language)
