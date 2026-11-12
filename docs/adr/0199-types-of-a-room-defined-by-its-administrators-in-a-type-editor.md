# ADR-0199: Types of a room, defined by its administrators in a type editor

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 that room administrators define the types their room needs, with a form instead of hand-written JSON)
- **Open question:** whether a room type can later be shared with the whole installation (or moved to another room) without redefining it under a new name; whether the installation's administrators should see and remove the types of rooms in the trash or deleted, whose names stay taken; whether the type editor should offer the review rule (ADR-0074) and the Word template (ADR-0167), which it keeps but does not edit
- **Question checked:** 2026-11

## Context and problem statement

Types are data (ADR-0001), but only the installation's administrators define them, through a trusted command after an administrator check (ADR-0028, ADR-0094), and only as hand-written JSON. A room whose work needs its own kind of item — an incident with a severity, a story with platforms — has to ask an installation administrator, and every new type is offered in every room. How can a room's administrators define the types of their room, without breaking the guarantees types give today, and how is a type defined without writing JSON?

## Decision drivers

- Types stay data, one list, versioned (ADR-0001); the persisted format changes only with a codec version (ADR-0025).
- A room's administrators act through their session, never a trusted command (D10); what they may not see they are not told.
- A type's name is how items, kanbans, queries and connectors name it; it must stay unambiguous.
- Removing a type stays possible only when nothing uses it (ADR-0094).
- Installation types keep working unchanged, and the ready-made templates (ADR-0162) stay usable.
- The common case — fields, a workflow, a start text — needs no JSON; JSON stays for what the form does not offer.

## Considered options

1. **A type may belong to one room (`TypeDef.Space`): its items live only there, its room's administrators define, redefine and remove it through their session; names stay unique across the installation.**
2. **Types per room in a namespace of their own** (`PLAT/incident`), so two rooms may each have an `incident`.
3. **Delegate the installation's type administration per room**: room administrators define installation types, and a per-room list hides those a room does not use.
4. **Keep types for installation administrators and only add the form.**

## Decision outcome

Chosen option: **1**, with a form-based type editor for every type.

- **Model:** `TypeDef.Space` names the room a type belongs to, empty for a type of the whole installation (codec version 55, together with ADR-0200; versions 1 to 54 are still read). A type keeps its room across its versions: redefining it in another room, or as an installation type, is refused, and so is a name any live type has, whichever room it belongs to. A removed name is free again, for any room.
- **Use:** an item of a room type is made, imported or synchronised only in its room (`Store.usable`); it does not move to another room, nor does a subtree that holds one; a kanban of another room cannot show it; linked tasks of another room cannot be of it; the task of an access request is of a type usable in the requested item's room (ADR-0162). A copy into another room is refused by the same rule.
- **Who:** `Session.DefineType`, `Session.RemoveType` and `Session.MayDefineTypes`: a room's administrators and the installation's (who are room administrators everywhere) define, redefine and remove a room's types; one who sees nothing of the room is told it does not exist. A room's administrators also add a transition to a room type from a kanban (ADR-0076). Installation types are still defined by installation administrators through the trusted command (ADR-0028). Removal of a room type follows ADR-0094: refused while a live or trashed item or a kanban uses it.
- **API:** `POST /api/types` with `space` defines a room type through the caller's session; `DELETE /api/types/{name}` removes a room type the same way. `GET /api/types` lists the types one sees — the installation's and those of the rooms one holds a role in — and with `space` only those that room's items may have; each answer says whether the caller may change the type (`editable`).
- **Listing:** the form that makes an item offers a room's types only in that room, and follows the chosen room. Boards, the calendar, the roadmap and the query language read the types the reader sees, so a room type appears where its items are; the query language finds `type = incident` like any type.
- **Type editor** (`assets/typeeditor.js`, `assets/typeparts.js`): name; whether items have a text; fields with name, kind, required, recommended, unique (for text, url and number), values (enum and multi), target types (item) and a word on rating and file; a workflow of ordered states, the initial state and transitions with their states to start from and to go to, with the preset *open → done*; the start text as Markdown (ADR-0167). Problems are named before anything is sent; the server checks again. The type as JSON stays, folded away, and is applied to the form. Installation administrators use it under Administration → Item types, choosing whether a new type belongs to the whole installation or a room; a room's administrators in the room's settings, where they also add the ready-made templates as types of the room, renamed when the name is taken. The editor keeps a type's review rule and Word template as they are.
- **MCP:** `list_types` takes `space`. Defining and removing types stays unoffered, for the installation and for rooms alike (`internal/mcp/tools.go`): it shapes what everyone in a room works with and is a person's decision.

### Consequences

- **Positive:** a room gets the types its work needs without an installation administrator, and other rooms are not offered them.
- **Positive:** one list of types, one name per type: items, kanbans, queries, exports and connectors keep naming types as before.
- **Positive:** no JSON for the common case; the JSON stays one fold away.
- **Negative / trade-offs accepted:** two rooms cannot each have a type of the same name; the second chooses another. Learning that a name is taken tells that some room has such a type (not which one, nor anything of it).
- **Negative / trade-offs accepted:** an older build cannot read a log with a room type (codec version 55).
- **Follow-ups / risks to watch:** the types of a room in the trash keep their names until an installation administrator removes them through the API; a room type whose room is deleted for good stays in the log.

## Pros and cons of the options

### Option 2: a namespace per room

Strongest form: every room is independent; `incident` in two rooms is not a conflict, and nothing about one room's types leaks to another.

- Good: no name taken by another room; renaming a room's type never touches another room.
- Bad: every place that names a type — items, kanbans, queries (`type = incident`), connectors, linked tasks, exports, MCP — would have to qualify or resolve names by room, and a query across rooms would need to tell same-named types apart. That is a change of the model's identity of types for a rare convenience.

### Option 3: delegated administration of installation types

Strongest form: no change of the model at all; room administrators get the existing command, and a per-room list hides what a room does not use.

- Good: no codec change.
- Bad: a room administrator could change the types every other room uses, which is exactly what a room administrator must not do; hiding is no boundary (the API and imports would still accept them).

### Option 4: keep types for installation administrators, add only the form

Strongest form: least change; the form already removes the JSON burden, and installation administrators keep control over what types exist.

- Good: one place of authority.
- Bad: it does not answer the request: rooms still wait for an installation administrator, and every type is offered everywhere.

## Links

- amends [ADR-0001](0001-unified-item-data-model.md) (types are data) and [ADR-0028](0028-item-api.md) (who defines types)
- relates to ADR-0022 (permissions), ADR-0025 (codec versions), ADR-0060 and ADR-0076 (kanbans), ADR-0074 (catalog rules), ADR-0094 (removing a type), ADR-0136 (query language), ADR-0162 (templates, access request tasks), ADR-0167 (start text, Word templates), ADR-0200 (field kinds)
