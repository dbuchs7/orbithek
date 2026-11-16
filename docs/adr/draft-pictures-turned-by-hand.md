# ADR-DRAFT: Pictures and drawings turned by hand, and the orientation written into the files given out

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-16
- **Deciders:** Project initiators (request of 2026-11-16 that photos and drawings in the album can be turned by 90°; asked where, how it is kept and what happens to a drawing, the requester chose the viewer and the bar of chosen pictures, the turn kept in the photo data and written into every file as it is given out, the stored file unchanged, and drawings turned like photos while the drawing editor shows a drawing as it was drawn)
- **Open question:** whether an entry of a file share (ADR-0197) should be turned, which needs the orientation its preview was made in; whether a new version of a picture or drawing should take over the turn of the version before; whether the album should also mirror pictures; whether a picture turned by hand should be compared with its unturned twin when duplicates are looked for (ADR-0196)
- **Question checked:** 2026-11

## Context and problem statement

The album shows a photo upright by the orientation its EXIF tells (ADR-0177). Many pictures stand wrong all the same: scans and pictures sent through messengers tell no orientation, a camera held up or down tells a wrong one, and a drawing exported or drawn on its side has none.
People want to turn such a picture by a quarter turn, one at a time or many at once, and want the files they take out of the album to stand as the album shows them.

How is a picture turned by hand, and how does the turn reach the files given out, when a stored file never changes (ADR-0025, ADR-0034, ADR-0073)?

The requester chose on 2026-11-16:

1. **Where:** in the viewer for one picture, and in the bar of chosen pictures for many.
2. **How it is kept:** in the photo data, shown turned everywhere in the album, and written into every file as it is given out; the stored file unchanged, the turn undone at any time.
3. **Drawings:** turned like photos in the album, in thumbnails and in the files given out; the drawing editor shows a drawing as it was drawn.

## Decision drivers

- **A stored file never changes:** its size and SHA-256 are in the log, incremental backups take only new files and appended bytes (ADR-0034, ADR-0073), and copies are recognised by their content (ADR-0055).
- **One truth for the position:** what the album shows and how a downloaded file stands agree.
- **Undo:** the orientation the file itself tells stays recoverable.
- **The same way as a time set by hand (ADR-0198):** the same permissions, the same way into the files given out, the same handling of copies.
- **Faces stay on their faces:** the boxes of faces found on a picture (ADR-0194) are fractions of the picture shown upright and must turn with it.
- **No new dependency:** the EXIF is written with the standard library, as the time is.

## Considered options

1. **The orientation recorded in the photo data and written into IFD0 of every file as the server gives it out; the stored file unchanged.**
2. The turn shown in the album only; files keep their own orientation.
3. Each turn stored as a new version of the picture with its pixels turned.
4. The turn kept as quarter turns beside the file's own orientation, rather than as the orientation the picture is shown in.

## Decision outcome

Chosen option: **1**.

### Turning

- **Route:** `POST /api/items/{ref}/attachments/{id}/turn` with `by`: `1` turns a quarter turn clockwise, `-1` counter-clockwise, `2` upside down.
- **Who may:** whoever may edit the picture's item; pictures and drawings (JPEG, PNG, GIF, WebP) only. A video is refused, and so is an entry of a file share, whose preview is made upright from its original (open question).
- **What is recorded:** the photo data carry the orientation the picture is shown in, `Orientation`, and `OrientationManual` (codec version 62), with the kind `AttachmentTurned` holding the photo data as they are afterwards, sealed like every photo data.
  - The new orientation is the file's turned by the quarter turns as it is shown (`photo.Turned`, the same composition in the item model): mirrored orientations stay mirrored.
  - A picture turned back to the orientation its file tells is no longer turned by hand; the server reads that orientation from the file again. No separate route goes back.
  - A picture stored before photo data were recorded has its data read from the file and recorded with the turn.
  - Everything else of the photo data stays: the time, the place and whether it is private.
- **Faces:** the faces found on the picture turn with it in the same batch, recorded by `AttachmentFacesSet` with their names.
- **Copies:** a copy takes over the orientation set by hand on its original. A copy uploaded as the server gave it, with the orientation written into it, is accepted as a copy (`Attachment.AsGiven`, ADR-0198).
- **Not offered over MCP:** the album is not offered to agents (ADR-0177), and a turn changes every file given out, as a time set by hand does.

### Thumbnails

- Thumbnails are made in the orientation the picture is shown in, as before (ADR-0177); their ETag and the server's cache key name a turn by hand.
- Browsers keep thumbnails for a day. The album therefore asks for a picture turned by hand with its orientation in the address (`o=`), in the grid, the viewer, the faces, the duplicates and the slides of a presentation, so a turned picture is never shown as it was.

