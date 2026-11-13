# ADR-0184: Pictures of the album are moved by dragging them onto a folder

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: moving photos of the album into folders by drag and drop; which folders take a drop, that chosen pictures move together and that the move is the existing one are defaults proposed with the implementation, confirmed by the requester; accepted 2026-11-10)
- **Open question:** whether a move should keep the picture's identity instead of copying it and deleting the original, and whether pictures should be dropped onto folders in the timeline, favourites and memories too
- **Question checked:** 2026-11

## Context and problem statement

The album moves pictures into a folder (ADR-0177): one turns on *Choose several*, chooses pictures, picks the target in *Move to folder…* among every readable folder and presses *Move*. Each picture is copied into the folder with its source (ADR-0054, ADR-0055), and the original is deleted where one may edit it and no text shows it. For a few pictures that belong in a folder shown right beside them, this takes five steps and a search through a list of every folder.

How is a picture moved into a folder in one gesture, without a second way of moving?

## Decision drivers

- **One move:** a picture dragged into a folder ends up exactly as one moved with *Move*: the same copy with its source, the same rule for deleting the original, the same message.
- **Access unchanged** (D9, D10, ADR-0022): only folders the album already shows take a drop; the server decides as before whether a file may be added and an original deleted.
- **No change to the server:** no route, event, codec version or budget.
- **Not the only way** (ADR-0091): dragging needs a pointer; choosing and *Move to folder…* stay for the keyboard and for screens where dragging does not work.

## Considered options

1. **Drag a picture, or every chosen picture with it, onto a folder shown in the folders view — a folder below the open one or one above it in the trail — and move it there as *Move* does.**
2. **The same, with a move on the server that keeps the file's identity.**
3. **Drop targets everywhere: a folder chooser that opens while one drags, in every view.**

## Decision outcome

Chosen option: **1** (defaults proposed with the implementation, confirmed by the requester).

- **What is dragged:** in the folders view every picture card can be dragged. A chosen picture takes every chosen picture with it; one not chosen goes alone. An older version in an opened stack is not dragged on its own: its stack moves with its top, as with *Move*.
- **Where it is dropped:** onto a folder listed below the open one, or onto a folder above it in the trail. The trail's top (*All pictures*) is no folder, and a collection (ADR-0175) holds nothing, so neither takes a drop. While pictures are dragged, the folders that take them are outlined; the one under the pointer is marked.
- **What happens:** each picture is moved as *Move* moves it (`copyTo` in `assets/photos.js`): copied into the folder with its source, the original deleted where one may and no text shows it, otherwise kept. A picture that already belongs to the target folder is left alone. The album says how many were moved and which failed, with the message of *Move*.
- **Files from outside:** files dropped from the computer onto an open folder are still uploaded into it (ADR-0177); a drag of pictures from within the album does not count as such a drop.
- **Hint:** a folder that takes a drop names it in its tooltip. The user manual describes the gesture and the way without a mouse.
- What a dragged picture takes with it is a pure function (`dragged` in `assets/photoparts.js`), tested without a browser.

### Consequences

- **Positive:** a picture goes into a neighbouring folder, or back up the trail, in one gesture; several go at once once chosen.
- **Positive:** no change to the persisted format, the routes, the budgets or the MCP tools.
- **Negative / trade-offs accepted:** as with *Move*, a moved picture is a new file in the folder: its versions are not taken along, its favourite marks and links to the original file do not follow it, and an original one may not delete stays where it was as well.
- **Negative / trade-offs accepted:** only folders on screen take a drop; a folder elsewhere is reached through *Move to folder…*.
- **Negative / trade-offs accepted:** browsers on phones offer HTML drag and drop only partly or not at all; there, choosing and *Move* remain the way.
- **Follow-ups / risks to watch:** if moves become frequent, a move on the server that keeps the file (option 2) would also keep versions and favourites; it would need an ADR of its own.

## Pros and cons of the options

### Option 2: a move on the server

Strongest form: a moved picture keeps its ID, every version, its favourite marks and its place data; nothing is downloaded and uploaded again; the move is one event and cannot half fail.

- Bad: a file would change its item, and with it its key, its access and its encryption key (ADR-0026, ADR-0045). That is a new command, a new event and a codec version, and it touches what copies of the file point to (ADR-0055). The request was for the gesture, not for a different move; the two are independent, and the gesture works unchanged on top of such a move later.

### Option 3: drop targets everywhere

Strongest form: one could drag from the timeline, the favourites or the memories, where photos are found by date, into any folder, without first opening the folder beside it.

- Bad: a chooser that opens during a drag hides the pictures one drags from, needs its own search and scrolling while a button is held, and duplicates *Move to folder…*, which already reaches every folder from every view.

## Links

- amended by [ADR-0225](0225-pictures-dropped-onto-a-folder-are-moved-or-copied-as-a-menu-asks.md) (a menu asks to move or copy)
- amends [ADR-0177](0177-photo-management-in-the-album.md) (moving chosen pictures to a folder)
- amended by [ADR-0188](0188-a-moved-file-keeps-its-identity.md) (a moved file keeps its identity)
- relates to ADR-0054 and ADR-0055 (copies with their source), ADR-0090 (folders), ADR-0091 (rules every view follows), ADR-0103 (stacks of versions), ADR-0175 (collections)
