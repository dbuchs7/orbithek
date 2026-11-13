# ADR-0222: Pictures added from the album to a folder are copied or moved, as one chooses

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 to decide the last open question of ADR-0187; asked whether *Add from the album…* should copy, move from folders only, always move or let one choose, the requester chose to choose in the dialog, the browser remembering the choice, copying by default; accepted on 2026-11-12)

## Context and problem statement

*Add from the album…* brings pictures already in the album into the open folder (ADR-0187). It copied each picture with its source (ADR-0054), so the original stayed. Moving became a real move of a picture with all its versions (ADR-0188). For tidying one's own photos, a move is what is wanted; for a picture that belongs to a page or a task, a copy is safer.

Should adding copy, move, or let the person choose?

## Decision drivers

- **The person decides:** whether a picture should leave where it is depends on what it is for, which the album cannot know.
- **No surprise deletions:** a picture must not vanish from a page without the person having chosen a move.
- **One move:** moving here is the move of ADR-0188, with its limits: a picture one may not change, a shared file's picture, or one a text shows stays where it is.
- **No server change** and no change to the persisted format.

## Considered options

1. **A choice in the dialog, *Copy* or *Move*, remembered in the browser, *Copy* by default.**
2. Move from album folders, copy from pages and tasks, without a choice.
3. Always move where allowed, as dragging onto a folder does.
4. Always copy, as before.

## Decision outcome

Chosen option: **1**.

- The dialog of *Add from the album…* for a folder shows *Copy* and *Move*. The hint below says what the choice does.
- *Copy* copies each picture chosen with its source; the original stays (ADR-0054).
- *Move* moves each picture chosen with its versions, labels, notes and place (ADR-0188). A picture that may not move stays where it is; the album says why on that picture, and the person may copy it instead.
- The browser remembers the choice for the next time; without one it is *Copy*.
- A collection is unchanged: adding to it gives labels and neither copies nor moves (ADR-0187).

### Consequences

- **Positive:** tidying one's photos into folders takes one choice; pictures of pages stay unless a move is chosen.
- **Positive:** nothing changes on the server.
- **Negative / trade-offs accepted:** one more control in the dialog; the choice is per browser, not per person.
- **Follow-ups / risks to watch:** whether dragging should offer the choice too, settled by ADR-0226.

## Pros and cons of the options

### Option 1 — a choice (chosen)

Strongest form: the person who knows what a picture is for decides, once, and the album remembers it.

- Good: no surprises; fits both tidying and reuse.
- Bad: one more control.

### Option 2 — move from folders, copy from pages

Strongest form: no control, and pictures of pages are never taken away.

- Good: right in most cases without asking.
- Bad: a rule the person has to learn; a picture uploaded to a page by mistake cannot be moved this way.

### Option 3 — always move

Strongest form: the same as dragging onto a folder.

- Bad: a file of a page or task disappears there when it is not shown in a text.

### Option 4 — always copy

Strongest form: nothing is ever taken away.

- Bad: tidying leaves duplicates behind, which the duplicate check (ADR-0196) then asks about.

## Links

- amended by [ADR-0226](0226-pictures-dropped-onto-a-folder-are-moved-or-copied-as-a-menu-asks.md) (the choice when dropping onto a folder)
- amends [ADR-0187](0187-pictures-are-added-where-one-is-in-the-album.md) (its last open question)
- relates to ADR-0054 (copies with their source), ADR-0184 (moving by dragging), ADR-0188 (a moved file keeps its identity), ADR-0196 (duplicates)
