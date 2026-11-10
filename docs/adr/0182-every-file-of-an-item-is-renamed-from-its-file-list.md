# ADR-0182: Every file of an item is renamed from its file list

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: renaming also in the file list of a page; that the rules of ADR-0180 hold for every kind of file is a default proposed with the implementation, confirmed by the requester; accepted 2026-11-10)
- **Open question:** whether a rename should reach the captions of image blocks and the copies of a file in other items, and whether agents should rename files over MCP; both stay open from ADR-0180
- **Question checked:** 2026-11

## Context and problem statement

ADR-0180 lets whoever may edit a picture rename it in the album, and left open whether the same should be offered on an item's list of files. That list shows every file of the item — pictures, drawings, videos, documents, BPMN models — and is where an editor uploads and deletes them. A document uploaded as `scan_0001.pdf` could only be renamed by uploading it again, which loses its versions and the places it is used.

Where is a file renamed outside the album, which files may be renamed, and by which rules?

## Decision drivers

- **One rule for one thing:** a file's name is the same name wherever it shows; renaming it should not follow different rules in different places.
- **A file stays what it is** (ADR-0180): its extension, and a drawing's marker, are kept.
- **Access unchanged** (ADR-0022): whoever may edit the item renames its files, as they delete and upload them there.
- **No new model:** ADR-0180's event and route already rename any file of an item.

## Considered options

1. **Rename every file of an item from its file list, by the rules of ADR-0180**, through the same route and event.
2. **Rename only pictures there**, as in the album.
3. **Rename the whole name, extension included, for files that are no pictures.**

## Decision outcome

Chosen option: **1** (a default proposed with the implementation, confirmed by the requester).

- Every file in an item's file list has *Rename* beside *Delete* for whoever may edit the item. The name becomes a field holding the name without its extension; Enter or leaving it saves, Escape keeps the name.
- The rules are ADR-0180's: every live version takes the name, in one `AttachmentRenamed` event; the extension and a drawing's marker are kept, an extension typed with the name is not doubled, a file without an extension stays without; copies in other items and the captions of image blocks keep theirs. The name is content: sealed, erased with its item, scanned for sensitive data.
- `PUT /api/items/{ref}/attachments/{id}/name` serves both places; its summary names files, not only pictures. The item model's command is `RenameFile`.
- The album and the file list share one way of renaming in place (`assets/rename.js`), so both behave alike.
- Not offered over MCP, as in ADR-0180.

### Consequences

- **Positive:** a document, a video or a model is renamed without losing its versions or the places it is used.
- **Positive:** no change to the persisted format, the routes or the budgets.
- **Negative / trade-offs accepted:** only the last extension is kept: `backup.tar.gz` renamed to `old` becomes `old.gz`. Names with several extensions are rare among the files of pages; the full name is typed again where it matters.
- **Negative / trade-offs accepted:** a file's type cannot be corrected by renaming; a file with a wrong extension is uploaded again.

## Pros and cons of the options

### Option 2: pictures only

Strongest form: exactly what the album offers, nothing more to explain.

- Bad: the file list shows every file alike; a *Rename* on some rows and not others is a rule the reader cannot see, and documents need renaming at least as often as pictures.

### Option 3: the whole name for other files

Strongest form: a document's extension does not decide how the wiki shows it; a user may want to correct a wrong one.

- Bad: two rules for one action, depending on a kind the user does not see; the media type is detected from the content anyway, so a changed extension only misleads programs that open the download.

## Links

- amends [ADR-0180](0180-pictures-of-the-album-are-renamed-with-all-their-versions.md) (renaming pictures of the album)
- relates to ADR-0022 (permissions), ADR-0045 (attachments), ADR-0053 (drawings), ADR-0055 (copies)
