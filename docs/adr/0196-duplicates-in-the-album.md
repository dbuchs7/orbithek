# ADR-0196: Duplicates in the album, found by the file and by how a picture looks

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-11
- **Deciders:** Project initiators. Request of 2026-11-11: the album checks photos for duplicates. The requester chose the same day:
  - a duplicate is the **same file or a picture that looks the same** (scaled, encoded again, its place removed), with a fingerprint kept with the picture and a new codec version;
  - the check runs **when files are added and as a view** of duplicates;
  - it covers **every picture the person may read**;
  - when adding, the person is **asked, skipping by default**.

  The rest are defaults proposed with the implementation.
- **Open question:** whether pictures stored before this record should get their fingerprints recorded in one run instead of being made again after every restart; whether the imports from Google Photos and shared iCloud albums should skip pictures that look the same, or the same file in other folders, as uploads ask; whether the view should offer to keep one picture of a group and delete the others in one step; whether WebP pictures should get a fingerprint when the server can decode them; whether SimilarBits (5) and the tolerance of proportions (2 %) fit real libraries
- **Question checked:** 2026-11

## Context and problem statement

The album (ADR-0177) takes pictures from uploads, ZIP archives exported from photo apps, Google Photos and shared iCloud albums. The same photo easily comes in twice: from two exports, from a phone and a camera's backup, once with its place and once without (ADR-0178). Only the imports from Google Photos and iCloud skip a file, and only one of exactly the same bytes in the same folder. ADR-0177 lists "similar or duplicate pictures beyond the same content" as a follow-up.

Every file already carries its SHA-256 (ADR-0045). The same picture saved again — scaled, encoded at another quality, its place emptied from the EXIF (`photo.StripPlace`) — has other bytes and so another SHA-256.

How does the album find duplicates when pictures are added and among those it holds, without showing anyone a picture they may not read?

## Decision drivers

- **Unreadable is non-existent** (D10, ADR-0022): neither the check nor the view names, counts or hints at a picture the caller may not read.
- **One binary, standard library** (ADR-0009, ADR-0024): no image library beyond Go's own decoders.
- **Content never on disk in plaintext** (ADR-0026): what is derived from a picture is content.
- **Bounded server load** (ADR-0015): decoding pictures is the dearest thing the album does.
- **The person decides**: a file that merely looks like another may be wanted (a burst of photos, an edited copy).
- **Intended copies are no duplicates**: a picture copied into a text or another folder remembers its source (ADR-0054, ADR-0055).

## Considered options

1. **The same file only**, by the SHA-256 that is already stored.
2. **The same file and a perceptual fingerprint** kept with the picture: a difference hash (dHash) of 64 bits, computed by the server when it stores a picture and by the browser for the files about to be added.
3. **A fingerprint made on demand** and kept in memory only, nothing persisted.
4. **Fingerprints computed in the browser only** and sent with the upload, as faces are found (ADR-0194).

## Decision outcome

Chosen option: **2, the same file and a perceptual fingerprint kept with the picture**, because it finds what the requester asked for — the same photo with other bytes — at the cost of one decode per upload, and keeps the computation that is stored on the server.

- **Fingerprint** (`internal/photo/fingerprint.go`): the picture is decoded (JPEG, PNG, GIF; at most `ThumbnailPixels`), reduced by area averaging to 9×8 cells of grey (0.299 R + 0.587 G + 0.114 B, premultiplied) in its upright position by the EXIF orientation, and each of 64 bits says whether a cell is brighter than its right neighbour; written as 16 hexadecimal digits. A picture smaller than the grid, or one whose cells differ by less than 3 of 255 (one colour, an empty page), has none. Two pictures **look the same** when their fingerprints differ in at most `photo.SimilarBits` (5) bits **and** their proportions differ by at most 2 %.
- **Kept with the picture**: `Attachment.Fingerprint`, sealed with the rest of the file (**codec version 53**). The server makes it when a picture is uploaded or imported, waiting for a worker of the thumbnails (`ThumbnailWorkers`); a copy of a whole item takes its original's; a move keeps it (ADR-0188). Videos and WebP have none; they are compared as files only.
- **Older pictures**: a picture stored without a fingerprint gets one made when the album compares it, at most `FingerprintReads` (100) per request, kept in memory by its SHA-256 until the server stops; answers say how many are `pending`, and the browser asks again while that number falls.
- **Who is compared**: the top of every stack of versions (ADR-0103) the caller may read, as the album lists them (`albumStacks`, `Session.Pictures`), so protections, restrictions and protected blocks (ADR-0160) apply unchanged. A copy and its original — two pictures whose versions or sources meet — are never duplicates.
- **When files are added** (`POST /api/album/duplicates`): the browser reads each file's SHA-256 (`crypto.subtle`) and, for a picture, draws it at most 256 pixels on its longer edge and computes the same fingerprint (`assets/fingerprint.js`), and asks with at most `DuplicateChecks` (200) files per request. The server answers, per file, up to ten readable pictures that are the same file (`same`) or look the same. Files chosen twice are found in the browser. A dialog (`assets/duplicates.js`) lists each duplicate with the pictures it matches and their folders; ticked files, all at first, are skipped; **Add all** and **Cancel** are offered. This applies to files chosen, dropped, pasted and unpacked from ZIP archives into a folder or a collection (ADR-0187); a PowerPoint import (ADR-0186) is not checked. Without a secure context (`crypto.subtle` missing) or for a file above 256 MB, nothing is checked.
- **The view Duplicates** (`GET /api/album/duplicates`): groups of pictures that are the same file or look the same, joined transitively, the newest group first, at most `PageItems` groups, narrowed by the room filter; each picture says whether another of its group is the same file. Its cards are the album's own, so pictures are opened, chosen and deleted as everywhere.
- **The browser is held to the server**: a test vector (`internal/web/jstest/fingerprint-vector.json`) written by the Go test is checked by the Node test; both compute exactly the same fingerprint for the same pixels. Decoders and the browser's scaling differ slightly, which the tolerance absorbs.
- **Not over MCP**: both routes serve the browser; an agent compares files by their SHA-256 (`internal/mcp/tools.go`).

