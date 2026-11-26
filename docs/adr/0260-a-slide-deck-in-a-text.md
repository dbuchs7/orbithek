# ADR-0260: A slide deck in a text, kept as it was saved and presented from the text

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators. Request of 2026-11-26: a page holds a slide deck that is presented from the page itself. The requester first chose a block that names an album folder and follows it (option 1 below, landed with #403). The same day, after #403, the requester decided that a deck keeps the slides as they were when it was saved (option 2), and chose:
  - the slides, their order and their speaker's notes are kept;
  - a button updates a deck from its folder;
  - only kept decks remain, and decks written by #403 keep their slides on their next edit;
  - whoever may edit the text edits the notes of its slides.

  The rest are defaults proposed with the implementation.
- **Open question:** whether a presentation file (.pptx) should be imported from the block's dialog straight into the text rather than through a folder of the album; whether exports should show the slides as pictures rather than name the deck; whether the copies a deck keeps should stay out of the album's listing, where they show as pictures of the text; whether copies no version of the text shows any more should be cleaned up; whether a deck should offer a presentation that other browsers follow (ADR-0176 left it out too)
- **Question checked:** 2026-11

## Context and problem statement

The album presents the pictures of a folder or collection as slides, with speaker's notes, a presenter's window and a presentation window (ADR-0176), in an order of slides kept by the folder or collection (ADR-0195); a presentation file becomes such a folder in one step (ADR-0186).
The text that explains a subject — the page of a review, of a decision, of a project — is somewhere else: a reader who wants the slides leaves the page for the album, and a presenter starts from the album and returns there.
A text already shows content it does not hold in blocks: an embed shows another item (ADR-0020), a view lists items (ADR-0203), a starmap draws a map (ADR-0252).
A text that records what a board was shown must also show, in each of its versions, the slides shown then (ADR-0043), even when the album's folder changes later.

How does a text show a slide deck that a reader steps through and a presenter presents from the text, as it was when the text was saved, without a parallel model for decks (D1), and without showing anyone a picture they may not read (D9, D10)?

## Decision drivers

- **A version keeps its slides.** A version of the text shows the slides, their order and their notes as they were saved; a later change in the album does not reach it.
- **One model (D1) and stable identity (D2).** A deck is no new domain object; a block names what it shows by immutable IDs.
- **No shadow data (D4).** What a body implies is a deterministic function of the body.
- **Access (D9, D10).** The slides of a text are read and protected as the text is; only who reads a folder may copy from it.
- **The album stays the workshop.** Slides are made, ordered and annotated in the album (ADR-0176, ADR-0186, ADR-0195); a deck takes them from there and is updated from there.
- **Little new on the server.** The presentation, the import, the order and copying files with their origin exist (ADR-0054, ADR-0055).

## Considered options

1. **A block that names an album folder or collection and shows its pictures as they are now** (the first version, #403), implying the relation `embeds`.
2. **A block that keeps copies of the folder's pictures as files of the text's item, in its order and with their notes, and names the folder as the source it is updated from.**
3. **A block that names the folder and the exact versions of its pictures** (file IDs of another item), kept from deletion and clean-up.
4. **Pictures inserted one by one from the album** (ADR-0054), as a text can hold them today.

## Decision outcome

Chosen option: **2** (the requester's choice), because only copies the text owns keep a version's slides whatever happens in the album, under the text's own access, protection, backup and erasure; the album's copying and presentation are reused unchanged.

### The block

- The block kind **`slides`** (codec version 71) keeps its slides in the block field **`Files`** (codec version 72): IDs of files of the item, in the order of the slides, each once, at most `MaxSlides` (500). `Ref` names the album folder or collection the slides came from, the deck's **source**, by its ID (D2); `Text` is a plain caption.
- **Slides are files of the item.** `checkSlideDecks` accepts only the item's own files; one deleted since stays only while the current body shows it in a deck, as for a picture (ADR-0045). A new item cannot start with a deck, since its files come after it.
- **The source is no reference.** A deck implies no relation to its folder (`Block.Embedded` names only an embed's item), so the folder may be trashed and deleted while texts keep their slides (D8 does not hold it). A source named anew must be a live folder or collection that the writer reads; one kept from the current body may be gone. A deck whose source is gone keeps its slides and cannot be updated.
- **The slides are shown files** (`Block.shownFiles`): a protected deck protects its slides (ADR-0160), the album counts them as used, so clean-up keeps their versions (ADR-0103) and moving them to another item is refused, as for a picture the body shows (ADR-0188).
- **Decks of codec version 71** name a source and keep no files. A body may keep such a deck unchanged; it shows the folder as it is now. The editor keeps its slides as soon as the text is edited, and no body may newly hold a deck without slides.

### Keeping and updating

- **Inserting** a deck asks for a folder or collection and a caption, then copies, in the browser, every picture of the folder in its order of slides into the item (`keepSlides` in `assets/slidesblock.js`):
  - each file is downloaded with the reader's rights and uploaded to the item with the writer's, as a copy of its source (`sourceItem`, `sourceAttachment`, ADR-0055);
  - its speaker's notes are set on the copy, a video keeps its poster, and a turn made by hand is made on the copy too (`turnBetween`);
  - the editor reports how far copying has come; the deck holds the copies' IDs.
- **Updating:** when the folder's slides differ from those kept — another number, another picture at a place (by its SHA-256), other notes (`deckChanged`) — the deck says so. While editing, *Update from the folder* copies the folder's slides anew and puts the new copies in the deck. The previous version of the text keeps the copies it showed.
- **Changing** the deck's folder copies anew; changing only its caption keeps the slides.
- **Notes** of the kept slides belong to the copies: whoever may edit the text edits them, in the presenter's window or the album.

### How it is shown

- The browser lists the item's pictures in the album (`GET /api/album?folder=<item>`, the album's filter by item) and picks the very versions the deck names, in its order, with their stack's notes (`keptSlides`).
- The block shows a bar with the deck's name, the number of slides and a link to its source in the album; the current slide on black (`thumbnail?edge=1600`); a strip of every slide; the caption below. A video slide plays in place (ADR-0192).
- The reader steps with buttons, the strip, and — the deck focused — the keys of a presentation (`keyStep`). *Full screen* shows the slide alone.
- *Present* opens the album's presentation for the deck: the address names the item and the deck's place among the decks of its text (`deck`, `n`, `deckSourceOf`), and the presenter's window reads the slides the deck keeps. The order is the deck's: it is not dragged in the presenter's strip. Ending returns to the text (`back`, `backOf`).
- **Code:** `assets/slidesblock.js` draws the block, copies the slides and asks in the dialog; `assets/slidesparts.js` holds what needs no browser, tested in `jstest/slides.test.mjs`; the block in a page, its copies, updating and its presentation are tested in `e2e/smoke.cjs`.

### Elsewhere

- **Copying an item** (ADR-0067) gives its decks the copy's own files; a source the copier cannot read live is forgotten.
- **Restoring** keeps the slides that came back, forgets a source that is not there, and leaves out a deck without a slide.
- **Exports** name the deck by its caption and the number of its slides (*Slide deck: … (n slides)*, `export.SlidesName`).
- **MCP** describes the block in the body format: an agent uploads the copies with their source and names them in `files`.
- **No new route, event kind or budget**; `MaxSlides` is a shape of the block.

### What is kept

| What | Kept in | Changed by |
|---|---|---|
| The slides, their order | copies as files of the item; their IDs in the block, sealed with the body | updating from the folder, editing the text |
| Speaker's notes | each copy's first version (ADR-0176) | whoever edits the text, in the presenter's window or the album |
| The source, the caption | the block | editing the text |
| Current slide, full screen | the reader's browser | gone when the text is left |

### Consequences

- **Positive:** every version of a text shows the slides, order and notes it was saved with; a board's record stays what the board saw.
- **Positive:** the slides follow the text's access, protection, backup and erasure (ADR-0022, ADR-0026, ADR-0160); a folder may be reorganised or deleted without breaking a text.
- **Positive:** the album remains where slides are made; a deck is brought up to date with one button, and says when it is behind.
- **Negative / trade-offs accepted:** every keeping copies the files: a deck takes the space of its slides again, and each update adds copies, which earlier versions of the text still show.
- **Negative / trade-offs accepted:** the copies are pictures of the text's item and show in the album beside their originals.
- **Negative / trade-offs accepted:** copying runs in the editor's browser and takes as long as the downloads and uploads; an editor left before it ends keeps the copies made so far as unused files.
- **Negative / trade-offs accepted:** a deck kept while a text is edited by several people at once may be kept twice; the second keeping wins and the first copies stay unused.
- **Negative / trade-offs accepted:** codec versions 71 and 72; older binaries cannot read a data directory once a text holds a deck.
- **Follow-ups / risks to watch:** the open questions above; folders deeper than the dialog lists.

## Pros and cons of the options

### Option 1: a block that follows its folder

Strongest form: slides, notes and order have one home; a slide changed in the album is changed in every text at once, and nothing is copied. The reference is an embed, so deletion, impact, context and restore need no new rule.

- Good: no copies; always current; the first version, landed and tested.
- Bad: a version of the text shows today's slides, not those it was saved with, which is wrong for a text that records what was presented — the reason the requester changed the decision.
- Bad: the folder cannot be deleted while a text shows it, and a reader of the text who does not read the folder sees nothing.

### Option 3: the exact versions of the folder's pictures

Strongest form: no copies; a version of the text names the very files it showed, and a new version of a drawing in the folder leaves it untouched.

- Good: no space used twice.
- Bad: a body would name files of another item, a reference the model does not have; clean-up and deletion in the folder would have to spare them, so the folder's owners could no longer manage their own files.
- Bad: notes are kept on a stack's first version for every version (ADR-0176); they would still change, so the deck would be kept only in part.
- Bad: the slides would follow the folder's access, not the text's.

### Option 4: pictures inserted one by one from the album

Strongest form: it works today without any change; each picture is a copy that stays as it was inserted (ADR-0054).

- Good: nothing to build; a frozen record.
- Bad: no stepping, no presentation from the text, no notes, no order; a deck of thirty slides is thirty pictures in a row, and updating it is redoing it.

## Links

- relates to ADR-0020 (embedding), ADR-0043 (versions of a body), ADR-0045 (pictures of an item), ADR-0054 and ADR-0055 (inserting by copy, origins of copies), ADR-0090 (album folders), ADR-0160 (protected blocks), ADR-0175 (collections), ADR-0176 (presenting pictures), ADR-0186 (importing a presentation), ADR-0195 (order of slides), ADR-0252 (a starmap in a text)
