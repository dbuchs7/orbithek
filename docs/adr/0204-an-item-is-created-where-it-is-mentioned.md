# ADR-0204: An item is created where it is mentioned

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12: typing a title that matches nothing offers to create it)
- **Open question:** whether the dialog should offer the fields a type requires, so that every type can be made from a mention; this record leaves such types to **+ New**
- **Question checked:** 2026-11

## Context and problem statement

While writing, people name things that do not have a page yet: a topic to explain later, a meeting, a decision to take. In the editor, "@" suggests people and items, and the link dialog (Ctrl+K) finds items by title (ADR-0096). When nothing matches, both end with "nothing found". To link the new thing, one leaves the text, creates the item with **+ New**, comes back, and mentions it — or writes its name as plain text and never links it.

How does the editor turn a title that matches nothing into an item and a mention of it, in one step, without weakening the rules of creating?

## Decision drivers

- One step from typing to a linked item; the text being written stays where it was.
- D2 and D4: the result is an ordinary mention (a mark with the item's ID), so the derived relation follows from the body.
- ADR-0022 and ADR-0163: creating stays the server's decision — the role in the room, edit rights on a parent, and no new item at critical storage. The UI may not offer what the server refuses as if it would work.
- Nothing is created by accident: typing alone never creates.
- No new route, event or codec change if the existing ones suffice.

## Considered options

1. **An offer at the end of "@" and of the link dialog's results, a dialog for type and room, then `POST /api/items` and the mention.**
2. **Create at once with defaults** (the room's page type, the current room), without a dialog.
3. **A "red link"**: insert a mention of a title that does not exist yet; the item is made when someone opens it.
4. **A server command "create and mention"** that makes the item and commits the host's body in one batch.

## Decision outcome

Chosen option: **1**, because it keeps the person in control of what is made and where, uses the one route every client creates with, and changes nothing in the model.

- **Offer:** when "@" is followed by a query and no item found has that title (letter case aside), the list ends with *Create «title» as …*; the link dialog offers the same below its results when what is typed is no web address. Enter or a click opens the dialog; Escape keeps the text as typed. The query after "@" holds no space (ADR-0096), so the dialog's title field is editable and takes the whole title.
- **Dialog (`assets/mentioncreate.js`):** the title, the type (default: the room's page type — one named `page`, else the first type with a text and no workflow) and the room (default: the edited item's room), among the rooms where one holds Editor or higher. Only types that require no field are offered; the daily note type (ADR-0203) is made from its day, never from a mention. A sentence says where the item goes: an item of a type with a text, made in the edited item's room, goes below the edited item, as a child page; anything else at the top of the chosen room. A child inherits its parent's restriction (D9), so the sentence says so when the edited item is restricted, and nothing goes below a daily note (ADR-0203), which is its author's alone: from a note, the item goes to the top of the room.
- **Creating:** `POST /api/items` as the person. The server decides as for any item; a refusal (no right, 507 at critical storage, a validation error) is shown in the dialog and nothing is inserted. While the storage is at its critical level the dialog says so and offers no button (ADR-0163).
- **Inserting:** the new item's mention replaces what was typed after "@" if it is still there, else it goes where the cursor is; from the link dialog, it becomes a mention with the text to show (as linking an existing item does). In a live session (ADR-0010) the mention is an ordinary edit of the block, saved when the block is released.
- **Where it applies:** every editor with writing aids: an item's text, a daily note (ADR-0203) and an item beside another (ADR-0205). `createEditor` and `liveBody` take `create(title)`, which the shell gives them.

### Consequences

- **Positive:** linking what does not exist yet is as quick as linking what does; the context of the new item (ADR-0205) names the text it was created from at once.
- **Negative / trade-offs accepted:** the item and the mention are two changes: when the session's save of the mention fails, the item stays without it, as an item made with **+ New** would. Types with required fields are not offered. The default page type is found by name and shape, a guess an administrator cannot change.
- **Follow-ups / risks to watch:** offering required fields in the dialog; a room's default type chosen by its administrators.

**Implementation so far:** everything above; tested by `e2e/smoke.cjs`. Not built: required fields in the dialog.

## Pros and cons of the options

### Option 2: create at once with defaults

Strongest form: the fastest path is no dialog at all; the type and room can be changed afterwards on the new item's page, and editors of other tools make a page from a link in one keystroke.

- Good: one keystroke.
- Bad: a typo or an unintended Enter makes an item in a shared room that someone must find and delete (deleting is refused while the mention refers to it, D8); the room's page type may not be what was meant. One dialog with good defaults costs one more Enter.

### Option 3: a red link

Strongest form: wikis have long marked links to pages not yet written; the writer is not interrupted, and the page is made when someone needs it, with full context.

- Good: no interruption; nothing made in vain.
- Bad: a mention must point at an item's ID (D2); a link to a title would be a reference by name, broken by any rename, and a derived relation to nothing breaks D4. It would need a new kind of reference and a codec change.

### Option 4: a server command "create and mention"

Strongest form: one batch makes the item and commits the host's body with the mention, so neither stands without the other, as `CreateEmbedded` does for embeds (ADR-0020).

- Good: atomic.
- Bad: the host's body is held by a live session (ADR-0010) whose blocks are locked by the person typing; a server-side commit would race with the session's own save of the same block. The session already saves the mention as an edit; the item alone is a harmless leftover if that save fails.

## Links

- relates to ADR-0096 (writing aids), ADR-0010 (live editing), ADR-0020 (inline items), ADR-0022 (permissions), ADR-0163 (nothing new at critical storage), ADR-0203 (daily notes), ADR-0205 (the context of an item)
