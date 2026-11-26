# ADR-0252: A starmap in a text, narrowed at first, a click opening the item

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-25
- **Deciders:** Project initiators. Request of 2026-11-25: starmaps can be embedded in items, filtered initially, and a click on an item opens it. The requester chose the same day:
  - the block keeps the room and a label, the types and kinds of node it leaves out, the neighbourhood of the item it is on, and a search text;
  - readers may change the filters while reading, without the item being saved;
  - a picture opens its item, a label its page and a bubble its room's map.

  The rest are defaults proposed with the implementation.
  Request of 2026-11-26, after the first part: when several types are chosen for display, all of them show; the dialog's boxes now choose what shows, and the item a text is about stays on its map.
- **Open question:** whether a reader's change of the filters should be kept per reader, as a remembered tab is; whether the block should keep the types it shows rather than those it leaves out, so that a type defined later does not appear on a map that named its types; whether exports should draw the map as a picture rather than name it; whether a map in a text should also offer the node's panel and the impact view (ADR-0159), here left to the starmap's own page
- **Question checked:** 2026-11

## Context and problem statement

The starmap shows a room's items, every room's items, or a label across rooms (ADR-0157, ADR-0191) on a page of its own.
People who write a page about a subject want the map of that subject beside the words that explain it: the pages that carry a label, the neighbourhood of the page itself, a room without its finished tasks.
A text already shows computed content in blocks — a task list (ADR-0114), a list of child pages (ADR-0042), a view of items (ADR-0203) — that keep what they show and hold no item.

How does a text show a starmap, what does it keep, how is it narrowed, and what does a click on it do?

## Decision drivers

- One model for content (D1) and no shadow data (D4): a map is a way of looking at items, not content; the block must not copy items.
- Unreadable is non-existent (D10): every reader sees the map as they may read it, no count of the rest.
- The same map as on its own page: symbols, filters, physics and names alike, so nothing is learnt twice and nothing drifts.
- Several maps may share a page, and a map must not take the page's scrolling or keys.
- The persisted format changes only where something must be kept (ADR-0025).

## Considered options

1. **A block of the kind `starmap` that keeps the room, a label, a search text, the hidden types and kinds and a depth; the browser draws the starmap's own view in a mode for texts, with its filters starting from the block.**
2. A starmap as an embed of an address: the block keeps the starmap's address (`#/starmap/PLAT?label=…`) and shows it in a frame.
3. A picture of the map, taken when the text is saved.

## Decision outcome

Chosen option: **1**, because it follows the view of items (ADR-0203), keeps nothing that can leak or go stale, and draws the very map the starmap's page draws.

- **The block.** Kind `starmap`, with no text, mention or reference:
  - `Space`: a room's key, `*` for every room the reader reads, or none for the room of the item the text is on, so a page moved to another room shows its new room;
  - `Label`: a label as stored (ADR-0044), or none;
  - `Search`: a search text of at most 200 characters, or none;
  - `HideTypes`: the types it leaves out, as a list of child pages names them (ADR-0240);
  - `HideKinds`: some of `done`, `label`, `outside` and `picture`, in this order — finished tasks (ADR-0250), labels, items of other rooms and pictures;
  - `Depth`: 1 to 3 for the neighbourhood of the item the text is on, as a separated neighbourhood reaches (ADR-0158), or 0 for the whole map.
  The server checks each, and the API sorts the lists and drops duplicates.
  The block is part of the sealed body (ADR-0026).
- **Format.** Codec version 70 adds the kind and the fields `Label`, `Search` and `HideKinds`; `Space`, `HideTypes` and `Depth` gain a meaning for it. Versions 1 to 69 are still read.
- **Which map.** Every room's map is read with the label from the server (`GET /api/starmap?label=`), so beyond the budget the label still narrows it before bubbles are drawn (ADR-0191); a room's map is read whole and narrowed to the label in the browser by the labels its nodes carry (`withLabel`), the item the text is on kept, as no route narrows a room's map by label.
  When the block asks for the neighbourhood and the map does not hold the item — left out by the label's narrowing elsewhere, by the budget or by another room chosen — the map says so and shows the whole map.
  When it holds the item, the item shows whatever the filters and the search say (`keep` in `filter` and `visible`), so a block that leaves out the item's own type still shows its neighbourhood; on the starmap's own page a station its filters hide still leaves nothing (ADR-0158).
