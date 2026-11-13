# ADR-0186: A presentation is imported into the album as slides with their notes

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: a PowerPoint importer that puts the slides as pictures into the album and their notes into the pictures' speaker's notes, through the album's Upload; asked how the slides become pictures and where they go, the requester first chose the presentation file together with the slides exported as PNG, and a new folder per presentation; on 2026-11-11, finding two exports too cumbersome, the requester asked for the presentation file alone, chose a renderer of our own in the browser and kept the exported pictures as the exact way; on 2026-11-12 the requester asked for charts to be drawn from their stored values)
- **Open question:** whether hidden slides should be imported as well, at the end or marked; whether a presentation imported again should replace the pictures of its folder as new versions instead of making a second folder; whether radar, bubble, stock, surface and the newer chart kinds (waterfall, treemap, funnel) should be drawn too; whether an external converter (an office suite behind an HTTP interface) should be offered, behind a setting, for exact slides from one file
- **Question checked:** 2026-11

## Context and problem statement

The album presents a folder's pictures as slides, ordered by name, with speaker's notes kept on each picture (ADR-0176). Much of what people present already exists as PowerPoint presentations, with the notes written in them. Bringing such a deck into the album by hand means exporting every slide as a picture, uploading them, renaming them so their names keep the order, and copying the notes slide by slide.

A presentation file (Office Open XML, `.pptx`) is a ZIP archive of XML parts: the order of the slides, each slide's shapes and text, its layout, master and theme, its notes page. It holds **no picture of its slides** — at most a thumbnail of the first. A picture of a slide has to be drawn from that description, as the presentation program does when it shows it.

How does a presentation become pictures in the album with its notes, from one file dropped onto a folder — given one binary in pure Go with no second executable (ADR-0009, ADR-0024), content only ever sealed on disk (ADR-0026), the UI's content security policy and no markup from strings (ADR-0014, ADR-0031), and nothing new kept for what can be derived?

## Decision drivers

