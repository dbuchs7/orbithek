# ADR-DRAFT: A folder of the album lies only in a folder or at the top of a room

- **Status:** Proposed
- **Implementation:** Not started
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12, after ADR-0200: a folder of the album should no longer be able to leave the album by being placed below a page or task. The requester chose the same day a rule the server holds for folders on every way in, and that folders already below another item stay where they are.
- **Open question:** whether the rule should become a general rule of types (which types an item may lie below), for collections and room types too; whether the album should list folders that lie below another item today; whether an item other than a folder may lie below a folder
- **Question checked:** 2026-11

## Context and problem statement

ADR-0090 made the album's folders items of the type `folder`.
The album lists a folder only below another folder or at the top of its room (`Session.Folders` walks the parent).
Making a folder (`Session.CreateFolder`) and a collection (`Session.CreateCollection`) in the album already refuses a parent that is not a folder, and ADR-0200 offers only folders and room tops as places to move a folder to.

Every other way still gives a folder any parent:

- the item page's *Move* dialog and dragging in the page tree (`assets/app.js`, `assets/tree.js`), both through `POST /api/items/{ref}/move`, which ends in `Store.Move` or `Store.Place`;
- `POST /api/items` and the MCP tool `create_item` with the type `folder` and a `parent`, which end in `Store.Create`;
- an import or connector that writes an item of the type `folder` (`Store.Upsert`, ADR-0023).

The one rule about folders the move route holds today is that of file shares: nothing moves into or out of the folders of a share but the bound folder itself (ADR-0197, in the route's handler).

A folder placed below a page or task in one of these ways disappears from the album, with its folders and pictures, though nothing is deleted and nothing tells the person where it went.

Which parents may a folder have, and where is that held?

## Decision drivers

- **The album stays whole:** every live folder one may read is reachable in the album's tree of folders.
- **One rule on every way in** (D3): a rule about what may be stored is held by the store, not only by the views that offer the choice, so the API, MCP, imports and every view meet the same refusal.
- **Replay unchanged** (ADR-0025): the log already holds folders below other items; `apply` must still produce the same state from every event written before.
- **No migration:** nothing is rewritten in existing data directories.
- **Type-agnostic capabilities** (D6): search, permissions, history, comments and notifications stay free of type-specific code.

## Considered options

1. **A rule for folders in the store's commands:** a folder's new parent is a folder or none; a parent that does not change is not checked.
2. **A general rule of types:** a type names the types its items may lie below, and the store checks it for every type.
3. **Only the views refuse:** the item page and the page tree no longer offer a non-folder as a folder's parent; the API stays as it is.
4. **No rule; the album shows folders wherever they lie.**

## Decision outcome

Chosen option: **1** (the requester's choice).

### The rule

- An item of the type `folder` gets as a new parent only an item of the type `folder` of the same room, or none (the top of its room).
- It is held where the store gives an item a parent: `Store.Create`, the check every move and placement shares (`Store.checkMove`, used by `Store.Move` and `Store.Place`), and `Store.Upsert`. The trusted commands and the sessions over them therefore refuse alike, whatever view, route or tool calls them.
- A parent that does not change is not checked. A folder that lies below a page today keeps its place, is reordered among its siblings (ADR-0041), renamed and deleted as before; moved into a folder or to the top of its room, it is back in the album; moved below another non-folder, it is refused.
- Moving another item is not affected, also when folders lie in the subtree it takes along: only the moved item's own parent changes (D5).
- The refusal is a validation error (HTTP 400) that names the parent and says that a folder lies only in a folder or at the top of a room. It names only the parent the caller gave, which the caller has already resolved (D10).
- The rule of file shares (ADR-0197) stays as it is, in the move route's handler; it is checked first, so a move into a share's folders is refused as before.
- The check runs when a command is given, never in `apply`. Events written before keep their effect on replay, and no event, field or codec version is new (ADR-0025).

### What the views do

- The page tree no longer shows a folder as droppable onto an item that is not a folder, and the item page's *Move* dialog offers a folder only folders and room tops as parents.
- The album needs no change: it already offers only these places (ADR-0090, ADR-0200).

### Consequences

- **Positive:** a folder can no longer leave the album by a move or a creation, through any view, the API, MCP or an import.
- **Positive:** nothing is migrated; existing data directories open and replay unchanged.
- **Negative / trade-offs accepted:** the item model's commands hold code for one type, as `CreateFolder` and `CreateCollection` already do. D6 names search, permissions, history, comments and notifications, which stay type-agnostic; moving and creating are not among them, but the rule is a step towards type-specific behaviour in shared commands, and a second such rule should lead to option 2.
- **Negative / trade-offs accepted:** folders that lie below a page or task today stay out of the album until someone moves them; the album does not list them.
- **Negative / trade-offs accepted:** a room type of another name (ADR-0201) used as a folder is not covered; the rule follows the type name `folder` the album defines.
- **Follow-ups / risks to watch:** an import whose source places folders below other items is refused for those items; the import reports them as it reports other refusals.

## Pros and cons of the options

### Option 2: a general rule of types

Strongest form: a type definition names the types its items may lie below (for example `folder` below `folder` only, a collection below `folder` only).
The store holds it for every type alike, so no command knows a type by name, D6 is kept in letter and spirit, and administrators can give room types (ADR-0201) the same guard.

- Good: type-agnostic, reusable for collections and room types, and visible in the type editor.
- Bad: a new field of the type definition is a change of the persisted format (a codec version, ADR-0025).
- Bad: the folder type already defined in existing data directories carries no such rule; it would have to be redefined, by an administrator or by a store command written on its own, before the rule held anywhere.
- Bad: the type editor, its manual pages and the API's type routes would grow for one case known today. When a second type needs such a rule, this option is the way, and option 1 is undone by it.

### Option 3: only the views refuse

Strongest form: the two views that offer the wrong choice today stop offering it; nothing changes in the model, and no command knows the folder type.

- Bad: the API, MCP's `create_item` and imports still place folders below pages, and an agent or script does so without seeing any view. A rule about what may be stored that only views hold contradicts D3.

### Option 4: no rule, the album shows folders wherever they lie

Strongest form: a folder below a project page is a legitimate place for that project's pictures; the album could list it at the top of its room or under the page's title, and nobody would be refused anything.

- Bad: the album's tree would mix folders with pages and tasks it cannot open, and the trail, *Move folder* and deleting a folder (ADR-0190, ADR-0200) would need rules for parents that are not folders.
- Bad: what a page holds and what a folder holds would overlap: a picture is a file of its item, and a page can hold pictures of its own without a folder.

## Links

- amends [ADR-0090](0090-album-folders-uploads-and-picture-labels.md) (folders are items of the type `folder`)
- relates to [ADR-0200](0200-a-folder-is-moved-from-the-album.md) (a folder is moved from the album), ADR-0041 (page tree and sibling order), ADR-0023 (import), ADR-0025 (persisted format), ADR-0175 (collections), ADR-0197 (folders that show a file share), ADR-0201 (room types)
