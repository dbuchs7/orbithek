# ADR-0187: Pictures are added where one is in the album, a collection's by its labels

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 to make loading photos into a folder or a collection as simple as possible; asked how pictures reach a collection, the requester chose uploading and choosing pictures already in the album; asked where uploads to a collection are kept, to be asked for a folder each time, the last one first; and chose a large drop area, adding from the album, dropping at the top and pasting with Ctrl+V)
- **Open question:** whether the folder chosen for a collection's uploads should be kept on the collection, for everyone who adds to it; whether pictures added from the album to a folder should be moved rather than copied
- **Question checked:** 2026-11

## Context and problem statement

Pictures reach the album by uploading into an open folder (ADR-0090, ADR-0177). At the top of the album and in a collection (ADR-0175) nothing could be added: a collection holds no files, so it offered only its rule, and an empty collection said *No pictures found* with no way on. A picture already in the album reached a folder only by moving it, and a collection only by labelling each picture by hand.

How are pictures added wherever one is in the album, with as few steps as possible, without a collection starting to hold files?

## Decision drivers

- **One way, everywhere:** choosing files, dropping them and pasting work alike in a folder, in a collection and at the top.
- **A collection holds nothing** (ADR-0175): it gathers by labels, so adding to it means giving labels.
- **No second store:** a picture is a file of an item (ADR-0045); it is kept in a folder one may edit.
- **No surprise copies of restricted pictures:** adding an existing picture to a collection must not copy it into another room.
- **No change to the persisted format** (ADR-0025).

## Considered options

1. **Added where one is: into the open folder; in a collection or at the top into a folder chosen each time, a collection's labels given to each picture; existing pictures copied into a folder or labelled for a collection.**
2. A collection keeps a folder of its own, made on its first upload.
3. Uploading only into folders, as before, with clearer hints.

## Decision outcome

Chosen option: **1**.

### Where pictures go

- **In a folder:** into it, as before.
- **In a collection:** the album asks which folder they are kept in. It lists the folders one may edit, the one chosen last first (kept in this browser), and offers *New folder…* in a room, the collection's room first. Each picture uploaded gets the labels of the collection's rule, so the collection shows it, whether it matches any or all of them. A presentation (ADR-0186) becomes its folder of slides inside the folder chosen, each slide labelled.
- **At the top:** the album asks for the folder the same way and then opens it.
- The folder listing (`GET /api/album/folders`) says for each folder whether one may edit it (`editable`), so the choice offers only folders one may add to.

### How pictures come

- **Upload pictures** at every level of the folders view; dropping files onto the album; pasting a picture with Ctrl+V outside a text field. A pasted screenshot named only *image.png* is named by the moment it was pasted.
- An empty folder or collection shows a large area: drop pictures here, paste one, choose files, or add pictures already in the album.
- **Add from the album…** opens the album to choose pictures one after another:
  - for a folder, each is copied into it with its source, as inserting copies (ADR-0054); the original stays;
  - for a collection, each gets the collection's labels and stays where it is. A picture one may not edit is not labelled, and the album says so; one that already shows is left.

### What stays on screen

- A change to a picture — its labels, a rename, a move, a deletion, an upload — shows the album again as far as it was shown, not only its first page. A picture's labels change on its card in place. Before, labelling a picture after *Show more* folded the album back to its first page.

### Consequences

- **Positive:** pictures are added in one step wherever one is, also to a collection; an empty collection explains how it fills.
- **Positive:** nothing new is persisted; a collection still holds no files.
- **Negative / trade-offs accepted:** uploading to a collection asks for a folder every time; the choice is remembered in the browser only.
- **Negative / trade-offs accepted:** a picture from another room that one may not edit cannot be added to a collection; it would need its labels from someone who may edit it.
- **Negative / trade-offs accepted:** adding to a folder from the album copies; the copy and its original are two pictures, linked by their source (ADR-0055).
- **Follow-ups / risks to watch:** the open questions above.

## Pros and cons of the options

### Option 1 — added where one is (chosen)

Strongest form: the same three ways work at every level, a collection stays a rule, and the folder that holds the files stays visible to the person adding them.

- Good: no new model; works for collections across rooms.
- Bad: one more question per upload to a collection.

### Option 2 — a collection's own folder

Strongest form: no question after the first upload; the files of a collection are found together.

- Good: fewer steps.
- Bad: a folder appears that nobody named; a collection gathering from many rooms keeps its uploads in one room only; the requester chose to be asked.

### Option 3 — only folders, clearer hints

Strongest form: nothing changes in what the album does; only the texts explain the way.

- Good: least code.
- Bad: a collection still cannot be filled from where one is, which is what was asked.

## Links

- amends [ADR-0175](0175-a-collection-in-the-album-gathers-labelled-pictures-of-every-room.md) (a collection is filled by giving its labels) and [ADR-0090](0090-album-folders-uploads-and-picture-labels.md) (uploads)
- relates to ADR-0054 (copies with their source), ADR-0177 (photo imports), ADR-0184 (moving by dragging), ADR-0186 (presentations)