- **One step.** Dropping the presentation onto a folder must be enough; an export beforehand is a step people skip or get wrong (the requester's finding after the first version).
- **Slides close to the original**, and honest where they are not: what cannot be drawn is said, not silently dropped, and an exact way remains.
- **One binary** (ADR-0009, ADR-0024): no office suite, no converter beside the binary, no C library.
- **Nothing new on the server that is not needed.** The album already takes pictures and notes through routes that check rights, seal content and scan it for sensitive data (ADR-0040); the import should use them, and the presentation's content should not leave the browser for anything else.
- **The order and the notes survive.** The album orders by name (ADR-0176); the slides must come in their presentation's order, which is not the order of the slide files inside the package once slides have been moved.

## Considered options

1. **The presentation file together with the slides exported as pictures, read and paired in the browser; uploaded through the album's routes.**
2. **The presentation file together with its export as PDF, the pages drawn into pictures in the browser by a vendored PDF renderer.**
3. **The presentation file alone, its slides drawn in the browser by a renderer of our own.**
4. **The presentation file alone, converted by an external service** (an office suite behind an HTTP interface), named by an environment variable.

## Decision outcome

Chosen option: **3, with 1 kept as the exact way** (the requester's choice of 2026-11-11). Option 1 alone was the first version (#263): exact, but two exports proved too cumbersome. A renderer of our own gives the one step without a dependency, a service or content leaving the browser; where a slide holds what it cannot draw, the import says so and the exported pictures still give exact slides.

### How it works

- **Upload** in an open folder of the album, and dropping onto it, take a `.pptx` or `.ppsx` file besides pictures and ZIP archives. When the files chosen — on their own or inside ZIP archives — hold one presentation, they are a **slide import**; otherwise the import of ADR-0177 runs unchanged. More than one presentation at a time is refused.
- **Reading** (`assets/pptx.js`): the browser unpacks the presentation with `assets/zipread.js` and reads, with a small XML reader of its own, the slide list in its order (`p:sldIdLst` and the presentation's relationships), each slide's `show` attribute, the text of its title placeholder and its notes page: the text of every shape there except the slide's picture, number, header, footer and date, a line per paragraph.
- **Drawing** (the presentation alone): `assets/pptxscene.js` resolves each slide into a scene — the background of slide, layout or master; the shapes of the master and layout (not their placeholders, unless `showMasterSp` says otherwise) and of the slide, in drawing order; placeholders take position, body properties and list styles from their layout's and master's counterparts; text inherits by level from the presentation's default text style, the master's title, body or other style, the placeholders' and the shape's list styles, paragraph and run properties, a shape style's font colour lying over the inherited styles; colours go through the colour map and theme with their transforms; fills and lines come from the shape or the theme's style references; groups scale their children's frame; tables take the presentation's table styles, and PowerPoint's default style where the file keeps none; SmartArt is drawn from the drawing the presentation keeps beside its data. `assets/pptxdraw.js` draws the scene onto a canvas, 1,920 pixels on the long edge: preset shapes (the common ones; others as rectangles) and freeform paths, solid, gradient and picture fills, lines with dashes and arrow heads, pictures cropped and clipped, text wrapped, aligned, bulleted and numbered, anchored in its box and shrunk as its autofit asks. Fonts the browser lacks are replaced by metric-compatible ones where it has them (Carlito for Calibri; the vendored Liberation Sans for Arial). The slide becomes a PNG, a JPEG when the PNG would exceed 4 MB.
- **Charts** are drawn from the values the presentation keeps for them (`assets/pptxchart.js`): every chart part caches its categories, series names and numbers beside its reference into the embedded workbook, so the workbook itself is not read. Column and bar charts, clustered, stacked and stacked to 100 %, line charts with or without markers, area charts, pie and doughnut charts and scatter charts are drawn, their 3-D forms as flat ones, with title (a single series' name where the chart has none and has not deleted it), legend, value axis with round steps from zero unless the values lie far from it, gridlines, category labels and data labels in the chart's number formats; series and points take their own colours or the theme's accents in turn.
- **What is not drawn:** other chart kinds (radar, bubble, stock, surface, and the newer kinds stored apart, such as waterfall), embedded objects without a fallback picture, and pictures the browser cannot decode (EMF, WMF, TIFF) appear as grey boxes; the result says how many slides hold such things and that the exported pictures give exact slides. Effects (shadows, glow, 3-D), text on a path, WordArt transforms and right-to-left shaping are not drawn.
- **Pairing** (the presentation with exported pictures): the pictures exported from the presentation program (*Slide1.PNG*, *Folie1.PNG*, …) are taken instead of drawing, matched with the slides by the last number in their names: by number when they number every slide or the shown ones, in order when they number the shown slides one after the other; a single slide takes a single picture. Pictures that fit neither way bring nothing in, and the album says how many slides and pictures it found.
- **Hidden slides** are left out, as a slide show leaves them out; the result says how many.
- **Where:** a folder named after the presentation's file is made below the open folder (`POST /api/album/folders`) and opens; in a collection, inside the folder chosen, each slide labelled (ADR-0187). Each shown slide's picture is uploaded into it (`POST /api/items/{ref}/attachments`) named by its place in the import, zero-padded, and its title — *03 Our goals.png* — cleaned of characters file names do not take and cut at 80 characters, so the album's order by name is the presentation's. The place choice of the import (ADR-0178) applies as to every upload.
- **Notes:** a slide's notes are set on its picture with `PUT /api/items/{ref}/attachments/{id}/notes` (ADR-0176) — sealed, scanned for sensitive data, shown in the presenter's window.
- **Nothing else** reaches the server: not the presentation file, not its other text. No new route, event, codec version or budget; no new dependency.

### What it does not take over

Animations, transitions, embedded videos and sounds, comments of the presentation and its sections. Older `.ppt` files (a binary format) are not read; they are saved as `.pptx` first.

### Consequences

- **Positive:** one file, one step; the deck is presentable at once with its notes; the server is unchanged, so rights, sealing and the scan for sensitive data hold as for every upload; nothing of the presentation leaves the browser but its pictures and notes.
- **Negative / trade-offs accepted:** drawn slides are an approximation — fonts, line breaks and spacing can differ from the presentation program, charts are laid out by rules of our own rather than the presentation program's, and effects are missing; a renderer of our own is code to keep up with a large format; a picture is a picture, its text is not searchable as text and not editable.
- **Follow-ups / risks to watch:** how close real presentations come (tested so far with generated files only); which chart kinds and options people miss; which preset shapes people miss; whether option 4 is wanted for exact slides from one file.

## Pros and cons of the options

### Option 1: presentation file and exported pictures

Strongest form: the presentation program is the only faithful renderer of its own files, and every version of it exports all slides as pictures in one step; the import's work is then reading order and notes, which is plain XML.

- Good: exact; no dependency; no server change; small code, tested in Node.
- Bad: two files to choose, and an export beforehand; pairing depends on the export's file names. Kept as the exact way, not as the only one.

### Option 2: presentation file and PDF export, drawn by a PDF renderer

Strongest form: a PDF export is as faithful as a picture export, is one file instead of many, and a renderer such as pdf.js draws it in the browser at any resolution, also for decks made in other programs.

- Good: faithful; one export file; resolution chosen at import.
- Bad: a large new vendored bundle with a worker and its own policy needs, for one import; the PDF export leaves hidden slides out by default, so pages and slides must be matched by counting, and a PDF of notes pages or handouts pairs wrongly without notice. It still needs an export beforehand, which is what the requester found too cumbersome.

### Option 3: the presentation file alone, drawn by our own renderer

Strongest form: one file, one step; most slides are titles, bullet points, shapes, tables and pictures in the theme's colours and fonts, which follow rules the format writes down; drawing them in the browser needs no dependency and keeps the content where it is.

- Good: the simplest for the person importing; no dependency, no service; content stays in the browser.
- Bad: an approximation — fonts the browser lacks, the layout of charts, effects and rare shapes differ or are missing; a renderer to maintain against a format of thousands of pages. Accepted, because the import says what it could not draw and the exact way remains.

### Option 4: the presentation file alone, converted by an external service

Strongest form: an office suite converts faithfully, the person chooses one file, and the binary stays single because the suite runs elsewhere, as photo sources and process engines do (ADR-0129, ADR-0177); run beside the server, the content stays in the operator's hands.

- Good: one step, close to exact; little code here.
- Bad: a further service to run and secure for one import, without which the one-step import does not work; the presentation's whole content leaves the server for it; an office suite's rendering of PowerPoint files is close but not identical. Left as an open question, to be offered behind a setting if the renderer proves not close enough.

## Links

- relates to ADR-0176 (presentations and speaker's notes), ADR-0177 (photo management, imports in the browser, `assets/zipread.js`), ADR-0178 (the place on import), ADR-0187 (pictures added in a collection), ADR-0090 (the album's folders)
