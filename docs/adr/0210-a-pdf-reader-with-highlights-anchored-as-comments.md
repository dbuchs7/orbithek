# ADR-0210: A PDF reader with highlights anchored as comments

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12: read PDF files in the browser, highlight passages and take notes on them)
- **Open question:** whether a text shows a PDF in a block of its own; whether the full-text search finds the text of PDFs; whether agents get a tool that lists an item's files
- **Question checked:** 2026-11

## Context and problem statement

A PDF is a file of an item (ADR-0045): stored encrypted beside the log, given out as a download that the server never shows as a page of this site. To read one, a person downloads it and opens it in another program; what they notice there — a passage that matters, a number to check — has no place in the wiki. Comments are part of their item (ADR-0035); an inline comment names a passage of the body by its quote (ADR-0047), a comment on a whiteboard names an element of a drawing by the drawing's first version (ADR-0171).

How does a person read a PDF of an item in the wiki, and where do their highlights and notes on it live?

## Decision drivers

- The product is one binary (ADR-0009, ADR-0024): what draws a PDF runs from the server's own origin, without a CDN, under the UI's content security policy (ADR-0014).
- The server never interprets a file as a page of this site (ADR-0045): a PDF must not become a document the browser runs with the site's cookies.
- A highlight is a remark on a place in a file: it is answered, resolved, deleted and erased like any comment, and its quote and note are content, sealed (ADR-0026).
- A highlight outlives a new version of the file where its text is still found, as a comment on a drawing outlives new versions of the drawing (ADR-0171).
- Comments stay type-agnostic (D6): a highlight names a file of an item, never the item's type.
- Unreadable is non-existent (D10), and a protected file is its protection's readers' alone (ADR-0160), its comments included.
- The persisted format changes only with a new codec version (ADR-0025).

## Considered options

1. **pdf.js in the browser, vendored; a highlight is a comment whose anchor names the PDF by its first version, a page, the quote and rectangles on the page.**
2. **The browser's own PDF viewer**, the file shown inline in a frame, with highlights kept beside it.
3. **Pages drawn by the server** as pictures, with the text extracted on the server.
4. **Highlights as a list of their own**, not comments, kept per person or per file.

## Decision outcome

Chosen option: **1**, codec version 58.

### The reader

- `#/pdf/{item}/{file}` shows a PDF file of an item, named by any of its versions; the reader shows the newest. `GET /api/items/{ref}/pdfs/{file}` answers the PDF's first version, its newest version and its versions, and is not found for a file that is no PDF (by the type the server read from its bytes, or its name) or one the caller may not have (ADR-0160). The file itself comes from its ordinary download (`GET /api/items/{ref}/attachments/{id}`), which stays a sandboxed download: pdf.js reads its bytes, the browser never opens it as a page.
- pdf.js (`pdfjs-dist` 6.4.299, Apache-2.0) is vendored as `internal/web/vendor` vendors bpmn-js and form-js: its prebuilt module and worker are copied unchanged to `static/vendor/pdf.js` and `pdf.worker.js` by `build.sh`, with what the worker loads on demand in `static/vendor/pdfjs/`: the standard fonts by Foxit, Adobe's character maps for East Asian text and the JPEG 2000 and JBIG2 decoders compiled to JavaScript. The UI's policy (`default-src 'self'`) admits the module worker from the same origin; the reader turns off evaluated code and WebAssembly (`isEvalSupported`, `useWasm`), so the policy needs neither `unsafe-eval` nor `wasm-unsafe-eval`, and colour profiles of images (qcms, WebAssembly only) are left out. The Liberation fonts of the package are left out for their license (GPL version 2 with a font exception); the browser's sans-serif stands in for a non-embedded Helvetica.
- Pages are drawn on canvases as they come into view, each with pdf.js's text layer over it (transparent spans pdf.js makes itself, ADR-0014), so text is selected and searched. The bar of tools moves by page, zooms by steps and to the width, searches the document's text (case and whitespace aside) and downloads the file. A side panel shows thumbnails, the document's table of contents when it has one, and the highlights. The page shown is in the address (`?page=`).
- The reader opens from the item's file list (*Read*), from a link in the item's text to one of its PDFs, from a highlight among the item's comments and from the comment overview (*Show in the PDF*, `?c=` naming the highlight). The album lists pictures, drawings and videos only; PDFs do not appear there.

### Highlights

