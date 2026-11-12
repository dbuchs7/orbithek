# ADR-0202: Field kinds for an item, several values, a rating and a file

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 for fields that refer to another item, hold several values, rate with stars and name a cover picture)
- **Open question:** whether a file field should follow the newest version of its file (ADR-0055) rather than name the version chosen; whether an item field may hold several items; whether listings should carry the key and title of referenced items so that tables and cards need no read per item; whether a field's targets should be checked again when a type is redefined with other targets
- **Question checked:** 2026-11

## Context and problem statement

A field holds text, a number, a yes or no, a date, a point in time, a person, one value of a list or a web address (ADR-0001, ADR-0074, ADR-0166). Work often needs more: a story belongs to an epic, an item applies to several platforms, an idea is rated, a card shows a cover picture. Without such kinds, people write an item's key into a text field — which breaks when the item moves (D2), is not a relation, and does not keep the target from being deleted (D8). How are these kinds held, validated, shown and queried, and how does a reference count as a relation?

## Decision drivers

- References use the immutable ID (D2), are a deterministic function of what is stored (D4), and keep their target from deletion while they point at it (D8, ADR-0021).
- Nothing a reader receives reveals an item they may not read (D10).
- Values are validated changes (D3); the persisted format changes only with a codec version (ADR-0025); values are content and sealed (ADR-0026).
- No type-specific code in cross-cutting capabilities (D6): relations from fields work like those from bodies.
- The query language, exports and MCP treat the new kinds as they treat the others (ADR-0071, ADR-0131, ADR-0136, ADR-0167).

## Considered options

1. **Four kinds — item, multi, rating, file — with self-describing values: a reference is a value of the type `ID`, a multi field a list of strings; the store derives a relation, named by the field, from every reference.**
2. **References as hand-made relations of a typed name**, with the field only a view of them.
3. **References as plain strings, resolved by the field's kind** whenever the store needs to know.

## Decision outcome

Chosen option: **1**.

- **Kinds:** `item` holds an `item.ID`, optionally limited to target types (`FieldDef.Targets`); `multi` holds a `[]string` of the field's `Values`, in the order of the values, an empty choice unsetting it; `rating` an `int` from 1 to 5; `file` the attachment ID of one file of the same item. A multi field needs values; only enum and multi fields have values, only item fields targets.
- **Validation:** a reference names an existing, live item other than the item itself, of a target type if the field names any; a value the item already holds may stay when its target was trashed or the targets changed. A file is one of the item's own, not deleted; a new item has no files, so a file field is set after it is made. Through a session, a reference must name an item the caller reads: one it may not read does not exist for it (D10).
- **Persistence:** codec version 55 (together with ADR-0201): `FieldDef.Targets`, and the value types `r` (a reference) and `l` (a list of strings, as JSON). A version below 55 holding either is refused. Values stay in the sealed part of the event, like every property.
- **Relations:** every reference derives a relation from the item to its target whose type is the field's name (`Relation.Field()` names the field). It is derived in `apply` from the property, live and on replay (D4), changes only with the property — not with the body — and goes with a purged item. So it counts everywhere references count: the relations panel shows it among the incoming and outgoing relations, the starmap and the impact view draw it as a relation named by the field (ADR-0157, ADR-0159), and trashing its target is refused while it points there (D8). A restore brings back an item field's target with the item that refers to it and writes item and file fields once targets and files are there (ADR-0124); a copy sets a file field to the copy's file.
- **Reading:** a reference to an item the reader may not read is left out of the item, of listings, query results and the history, not even its ID shown (D10); sorting counts it as empty.
- **Query language:** an item field compares by an item's key or ID (`epic = PLAT-3`), a multi field holds `= x` when it holds x among others and takes `IN`, `!=` and `NOT IN`, a rating compares and sorts as a number; `ORDER BY` an item field sorts by the target's key.
- **Exports:** CSV names a referenced item by its key, a file by its name, a multi field's values joined by ", "; the page exports (Word, PDF, Markdown) and a Word template's controls name a referenced item by key and title and a rating as a number of five.
- **API and MCP:** `PUT /api/items/{ref}/properties/{field}` and `POST /api/items` take an item by its ID or key (resolved as the caller reads it), a list or one value for a multi field, a whole number for a rating; `set_property`, `create_item` and `query_items` describe them to agents.
- **UI:** `assets/fields.js` draws and edits the four kinds for every view: `fieldInput(f, value, ctx)` — a search picker for an item, check boxes for several values, five stars, a choice of the item's files with a thumbnail — and `fieldView(f, value, ctx)` for reading; kanban cards show stars, values and the referenced item; an unreadable target reads as *not shown*.

### Consequences

- **Positive:** a reference is a real relation: it survives moves and re-keying, shows up wherever relations do, and protects its target.
- **Positive:** values describe themselves, so deriving relations needs no type lookup and replays the same whatever later versions of the type say.
- **Negative / trade-offs accepted:** an older build cannot read such values (codec version 55).
- **Negative / trade-offs accepted:** a file field names a version of a file; a new version of it is not followed (open question).
- **Follow-ups / risks to watch:** views that show many items with item fields read each target once per page; listings may later carry what a reference is called.

## Pros and cons of the options

### Option 2: references as hand-made relations

Strongest form: relations exist already, with their own events, history and permission handling; a field would only be a named view of the relations of that type, and a person could add several.

- Good: no new value type.
- Bad: a property would no longer be what the item holds (D4: anything queryable is a property); setting a field would be two kinds of change; validation, required fields and the query language would need to read relations as values; an import setting the field would have to add and remove relations.

### Option 3: references as strings resolved by the field's kind

Strongest form: no codec change for references; a reference reads like any text in the API.

- Good: smaller format change.
- Bad: what a value means would depend on the type version consulted; a redefinition from item to text would turn a relation into text silently on replay, and every place deriving relations would need the type — a function of more than the stored value (D4).

## Links

- amends [ADR-0001](0001-unified-item-data-model.md) (field kinds)
- relates to ADR-0021 (deletion only when unreferenced), ADR-0025 and ADR-0026 (codec and sealing), ADR-0055 (file versions), ADR-0071 and ADR-0131 (exports), ADR-0124 (restore), ADR-0136 (query language), ADR-0157 and ADR-0159 (starmap, impact), ADR-0166 (datetime), ADR-0167 (Word templates), ADR-0201 (room types and the type editor)
