# ADR-0171: Comments on the elements of a drawing

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09: comments on the whiteboard; accepted 2026-11-09)

## Context and problem statement

Comments are part of their item (ADR-0035); an inline comment names a passage of the body by the text it quotes (ADR-0047). On a whiteboard (ADR-0169) people comment on a note, a shape or an arrow: the remark belongs next to that element, is answered and resolved like any thread, and must still be found when the drawing has been saved again under a new version.

How is a comment anchored to an element of a drawing?

## Decision drivers

- A comment is not an edit: commenting does not change the drawing or the body, and works while others hold elements in a live session.
- The anchor survives new versions of the drawing: a drawing's file changes with every save (ADR-0055), its elements keep their IDs.
- Everything that names what a comment is on is content: sealed and erased with the item (ADR-0026).
- Comments stay type-agnostic (D6): an anchor names a place in an item, never what type the item is.
- The persisted format changes only with a new codec version (ADR-0025).

## Considered options

1. **The anchor names the drawing by its first version and the element by its Excalidraw ID**, and quotes the element's text, if it has any.
2. **A comment kept in the scene**, as data of the element.
3. **An ordinary comment on the item** whose text says which element it means.

## Decision outcome

Chosen option: **1**, codec version 41.

- `item.Anchor` gains `Drawing`, the ID of the drawing's first version (`Picture.First`), and `Element`, the element's ID within its scene (1 to 128 letters, digits, `_`, `-`, `.` and `:`). Both are content, sealed with the anchor. `Quote` holds the element's text when it has one and may be empty for such an anchor; a passage of the body still needs its quote.
- `Drawing` must name a file of the item. The element is not checked against the scene: the scene lives in a picture the server does not read on every comment, and an element can be deleted and its deletion undone. A comment whose element is gone says so, as a comment whose passage is gone does (ADR-0047).
- **On the whiteboard** a panel lists the comments on the drawing's elements, threads with their replies; *Comment* on a selected element starts one; an element with comments carries a badge with their number, and a click on it opens its thread. The others in the session see a new comment at once (the session passes the word, ADR-0169).
- **On the item's page** a comment on an element shows as *on an element of a whiteboard*, with its quote, and leads to the whiteboard.
- **MCP:** `add_comment` takes the anchor `{drawing, element, quote}`.

### Consequences

- **Positive:** threads, replies, resolving, the comment feed and erasure work for elements without new code; a comment outlives every new version of the drawing.
- **Negative / trade-offs accepted:** a copy of the drawing into another item (ADR-0054) does not take its comments along, as a copied page does not take its inline comments.
- **Negative / trade-offs accepted:** the codec changes; older builds cannot read data written by this one (ADR-0025).

## Pros and cons of the options

### Option 2: comments in the scene

Strongest form: the comment travels with the drawing wherever it is copied.

- Bad: every comment would be a new version of the picture; anyone who reads the picture reads the comments, outside the comment's own lifecycle (resolving, deleting, erasure); commenting would need the right to edit the drawing.

### Option 3: an ordinary comment

Strongest form: no change at all.

- Bad: the whiteboard cannot place the comment by its element, nor count the comments of an element; the reference is prose.

## Links

- extends [ADR-0047](0047-inline-comments-anchored-by-quoted-text.md) (anchors) and [ADR-0035](0035-comments-are-part-of-their-item.md) (comments)
- relates to [ADR-0169](0169-live-whiteboards-on-drawings.md) (live whiteboards), [ADR-0170](0170-workshop-tools-on-a-whiteboard.md) (workshop tools), [ADR-0025](0025-event-log-file-and-in-memory-state.md) (codec)
