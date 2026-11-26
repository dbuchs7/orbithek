# ADR-DRAFT: A slide deck in a text, shown from an album folder and presented from the text

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26: a page holds a slide deck that is presented from the page itself. Of the three ways laid out to the requester — a block that names an album folder, the slides as files of the page itself, and pictures inserted one by one from the album as today — the requester chose the block that names an album folder (option 1 below) the same day. The rest are defaults proposed with the implementation.
- **Open question:** whether a deck should follow its folder (as decided here) or keep the slides as they were when the text was saved, so that an older version of the text shows the slides presented then; whether a presentation file (.pptx) should be imported from the block's dialog straight into a new folder rather than in the album; whether exports should show the slides as pictures rather than name the deck; whether a deck should offer a presentation that other browsers follow (ADR-0176 left it out too)
- **Question checked:** 2026-11

## Context and problem statement

The album presents the pictures of a folder or collection as slides, with speaker's notes, a presenter's window and a presentation window (ADR-0176), in an order of slides kept by the folder or collection (ADR-0195); a presentation file becomes such a folder in one step (ADR-0186).
The text that explains a subject — the page of a review, of a decision, of a project — is somewhere else: a reader who wants the slides leaves the page for the album, and a presenter starts from the album and returns there.
A text already shows content it does not hold in blocks: an embed shows another item (ADR-0020), a view lists items (ADR-0203), a starmap draws a map (ADR-0252).

How does a text show a slide deck that a reader steps through and a presenter presents from the text, without a second home for slides, notes and their order, without a parallel model for decks (D1), and without showing anyone a picture they may not read (D10)?

## Decision drivers

- **One home for slides.** Slides, their order and their speaker's notes are kept by the album already (ADR-0176, ADR-0195); a second place for them would drift apart from the first.
- **One model (D1) and stable identity (D2).** A deck is no new domain object; a block names what it shows by its immutable ID.
- **No shadow data (D4).** What a body implies is a deterministic function of the body; the body holds no copy of a picture.
- **Access (D9, D10).** Every reader sees the pictures they read and nothing of what they do not read; only who reads a folder may place it in a text.
- **No deletion while referenced (D8).** A folder a text shows must not vanish from under it.
- **Little new.** The presentation, the import and the order exist; the server should need no new route or event for the deck itself.

## Considered options

1. **A block of the kind `slides` that names an album folder or collection by its ID, implies the relation `embeds` to it and shows its pictures as slides, read with each reader's rights; *Present* opens the album's presentation, which returns to the text.**
2. **The slides as files of the text's own item**, shown and presented from the text.
3. **Pictures inserted one by one from the album** (ADR-0054), as a text can hold them today.

## Decision outcome

