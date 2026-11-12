# ADR-0203: Views of the items of a type as a table, a gallery or a list, grouped by a field, also placed in a text

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12: the items of a type are shown as a table with chosen columns, as a gallery of cards and as a compact list, grouped by a field, editable in place, reachable from a room and placeable in a text)
- **Open question:** whether a person may save a view under a name, as calendar views are saved in the accounts journal, rather than keep it as an address or a block; whether exports of a text should list the items a view shows, as they list a task list's tasks, instead of naming the view; whether the browser should read past the boards' limit, or the server group and count, for a type with many thousands of items
- **Question checked:** 2026-11

## Context and problem statement

Items of a type are seen as rows of the room's list, as cards of a board by state (ADR-0036), on a kanban (ADR-0060), on a time axis (ADR-0164) or in a calendar (ADR-0166). What is missing is the plain overview of a catalogue or a backlog: a table whose columns are the fields one cares about, sorted and grouped by a field, a gallery that shows each item by its picture, and a compact list. People also want such an overview inside a text, beside the words that explain it, and want to correct a value where they see it instead of opening every item.

The question is where such a view lives, what it keeps, how its cells change an item, and what a reader sees of items they may not read.

## Decision drivers

- One model for content (D1): a view is not content; the items are.
- Unreadable is non-existent (D10): a link or a text that shows a view must show each reader exactly the items they read, no count of the others.
- Validated changes (D3): a cell edited in place is the same command as the field's input on the item page; a state changes only by a transition.
- Fields of new kinds (item, multi, rating, file, being added by another change) must not break a view: an unknown kind degrades to text.
- The persisted format changes only where something must be kept (ADR-0025).

## Considered options

1. **A view computed in the browser from the item listing**, its state in the address at `#/view`, and a block of the kind `view` in a text that keeps only what the view shows
2. **Views as stored objects** of a room, like kanbans (ADR-0060), with a name, columns and grouping
3. **Views computed by the server**: a route that answers the groups, counts and rows of a view
4. **A view as an item** of a type of its own, whose properties describe it

## Decision outcome

Chosen option: **1**, because it keeps nothing that can go stale or leak, follows what the roadmap and the calendar already do, and needs no new store.

- **The view.** `#/view?layout=&space=&type=&query=&group=&sort=&col=…` shows the readable items of a type (or of every type) in a room (or every room the reader reads), narrowed by a query of the query language (ADR-0136), as a **table**, a **gallery** or a **list**. It reads `GET /api/items` with those parameters, up to the boards' limit, and says when it stopped there. It is reached from a room's views, as **Views** and **View of a type…**, and its address can be shared.
- **Columns** are the key, title, state, labels, room, type, when the item last changed, and every field of the type (`field:<name>`). Without a choice, a view shows the key, title, state, the first four fields, labels and the last change. Listings now carry `updated`, the time of the item's last own change as the query language's field `updated` reads it (`Session.LastChanges`).
- **Grouping** is by the state, by label (an item stands in the group of each of its labels) or by a field of the kinds enum, user and bool. Groups follow the workflow's states and the enum's values in the type's order, then other values, yes before no, and the items without a value last; each header folds and tells its count.
- **Sorting** is by any column, either way, empty values last; a click on a table's heading sorts, a second the other way.
- **The gallery** shows each item's cover: the first picture of its text, or else the file of its first field of the kind `file` (an attachment ID or an object naming one), as the server's thumbnail; then the title and the chosen fields.
- **Editing in place.** Where the reader may edit in the item's room, a cell of the title, of a field of a kind the item page edits, or of the state offers its input: the item page's own input of the field (`propertyInput`), one `PUT …/properties/{field}` per change, or a choice of the transitions that leave the state, `POST …/transitions`. The server decides again on every change.
- **Fields by kind.** Values are written by `fieldText` (`assets/itemviews.js`) and drawn by `renderField` (`assets/itemview.js`): a person by name, yes or no, a date, a point in time where the reader is, a list joined, a rating as stars, an object by its title, name, key or ID, and any other kind as its text. These two functions are the one place to point at a shared renderer of fields once one exists.
- **In a text.** A block of the kind `view` keeps `Layout`, `ItemType`, `Space`, `Query`, `Group`, `Columns` and `Sort`, and no text, mention or reference (D4). The server checks the layout, the room's key, the names, a query that parses, at most `MaxViewColumns` columns each once, and a sort by a column. The block is part of the sealed body (ADR-0026). The editor inserts it with **View of items…**, changes it with **Change the view…**, and every reader's browser computes it from the listing as they read it (D10). Exports name the view (`export.ViewName`) as they name a diagram.
- **Format.** Codec version 56 adds the block kind `view` and its fields; earlier versions are still read (shared with ADR-0204).
- **MCP** (ADR-0012): no tool is added for the view itself; an agent reads the same items with `query_items`, and writes a view block with `commit_body`, whose description of blocks names the kind.

### Consequences

- **Positive:** a catalogue, a backlog or a list of decisions has an overview without anything new to keep; a link and a text show each reader what they read; values are corrected where they are seen, with the item page's validation.
- **Negative / trade-offs accepted:** a view reads at most the boards' limit of items and groups them in the browser; a large type needs a narrowing query. A view in a text reads its items again whenever the text is shown. Whether one may edit a cell is judged by the room role first; a restriction that forbids it is answered by the server's refusal, shown in the cell.
- **Follow-ups / risks to watch:** saved, named views; exports that list the items; the shared field renderer for new kinds.

## Pros and cons of the options

### Option 2: views as stored objects

Strongest form: a kanban already shows that a named, stored configuration of a room is useful: people find it again by name, it has an owner and it can be the room's start page. Stored views could carry permissions and appear in navigation.

- Good: names, a place in the room's navigation, no long addresses.
- Bad: a new kind of stored object with its own events, rights, erasure and trash, for what is in the end a set of display choices; a stored view drifts from the type when fields are renamed. A block in a text and an address cover sharing without it; saving views by name can be added later as a person's own setting.

### Option 3: views computed by the server

Strongest form: the server already selects and orders items with the query language over the live state; it could group and count too, page within groups, and serve views of any size without the browser reading the whole listing.

- Good: scales past the boards' limit; one place that groups.
- Bad: a second API shape for what the listing already answers, and grouping by labels and by people's names belongs to the reader's language and names. The listing with a query narrows enough for the sizes seen today; the server can take over grouping later behind the same view.

### Option 4: a view as an item

Strongest form: items are the one model (D1); a view as an item would be found, labelled, commented, protected and versioned like everything else.

- Good: every universal capability for free.
- Bad: a view is no content but a way of looking at content; making it an item puts display settings into the search, the governance check and the tree, and still needs the same browser code to draw it. A block in a page already gives a view a place among items, with the page's history and rights.

## Links

- relates to ADR-0012 (MCP), ADR-0025 (codec), ADR-0026 (sealed content), ADR-0036 (boards), ADR-0044 (labels), ADR-0060 (kanbans), ADR-0114 (task lists in a text), ADR-0136 (query language), ADR-0164 (roadmap), ADR-0166 (calendar), ADR-0204 (a label's page and its order)