- Whoever may comment on the item (who may edit it, ADR-0035) selects text on one page and highlights it in one of five colours (`item.HighlightColors`: yellow, green, blue, pink, purple), at once or with a note. A highlight is a comment: `item.Anchor` gains `File`, the PDF's first version; `Version`, the version it was made on; `Page`, from 1; `Rects`, the selection's rectangles in fractions of the page from its top left corner, one per run of a line, at most the budget `HighlightRects` (200); and `Color`. `Quote`, `Prefix` and `Suffix` hold the marked text with up to 32 characters around it. All of it is content, sealed with the anchor. The note is the comment's text and may be empty for a highlight, unlike any other comment.
- The store checks that `File` is the first version of a PDF of the item that is not deleted, `Version` one of its versions, the page positive, the rectangles finite and on the page, the colour one of the five, and that a highlight names a quote or a place. A highlight needs no body: a PDF is a file of any item. The server answers the anchor with `file` as the first version and `version` as the one named.
- A highlight is drawn where its rectangles say when the reader shows the version it was made on. On another version, or when an agent named only a page and a quote, the reader looks for the quote, whitespace aside and told apart by its prefix and suffix, on its page first and then on every page, and draws it where it is found; a highlight whose quote is gone says so and stays in the list, as a comment whose passage is gone does (ADR-0047).
- The side panel lists the PDF's highlights by page and place, with their notes and replies; one who may comment answers, resolves and deletes them there as on the item's page. *Export as Markdown* writes them in the browser, a heading per highlight with its page, the quote as a quotation, the note with its author and the replies.
- On the item's page a highlight shows as *Highlighted in {file}, page {n}* with its quote and *Show in the PDF*; it is a thread like any other, never marked in the body.
- **Protected files:** a comment on a file the reader may not have — a highlight in a protected PDF, and now also a comment on an element of a protected drawing (ADR-0171) — is left out with its replies wherever comments are read: the item's comments, the item as read, the comment overview, search, queries and mentions (`Store.commentsOnHiddenFiles`). A deleted comment keeps the file it was on, so its replies stay hidden. Today no block shows a PDF, so a PDF is protected only with its item (ADR-0078); the rule covers the day one does.
- A PDF with highlights stays in its item: moving the file (ADR-0188) is refused, as for a drawing with comments.
- **MCP:** `list_comments` reads highlights; `add_comment` takes the anchor `{file, page, quote, color}`. The reader's route is not offered as a tool: it serves a browser that draws pages.

### Consequences

- **Positive:** threads, replies, resolving, deleting, the comment overview, mentions, notifications and erasure work for highlights without new code; a highlight survives every new version of the file where its text is found.
- **Positive:** nothing new is stored on the server but comments; the reader is computed in the browser from the file and the comments.
- **Negative / trade-offs accepted:** about 4 MB more in the binary (pdf.js 0.5 MB, its worker 1.3 MB, character maps 1.7 MB, decoders 0.6 MB, fonts 0.2 MB).
- **Negative / trade-offs accepted:** the codec changes; older builds cannot read data written by this one (ADR-0025).
- **Negative / trade-offs accepted:** a scanned PDF without a text layer cannot be highlighted: the reader highlights selected text only, although the anchor admits a place without a quote.
- **Negative / trade-offs accepted:** a copy of the PDF into another item (ADR-0054) does not take its highlights along, as a copied drawing does not take its comments.
- **Follow-ups / risks to watch:** pdf.js updates often; an update is a change of `package.json` and `build.sh`'s copy, checked by the asset and notice tests and `e2e/smoke.cjs`. The text layer's look follows pdf.js's viewer stylesheet, adapted in `style.css`, and may need adapting with an update.

## Pros and cons of the options

### Option 2: the browser's own viewer

Strongest form: no code to ship or update, the best rendering each browser has, printing and forms included.

- Bad: the file would have to be served inline as `application/pdf` from the site's origin, against ADR-0045; the viewer is a black box: no selection, page or position reaches the page, so highlights could not be drawn or made; browsers differ, and some show no PDFs at all.

### Option 3: pages drawn by the server

Strongest form: the browser receives pictures only; nothing of the file runs in it, and the text could also feed the full-text search.

- Bad: a PDF renderer in pure Go does not exist at the quality needed, and a C library breaks the single binary without CGO (ADR-0004, ADR-0009); every page at every zoom costs the server time and memory; selecting text over a picture needs the positions of every glyph anyway.

### Option 4: highlights apart from comments

Strongest form: private highlights, as many readers keep them, without filling the item's comments.

- Bad: a second lifecycle for what comments already have — answering, resolving, deleting, erasing, notifying — and a second place for remarks on an item; personal notes (ADR-0107) remain for what one keeps to oneself.

## Open questions

- **A PDF in a text:** a block kind that shows a PDF's first page and opens the reader would let a text show a PDF as it shows a picture; it touches the editor, the exports, protected blocks (ADR-0160) and the codec. Until then a text links to the PDF.
- **Full-text search of PDFs:** the search (ADR-0033) scans the in-memory state; a PDF's text would have to be extracted (in the browser on upload, or on the server) and kept, sealed, as content of the item.
- **Agents and files:** no MCP tool lists an item's files, so an agent names a PDF whose file it was given or read in a highlight.
- **Private highlights:** whether a person may keep highlights to themselves, as personal notes (ADR-0107), rather than as comments everyone who reads the item sees.
- **Versions:** the reader shows the newest version; reading an older version with its highlights where they were made is not offered.

## Links

- extends [ADR-0047](0047-inline-comments-anchored-by-quoted-text.md) and [ADR-0171](0171-comments-on-the-elements-of-a-drawing.md) (anchors), [ADR-0035](0035-comments-are-part-of-their-item.md) (comments)
- relates to [ADR-0045](0045-attachments-encrypted-files-beside-the-log.md) (files), [ADR-0160](0160-protected-blocks-and-pictures-of-a-page.md) (protected files), [ADR-0129](0129-bpmn-diagrams-and-processes-from-a-process-engine.md) and [ADR-0144](0144-forms-and-decisions-from-a-process-engine.md) (vendored viewers), [ADR-0188](0188-a-moved-file-keeps-its-identity.md) (moving files), [ADR-0025](0025-event-log-file-and-in-memory-state.md) (codec), [ADR-0015](0015-resource-budgets-and-paged-listings.md) (budgets)
