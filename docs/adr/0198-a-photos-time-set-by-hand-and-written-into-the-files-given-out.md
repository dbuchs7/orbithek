# ADR-0198: A photo's time set by hand, and written into the files given out

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-11
- **Deciders:** Project initiators (request of 2026-11-11 that the time a picture was taken can be changed; asked where, how exactly and whether the file changes too, the requester chose the viewer and the bar of chosen pictures, setting one time or shifting by a span, a date with its time, and the time written into the file; told that a stored file never changes, the requester chose to have the time written into every file as it is given out)
- **Open question:** whether a video's time set by hand should be written into its movie header as it is given out; whether XMP dates that disagree with the time set by hand should be written too or dropped; whether a time should be shifted by the offset it names when only the zone was wrong
- **Question checked:** 2026-11

## Context and problem statement

The album reads when a photo was taken from its EXIF (ADR-0177) and orders the timeline, the memories and the search by it. Many pictures tell no time (scans, pictures sent through messengers, many downloads), and others tell a wrong one: a camera whose clock was never set, or set to another zone. People want to say when such a picture was taken, one at a time or for many at once, and want the files they download to say the same as the album.

How is a photo's time set by hand, and how does it reach the files people take out of the album, when a stored file never changes (ADR-0025, ADR-0034, ADR-0073)?

The requester chose on 2026-11-11:

1. **Where:** in the viewer for one picture; in the bar of chosen pictures for many, either setting one time for all or shifting each by the same span.
2. **How exactly:** a date with its time to the second; the offset from UTC chosen when it is known.
3. **The file:** the time is written into the file. Told that the stored file cannot change without breaking the log, the backups and the recognition of copies, the requester chose to have it written into every file as it is given out.

## Decision drivers

- **A stored file never changes:** its size and SHA-256 are in the log, incremental backups take only new files and appended bytes (ADR-0034, ADR-0073), and copies are recognised by their content (ADR-0055).
- **One truth for the time:** what the album shows and what a downloaded file tells agree.
- **Undo:** the time the file itself tells stays recoverable.
- **Permissions as for the place:** whoever may edit a picture's item sets its time (ADR-0181); a time is no secret, so a place kept private (ADR-0178) does not restrict it.
- **No new dependency:** the EXIF is written with the standard library, as it is read (ADR-0177).

## Considered options

1. **The time recorded in the photo data and written into every file as the server gives it out; the stored file unchanged.**
2. The time recorded in the photo data only; files keep their own.
3. Each change stored as a new version of the picture with its EXIF rewritten.
4. The stored file overwritten.

## Decision outcome

Chosen option: **1**.

### Setting the time

- **Routes:**
  - `PUT /api/items/{ref}/attachments/{id}/taken` with `taken` (`2006-01-02T15:04:05`) and `offset` (`+02:00`, or empty when not known) sets a picture's time.
  - `DELETE` on the same address goes back to the time the file itself tells, which the server reads from the file again.
- **Who may:** whoever may edit the picture's item; pictures and videos of the album only.
- **What is recorded:** photo data carry `TakenManual` (codec version 52), with the kind `AttachmentTakenSet` holding the photo data as they are afterwards, sealed like every photo data.
  - Times lie in the years 1800 to 2200, the years the EXIF reader takes.
  - Everything else of the photo data stays, the place and whether it is private above all.
  - A picture stored before photo data were recorded has its data read from the file and recorded with the time.
- **Copies:** a copy takes the time set by hand on its original. A copy uploaded as the server gave it, with the time written into it, is accepted as a copy: the server checks that its content is the original as given to the copier (`Attachment.AsGiven`, which replaces `WithoutPlace` and also covers a place left out, ADR-0178).
- **Not offered over MCP:** the album is not offered to agents (ADR-0177), and a time set by hand changes every file given out.

### The files given out

- **Where:** every way a file leaves the server through `openFor`: downloads, ZIP archives of chosen pictures, exports of pages and copies made by the server, for every reader, the uploader included.
- **How:** `photo.SetTaken` writes `DateTimeOriginal` into the Exif directory and `OffsetTimeOriginal`, or removes it when no offset is known.
  - JPEG: the first Exif segment, or a new one after the APP0 segments.
  - PNG: the `eXIf` chunk, or a new one after `IHDR`.
  - WebP: the `EXIF` chunk, or a new one before XMP; a simple WebP is given a `VP8X` header for its canvas.
  - Nothing the file holds is moved: a changed IFD is written again after the TIFF structure and what pointed to it points to the new one, so maker notes and thumbnails keep their offsets.
  - A file it cannot write into is given as it is, and the server logs it; the photo data still carry the time.
- **Caching:** the download's ETag names the time set by hand, so a changed time is a changed file; the browser asks again before it copies a picture.
- **Videos:** the time set by hand counts in the album; the file keeps its own (open question).

### In the UI

- The viewer has the fields *Time taken* (date and time to the second) and *Offset from UTC* (not known, or every quarter of an hour from −12:00 to +14:00), *Set the time* and, after a time set by hand, *Back to the file's time*; the facts say *set by hand*.
- The bar of chosen pictures has *Set the time…*: *Set to* one time for all, or *Shift by* days, hours and minutes, later or earlier, each picture keeping its offset; a picture that tells no time is not shifted.

### Consequences

- **Positive:** scans and pictures of a wrong clock get their time in a few steps, many at once; the album and every file taken out of it agree; the stored original stays and the change is undone at any time; no storage is added.
- **Negative / trade-offs accepted:**
  - A file given out is read whole into memory to write its EXIF, as a file without its place already is (ADR-0178).
  - A file given out is no longer the stored file byte for byte: its size and hash differ from those the API reports.
  - XMP dates in a file are left as they are and may disagree with its EXIF.
  - A video downloaded keeps the time its file tells.
- **Follow-ups / risks to watch:** the open questions above.

## Pros and cons of the options

### Option 1 — written as the file is given out (chosen)

Strongest form: the album and every file people take out agree, while the stored file, the log, the backups and the recognition of copies stay as they are, and the original time stays recoverable.

- Good: one truth for readers; nothing stored changes; undo at any time; no storage added.
- Bad: each file given out is rewritten in memory; a file given out differs from the stored one.

### Option 2 — photo data only

Strongest form: the smallest change; the files stay exactly as uploaded, and the album still orders pictures right.

- Good: nothing written into files; nothing read whole into memory.
- Bad: a downloaded picture tells another time than the album, and other programs order it wrongly; the requester asked for the file.

### Option 3 — a new version per change

Strongest form: the file stored is the file given out, and the history of versions shows each change.

- Good: downloads are stored files; the old version is kept.
- Bad: every change stores the whole picture again (shifting 500 pictures stores 500 files); faces belong to a version (ADR-0194) and would be lost; the retention of versions (ADR-0104) would decide over a time's history.

### Option 4 — overwrite the stored file

Strongest form: one file, telling one time, wherever it goes.

- Good: no rewriting as files leave the server.
- Bad: breaks that a stored file never changes: its size and hash in the log, incremental backups that take only new files (ADR-0034, ADR-0073) and copies recognised by content (ADR-0055); the original time is lost.

## Links

- amends ADR-0177 (photo management in the album)
- relates to ADR-0178 (a place kept to its uploader) and ADR-0181 (a place set by hand)
