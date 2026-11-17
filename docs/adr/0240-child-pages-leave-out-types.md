# ADR-0240: A list of child pages leaves out the types chosen

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-17
- **Deciders:** Project initiators. Request of 2026-11-17: a list of child pages gets a filter by type, for tasks; the requester chose that types are left out (rather than chosen to be shown), that the items below a left-out item are left out with it, and that the change goes into a pull request of its own. The rest are defaults proposed with the implementation, to be confirmed by the requester.
- **Open question:** whether a list should show a left-out item's pages that are not left out themselves, in its place
- **Question checked:** 2026-11

## Context and problem statement

ADR-0239 lets a list of child pages show more than one level.
The page tree holds every item below a page, not only pages: tasks made below a page (ADR-0114), decisions, access requests (ADR-0162).
A list of every level below a project page therefore lists its tasks between its chapters, and the deeper it goes, the more of it is tasks.
ADR-0239 named this as an open question.

How does a list of child pages leave out the items of some types, without type-specific code and without revealing what the reader may not read?

## Decision drivers

- **D6:** no code that knows tasks; the filter names types as content, as a view does (ADR-0203).
- **D10:** nothing the list shows or marks counts what the reader may not read; a left-out item counts as little.
- **The format is a contract (ADR-0025):** every list written before keeps showing every type.
- **New types appear:** a type defined later should show in a list unless someone leaves it out.
- **One answer everywhere:** editor, reading view, exports and agents leave out the same items.

## Considered options

1. **Types left out**, a list of type names on the block, each item of them left out with everything below it.
2. **Types shown**, a list of type names on the block, every other type left out.
3. **A switch for items with a workflow**, the block leaving out every item whose type has a workflow.

## Decision outcome

Chosen option: **1, types left out**, because it says what the request asks ("without tasks"), lets types defined later show without anyone choosing them, and keeps every list written before as it was.

- **The field:** `Block.HideTypes` on a block of the kind `children`: 1 to `MaxHiddenTypes` (24) type names, each once and in sorted order, or none for every type.
  A name need not name a type that exists, so a type removed later leaves the body valid.
  The API sorts and folds a list it is sent; an empty list is none.
  Codec version 67; versions 1 to 66 are still read, their lists showing every type.
- **What is left out:** an item of a left-out type with everything below it, as the requester chose.
  `Session.ChildPages` passes such items by as it walks, so they take nothing of the budget `ChildPages` and do not make their parent's `more`.
- **The API:** `GET /api/spaces/{space}/tree` takes `hide`, type names separated by commas, with `depth`; MCP's `get_tree` offers it, and its description of blocks names the field.
- **The editor:** beside the depth, *Types* says *all* or *without* the types left out; a click opens a box per type of the item's room (`GET /api/types?space=`), and a type the list leaves out that is no longer defined stays offered so it can be shown again.
  Type names are content and stay untranslated.
  The dialog that inserts a list asks for the types too, beside the depth, every type of the room ticked at first (requested on 2026-11-17).
- **Exports:** Word, PDF and Markdown leave out the same items; the server reads the children once per page without leaving any out and each list leaves out its own types.

### Consequences

- **Positive:** a project page lists its chapters without its tasks; lists written before are unchanged.
- **Negative / trade-offs accepted:** a page below a task is left out with the task, even where a reader would want it; exports read the children before leaving types out, so with many left-out items a list in an export can reach the budget sooner than in the editor.
- **Follow-ups / risks to watch:** the open question above.

## Pros and cons of the options

### Option 1: types left out

Strongest form: the list keeps showing what it showed and leaves out exactly what someone chose to leave out, new types included.

- Good: matches the request; robust to new types; no change for existing bodies.
- Bad: a list meant to show pages only must name every other type, and a new type of items appears in it until someone leaves it out.

### Option 2: types shown

Strongest form: a list of pages only says so directly, and a new type of items never slips into a carefully kept list.

- Good: precise for a list that should show one type.
- Bad: a new type of pages stays hidden until someone adds it; leaving out one type means naming all the others.

### Option 3: a switch for items with a workflow

Strongest form: one tick, no type names; tasks, decisions and access requests all have workflows, pages do not.

- Good: simplest to use.
- Bad: ties the filter to a property of types rather than to the types a person names; a page type with a review workflow would vanish too; no way to leave out one workflow type and keep another.

## Links

- extends ADR-0239 (a list of child pages shows the levels chosen) and ADR-0042
- relates to ADR-0203 (views name types), ADR-0081, ADR-0015, ADR-0025, ADR-0012