- **Drawing.** `embeddedStarmap` in `assets/starmap.js` draws the starmap's own view in a mode for texts (`embed`): its element ids begin with a prefix of their own, so several maps share a page; the filters, the search and the drill path start from the block; there is no room chooser, panel or impact view; the address stays as it is; the filters and the legend fold below the map; the canvas is lower; the wheel zooms only with Ctrl, and touch scrolls the page vertically, so the page scrolls past the map; Escape answers only within the map.
  Items show the page tree's symbols and finished tasks are green (ADR-0250), with the types the page shares (`configureStarmaps`).
- **Reading.** Readers may change the search, the filters and the depth; nothing of that is kept, and the text shows the map as the block says again on the next visit.
- **A click** opens what a node stands for (`nodeHref`): an item, a picture's item, a label's page (ADR-0204), a bubble's room map. The keyboard does the same with Enter; Shift+Enter still separates a node.
- **Editing.** **Insert…** → **Starmap…** opens a dialog with the room, what the map shows, the label (suggested from the labels in use), the search text and the types and kinds to show; **Change the starmap…** changes it.
  A ticked box shows its type or kind, as in the map's own filters, every one ticked at first; the block keeps what is not ticked (`HideTypes`, `HideKinds`), and a type it leaves out that is no longer listed stays left out.
  The first version asked for the types to leave out, so ticking the types one wanted to see hid them.
  A line above the map says how it is narrowed.
- **Exports** name the map (`export.StarmapName`), as they name a view.
- **MCP** (ADR-0012): no tool is added; an agent writes the block with `commit_body`, whose description of blocks names the kind and its fields.

### Consequences

- **Positive:** a page explains a subject with its map beside it; each reader sees what they read; the map is the starmap's own, with its symbols, filters and physics.
- **Negative / trade-offs accepted:** a text with a map reads the starmap again whenever it is shown, within the budget `StarmapNodes`; a room's map narrowed to a label is first read whole; the label in a block is not renamed with a label, as a task list's is not; a map in a text has no panel and no impact view.
- **Follow-ups / risks to watch:** a route that narrows a room's map by label, should rooms grow past the budget; keeping a reader's filters; a picture of the map in exports.

## Pros and cons of the options

### Option 2: an embed of the starmap's address

Strongest form: the starmap's address already holds a room, a label and a drill path, so the block would keep one string and show whatever the starmap's page will ever learn, with no second set of fields to validate and no second mode to draw.

- Good: one place that knows what a map shows; new filters reach texts without a new codec version.
- Bad: the address holds neither the hidden types and kinds nor a search, and the neighbourhood of *this* item cannot be written as an address that survives a copy of the page; a frame would load the whole UI once per map, with its own sign-in, header and scrolling; and a string the server cannot check is shadow data in the body (D4).

### Option 3: a picture of the map, taken when saved

Strongest form: a picture reads anywhere — in exports, in mail, offline — costs nothing when shown, and looks the same to everyone.

- Bad: it shows the author's map to every reader, items they may not read included, which breaks D10; it goes stale as soon as an item changes; and it cannot be filtered or clicked, which the request asks for.

## Links

- relates to ADR-0157 (starmap of a room), ADR-0158 (live physics, separating a neighbourhood), ADR-0191 (starmap of all rooms), ADR-0203 (views in a text), ADR-0240 (types left out), ADR-0250 (symbols and finished tasks), ADR-0114 (task lists in a text), ADR-0025 (codec), ADR-0026 (sealed content)
