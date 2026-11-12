# ADR-0200: A folder is moved from the album, with what it holds

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: folders in the album can be moved. The requester chose the same day both a button with a choice of places and dragging, and that a folder may go into another room after a warning. Accepted 2026-11-12.
- **Open question:** whether collections should be moved from the album the same way; whether the choice of places should reach folders deeper than four levels
- **Question checked:** 2026-11

## Context and problem statement

ADR-0090 made folders items of the type `folder` and left renaming, moving and deleting them to their page. ADR-0190 brought deleting into the album. Moving is still done on the folder's page, which people do not look for while they sort pictures, and the page offers every item as a parent, not only folders, so a folder moved there below a page leaves the album.

How is a folder moved from the album without a second way of moving?

## Decision drivers

- **One way of moving** (D5, ADR-0041): a folder is an item; its move is the item's move with its subtree, its pictures being its files.
- **Access unchanged** (ADR-0022, D9): whoever may edit the folder and everything below it, and create where it goes, may.
- **The album stays whole:** a folder goes only into a folder or to the top of a room, where the album lists it.
- **Usable without a mouse**, and as quick as moving pictures by dragging (ADR-0184).

## Considered options

1. **A button with a choice of places, and dragging a folder onto a folder or the trail; another room after a warning.**
2. **A button with a choice of places only.**
3. **Dragging only.**
4. **Within the folder's room only.**

## Decision outcome

Chosen option: **1** (the requester's choice).

- An open folder offers **Move folder** to whoever the album lists it as editable for. It opens a choice of places: the top level of every room the person sees and every folder they may fill, as far as the album's list of folders reaches (four levels), each room's after its top. The folder itself, the folders below it and the place where it is are not offered (`folderTargets` in `assets/photoparts.js`).
- A folder one may edit can be dragged onto a folder listed beside it or onto a folder above it in the trail; dropped onto the trail's top, **All pictures**, it goes to the top level of its own room (`folderDrop`).
- A move into another room asks first, since that room's access then holds.
- The album calls the item's move (`POST /api/items/{ref}/move` with `space` and `parent`). The server decides as for any item: everything below editable, the right to create at the target, no open linked task leaving its room, not below itself, no duplicate a catalog rule forbids. A refusal is shown as the server words it.
- After the button's move the trail leads to the folder where it went, through the folders above it; after a drag the folder list is shown again.
- No route, event or format is new. Moving is not offered over MCP beyond the item's move it already has.

### Consequences

- **Positive:** folders are sorted where people work with them, by keyboard or by dragging.
- **Positive:** the album offers only places where it lists folders, so a folder no longer leaves the album by being moved below a page.
- **Negative / trade-offs accepted:** folders deeper than four levels are not offered as places by the button; dragging reaches the folders shown.
- **Negative / trade-offs accepted:** the room top is offered without knowing whether the person may create there; the server refuses, and the album shows it.
- **Negative / trade-offs accepted:** collections are not moved from the album; their page moves them.

## Pros and cons of the options

### Option 2: a button only

Strongest form: one way, usable with every input and screen reader, with every place named.

- Bad: sorting many folders takes three steps each where pictures are moved by one drag (ADR-0184); the album would treat its two kinds of content differently.

### Option 3: dragging only

Strongest form: the quickest gesture, the same as for pictures, and no form.

- Bad: it cannot be done without a mouse, and the trail does not reach another room or a folder outside the open branch.

### Option 4: within the room only

Strongest form: access never changes by a move from the album; who may see the pictures stays as it was.

- Bad: the item's page already moves a folder to another room under the same checks; withholding it in the album only sends people there. The warning before the move tells what changes.

## Links

- amends [ADR-0090](0090-album-folders-uploads-and-picture-labels.md) (folders are moved on their page)
- relates to ADR-0041 (page tree and sibling order), ADR-0184 (pictures are moved by dragging them onto a folder), ADR-0190 (a folder is deleted from the album), ADR-0022 (permissions)
