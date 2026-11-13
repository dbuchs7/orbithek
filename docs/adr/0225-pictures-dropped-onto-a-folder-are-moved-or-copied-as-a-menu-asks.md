# ADR-0225: Pictures dropped onto a folder are moved or copied, as a menu asks

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 to offer the choice of ADR-0222 when dragging onto a folder too, settling its open question; asked how, the requester chose a menu where the pictures are dropped, with Ctrl or Alt copying and Shift moving without it)
- **Open question:** whether the album should also offer the choice when a picture is dropped onto a page's text or a whiteboard, where it is copied today
- **Question checked:** 2026-11

## Context and problem statement

A picture dragged onto a folder is moved there with its versions (ADR-0184, ADR-0188). *Add from the album…* lets one copy or move (ADR-0222). Two ways to the same place behaved differently, and a picture could not be copied into a folder by dragging.

How does one choose between moving and copying while dragging, without slowing down the common case?

## Decision drivers

- **Discoverable:** the choice must be visible, not only behind a key one has to know.
- **Fast for those who know:** a key, as in file managers, skips the question.
- **No surprise:** nothing moves or is copied until the person says so; Escape changes nothing.
- **One move, one copy:** the move of ADR-0188 and the copy with its source of ADR-0054, with their limits.
- **No server change.**

## Considered options

1. **A menu where the pictures are dropped, *Move here*, *Copy here* and *Cancel*; Ctrl or Alt copies and Shift moves without it.**
2. Only a key while dropping, as in file managers.
3. Follow the choice last made in *Add from the album…*.

## Decision outcome

Chosen option: **1**.

- Dropping pictures onto a folder, below the open one or in the path, opens a small menu where they were dropped. *Move here* has the focus; the arrow keys go through it, Enter takes one; Escape, a click elsewhere or *Cancel* changes nothing.
- *Move here* moves each picture with its versions (ADR-0188); a picture that may not move stays and the album says why.
- *Copy here* copies each picture with its source (ADR-0054); the originals stay.
- Holding Ctrl or Alt while dropping copies without the menu, holding Shift moves without it. While dragging with Ctrl or Alt, the pointer shows a copy.
- The folder's hint says that pictures are moved or copied and a folder is moved. A folder dragged onto a folder is still only moved (ADR-0200).

### Consequences

- **Positive:** dragging can copy, and both ways to a folder offer the same choice.
- **Negative / trade-offs accepted:** a drop without a key takes one more click than before.
- **Negative / trade-offs accepted:** the keys are not shown while dragging beyond the pointer's copy sign; the manual names them.
- **Follow-ups / risks to watch:** the open question above.

## Pros and cons of the options

### Option 1 — a menu with keys (chosen)

Strongest form: visible to everyone, and as fast as a file manager for those who hold a key.

- Good: discoverable and safe.
- Bad: one click more for the common move.

### Option 2 — keys only

Strongest form: no extra click; familiar from file managers.

- Bad: the choice is hidden; on a touch device or without knowing the key there is no copy.

### Option 3 — the remembered choice

Strongest form: no extra click and no key.

- Bad: what a drag does depends on a setting made elsewhere, which surprises.

## Links

- amends [ADR-0222](0222-pictures-added-from-the-album-to-a-folder-are-copied-or-moved.md) (its open question) and [ADR-0184](0184-pictures-are-moved-by-dragging-them-onto-a-folder.md) (dragging onto a folder)
- relates to ADR-0054 (copies with their source), ADR-0188 (a moved file keeps its identity), ADR-0200 (moving folders)
