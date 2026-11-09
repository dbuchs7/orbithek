# ADR-0172: A drawing opens as a whiteboard or alone, and is not saved alone while it is live

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09: choose, when a whiteboard is opened again, whether it opens as a whiteboard or as a drawing, so a group can work on it more than once; accepted 2026-11-09)

## Context and problem statement

A whiteboard is a drawing (ADR-0169): one stack of PNG versions in one item, shown by an image block of the body. The page offered two ways into it, but not side by side. In reading mode a click on the picture showed it enlarged; *Open as whiteboard* stood only among the page's files. In editing mode *Redraw* opened the drawing editor (ADR-0053), where one person works alone and whoever saves last wins. A group that came back to its whiteboard through the page found the single editor first and did not know how to work on it together again.

Both ways are needed: a whiteboard is worked on by a group in a workshop and tidied by one person afterwards. The two must not overwrite each other: a version saved from the drawing editor while a live session runs would be replaced by the session's next checkpoint, which writes the session's scene (ADR-0169), and the edit made alone would be lost without a word.

How does one choose the way a drawing opens, and what keeps the two ways apart?

## Decision drivers

- A whiteboard stays a drawing (D1): no flag on the file says which way it opens; any drawing can be opened either way.
- Choosing is where the drawing is: on the picture in the page, in both modes, not only in the list of files.
- No edit is lost silently: the server, not only the page, refuses what a checkpoint would overwrite.
- Access is the item's (D9, D10): *Edit alone* is offered only to one who may edit the item.

## Considered options

1. **Both ways beside every drawing, and no version from outside a live session while people are in it.**
2. **A kind on the drawing** (whiteboard or drawing) that decides how it opens.
3. **Merging an edit made alone into the live session** when it is saved.

## Decision outcome

Chosen option: **1**.

- **Reading mode.** Below every drawing of the body: *Open as whiteboard* and, for one who may edit the item, *Edit alone as a drawing*. The latter opens the item in editing mode with that drawing's editor open (`#/item/{key}?edit&draw={file}`; the address drops `draw` once the editor is open).
- **Editing mode.** Beside *Redraw* on a drawing: *Open as whiteboard*.
- **While people are on it.** The page asks the server for the live whiteboards (`GET /api/whiteboards`, which lists every version of each drawing in a session, so a block that shows an older version is recognised too, and counts the people in it with a browser, `people`, besides all participants). While people are on it, the page offers *Join the whiteboard (n people are on it)* instead of both ways and says that the drawing cannot be edited alone meanwhile; *Redraw* says the same instead of opening the editor. Agents do not count: a session only agents are in gives way (below). The module *Whiteboards* offers *as a drawing* beside every drawing one may edit.
- **The server.** A new version of a drawing uploaded from outside its session (`POST /api/items/{ref}/attachments` with a source in the same item) is refused with `409 Conflict` while someone is in the session with a browser, and while what was done in it is still to be saved (ADR-0169, *Without a browser*): that save writes the session's scene into the newest picture, whichever it is. A session with nothing to save and only agents in it (an agent counts for two minutes after its last change) is closed instead and the version taken, so an agent's next change starts from the new version. The session's own checkpoints are not affected (they have their own route).

### Consequences

- **Positive:** a group finds its whiteboard where it left it, and one person can tidy it afterwards; neither way overwrites the other.
- **Negative / trade-offs accepted:** one who has the drawing editor open when a session starts finds out on saving: the save is refused, and the edit must be redone in the session. The page warns before, not during, the editor's work.
- **Negative / trade-offs accepted:** the page knows of a session only when it is shown; a session that starts while a page is open is noticed on the next visit or by the server's refusal.
- **Negative / trade-offs accepted:** the server checks the session before it stores the version, not in the same step: a session that opens in the milliseconds between is not noticed, and its next save writes over that version, which stays among the drawing's older versions (ADR-0104).
- **Follow-ups / risks to watch:** a browser left open on the whiteboard keeps everyone from editing alone until it is closed; the page names how many are on it.

## Implementation (2026-11-09)

Built: both ways below every drawing in reading mode and *Open as whiteboard* beside *Redraw* (`assets/editor.js`), the live whiteboards with the versions of their drawings and their people (`GET /api/whiteboards`), the address that opens one drawing's editor (`assets/app.js`), *as a drawing* in the module *Whiteboards*, and the server's refusal (`whiteboard.Hub.Yield`, `internal/server/api_attachments.go`).

## Pros and cons of the options

### Option 2: a kind on the drawing

Strongest form: no choice to make each time; a whiteboard always opens as one, so no one ends up alone in the editor by mistake.

- Bad: the same picture needs both ways in its life (the workshop, then the tidying); a kind would have to be switched back and forth, and it is a new property of a file without a new capability (D1).

### Option 3: merging an edit made alone

Strongest form: nobody is ever refused; the editor's work joins the session.

- Bad: the editor's scene is a whole picture with no record of which elements changed since it was opened; merging it would mean guessing, element by element, against changes others made meanwhile under their locks (ADR-0169). A refusal that says why is honest; a guessed merge is not.

## Links

- extends [ADR-0169](0169-live-whiteboards-on-drawings.md) (live whiteboards, checkpoints) and [ADR-0053](0053-drawings-with-excalidraw-on-a-page-of-their-own.md) (the drawing editor)
- relates to [ADR-0055](0055-copied-files-carry-their-source.md) (versions) and [ADR-0104](0104-rules-for-the-older-versions-of-pictures.md) (older versions)