### Consequences

- **Positive:** the same photo is found across folders and rooms, also when its place was removed or it was saved smaller; uploads ask before a duplicate is stored; the existing pictures can be tidied up in one view.
- **Negative / trade-offs accepted:**
  - a data directory written by this build cannot be opened by an older one (codec version 53);
  - every upload of a picture decodes it once more on the server;
  - after a restart, fingerprints of pictures stored before this record are made again, a share per request;
  - a 64-bit difference hash also takes pictures that are merely very alike (a burst of photos) for the same; the dialog and the view show them as "looks the same", and the person decides;
  - pictures that differ in proportions — cut, or turned without an EXIF orientation — are not found;
  - the check sends nothing of the files but their digests and fingerprints, yet it costs the browser a read of every file before the upload.
- **Follow-ups / risks to watch:** the open questions above; the cache of fingerprints made in memory holds 16 digits per stored picture and is not bounded by its own budget, since the store bounds the pictures.

## Pros and cons of the options

### Option 1: the same file only

Strongest form: the SHA-256 is already stored and exact; no false match is possible, no format changes, no decoding costs anything, and a person who wants a smaller copy keeps it.

- Good: no codec change; nothing to tune.
- Bad: misses exactly the duplicates the requester named: the same photo exported again, scaled by a photo app or stored without its place, all of which change the bytes. The requester chose against it.

### Option 2: the same file and a fingerprint kept with the picture (chosen)

- Good: finds re-encoded, scaled and place-stripped copies; computed once per picture; standard library only; the stored value is the server's own computation.
- Bad: a codec version; one decode per upload; a tolerance that can take very alike photos for the same.

### Option 3: a fingerprint made on demand, in memory only

Strongest form: no persisted format changes at all; the fingerprint is a pure function of the content, so a cache keyed by the SHA-256 is always right, and the first view after a restart is the only dear one.

- Good: no codec change; nothing to migrate.
- Bad: after every restart each picture is decoded again before the album can compare it; for a large library that is minutes of work before the first upload can be checked fully. It is kept as the fallback for pictures stored before this record.

### Option 4: fingerprints computed in the browser only

Strongest form: the browser already decodes the picture to find faces (ADR-0194), decodes WebP and HEIC where the server cannot, and the same code would compute both sides, so no difference between decoders arises.

- Good: no server decode; WebP gets a fingerprint.
- Bad: the stored value would be whatever a client sends, also from scripts and agents; pictures added by imports, copies and the API would have none until a browser with edit rights opened them; a reader who may not edit could not complete the fingerprints of older pictures.

## Links

- amends ADR-0177 (photo management), whose follow-up it takes up
- relates to ADR-0045 (attachments and their SHA-256), ADR-0054 and ADR-0055 (copies and their source), ADR-0103 (versions as stacks), ADR-0160 (protected blocks), ADR-0178 (place on import), ADR-0186 (presentations), ADR-0187 (where pictures are added), ADR-0188 (moves), ADR-0189 (videos), ADR-0194 (faces)
- ADR-0015 (budgets `DuplicateChecks`, `FingerprintReads`), ADR-0025 (codec version 53), ADR-0026 (sealed content), ADR-0022 (D10)
