# ADR-0195: The slides of a folder or collection are ordered by hand, by dragging

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-11
- **Deciders:** Project initiators (request of 2026-11-11: the order of the slides in a folder is changed by drag and drop, because ordering them by file name is not practical; asked where the order is changed, for whom it holds, where pictures added later go and whether collections have one too, the requester chose the album and the presenter's window, one order for everyone kept on the folder, new pictures at the end, and folders and collections alike; built in #282 and accepted on 2026-11-11)
- **Open question:** whether the album should show a folder in its order of slides by itself once it has one, rather than on choosing it; whether pictures may also be dragged into the order in the timeline's or a filtered whole album's presentation, which belongs to no folder
- **Question checked:** 2026-11

## Context and problem statement

The album presents the pictures of a folder or a collection as slides (ADR-0176). Their order is the order of their names, numbers by their value; ADR-0176 left an order of its own as an open question and named it as a follow-up. In practice the names carry the order: *01 Intro*, *02 Plan*, … Moving one slide means renaming it and often several others, and a name chosen for the order is not a name chosen for the picture. A presentation imported from PowerPoint (ADR-0186) is named by its place for the same reason.

The album already moves pictures by dragging them onto a folder (ADR-0184); within a folder they cannot be moved at all, and the folders view shows them newest first.

How is the order of slides set by dragging, kept for the next presentation and for other presenters, without breaking the rule that nothing reveals what a caller may not read (D10), given that a collection gathers pictures of every room (ADR-0175)?

## Decision drivers

- **Kept and shared:** a presentation is given again, on another computer and by colleagues; the order must be on the server, as the speaker's notes are (ADR-0176).
- **Robust to new versions and moves:** a drawing saved again is a new version of its file (ADR-0103), and a file moved to another item keeps its identity (ADR-0188); neither should lose its place.
- **Nothing revealed (D10):** a collection's order may name pictures some of its readers do not read. Such a reader must neither see them nor be able to tell where they are, and their changes must not lose those pictures' places.
- **Validated changes (D3) and no shadow data (D4):** the order is written by a command that checks every picture it names; it is not text a person types.
- **Bounded without a new budget (ADR-0015):** what the order holds is bounded by what the folder or collection holds.
- **Paged album:** the folders view loads 48 pictures at a time; a drop must be meaningful with only part of a folder on screen.
- **Not the only way (ADR-0091):** dragging needs a pointer; there must be a way without one.

## Considered options

1. **An order on the folder's or collection's item, of the first versions of its stacks, changed by moves ("these pictures go before that one"); the album lists pictures in it on asking (`order=slides`), those it does not name after the others by name; dragged in the album and in the presenter's strip.**
2. **A position on each picture** (a number, or a fractional rank, kept on the stack's first version like the notes).
3. **The order as a property of the folder's type**, a text field holding the file IDs.
4. **A personal order per person**, kept in the accounts journal.
5. **Renaming kept as the way, made easier**: a dialog that renumbers every picture's name in the order dragged.

## Decision outcome

Chosen option: **1** (the requester's choices for where, for whom, new pictures and collections; the form of the stored order and of the change proposed with the implementation).

### What is kept

- A folder or a collection keeps its **order of slides** as a list of the first versions of its stacks (`Item.PictureOrder`), set by the event kind `PicturesOrdered`, which carries the whole list after the change in the existing field `order` (codec version 52); an empty list clears it.
- The list names files, never their content: it is **not sealed** (ADR-0026), as the order of a page's children (ADR-0041) and of a kanban's cards (ADR-0076) are not.
- It is **no reference** (D8): a file it names may be deleted or moved away; it is then passed over, and forgotten with the next change. It is never given to a caller — the history and the item's JSON leave it out — and only sorts what a caller reads (D10).
- It is bounded by what the folder or collection holds: each change keeps only stacks that still have a version that is not deleted (in the folder, or anywhere for a collection), so no budget is added.

### How it changes

- `PUT /api/album/order/{ref}` with `pictures` (first versions, in the order they are to have) and `before` (a first version, or none for the end) moves pictures; `DELETE /api/album/order/{ref}` clears the order. Whoever may edit the folder or collection may (D9). Every picture named must be one the caller reads there; otherwise the answer is *not found*, as for any picture the caller does not read.
- The server builds the new list in one deterministic step (`Session.PlacePictures`): the stacks the order named, as far as they still exist; then the stacks the caller reads that it did not name, by name, as the caller saw them; then the moved ones are taken out and put before `before`. Stacks the caller does not read keep their places among the others. The list replaces the old one in one event; replaying it yields the same state (`applyPictureOrder` sets the list and nothing else).
- A move rather than a whole list was chosen because it works with part of a folder on screen, with pictures the caller does not read, and with another person's change made meanwhile.

### How it is shown

- `GET /api/album?order=slides` lists the stacks in the order of the folder or collection the filter narrows to, those the order does not name after the named ones, **by name** — numbers by their value, letters regardless of case — then by item key and file, so every window agrees. The listing says whether a picture it lists has a place in the order (`ordered`) — pictures the caller does not read and files that are gone do not count, so it tells nothing of them — and whether the caller may change it (`orderable`). Without a folder or collection, `order=slides` is the order by name.
- The comparison by name moves from the browser to the server (`item.ComparePictureNames`), so the album and the presentation share one order; the presentation now asks for `order=slides` and keeps the order it gets. Names that differ only in accents may order slightly differently from the browser's collator used before; this is accepted.
- **New pictures** come after the placed ones, by name; a new version of a drawing keeps its place, since the order names first versions. A presentation imported from PowerPoint into a new folder (ADR-0186) needs no order: its names carry it.

### Where it is dragged

- **Album:** in an open folder or collection, a choice between **Newest first** and **Order of the slides**, kept in this browser. In the order of slides, whoever may change it drags a picture onto another: onto its left half it goes before it, onto its right half after it; chosen pictures go together (`orderDrop` in `assets/photoparts.js`). Dragging onto a folder still moves pictures there (ADR-0184). After the last picture shown while more follow, the place is not known; the album asks to show more first. A picture's menu offers **Move one place earlier** and **Move one place later**, the way without a pointer. **Order by name** clears the order.
- **Presenter's window:** the slides of the strip are dragged the same way, one at a time; the order is kept at once, the slide shown stays the slide shown, and the presentation window follows with the next state message. Nothing new goes over the `BroadcastChannel`.
- **MCP:** not offered, as the album is not (ADR-0012, ADR-0176); `internal/mcp/tools.go` says so.

### Consequences

- **Positive:** the order of slides is set where the slides are seen, kept for everyone and every computer, and independent of the names; pictures can be named for what they show.
- **Positive:** the order survives new versions of a drawing and moves that keep a file's identity, also into another folder while it stays in a collection.
- **Positive:** a collection's order does not reveal or disturb pictures a reader does not read.
- **Negative / trade-offs accepted:** a new event kind and codec version 52; older binaries cannot read a data directory once an order is set.
- **Negative / trade-offs accepted:** the folders view shows the order of slides only on choosing it; newest first stays the default, as photos are mostly found by date.
- **Negative / trade-offs accepted:** a collection's order is kept for a picture that leaves the collection and comes back; it is forgotten only when the file is gone.
- **Negative / trade-offs accepted:** HTML drag and drop works only partly on phones; there the menu's single steps remain.
- **Follow-ups / risks to watch:** the open questions above; moving several slides at once in the presenter's strip.

## Pros and cons of the options

### Option 2: a position on each picture

Strongest form: a fractional rank on the stack's first version moves one picture with one small event, never rewrites a list, and follows the picture wherever its first version goes; it needs no merge for pictures others do not read.

- Good: one event per moved picture; no list to prune.
- Bad: a rank on the picture belongs to the picture, not to the folder or collection; a picture in a collection and in its folder would share one rank for two orders, and a picture in two collections could not have two places.
- Bad: ranks are written on items of other rooms when a collection is ordered: whoever orders a collection would need to edit every picture's item, not the collection.
- Bad: fractional ranks grow without bound under repeated moves between the same neighbours and need renumbering, which is the list rewrite again.

### Option 3: a property of the folder's type

Strongest form: no new event kind or codec version; the order is written as properties are, validated by the type (D3), and shows on the folder's page.

- Bad: folders are of a type without fields (ADR-0090) and collections of a fixed shape (ADR-0175); every existing data directory holds both types without such a field, so the album would have to define new versions of them on first use.
- Bad: the field would be text a person could type on the item's page, with file IDs no one can read; in a collection it would show IDs of pictures a reader does not read, against D10.

### Option 4: a personal order

Strongest form: no one's order is changed by another; readers who may not edit a folder could still order it for themselves.

- Bad: a presentation given by a colleague, or prepared by one person and given by another, would need the order set twice; the requester chose one order for everyone.

### Option 5: renaming made easier

Strongest form: nothing new is persisted; the order stays visible in every file name, also in downloads and archives.

- Bad: names chosen for the order are what the request asks to be rid of; renaming many pictures for one move writes an event per picture and touches pictures whose content did not change.

## Links

- amends [ADR-0176](0176-pictures-of-the-album-are-presented-as-slides-with-speakers-notes.md) (the slides' order by name)
- relates to ADR-0041 (order of children), ADR-0076 (order of a kanban's cards), ADR-0090 (folders), ADR-0091 (rules every view follows), ADR-0103 (stacks of versions), ADR-0175 (collections), ADR-0184 (moving by dragging), ADR-0186 (presentations imported), ADR-0188 (a moved file keeps its identity)
