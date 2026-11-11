# ADR-0176: Pictures of the album are presented as slides, with speaker's notes, in two windows

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: a presentation mode for drawings and photos of the album, in a second browser window so that the presenter reads their notes while presenting, with a laser pointer, zoom and stepping forward and back; asked where the notes are kept, which pictures form a presentation and who watches, the requester chose a field of its own on the picture, the pictures of a folder ordered by name, and a second window of the same browser; accepted 2026-11-10)
- **Open question:** whether a presentation also needs an order of its own, apart from the names of its pictures; whether people elsewhere should follow it in their own browsers, as a whiteboard's presentation is followed (ADR-0170); whether the presentation window should place itself on the second screen where the browser allows it
- **Question checked:** 2026-11

## Context and problem statement

The album (ADR-0054) holds the drawings and photographs of every item one may read, in folders (ADR-0090). People make slides as drawings or keep them as pictures and want to show them to an audience: full screen on a projector, while they themselves read what they mean to say, step forward and back, point at a detail and enlarge it.

A whiteboard already has a presentation (ADR-0170): its frames are the slides and other participants follow the presenter. That covers one drawing and a live session on the server; it has no notes, and it shows nothing of a folder of photographs.

Which pictures form a presentation and in which order, where are the speaker's notes kept, and how do the presenter's view and the audience's view stay together — given one binary (ADR-0024), the UI's content security policy (ADR-0014, ADR-0031), content only ever sealed on disk (ADR-0026) and nothing revealed that a caller may not read (D10)?

## Decision drivers

- **Notes are kept.** A presentation is given more than once; what the presenter writes for a slide must still be there the next time and on another computer.
- **Notes are content** (ADR-0026, ADR-0040): sealed in the log, erased with their item, scanned for sensitive data like every text of an item.
- **A drawing keeps its notes when it is drawn again.** Every save of a drawing is a new version of the file (ADR-0103); notes on one version would be lost with the next.
- **Access unchanged** (ADR-0022, ADR-0160): a presentation shows only pictures the presenter sees in the album; notes are written by those who may edit the picture.
- **Nothing new on the server for what is momentary.** The current slide, the zoom and the laser pointer last as long as the presentation; they are not content.
- **The UI's strings come from the catalogue** (ADR-0017); the UI creates elements only with `h()` (ADR-0014).

## Considered options

1. **Notes as a sealed field of the picture, on its first version; the album's pictures as slides by name; two windows of one browser talking over a `BroadcastChannel`.**
2. **Notes in the text of the album's folder**, a section per slide headed by the picture's name.
3. **Notes only in the presenter's window**, typed there and kept in the browser.
4. **The presentation as a live session on the server**, which the presentation window, and anyone else, follows.

## Decision outcome

Chosen option: **1** (the requester's choices).

### Speaker's notes

- A picture's **notes** are plain text, a new field `Notes` of the attachment, set with the event kind `AttachmentNotesSet` (codec version 44). Like a stack's own rule for its versions (ADR-0104), they are kept on the **first version** of the file: every version shows them, and they stay when a drawing is saved again or when its first version is deleted. Any version of the stack names it.
- They are **content**: sealed with the rest of the attachment, removed by `Event.erase`, and read by the scan for sensitive data (ADR-0040) — notes with a password restrict their item at once, and the governance report names them.
- **Who:** whoever may edit the picture's item writes them, with `PUT /api/items/{ref}/attachments/{id}/notes` (`{notes}`; empty clears them); line ends become `\n` and the notes are trimmed. Whoever sees the picture in the album reads them: `GET /api/album` gives a stack's `notes`.
- **Bound:** the request body's budget (`RequestBody`, ADR-0015), as for comments; no bound of their own.
- **Where they are written:** on the album's card (*Add speaker's notes*), and in the presenter's window, beside the slide.

### Which pictures, in which order

- *Present* in the album shows the pictures **the album shows now**: an open folder's or an open collection's (ADR-0175), narrowed by its filters (text, kind, label), or the whole album so narrowed. Of each stack the newest version is the slide.
- The slides are **ordered by name**, numbers by their value (`2 Plan` before `10 Outlook`); pictures of the same name by item and file, so the order is the same in every window. Whoever wants another order renames the pictures.

### Two windows

- The click on *Present* opens the **presentation window** — a browser opens a window only on a user's click — and turns the album's window into the **presenter's window**. Both take the whole window, without the top bar.
- **The presenter's window** shows the current slide, the next one, the notes, the time since the start, the time of day, every slide as a strip to jump to, and whether the presentation window is connected. It can open the presentation window again.
- **The presentation window** shows the slide alone on black and goes full screen on a click or `F`, which only a gesture in that window allows. It shows *waiting* until the presenter's window answers.
- The two talk over a **`BroadcastChannel`** named by a random id of the presentation, within this browser only. The presenter's window holds the state — the deck, the slide, zoom and centre, laser and black screen — and sends all of it on every change; the presentation window asks for it when it opens and sends what its keys and pointer do. Messages are checked before they count. Nothing of it goes to the server, and no new route, event or budget is needed for it.
- Ending the presentation closes the presentation window and returns to the album.