### The files given out

- **Where:** every way a file leaves the server through `openFor`: downloads, ZIP archives of chosen pictures, exports of pages and copies made by the server, for every reader.
- **How:** `photo.SetOrientation` writes the orientation into IFD0 of the EXIF, through the same writer as the time set by hand:
  - an orientation IFD0 holds is set where it is;
  - an IFD0 without one is written again after the TIFF structure with it, and the header points to it;
  - a JPEG, PNG or WebP without EXIF is given EXIF that tells its orientation, as `photo.SetTaken` gives it a time.
  - A GIF has no EXIF and is given as it is stored; the album still shows it turned.
  - A file it cannot write into is given as it is, and the server logs it.
- **Caching:** the download's ETag names the orientation set by hand, so a picture turned again is another file.

### Drawings

- A drawing is a PNG with its scene (ADR-0053): it is turned as a picture is, in the album, its thumbnails and the files given out.
- The drawing editor reads the scene, not the pixels: it shows the drawing as it was drawn. A new version saved from the editor is a new file with photo data of its own and stands as drawn (open question).

### In the UI

- The viewer has *Turn left* and *Turn right* for who may edit the picture; the facts say *turned by hand*. After a turn the picture and its faces are shown again, and its card in the album with it.
- The bar of chosen pictures has *Turn left* and *Turn right*; videos and entries of a file share among the chosen are left as they are.

### Consequences

- **Positive:** scans, messenger pictures and drawings on their side stand right in a click, many at once; the album and every file taken out of it agree; the stored original stays and the turn is undone by turning back; no storage is added; faces stay on their faces.
- **Negative / trade-offs accepted:**
  - A file given out is read whole into memory to write its EXIF, as a file with a time set by hand already is (ADR-0198).
  - A file given out is no longer the stored file byte for byte: its size and hash differ from those the API reports.
  - Whether a page shows a turned PNG or WebP upright depends on the browser honouring its EXIF orientation. Chromium was checked for JPEG and PNG; other browsers were not. The album itself does not depend on it: it shows thumbnails the server turned.
  - A picture turned by hand keeps the fingerprint of its file's position (ADR-0196) while its proportions are now those it is shown in, so its unturned twin is no longer found as its duplicate.
  - A new version of a picture or drawing does not take over the turn of the version before.
- **Follow-ups / risks to watch:** the open questions above.

## Pros and cons of the options

### Option 1 — written as the file is given out (chosen)

Strongest form: the album and every file people take out stand alike, while the stored file, the log, the backups and the recognition of copies stay as they are, and the file's own orientation stays recoverable; it reuses the writer and the handling of copies the time set by hand already proved.

- Good: one truth for readers; nothing stored changes; undo at any time; no storage added; thumbnails already turn by the orientation recorded.
- Bad: each file given out is rewritten in memory; a file given out differs from the stored one; how a page shows a turned PNG depends on the browser.

### Option 2 — the album only

Strongest form: the smallest change; the files stay exactly as uploaded, nothing is held in memory, and the album, where people look at pictures, stands right.

- Good: no file rewritten; no browser dependence for the album.
- Bad: a downloaded or copied picture stands wrong again in every other program; the requester asked for the files to stand as the album shows them.

### Option 3 — a new version per turn

Strongest form: the file stored is the file given out, every program shows it right without reading EXIF, and the history of versions shows each turn.

- Good: downloads are stored files; GIFs turn too; no browser dependence.
- Bad: every turn stores the whole picture again; a JPEG is decoded and encoded again and loses quality each time, since the standard library cannot turn a JPEG losslessly; faces belong to a version (ADR-0194) and the retention of versions (ADR-0104) would decide over a turn's history; a drawing's scene would no longer match its pixels.

### Option 4 — quarter turns beside the file's orientation

Strongest form: the file's own orientation stays in the photo data, so duplicates could be compared in the file's position, and going back needs no reading of the file.

- Good: the file's orientation is never read again; duplicates keep comparing as before.
- Bad: every place that turns a picture — thumbnails, the album's proportions, faces, the files given out — would compose two values instead of reading one; a copy uploaded as given already tells the turned orientation in its file, so taking over the quarter turns would turn it twice unless every copy path tells the two cases apart.

## Links

- amends ADR-0177 (photo management in the album)
- relates to ADR-0198 (a time set by hand, whose writer and handling of copies this record reuses), ADR-0194 (faces), ADR-0196 (duplicates), ADR-0197 (file shares) and ADR-0053 (drawings)
