# ADR-0173: Pictures of the album export as PNG and JPEG, drawings also as SVG

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09: an export of pictures from the album for the people entitled to it, at least as PNG and JPEG and perhaps as SVG; asked who is entitled, the requester chose everyone who may read a picture, and SVG for drawings only)
- **Open question:** whether an organisation needs exports limited to a role and recorded, although every reader receives a picture's bytes to see it; and whether drawings should also export as PNG and JPEG drawn larger from their scene
- **Question checked:** 2026-11

## Context and problem statement

The album (ADR-0054) shows the images and drawings of every item one may read. A picture can be copied into another item (ADR-0108) or opened as its file, in the format it was stored in. People also need a picture outside the wiki, in a format the receiving program takes: a slide, a document, a print shop, a diagram tool. A drawing exists only as a PNG that carries its scene (ADR-0053); a version that scales without loss does not exist.

Who may take a picture out, in which formats, and where is it converted, given one binary (ADR-0024), the UI's strict content security policy (ADR-0014, ADR-0031) and the rule that nothing reveals what a caller may not read (D10)?

## Decision drivers

- **Access unchanged (ADR-0022, ADR-0160):** an export gives no one a picture they could not already see in the album; a protected block's file stays with the protection's readers.
- **One binary, no new dependency (ADR-0009, ADR-0024):** no converter next to the server.
- **Policies unchanged:** the UI keeps `default-src 'self'`; Excalidraw runs on the drawing page only, under its own policy (ADR-0053), which allows no code compiled from strings.
- **Fidelity:** a file already in the asked format leaves as stored; a drawing as SVG is drawn in lines and shapes, not as pixels.
- **What a right can hold:** the browser receives a picture's bytes in order to show it. Whatever governs the export button does not govern those bytes.

## Considered options

1. **Export in the browser from the download the reader already makes:** a canvas writes PNG and JPEG, the drawing page draws a drawing's scene as SVG; everyone who sees a picture in the album may export it.
2. **A server route that converts pictures** (`GET /api/items/{ref}/attachments/{id}/export?format=`), in Go.
3. **An export right of its own** — a setting or a role that decides who is offered the export.
4. **SVG for every picture**, a raster image wrapped in an SVG `image` element.

## Decision outcome

Chosen option: **1**, with SVG for drawings only (the requester's choices).

- **Who:** whoever sees a picture in the album, that is, who may read its item and, for the file of a protected block, the protection (ADR-0160). The bytes come from `GET /api/items/{ref}/attachments/{id}` with the reader's session, the same request that shows the picture. No route, no permission, nothing persisted and no MCP tool is added.
- **Where:** every picture of the album, the older versions of a stack included (ADR-0103), has **Export** with *PNG* and *JPEG*, a drawing also *SVG*. The album dialog that inserts a picture into a text does not offer it.
- **PNG and JPEG:** a file already in the format is saved as stored, so a drawing's PNG keeps its scene and opens in the drawing editor again. Otherwise the picture is drawn on a canvas at its own size and written anew: JPEG with quality 0.92 on white, as JPEG has no transparency; an animated GIF gives its first frame.
- **SVG:** a hidden frame loads the drawing page and hands it the drawing (`{type: "svg", scene}`); the page reads the scene from the PNG and draws it with Excalidraw's `exportToSvg` — light, on its background and with its margin, as the picture in pages — and answers with the file (`{type: "svg", blob}`). The fonts of its text are embedded as data, so the file shows the same elsewhere; the scene is not embedded. The drawing bundle exports `exportToSvg` for this.
- **Fonts:** Excalidraw cuts embedded fonts down to the glyphs used with a decoder that compiles code from strings. The drawing page's policy forbids that, so the attempt fails, is reported as a policy violation and the whole font file is embedded anyway. `build.sh` changes the bundle to embed the font files without trying, as it already removes the fonts' CDN fallback; the build fails when the code it replaces is not found exactly once. The policy stays as it is.
- **File name:** the picture's name without its extension or a drawing's marker, with `.png`, `.jpg` or `.svg`; characters file systems refuse become a dash.
- **Code:** `assets/picexport.js`; its decisions (formats, names, stored or converted) are tested in Node (`jstest/picexport.test.mjs`), the conversions in `e2e/smoke.cjs`.

### Consequences

- **Positive:** no server code, no new dependency, no change to the persisted format; who may export is exactly who may see.
- **Positive:** a drawing leaves as a vector that scales without loss, and as a PNG that stays editable.
- **Negative / trade-offs accepted:** there is no export right. Everyone who reads a picture can export it, as they could download it before. An organisation that must keep a picture in the wiki restricts reading it (ADR-0022, ADR-0078, ADR-0160).
- **Negative / trade-offs accepted:** exports are not audited. The server cannot tell an export from a view; both are the same download.
- **Negative / trade-offs accepted:** a photograph exported in its own format keeps its metadata (such as EXIF location), as the download does; a converted file drops it.
- **Negative / trade-offs accepted:** a drawing whose whiteboard the server saved without a browser (ADR-0169) carries a scene newer than its pixels; its SVG shows the scene, its PNG and JPEG the stored pixels.
- **Negative / trade-offs accepted:** an SVG with text holds the whole font files its characters need, a few to some tens of kilobytes each, instead of the glyphs alone.
- **Negative / trade-offs accepted:** PNG and JPEG keep the stored size; a drawing is as large as Excalidraw saved it.
- **Follow-ups / risks to watch:** the same export on an item's file list and on a picture in the reading view; PNG and JPEG of a drawing drawn from its scene at a chosen scale. An Excalidraw update must keep the build's font change finding its code (the build fails otherwise).

## Pros and cons of the options

### Option 2: a server route

Strongest form: one route serves the UI, the API and agents alike; the conversion is deterministic and tested in Go; the server could record each export.

- Good: works without a browser, also for MCP clients.
- Bad: WebP is not decoded by the standard library; it needs `golang.org/x/image`, a new dependency.
- Bad: a drawing's SVG cannot be made on the server without a second renderer of Excalidraw's scene; SVG would stay in the browser anyway.
- Bad: it re-encodes what the reader may already download, so it adds code and no protection; a record of exports would be one of the button, not of the bytes.

### Option 3: an export right of its own

Strongest form: an organisation decides who may take pictures out; a missing button discourages it, and a record shows who did.

- Bad: it governs the button, not the file. The album, the page and the download hand every reader the bytes; holding them back would mean not showing the picture.
- Bad: a permission outside the roles and restrictions of ADR-0022, for one capability.
- The requester chose against it.

### Option 4: SVG for every picture

Strongest form: one format for every picture, for programs that take only SVG.

- Bad: the SVG would hold the same pixels, a third larger in base64 and no sharper; it suggests a drawing in lines where there is none.
- The requester chose against it.

## Links

- amends [ADR-0054](0054-an-album-of-readable-pictures-inserted-by-copy.md) (the album)
- relates to ADR-0022 (permissions), ADR-0053 and ADR-0126 (drawings, the Excalidraw bundle), ADR-0071 (exporting pages and lists), ADR-0103 (versions in the album), ADR-0108 (copying from the album), ADR-0160 (protected pictures), ADR-0169 (whiteboards)