### Tools

- **Stepping:** arrows, page up and down (the keys of presentation remotes), space, Home and End, the buttons and the strip; a click on the presentation window moves on. A new slide starts unzoomed.
- **Zoom:** `+`, `−` and `0`, the buttons, the mouse wheel about the point it is on, dragging a zoomed slide, a double click back to the whole slide. The centre is a point of the picture, from 0 to 1, so both windows show the same part of the slide, whatever their size; neither shows past the picture's edge.
- **Laser pointer:** `L` or the button; the pointer's spot over the slide is sent as a point of the picture and drawn in both windows as a red dot with a short fading trail. It works from either window.
- **Black screen:** `B` blacks the presentation window out, and back.
- **Code:** `assets/present.js` draws; its geometry, order, keys and messages are plain functions in `assets/present-parts.js`, tested in Node (`jstest/present.test.mjs`); the two windows are tested in `e2e/smoke.cjs`.

### What is kept

| What | Kept in | After the presentation |
|---|---|---|
| Speaker's notes | the picture's first version, sealed | in the album, for every version |
| Order of the slides | the pictures' names | as the names are |
| Current slide, zoom, laser, black | the presenter's window | gone |

### Left out of this decision

- Following a presentation from other browsers or computers (option 4).
- Placing the presentation window on the second screen by itself (the Window Management API needs a permission and is not offered by every browser).
- The album and the notes for agents over MCP: the album is not offered there (ADR-0012); `internal/mcp/tools.go` says so.

### Consequences

- **Positive:** notes travel with the picture, under its access and encryption, and survive new versions of a drawing.
- **Positive:** the presentation itself adds no route, no persisted state and no load on the server; the slides come by the same downloads the album makes.
- **Negative / trade-offs accepted:** both windows must be in one browser on one computer; a presenter on a laptop with a projector has that, a presentation to remote viewers needs screen sharing.
- **Negative / trade-offs accepted:** the order is the names' order; renaming a picture is the way to move it. A picture copied into another folder (ADR-0108) is a file of its own and has no notes.
- **Negative / trade-offs accepted:** a drawing is shown as the PNG it was saved as; zooming far into it enlarges pixels, as the picture in a page does.
- **Negative / trade-offs accepted:** whoever reads a picture reads its notes. Notes that must stay with fewer people belong in a picture of an item restricted to them (ADR-0022, ADR-0078).
- **Follow-ups / risks to watch:** an order of slides kept apart from the names; notes as rich text; a presentation followed by others; the second screen chosen by the browser.

## Pros and cons of the options

### Option 2: notes in the folder's text

Strongest form: no change to the persisted format; the notes are an ordinary page, edited with the editor, versioned and commented like any text; a folder becomes a document of its presentation.

- Good: no new event kind, no codec version.
- Bad: a folder has no body (ADR-0090); it would need one, and the album's folders would become pages of another shape.
- Bad: notes are tied to slides by names; renaming a picture loses its notes silently, and the same picture in a filtered view of the whole album has no folder text at all.
- The requester chose against it.

### Option 3: notes only in the presenter's window

Strongest form: nothing on the server, nothing persisted; the simplest to build.

- Bad: the notes are lost with the browser's storage and are not there on another computer or for a colleague giving the same talk.
- The requester chose against it.

### Option 4: a live session on the server

Strongest form: anyone may follow from anywhere, as on a whiteboard (ADR-0170); the presenter's window and the presentation window need not share a browser.

- Good: remote viewers without screen sharing.
- Bad: sessions, a stream of events, budgets and access checks per viewer for what lasts an hour and is content of nobody; a viewer would need to read every picture of the deck, which the server would have to decide per slide.
- Bad: the second window on the presenter's own computer, the case asked for, gains nothing from it.
- The requester chose a second window of the same browser; it remains open as a follow-up.

## Links

- amends [ADR-0054](0054-an-album-of-readable-pictures-inserted-by-copy.md) (the album)
- amended by [ADR-0192](0192-videos-as-slides-and-from-google-photos-and-icloud.md) (videos as slides)
- amended by [ADR-0195](0195-the-slides-of-a-folder-or-collection-are-ordered-by-hand.md) (an order of slides set by hand)
- relates to ADR-0026 (sealed content), ADR-0175 (collections of the album), ADR-0040 (sensitive data), ADR-0090 (folders and labels of pictures), ADR-0103 (versions in the album), ADR-0104 (the rule of a stack on its first version), ADR-0170 (a whiteboard's presentation), ADR-0173 (exporting pictures)