Chosen option: **1** (the requester's choice), because it reuses the album's slides, notes, order, import and presentation unchanged, and the block's reference is an embed in all but its look, so deletion, impact, context and restoring treat it as one.

### The block

- The block kind **`slides`** names an album folder or collection in `Ref`, by its ID (D2), and has a plain caption in `Text`; it has no marks, mentions, alignment or other field. Codec version 71 (`item.Slides`).
- A text may hold it only when `Ref` names an item of the type `folder` (ADR-0090) or `collection` (ADR-0175): `checkSlideDecks` refuses anything else, on every commit and on creation. The type of an item never changes, so a body that passed once keeps passing.
- Like an embed, the block **implies the relation `embeds`** to its folder (`Block.Embedded`, used by `Doc.refs` and `refSecrets`). It follows that:
  - only who reads the folder may place it in a text, and a new reference to an item in the trash is refused, as for every reference (ADR-0022);
  - the folder is not trashed while a live text shows it (D8, ADR-0021);
  - the impact view and the context of the folder name the text (ADR-0159, ADR-0207);
  - a protected block (ADR-0160) keeps the relation from those who do not read it;
  - a restore leaves out a deck whose folder did not come back, as it leaves out such an embed.
- A text with a deck is not empty (governance, ADR-0040).

### How it is shown

- The browser reads the folder or collection (`GET /api/items/{ref}`) and its pictures in its order of slides (`GET /api/album?folder=…&order=slides`, or `collection=`, ADR-0195), page by page, with the reader's rights: a reader sees the pictures they read, and a folder they do not read shows as *cannot be shown*, worded the same as a folder that is gone (D10).
- The block shows a bar with the deck's name, the number of slides and a link to the folder in the album; the current slide on black, scaled and turned upright by the server (`thumbnail?edge=1600`, ADR-0177, ADR-0231); a strip of every slide; the caption below. A video slide plays in place (ADR-0192).
- The reader steps with buttons, the strip, and — the deck focused — the keys of a presentation: arrows, page up and down, space, Home and End (`keyStep` in `assets/slidesparts.js`). *Full screen* shows the slide alone.
- *Present* opens the album's presentation — the presenter's window with notes, the next slide, the time and the strip, and the presentation window for the projector (ADR-0176) — for the deck's folder or collection. The address carries the item the text is on (`back`, a key or ID, checked by `backOf` in `assets/present-parts.js`); ending the presentation returns to that text rather than to the album. Whoever may change the order drags the strip in the presenter's window as before (ADR-0195).
- While editing, the slides show without *Present*, beside *Change the slide deck…*; the dialog lists the folders the reader reads (down to four levels below the top, at most 300 with the collections, the rest reached through the album), previews the first slides, and takes the caption.
- **Code:** `assets/slidesblock.js` draws the block and the dialog; `assets/slidesparts.js` holds what needs no browser, tested in `jstest/slides.test.mjs`; the block in a page and its presentation are tested in `e2e/smoke.cjs`.

### Elsewhere

- **Exports** name the deck by its caption (*Slide deck: …*, `export.SlidesName`); they do not show the slides, which each reader reads with their own rights.
- **MCP** describes the block in the body format; agents name the folder by its ID. The album itself stays out of MCP (ADR-0012, ADR-0176).
- **Nothing new on the server** for showing or presenting: no route, no event kind, no budget. The deck's slides come by the album's listing and downloads.

### What is kept

| What | Kept in | Changed by |
|---|---|---|
| Which folder or collection, the caption | the block in the body, sealed with it | editing the text |
| The slides | the folder's pictures | the album |
| Their order | the folder or collection (ADR-0195) | the album, the presenter's strip |
| Speaker's notes | each picture's first version (ADR-0176) | the album, the presenter's window |
| Current slide, full screen | the reader's browser | gone when the text is left |

### Consequences

- **Positive:** a deck is presented from the text that explains it, with every tool of the album's presentation, and the presenter is back in the text afterwards.
- **Positive:** slides, notes and order have one home; a slide changed in the album is changed in every text that shows the deck.
- **Positive:** the reference is an embed for deletion, impact, context, protection and restore; no part of the server learned a new kind of reference.
- **Negative / trade-offs accepted:** a deck follows its folder: an older version of the text shows today's slides, not those it showed when it was saved (see the open question).
- **Negative / trade-offs accepted:** the folder and the text have their own access; a reader of the text who does not read the folder sees that the deck cannot be shown.
- **Negative / trade-offs accepted:** a folder shown in a text cannot be trashed until the text no longer shows it; the album's refusal names the text.
- **Negative / trade-offs accepted:** codec version 71; older binaries cannot read a data directory once a text holds a deck.
- **Follow-ups / risks to watch:** the open questions above; folders deeper than the dialog lists.

## Pros and cons of the options

### Option 2: the slides as files of the text's own item

Strongest form: a text is complete in itself — its access, its export, its copy, its trash and its history cover the slides with the words; there is no second place to keep in step, and a version of the text could keep the slides it showed.

- Good: one item, one access, one history; copying or exporting the text takes the slides with it.
- Bad: the order of slides and the presentation are kept by folders and collections (ADR-0195); a text would need an order of its own, a second implementation of what the album has, or a new kind of container — a parallel model the album was made to avoid.
- Bad: a deck used in two texts is two copies, whose notes and order drift apart.
- Bad: a text's files and its other pictures mix; which files are slides would be data of its own.

### Option 3: pictures inserted one by one from the album

Strongest form: it works today without any change; each picture is a copy that stays as it was inserted (ADR-0054), so a text keeps the slides it showed.

- Good: nothing to build; a frozen record of what was shown.
- Bad: no stepping, no presentation from the text, no notes, no order; a deck of thirty slides is thirty pictures in a row.
- Bad: a slide changed in the album is not changed in the text; the copies learn of a changed original (ADR-0055) but are updated one by one.

## Links

- relates to ADR-0020 (embedding), ADR-0054 and ADR-0090 (album and folders), ADR-0175 (collections), ADR-0176 (presenting pictures), ADR-0186 (importing a presentation), ADR-0195 (order of slides), ADR-0252 (a starmap in a text)
