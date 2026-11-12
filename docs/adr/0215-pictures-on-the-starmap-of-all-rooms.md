# ADR-0215: Pictures and drawings on the starmap of all rooms, joined to the items that use them

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: photos and drawings are shown on the starmap too, together with the pages and tasks where they are used. This answers the open question of ADR-0191 whether pictures belong on the starmap of all rooms. The order of priority, the treatment of copies and the absence of pictures in bubbles are defaults proposed with the implementation.
- **Open question:** whether a label should also bring in the pictures that carry it as an album label (ADR-0090), not only the pictures of the items that carry it; whether a bubble should say how many pictures its items hold; whether a copy's original whose item a label leaves out should come in as an outside node, as on a room's map
- **Question checked:** 2026-11

## Context and problem statement

The starmap of all rooms (ADR-0191) leaves pictures out: the album shows them across rooms, and they would take half the budget `StarmapNodes`. A room's starmap (ADR-0157) shows them, each joined to its item and a copy to its original. Across rooms, a reader therefore cannot see which page or task uses a photo or drawing. An album with thousands of photos in folders, none of them shown by a text, would fill the budget before the pictures pages and tasks use.

Which pictures does the map of all rooms show, and how are they joined to where they are used?

## Decision drivers

- **Unreadable is non-existent (D10):** no picture, edge or count names an item or a file the reader may not read, including files of protected blocks (ADR-0160).
- **Bounded** (ADR-0015): within `StarmapNodes`, derived per request, nothing stored.
- **Items first:** pictures must never turn a map of items into bubbles or push items off it.
- **Used before stored:** a picture a text shows says more about a page or task than a photo kept in a folder.

## Considered options

1. **All pictures of the map's items, after the items: those a text shows first, each copy's original after it, then the shared labels, then the other pictures.**
2. **Only the pictures a text shows, with their originals.**
3. **All pictures in the album's order (newest first), as a room's map does.**

## Decision outcome

Chosen option: **1**.

- **Only within the budget for items.** Whether the map shows items or bubbles is decided by the items alone, as before. In bubbles there are no pictures.
- **Which pictures:** the newest version of each picture file (the album's `ImageTypes`; drawings are PNG files) of the items on the map, as a room's map lists them (ADR-0054, ADR-0103). With a label, those are the items that carry it.
- **Order** within what the items leave of the budget:
  1. pictures a text of their item shows (an image block of any version of the file), newest first, each copy followed by its original when the original's item is on the map;
  2. the labels that at least two items carry;
  3. the other pictures, newest first.
  The map says `truncated` when pictures or labels are cut.
- **Edges:** every picture is joined to its item (`picture`), a copy to the newest version of its original (`copy`) when both are on the map. An original whose item is not on the map, or not readable, gives no node and no edge.
- **UI:** unchanged; it already draws pictures, their panel and the *Pictures* filter for a room's map.
- No format, event, route or budget is new. MCP is unchanged.

### Consequences

- **Positive:** photos and drawings are seen beside the pages and tasks that use them, across rooms, and an album photo inserted into a page is joined to the page's copy.
- **Positive:** a large album does not crowd out the pictures that texts show, nor the labels.
- **Negative / trade-offs accepted:**
  - A map whose items fit may now say it is truncated, because pictures fill the rest of the budget.
  - Pictures kept in folders and shown by no text are drawn only when room is left.
  - With a label, a copy's original in an item without that label is not shown.

## Pros and cons of the options

### Option 2: only the pictures a text shows

Strongest form: exactly what was asked — pictures where they are used — and an album never costs budget.

- Bad: a drawing or photo attached to a task without being placed in its text, which is common for tasks, would never appear, though the task holds it.

### Option 3: all pictures newest first

Strongest form: the same rule as a room's map, nothing new to explain.

- Bad: the newest photos of a large album take the budget before the pictures pages and tasks show, and before the labels.

## Links

- answers the open question on pictures of [ADR-0191](0191-a-starmap-of-all-rooms.md)
- relates to ADR-0157 (the starmap of a room), ADR-0054 and ADR-0055 (the album and copies), ADR-0160 (protected blocks), ADR-0015 (budgets)
