# ADR-0224: A picture's own labels on the starmap

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: album labels are taken into account on the starmap, answering the open question of ADR-0215 whether a label should also bring in the pictures that carry it as an album label. The requester chose the same day:
  - album labels act in the label filter **and** in the label stars;
  - on **both maps**, the map of a room and the map of all rooms.

  The rest are defaults proposed with the implementation.
- **Open question:** whether the labels' starmap (ADR-0165) should count album labels too; whether a label narrowing the map of all rooms should bring in an item that holds a labelled picture as a marked node of context rather than as one of the map's items; whether agents should read a picture's labels in `get_starmap` over MCP
- **Question checked:** 2026-11

## Context and problem statement

A picture has labels of its own (ADR-0090), kept apart from its item's: kanbans, label boards, the task list and `/api/labels` never show them, so that labelling a picture does not label its item's cards.
The starmaps (ADR-0157, ADR-0191, ADR-0215) draw pictures but read only item labels: a label star joins items, and *Show a label in every room* finds items only.
A photo in an album folder that carries a label through the album alone is therefore never found by that label on the starmap, and labels that join pictures are not drawn.

How do a picture's own labels act on the starmap without merging them into its item's labels?

## Decision drivers

- **Labels kept apart (ADR-0090):** a picture's label must not become its item's label.
- **Unreadable is non-existent (D10):** a label on a file the reader may not read (a protected block's file, ADR-0160) must neither draw an edge nor bring in an item.
- **Bounded** (ADR-0015): within `StarmapNodes`; items first (ADR-0215).
- **Pictures in their context:** a picture found by a label is shown with the page or task that holds it.

## Considered options

1. **A picture's own labels count as an item's on the starmap: label stars join pictures, and a label brings in the pictures that carry it with the items that hold them.**
2. **Only the label filter of the map of all rooms reads album labels; label stars stay item labels.**
3. **A picture carries its item's labels on the starmap.**

## Decision outcome

Chosen option:
**1**, as the requester chose.

- **A picture's labels** on the starmap are the labels of its own that any version of it carries which the reader reads; a picture node carries them in `Labels`.
  An item's labels never pass to its pictures, nor a picture's to its item.
- **Label stars, on both maps:** a label is a star when at least two items or pictures of the map carry it, joined to each by a `label` edge.
  On the map of all rooms the stars keep their place in the budget (ADR-0215: after the pictures a text shows, before the other pictures); a star left with fewer than two carriers once the budget is spent is dropped.
- **The label filter of the map of all rooms:** `label` keeps the items that carry it and the items that hold a picture that carries it.
  The pictures are those of the items that carry the label and those that carry it themselves; another picture of an item that holds a labelled one is not on the map.
  Holding items count in `items` and in the bubbles beyond the budget.
- **UI:** *Show a label in every room* suggests the labels of items (`/api/labels`) and of pictures (`/api/album/labels`) in one list, counted together (`mergeLabelCounts`).
  A label's panel lists the pictures that carry it besides the items.
  The map's search already finds a node by its labels.
- No format, event, route or budget is new.
  MCP is unchanged.

### Consequences

- **Positive:** a photo labelled in the album is found by its label on the starmap, with the page, task or folder that holds it.
- **Positive:** labels that join pictures across items and rooms are visible.
- **Negative / trade-offs accepted:**
  - The map of a label holds items that do not carry it, because they hold a picture that does; they look like the other items.
  - Label stars may take more of the budget, since pictures count as carriers.
  - The labels' starmap (ADR-0165) still counts item labels only.

## Pros and cons of the options

### Option 2: the label filter only

Strongest form: the smallest change that answers the question asked, and the label stars keep meaning exactly what `/api/labels` means.

- Bad: a label shared by pictures alone would find them by filter yet never be drawn as a star, so the two parts of one map would disagree about what a label is.

### Option 3: a picture carries its item's labels

Strongest form: no second vocabulary on the map; a picture appears wherever its item's label does.

- Bad: it does not find the pictures labelled in the album at all, which is what was asked, and it would label every photo of a folder alike, which ADR-0090 ruled out.

## Links

- answers an open question of [ADR-0215](0215-pictures-on-the-starmap-of-all-rooms.md)
- relates to ADR-0090 (labels of pictures), ADR-0157 (the starmap of a room), ADR-0191 (the starmap of all rooms), ADR-0165 (the labels' starmap), ADR-0160 (protected blocks), ADR-0015 (budgets)
