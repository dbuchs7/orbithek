# ADR-0188: A file moved to another item keeps its identity

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: a move keeps the picture's identity, the open question of ADR-0184. The requester chose the same day:
  - a picture that cannot leave its item stays where it is, and the album says which and why, instead of being copied;
  - agents get no tool for moving.

  The rest are defaults proposed with the implementation, confirmed by the requester: refusing protected files and drawings with comments, taking deleted versions along, and favourites by file. Accepted 2026-11-10.
- **Open question:** whether a picture a text shows should move with a copy left in the text; whether moving should be offered in an item's file list as well as in the album; whether pictures should be dropped onto folders in the timeline, favourites and memories (from ADR-0184)
- **Question checked:** 2026-11

## Context and problem statement

The album moves pictures into a folder by dragging them (ADR-0184) or with *Move to folder…* (ADR-0177). Both copy each picture into the folder with its source (ADR-0054, ADR-0055) and delete the original where one may. The copy is a new file:
- its older versions stay behind and are deleted with the original;
- it has a new ID, so the favourite marks on it (ADR-0177) do not follow;
- the copy is made by the mover, so a place its uploader kept private (ADR-0178) becomes the mover's;
- copies of it in pages lose their original.

A picture one may not change, or that a text shows, was copied silently and stayed where it was as well.

A file is encrypted with its item's key, and the encryption is bound to the item and the file's ID (ADR-0026, ADR-0045). How does a file go to another item as the same file, without a second model of files and without weakening its encryption or its access?

## Decision drivers

- **One model (D1, D9, D10):** a file stays a file of exactly one item; who reads it is decided by that item, before and after the move. Nothing tells a reader of one item about the other.
- **Identity:** after the move, the file is recognisably the same file: its ID, every version, its labels, notes, rule of versions, photo data and who added it.
- **Nothing lost:** a text that shows the file, a protection over it and comments on a drawing are not left pointing at nothing.
- **Content never in plaintext on disk** (ADR-0026): the content is sealed with the new item's key; the old ciphertext goes.
- **Crash safety** (ADR-0008, ADR-0025): a crash at any moment leaves the file in exactly one item, readable.

## Considered options

1. **Move the file's records to the new item in one batch of events, and seal its content again with the new item's key, under the same ID.**
2. **Keep copying, and make the copy carry the original's versions, marks and author.**
3. **Give files an identity independent of items, with their own keys, and let an item point to them.**

## Decision outcome

Chosen option: **1**.

### What moves

- A move takes a file with **every version of its stack** (ADR-0103), deleted versions included. They still link the stack and hold its first version's notes and rule (ADR-0104, ADR-0176).
- Each version keeps its ID, name, type, size, checksum, labels, notes, rule, photo data, who added it and when. A place its uploader keeps private stays theirs (ADR-0178).
- Within the stack, versions point to each other in the new item as they did in the old. A stack that began as a copy of a third item's file keeps naming that original.
- **Copies elsewhere** that name the moved file as their original find it in its new item (ADR-0055), shown only to those who may read it there.
- **Favourites** name the file by the ID of its first version, so they follow it. Marking it again in its new item changes nothing; unmarking removes the mark as it was made.

### What stays

The file is not moved, and the album says which file and why, when:
- one may not edit both items;
- the item's text shows any version of it (it stays with the text);
- it is protected in its item (ADR-0160);
- comments are on elements of the drawing (ADR-0171);
- it is a drawing open as a live whiteboard with people on it (ADR-0172).

Nothing is copied in its place: the requester chose that a move means one thing only. Whoever wants a copy uses *Copy*.

### How

- **Events (codec version 49):** one batch.
  - `AttachmentMovedIn` on the new item, per version: the version with all it holds and `Move` (the item it came from, who added it, when, whether deleted). Sealed with the new item's key; erased with the new item.
  - Then `AttachmentMovedOut` on the old item, per version: its ID and the item it went to. It holds no content.
- **Applying:** the old item drops the version's records, and the new item takes them as they were. The store remembers where a moved file went (`moved`), derived from the events, so copies follow it. `applyToState` stays deterministic: everything comes from the events.
- **Content:**
  - The server reads each live version and seals it again with the new item's key under the same ID (`storage.MoveAttachmentContent`); the checksum must match the record.
  - Only then is the move recorded. Recording it removes the old ciphertext, as a deletion does.
  - A crash before the move is recorded leaves an unrecorded file in the new item, which the next start removes. A crash after leaves the old file, which the next start removes too.
- **Command:** `Session.PlanMove` names the versions to store and checks everything. `Session.MoveFile` checks again inside the write. It refuses if the stack changed since, so no version is recorded without its content.
- **Route:** `POST /api/items/{ref}/attachments/{id}/move` with `to`, for whoever may edit both items.
- **Album:** drag and drop (ADR-0184) and *Move to folder…* both use it. The album reports how many pictures moved and lists those that stayed, with the reason.
- **Watchers** of the new item are told that a file came, as for an upload. The item's last change counts the move on both items.
- **MCP:** not offered (the requester's choice). A move takes a file out of an item and changes who may read it.

### Consequences

- **Positive:** a picture moved is the same picture: versions, labels, notes, place, author and favourites go with it, and copies keep their original.
- **Positive:** the file's encryption stays bound to the item that holds it.
- **Positive:** nothing is silently copied any more; a move does one thing.
- **Negative / trade-offs accepted:**
  - Codec version 49: a data directory written by this build cannot be opened by an older one.
  - A move reads and writes the whole content of every live version, and holds both copies on disk until it is recorded. At critical storage it is refused like an upload (ADR-0163).
  - A picture a text shows can no longer be "moved" at all from the album. It is removed from the text first, or copied.
  - Each item's history shows the move as events of their own kind, naming neither the file nor the other item.

## Pros and cons of the options

### Option 2: a richer copy

Strongest form: no new event kinds and no new codec version. The existing copy path (ADR-0055) already reads the content with the caller's rights, and the copy could be given the original's versions and marks.

- Bad: the file still gets a new ID, so every reference by ID (favourites, copies, comments on drawings, image blocks) breaks or needs rewriting.
- Bad: who added the file and when would be written by someone else's request, which a copy may not claim (ADR-0055 makes copies prove their content, not their author).

### Option 3: files independent of items

Strongest form: a file's identity would not depend on any item. Moving would be a pointer change with no re-encryption, and several items could show one file.

- Bad: a second model of content beside items, with its own keys, access and erasure, against D1.
- Bad: a file in several items would need rules for who reads it, as ADR-0090 rejected for folders.

## Links

- amends [ADR-0184](0184-pictures-are-moved-by-dragging-them-onto-a-folder.md) (its move, and its open question on identity)
- amends [ADR-0177](0177-photo-management-in-the-album.md) (moving chosen pictures)
- amends [ADR-0055](0055-copied-files-carry-their-source.md) (copies follow a moved original)
- relates to ADR-0008, ADR-0025 and ADR-0026 (the log, its format and encryption), ADR-0045 (attachments), ADR-0103 and ADR-0104 (versions and their rule), ADR-0160 (protected blocks), ADR-0171 and ADR-0172 (comments on drawings, live whiteboards), ADR-0178 (a place for its uploader alone), ADR-0012 (MCP)
