# ADR-0180: Pictures of the album are renamed, with all their versions, keeping their file type

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: rename drawings and pictures in the album; the choices below — every version renamed, the file type kept, copies left alone, the drawing editor keeping the name — are defaults proposed with the implementation and await the requester's confirmation)
- **Open question:** whether a rename should also reach the captions of the image blocks that show the picture, and the copies of it in other items (ADR-0055); whether renaming should be offered on an item's list of files as well as in the album, and to agents over MCP
- **Question checked:** 2026-11

## Context and problem statement

A picture's name is the file name it was uploaded or first drawn with (ADR-0045, ADR-0053): `IMG_0042.JPG` from a camera, or a drawing named after its first caption. The album shows it on every card, searches by it, exports under it (ADR-0173) and orders a presentation by it (ADR-0176). There was no way to change it short of uploading the file again, which loses its versions, labels, notes and the places it is used.

What does a rename change, which versions does it reach, what does it keep, and how does the name survive the next version of a drawing?

## Decision drivers

- **A name is content** (ADR-0026, ADR-0040): sealed in the log, erased with its item, scanned for sensitive data like every text of an item.
- **The versions of a file are one picture** in the album (ADR-0103); a name on one version only would split the stack's name.
- **A drawing stays a drawing.** The UI recognises one by the marker `.drawing.png` at the end of its name (ADR-0053); a rename must not turn it into a plain picture or a picture into something else.
- **Copies are their own** (ADR-0054, ADR-0055): a copy takes on its item's access and diverges by design.
- **Access unchanged** (ADR-0022, ADR-0160): whoever may edit the item renames its files.

## Considered options

1. **Rename every version of the file; keep the extension and the drawing marker; leave copies alone; the drawing editor keeps the name for the next version.**
2. **Rename only the version chosen**, as a file of its own.
3. **A display name kept on the first version**, beside the file names, which stay as stored.
4. **Free renaming of the whole file name**, extension included.

## Decision outcome

Chosen option: **1** (defaults proposed with the implementation).

### What a rename does

- `PUT /api/items/{ref}/attachments/{id}/name` with `{name}`, the name without its extension. Any version of the file names it.
- The file **keeps its extension**, and a drawing its marker `.drawing.png`; an extension typed with the name is not doubled (`Lake.jpg` for `IMG_0042.JPG` gives `Lake.JPG`). The name follows the rules of every file name: 1 to 255 characters, no slashes or control characters, not empty.
- **Every live version** of the file takes the name, in one event: the new kind `AttachmentRenamed`, which names the file's first version and carries the name (codec version 47). Applying it renames each version whose chain leads to that first version, so live replay and recovery give the same state. A name unchanged is not recorded.
- **Copies** in other items keep their names; they are files of their own (ADR-0055).
- **Image blocks** keep their captions: a caption is text of the body, written by whoever wrote the body.
- The name is content: sealed, erased by `Event.erase`, and read by the scan for sensitive data, which already reads file names.

### The next version keeps the name

- A whiteboard's checkpoint already names a new version after the newest one (ADR-0169).
- The drawing editor in a page named a new version after the image block's caption. It now **keeps the drawing's current name**, read from the download it makes anyway (`Content-Disposition`, `assets/filename.js`), and names a file after the caption only when the download names no drawing. A new drawing is still named after its caption.

### Where

- In the album, *Rename* in a card's menu of further actions, for whoever may edit the picture: the name on the card becomes a field; Enter or leaving it saves, Escape keeps the name.
- Not offered over MCP, as the album is not (ADR-0012); `internal/mcp/tools.go` says so.

### Consequences

- **Positive:** a picture is renamed without losing its versions, labels, notes, protection or the places that show it; a presentation is reordered by renaming (ADR-0176).
- **Positive:** the file type cannot be broken from the UI or the API.
- **Negative / trade-offs accepted:** changing an image block's caption no longer renames the drawing at its next save; the drawing is renamed in the album.
- **Negative / trade-offs accepted:** a copy elsewhere keeps the old name; it says where it came from (ADR-0055).
- **Negative / trade-offs accepted:** the event log keeps every former name in the item's history, sealed, as it keeps every former title.

## Pros and cons of the options

### Option 2: rename one version

Strongest form: the smallest change; each version is a file with a name of its own already.

- Bad: the album shows a stack under its newest version's name; renaming an older one would change nothing visible, renaming the newest would leave the stack named two ways, and the next version would bring the old name back.

### Option 3: a display name on the first version

Strongest form: the stored file names stay as they were uploaded, a record of where the file came from; the display name is one field, like the notes (ADR-0176).

- Bad: two names for one picture — the export, the download, search and copies would each have to choose which; every reader of a name would need to know of the second one.

### Option 4: rename the whole file name

Strongest form: the most freedom; a user who knows what they do may change the type a file claims.

- Bad: a drawing renamed without its marker opens as a plain picture and loses its scene to the editor; the media type is detected from the content anyway, so a changed extension only misleads.

## Links

- amends [ADR-0054](0054-an-album-of-readable-pictures-inserted-by-copy.md) (the album)
- relates to ADR-0026 (sealed content), ADR-0040 (sensitive data), ADR-0045 (attachments), ADR-0053 (drawings), ADR-0055 (copies), ADR-0103 (versions in the album), ADR-0169 (whiteboards), ADR-0173 (export), ADR-0176 (presentations)
