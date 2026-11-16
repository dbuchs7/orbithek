# ADR-DRAFT: A page is moved into another room from its head and its tree

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: wiki pages can be moved into other rooms, as a page at the top or below another page. Asked the same day, the requester said the existing form had not been found, and chose a better choice of the target, a warning before a move into another room and moving from the page tree. Offering the move over MCP was weighed and left out by the requester, as ADR-0012 decided.
- **Open question:** whether a page should be dragged from the tree of one room onto another room; whether moving should be offered over MCP after all, which would amend ADR-0012
- **Question checked:** 2026-11

## Context and problem statement

ADR-0067 lets a page be moved with its child pages by a form, to the top of any room one may create in or below a page there.
The server checks everything (ADR-0022, ADR-0041, ADR-0201): every moved item editable, the right to create at the target, no item of a room's own type leaving its room, not below itself.
The capability exists, but people do not find or trust it:

- The button sits in the last card of the side column, below comments, files and history, beside *Copy…*; a person looking for how to move a page into another room did not find it.
- The target is a plain list of the first 500 items of the room, of every type, by title, without their place in the tree; in a larger room the target may be missing, and two pages of the same title cannot be told apart.
- A move into another room changes who may read and edit the pages, since that room's roles apply from then on, and gives them keys of that room; the form does not say so.
  The album warns before a folder goes into another room (ADR-0200); a page does not.
- The page tree moves pages by dragging within its room only (ADR-0041); the move into another room starts on the page itself.

How is a page moved into another room where people look for it, to a target they can find and recognise, knowing what the move changes?

## Decision drivers

- **One way of moving** (D5, ADR-0041, ADR-0200): the item's move with its subtree, `POST /api/items/{ref}/move`; no new route, event or format.
- **Access unchanged** (ADR-0022, D9): the server decides as for every move; the UI only leaves out what it would refuse anyway.
- **Nothing unreadable shown** (D10): the targets offered are the readable pages of the room, as its tree and its listing give them.
- **Findable** where pages are worked with: the page's head and the page tree.
- **Usable without a mouse** and on narrow screens.
- **A person decides** a change of who sees what (ADR-0012): the move stays out of the agents' tools.

## Considered options

1. **A dialog opened from the page's head and from a button on each row of the page tree: the room, the top or a page of the room chosen in its tree or found by its title or key, the chosen place named, and a warning before a move into another room.**
2. **The form of ADR-0067 kept where it is, with a better list of targets and the warning.**
3. **Dragging a page from the tree onto another room.**
4. **Option 1 and a tool `move_item` for agents over MCP.**

## Decision outcome

Chosen option: **1** (the requester's choice).

- **Where:** *Move…* stands in the page's head, beside *New child page*, for whoever may edit the page.
  On every row of the page tree a button **⇄** opens the same dialog for that page, where the reader may create pages in the room; it shows while the row is pointed at or holds the focus, and always where nothing points (touch screens).
  *Copy…* stays in the side column with its form (ADR-0067); the move is no longer there.
- **The dialog** (`assets/movepage.js`, `assets/moveparts.js`) offers the rooms one may create pages in, one's favourites first (ADR-0145), the page's own room chosen.
  *At the top of the room* or *Below a page*: the room's pages are listed as its tree, a level at a time from `GET /api/spaces/{space}/tree` and paged as the tree is, and the page being moved is left out with everything below it.
  Typing in *Find a page* lists the room's items whose key, title, type, state or labels hold every word (`GET /api/items?space=…&q=…`, at most 20), each with its key and type.
- **The chosen place is named** from the room down to the target, read through the target's parents as far as the reader reads them; a target below the page itself is told before anything is sent.
- **Into another room** the dialog says, before the move, that the room's roles then decide who may read and edit the pages, that restrictions set on the pages go with them, and that the pages get keys of that room while links with the old keys keep working (D2, D7); its button reads *Move into {room}*.
- The move is `POST /api/items/{ref}/move` with `space`, `parent` and `before` empty, placing the page last among its new siblings; a refusal is shown in the dialog as the server words it.
  After moving the page that is open, it is shown at its new key; after moving another page from the tree, the view is shown again.
- **Not over MCP.** ADR-0012 keeps moving out of the agents' tools, since a move changes who sees what; this record leaves that unchanged.

### Consequences

- **Positive:** moving a page, also into another room, is offered where pages are worked with, by keyboard and by pointer.
- **Positive:** every readable page of a room can be chosen as the target, by its place in the tree or by its title or key, however large the room.
- **Positive:** a move into another room no longer changes access unannounced.
- **Negative / trade-offs accepted:** a search hit shows its key and type, not its place in the tree; the place is named once it is chosen.
- **Negative / trade-offs accepted:** the rooms offered are those whose role allows creating pages; a restriction on the target still refuses, and the dialog shows the server's reason.
- **Negative / trade-offs accepted:** the page is placed last below its target; its place among the siblings is changed afterwards by dragging in the tree or with *Move up* and *Move down*.
- **Follow-ups / risks to watch:** whether people want to drag a page onto another room; whether agents should move pages.

## Pros and cons of the options

### Option 2: the form kept where it is

Strongest form: no new place to learn, one card holding copying and moving side by side, and the smallest change.

- Bad: the reason the capability went unused was where it is; a better list in the same place is still not found.
- Bad: the tree, where people arrange pages, would still not move a page into another room.

### Option 3: dragging onto another room

Strongest form: the quickest gesture, the one the tree already uses within a room and the album uses for folders (ADR-0200).

- Bad: the tree shows one room; a target in another room would need a second tree or a list of rooms beside it, and the gesture reaches only what is on the screen.
- Bad: it cannot be done without a pointer, and a drop changes access at once, where a warning belongs first.

### Option 4: also a tool for agents

Strongest form: agents already create, retitle and fill pages and tidy a room's tree as people ask; without moving they leave the last step to a person, and the server checks the agent's caller as it checks everyone.

- Bad: ADR-0012 decided that changing who sees what stays a person's decision, and the user manual tells people so; a move into another room, and even one below a restricted page in the same room, changes it.
  Changing that is a decision of its own, recorded as an open question here.

## Links

- amends [ADR-0067](0067-copying-pages-with-their-child-pages.md) (moving by a form in the side column)
- relates to ADR-0041 (page tree and sibling order), ADR-0200 (a folder is moved from the album), ADR-0012 (moving is not offered over MCP), ADR-0022 (permissions), ADR-0145 (favourite rooms first), ADR-0201 (types of a room)
