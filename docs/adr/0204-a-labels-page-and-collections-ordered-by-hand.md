# ADR-0204: A label's page, and collections ordered by hand

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12: a label gets an optional page that says what it is about and shows its items; such a page with an order set by hand is a collection; adding an item to a collection is giving it the label)
- **Open question:** whether several pages of one label in different rooms should be shown together rather than only the oldest readable one; whether a label's page should be offered as the label's own page in the label board and the labels' starmap too; whether renaming a page's label should carry its order and offer to relabel the items
- **Question checked:** 2026-11

## Context and problem statement

Labels are strings on their items (ADR-0044): they group items across rooms, but a label cannot say what it means, who keeps it or in what order its items are best read. A reading list, an onboarding path or the chapters of a topic need exactly that: a page that explains the label and lists its items in an order someone chose. People call such an ordered set a collection; adding an item to it should not be a second mechanism next to labels.

The question is where a label's description lives, how the label's view finds it, where an order set by hand is kept, and what a reader learns of items they may not read.

## Decision drivers

- One model for content (D1): a description is content, so it is an item, not a property of a label or a system object.
- Stable identity (D2): an order names items by their IDs.
- Unreadable is non-existent (D10): neither the page found for a label nor its order may tell of an item the reader does not read.
- Labels stay as they are (ADR-0044): no change to how labels are stored, set or counted.
- An order set by hand already exists for the album's slides (ADR-0195); the same shape should serve items.

## Considered options

1. **A page of the ready-made type `topic`** whose text field `label` names the label, found by that field; the order kept on the page as item IDs in an event of its own
2. **Labels as items** (the option ADR-0044 rejected), with the description and order on the label item
3. **A description and order kept per label as system data**, outside the items
4. **The order as a field of each item** (a number per item and label)

## Decision outcome

Chosen option: **1**.

- **The type.** `item.TypeTemplates` gains `topic`: a body and a required, unique text field `label` (unique per room, ADR-0074). Any type with a body and a text field `label` can hold a label's page. **Describe this label** on a label's view makes one in a room the person may create in (`POST /api/labels/{label}/page`, `Session.DescribeLabel`), titled by the label unless given a title; the type is defined from its template on first use, as the album defines its collection's type (ADR-0175), and a type of that name that cannot hold a page is not taken over. While nothing new may be made (ADR-0163), neither the type nor the page is made.
- **Finding it.** `GET /api/labels/{label}/page` answers the page of the label the caller reads: among the live items whose type holds a page and whose field names the label (normalised as labels are), the one made first (`Session.TopicOf`). A label with pages in several rooms shows each reader the oldest they read. The label's view shows it on top, its title, the beginning of its text and **Read the page**.
- **Its items.** A label's page shows the live items carrying its label, other than itself, with the views of ADR-0203 (table, gallery or list, grouped and sorted; the layout, grouping and sort in the page's address).
- **The order.** Whoever may edit the page moves items before another or to the end (`PUT /api/items/{ref}/order` with `items` and `before`, `Session.PlaceItems`), by dragging or with the arrows and Alt+↑ and Alt+↓; `DELETE` forgets it. The page keeps the order as `Item.ItemOrder`, the IDs of the items in their order, recorded as the event `ItemsOrdered` in the field `order`. As with the slides of ADR-0195, a move writes the whole order: what it named and still carries the label, then the items the mover reads and it did not name in the order they were made, then the moved ones take their place; items the mover does not read keep theirs. Items without a place come after the ordered ones, in the order they were made.
- **Sealing.** The order is not sealed: it names items by ID only, no content (ADR-0026), like `PicturesOrdered` (codec version 52). It is no reference (D8): an item that loses the label, is trashed or purged is passed over and forgotten with the next move.
- **What a reader learns.** `GET /api/items/{ref}/order` and the label's page answer only the IDs of items the caller reads, in their order (`Session.ItemOrder`); the history does not show the order. A reader who does not read an item sees neither it nor a gap.
- **Collections.** A label's page with its order is a collection. **Add to collection…** on an item lists the readable pages of the type `topic` and gives the item the chosen page's label (`PUT /api/items/{ref}/labels`), so labels remain the one mechanism.
- **Format.** Codec version 56 adds the kind `ItemsOrdered` (shared with ADR-0203); earlier versions are still read and have no order.
- **MCP** (ADR-0012): `get_label_page` reads a label's page with its order; describing a label is `create_item` with the type `topic`; ordering by hand is a person's arrangement and is not offered.

### Consequences

- **Positive:** a label can explain itself and present its items in a chosen order without changing how labels work; collections need no new mechanism; the order is safe to keep beside content because it holds none.
- **Negative / trade-offs accepted:** only one page per label is shown to a reader, the oldest; another room's page is reached through that room. A move rewrites the whole order in one event, which grows with the items carrying the label. The topic list in **Add to collection…** reads at most one page of the listing.
- **Follow-ups / risks to watch:** showing a label's page on the label board and the labels' starmap; offering several pages of one label; carrying an order when a page's label is renamed.

## Pros and cons of the options

### Option 2: labels as items

Strongest form: a label item would be the one place for its name, description, owner, order and rights; renaming a label would be renaming one item, and relations would make "carries this label" explicit.

- Good: one identity per label, renaming without touching every item.
- Bad: ADR-0044 rejected it for good reasons that still hold: a relation per label and item, labels no longer part of their item's sealed content, and every listing, query and count rebuilt around relations. A page that describes a label gives most of the benefit without moving labels.

### Option 3: description and order as system data

Strongest form: a small table per label beside the item store would be quick to read, need no type and could not collide with a room's own types.

- Good: no ready-made type, no question which of several pages counts.
- Bad: a description is text a person writes, so it is content (D1): it would need sealing, erasure, history, comments and rights of its own outside the item model. As an item it has all of them already.

### Option 4: the order as a field of each item

Strongest form: a number on each item (its place in a collection) is queryable, sorts in the query language and needs no new event kind.

- Good: ordering by the query language's ORDER BY.
- Bad: one field per label on every type, written on many items for one move, each move a change of items the mover may not read or edit; and a field holds no order across rooms the mover cannot see. The order belongs to the page that someone curates, as the slides' order belongs to the folder (ADR-0195).

## Links

- relates to ADR-0012 (MCP), ADR-0022 (permissions), ADR-0025 (codec), ADR-0026 (sealed content), ADR-0044 (labels), ADR-0074 (unique fields), ADR-0162 (ready-made types), ADR-0163 (nothing new at critical storage), ADR-0175 (collections in the album), ADR-0195 (slides ordered by hand), ADR-0203 (views of items)
