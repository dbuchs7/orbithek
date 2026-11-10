# ADR-0178: The place of a photo for its uploader alone

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 to show where a photo was taken to the person who added it only, the open question of ADR-0177; asked when the choice is made and what others get, the requester chose on import and later for each picture, and the place hidden everywhere, the file included)
- **Open question:** whether an administrator, or the erasure role, should see a place kept private, for instance to answer a request about personal data; whether a picture added by an agent or a connector should have its place kept to the person whose token it used
- **Question checked:** 2026-11

## Context and problem statement

The album records where a photo was taken and tells it to every reader of the picture's item, as the file itself does (ADR-0177). People keep photos of their home, their children and their travels in rooms others read. ADR-0177 lets an import remove the place from the file, which loses it for the uploader too. What is missing is the middle: the uploader keeps the place for their own map and memories, everyone else does not learn it.

How is a place kept to the person who added the photo, for photos to come and for those already stored, when the place is written into the file that every reader may download?

The requester chose on 2026-11-10:

1. **When:** on import, as a third choice beside keeping and removing the place, and later for each picture, also for pictures stored before.
2. **What others get:** no place anywhere: not in the photo data, not on the map, not in the file they download, not in an archive, an export or a copy.

## Decision drivers

- **D10 in spirit:** a reader who may not know a place learns nothing of it, not even that there is one.
- **One file:** the stored file stays as it was taken; the uploader keeps the original.
- **No second model of access:** the decision belongs to the one who added the picture, as the attachment records them (`Author`), not to a new list of readers.
- **Every way out of the server:** download, thumbnail, archive, Word and Markdown export, copies into other items, the album's views.

## Considered options

1. **A mark on the photo data, `PlacePrivate`, set on import or later by the uploader alone; the item model hides the place from every other session; the server gives every other reader the file without the place, made as the import's removal makes it.**
2. Store a second file without the place beside the original.
3. Hide the place in the album's views only.
4. Restrict the whole picture to its uploader.

## Decision outcome

Chosen option: **1**.

- **The mark:** photo data carry `PlacePrivate` (codec version 46), sealed with the attachment. Only a photo with a place can be marked.
- **Choosing:**
  - An import says `place=private` (an upload) or `place: "private"` (an import from Google Photos or a shared iCloud album), beside keeping and `strip` (ADR-0177).
  - Later, `PUT /api/items/{ref}/attachments/{id}/place` with `private` keeps the place to oneself or shows it again (`AttachmentPlaceSet`), allowed only to the one who added the picture while they may read it.
  - A picture stored before photo data were recorded has its data read from the file and recorded with the choice.
- **What others read:** every session other than the uploader's gets the photo data without latitude, longitude, height and the mark (`seenBy`): in the item's files, the album, its stacks and origins. The album's map, `place=on` and the months follow from that.
- **What others get as files:** the server gives every other reader the file without the place — the GPS directory emptied, XMP dropped, as `photo.StripPlace` does on import. This covers the download (its own tag, `-noplace`), the archive of chosen pictures, Word and Markdown exports, and copying an item. Thumbnails hold no metadata anyway. A picture whose place cannot be removed is refused to them, never given with it.
- **Copies:**
  - A reader who copies such a picture into another item uploads what they downloaded. The server finds it to be the source without its place and lets it stand as a copy (`Attachment.WithoutPlace`, never persisted); the copy has no place.
  - The uploader's own copy keeps the place, and keeps it private.
  - Who already holds the original may copy it as it is: the server tells them nothing new, and the copy keeps the place private from everyone else.
- **Not offered to agents:** the new route, like the album's (ADR-0177).

### Consequences

- **Positive:** people keep their places for themselves without losing them; the original stays whole; nothing new decides who reads a picture.
- **Negative / trade-offs accepted:**
  - Giving a hidden-place picture to others holds it in memory while its place is removed, once for every download.
  - An administrator reading the room sees no place either.
  - Place names in IPTC or a camera maker's notes are not looked at, as on import.
  - A picture whose uploader's account is gone keeps its place hidden from everyone.
- **Follow-ups / risks to watch:** a default choice per room or per person; the open questions above.

## Pros and cons of the options

### Option 1 — a mark on the photo data, the file stripped for others (chosen)

Strongest form: one decision, made by the one who knows what the photo shows, holding everywhere the server gives the picture away.

- Good: original kept; every exit covered; no new access model.
- Bad: every download by others rewrites the picture in memory; a codec version.

### Option 2 — a second stored file without the place

Strongest form: no rewriting at every download, and a copy for others is a plain file with its own hash.

- Good: fast downloads; simple copies.
- Bad: two files per picture to encrypt, back up, erase and keep in step with versions and protections (ADR-0160); a change of the choice writes or deletes a file.

### Option 3 — hide it in the views only

Strongest form: small, and the map and info panel are where people look.

- Good: no change to downloads.
- Bad: the place stays in the file every reader downloads, so it is not hidden; the requester asked for it hidden everywhere.

### Option 4 — restrict the picture

Strongest form: the existing restrictions (ADR-0078) already keep things to one person.

- Good: nothing new.
- Bad: the picture itself disappears for everyone else, while only its place was meant.

## Links

- amends ADR-0177 (photo management in the album), which left this as an open question
- relates to ADR-0022 (permissions), ADR-0026 (erasure), ADR-0055 (copies carry their source), ADR-0071 and ADR-0131 (exports)
