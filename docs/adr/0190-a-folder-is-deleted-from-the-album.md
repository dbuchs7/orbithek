# ADR-0190: A folder is deleted from the album, into the trash with what it holds

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: folders can be deleted. The requester chose the same day that a folder goes to the trash with its folders and pictures, restorable, after being told how much goes with it.
- **Open question:** whether collections should be deleted from the album the same way; whether the album should list what is in the trash
- **Question checked:** 2026-11

## Context and problem statement

ADR-0090 made folders items of the type `folder` and accepted that a folder is not renamed, moved or deleted from the album, but on its page, as any item. In use, the album is where folders are made and filled. Leaving it to delete one, and finding the folder's page, is a step people do not expect; and they cannot see from the page how many pictures go with the folder.

How is a folder deleted from the album without a second way of deleting?

## Decision drivers

- **One way of deleting** (D7, D8, ADR-0021): a folder is an item; its deletion is the item's move to the trash, with its subtree, restorable, refused while something outside it refers to it.
- **No surprise:** the person sees how much goes with the folder before it goes.
- **Access unchanged** (ADR-0022): whoever may edit the folder and everything below it may.

## Considered options

1. **"Delete folder" in the album moves the folder to the trash with its subtree, after naming how many folders and pictures go with it.**
2. **Only an empty folder can be deleted.**
3. **The folder and what it holds are deleted for good.**

## Decision outcome

Chosen option: **1** (the requester's choice).

- An open folder offers **Delete folder** to whoever the album lists it as editable for.
- The album counts the folders below it and the pictures and videos in them, as far as the person may read them, and asks to confirm with these numbers.
- It then calls the item's move to the trash (`POST /api/items/{ref}/trash`). The folder goes with its folders; their pictures are their files and go with them.
- The server decides as for any item: everything below must be editable, no open linked task below it, and nothing outside the subtree may refer to it. A refusal names what refers to it.
- The album returns to the folder above and links the folder's page, where **Restore** brings it back with everything in it. Deleting for good stays on the page, as for every item.
- No route, event or format is new.

### Consequences

- **Positive:** folders are made, filled and deleted where people work with them.
- **Positive:** a deleted folder is restored whole, pictures included.
- **Negative / trade-offs accepted:** the counts cover what the person may read; a picture protected from them is not counted, though it goes with the folder or keeps it from going, as the server decides.
- **Negative / trade-offs accepted:** the trash is not shown in the album; the folder is found again through the link, search or its room.

## Pros and cons of the options

### Option 2: only empty folders

Strongest form: nothing goes by accident; a person empties a folder knowingly first.

- Bad: emptying a folder of hundreds of photos is many steps. The trash already makes the deletion reversible, which is the protection option 2 seeks.

### Option 3: deleted for good

Strongest form: the space is freed at once, and nothing lingers.

- Bad: irreversible in one click, against the trash every other item has (ADR-0021); purging from the item's page stays possible for that.

## Links

- amends [ADR-0090](0090-album-folders-uploads-and-picture-labels.md) (folders are deleted on their page)
- relates to ADR-0021 (deletion only when unreferenced), ADR-0022 (permissions), ADR-0177 (the album as a photo library)
