# ADR-0184: A presentation is imported into the album as slides with their notes

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: a PowerPoint importer that puts the slides as pictures into the album and their notes into the pictures' speaker's notes, through the album's Upload; asked how the slides become pictures and where they go, the requester chose the presentation file together with the slides exported as PNG, and a new folder per presentation)
- **Open question:** whether hidden slides should be imported as well, at the end or marked; whether a presentation imported again should replace the pictures of its folder as new versions instead of making a second folder; whether the server should one day draw the slides itself, so that the presentation file alone suffices
- **Question checked:** 2026-11

## Context and problem statement

The album presents a folder's pictures as slides, ordered by name, with speaker's notes kept on each picture (ADR-0176). Much of what people present already exists as PowerPoint presentations, with the notes written in them. Bringing such a deck into the album today means exporting every slide as a picture, uploading them, renaming them so their names keep the order, and copying the notes slide by slide.

A presentation file (Office Open XML, `.pptx`) is a ZIP archive of XML parts: the order of the slides, each slide's shapes and text, its notes page. It holds **no picture of its slides** — at most a thumbnail of the first. A faithful picture of a slide needs a layout engine that knows themes, masters, fonts, charts, diagrams and effects, as the presentation program or an office suite has.

How does a presentation become pictures in the album with its notes — given one binary in pure Go with no second executable (ADR-0009, ADR-0024), content only ever sealed on disk (ADR-0026), the UI's content security policy and no markup from strings (ADR-0014, ADR-0031), and nothing new kept for what can be derived?

## Decision drivers

- **Faithful slides.** A slide shown to an audience must look as its author made it; an approximation that loses a chart or a font is worse than no import.
- **One binary** (ADR-0009, ADR-0024): no office suite, no converter beside the binary, no C library.
- **Nothing new on the server that is not needed.** The album already takes pictures and notes through routes that check rights, seal content and scan it for sensitive data (ADR-0040); the import should use them.
- **The order and the notes survive.** The album orders by name (ADR-0176); the slides must come in their presentation's order, which is not the order of the slide files inside the package once slides have been moved.
- **Errors say what to do.** A mismatch between pictures and slides brings nothing in and tells how many of each were found.

## Considered options

1. **The presentation file together with the slides exported as pictures, read and paired in the browser; uploaded through the album's routes.**
2. **The presentation file together with its export as PDF, the pages drawn into pictures in the browser by a vendored PDF renderer.**
3. **The presentation file alone, its slides drawn in the browser by a renderer of our own** (text, pictures, simple shapes).
4. **The presentation file alone, converted by an external service** (an office suite behind an HTTP interface), named by an environment variable.

## Decision outcome

Chosen option: **1** (the requester's choice), because it gives slides exactly as the presentation program draws them without a new dependency, and needs no change to the server.

### How it works

- **Upload** in an open folder of the album, and dropping onto it, take a `.pptx` or `.ppsx` file besides pictures and ZIP archives. When the files chosen — on their own or inside ZIP archives — hold one presentation, they are a **slide import**; otherwise the import of ADR-0177 runs unchanged. More than one presentation at a time is refused.
- **Reading** (`assets/pptx.js`): the browser unpacks the presentation with `assets/zipread.js` and reads, with a small XML reader of its own, the package's main part, the slide list in its order (`p:sldIdLst` and the presentation's relationships), each slide's `show` attribute, the text of its title placeholder and its notes page: the text of every shape there except the slide's picture, number, header, footer and date, a line per paragraph. No markup is built from it; it is text throughout.
- **Pairing:** a presentation file holds no picture of its slides, so the pictures exported from the presentation program (*Slide1.PNG*, *Folie1.PNG*, …) are matched with the slides by the last number in their names: by number when they number every slide or the shown ones, in order when they number the shown slides one after the other, as an export that left the hidden ones out would; a single slide takes a single picture. Pictures that fit neither way bring nothing in, and the album says how many slides and pictures it found. Without pictures, the album explains how to export them.
- **Hidden slides** are left out, as a slide show leaves them out; the result says how many.
- **Where:** a folder named after the presentation's file is made below the open folder (`POST /api/album/folders`) and opens. Each shown slide's picture is uploaded into it (`POST /api/items/{ref}/attachments`) named by its place in the import, zero-padded, and its title, with the picture's extension — *03 Our goals.png* — cleaned of characters file names do not take and cut at 80 characters, so the album's order by name is the presentation's. The place choice of the import (ADR-0178) applies as to every upload.
- **Notes:** a slide's notes are set on its picture with `PUT /api/items/{ref}/attachments/{id}/notes` (ADR-0176) — sealed, scanned for sensitive data, shown in the presenter's window.
- **Nothing else** reaches the server: not the presentation file, not its other text. No new route, event, codec version or budget.

### What it does not take over

Animations, transitions, embedded videos and sounds, comments of the presentation and its sections. Older `.ppt` files (a binary format) are not read; they are saved as `.pptx` first.

### Consequences

- **Positive:** slides are exactly as the presentation program draws them; the deck is presentable at once with its notes; the server is unchanged, so rights, sealing and the scan for sensitive data hold as for every upload.
- **Negative / trade-offs accepted:** two exports where one would be wished — the presentation and its pictures; a picture is a picture, its text is not searchable as text and not editable; a renamed or renumbered export does not pair.
- **Follow-ups / risks to watch:** whether people find the export step; whether presentation programs other than PowerPoint name their exported pictures by slide number (the pairing in order covers consecutive numbers whatever their start).

## Pros and cons of the options

### Option 1: presentation file and exported pictures

Strongest form: the presentation program is the only faithful renderer of its own files, and every version of it exports all slides as pictures in one step; the import's work is then reading order and notes, which is plain XML.

- Good: faithful; no dependency; no server change; small code, tested in Node.
- Bad: two files to choose; pairing depends on the export's file names.

### Option 2: presentation file and PDF export, drawn by a PDF renderer

Strongest form: a PDF export is as faithful as a picture export, is one file instead of many, and a renderer such as pdf.js draws it in the browser at any resolution, also for decks made in other programs.

- Good: faithful; one export file; resolution chosen at import.
- Bad: a large new vendored bundle with a worker and its own policy needs, for one import; the PDF export leaves hidden slides out by default, so pages and slides must be matched by counting, and a PDF of notes pages or handouts pairs wrongly without notice. It still needs two files, so it saves little over option 1.

### Option 3: the presentation file alone, drawn by our own renderer

Strongest form: one file, one step; most slides are titles, bullet points and pictures, which a renderer of text boxes and images draws well enough.

- Good: the simplest for the person importing.
- Bad: the slides are not what the author made — themes, master graphics, theme fonts not installed in the browser, charts, diagrams, gradients and effects are missing or wrong, and the error is silent; it is a layout engine to maintain against a format of thousands of pages. This fails the first driver.

### Option 4: the presentation file alone, converted by an external service

Strongest form: an office suite converts faithfully, the person chooses one file, and the binary stays single because the suite runs elsewhere, as photo sources and process engines do (ADR-0129, ADR-0177).

- Good: one step, close to faithful.
- Bad: a further service to run and secure for one import; the presentation's whole content leaves the server for it, which the sealing of ADR-0026 is meant to avoid; an office suite's rendering of PowerPoint files is close but not identical. It may follow later behind a setting if option 1 proves too cumbersome.

## Links

- relates to ADR-0176 (presentations and speaker's notes), ADR-0177 (photo management, imports in the browser, `assets/zipread.js`), ADR-0178 (the place on import), ADR-0090 (the album's folders)
