# ADR-0264: A search field in a text, narrowed to a room or to a page and the pages below it

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-27
- **Deciders:** Project initiators. Request of 2026-11-27: a search field can be placed in an item's text, and its search can be narrowed to the room, the page, the page with its child pages, or the child pages only. The requester chose the same day:
  - the page the search is narrowed to is chosen freely when the field is placed, the page the text is on by default;
  - the hits appear directly below the field, in the text;
  - readers may choose another scope while reading, without the item being saved.

  The rest are defaults proposed with the implementation.
- **Open question:** whether the block should also keep a type, a state or a label to narrow by, as the search page's filters do; whether a reader's choice of scope should be kept per reader; whether the page a field names should be renamed in exports by its title for readers who read it; whether the browser end-to-end test should cover the field
- **Question checked:** 2026-11

## Context and problem statement

The full-text search (ADR-0033) is reached from the top bar and narrows by room, type, state and label.
People who keep a handbook, a knowledge base or a project's documentation as a tree of pages want a search that stays within that tree, placed where readers look for it: on the handbook's start page.
A text already shows computed content in blocks — a task list (ADR-0114), a view of items (ADR-0203), a starmap (ADR-0252) — that keep what they show and hold no item.

How does a text show a search field, what does it keep, how is its search narrowed to a page and the pages below it, and what may a reader change?

## Decision drivers

- One model for content (D1) and no shadow data (D4): a search is a way of finding items, not content; the block must not copy items.
- Stable identity (D2): a page the field names is named by its ID, never by its key.
- Unreadable is non-existent (D10): every reader finds only what they read, and a page they may not read is not found as a scope.
- Type-agnostic search (D6): the narrowing to a page and the pages below it follows containment (D5), whatever the types of the items are.
- The same search as the search page: words, stars and snippets alike, so nothing is learnt twice.
- The persisted format changes only where something must be kept (ADR-0025).

## Considered options

1. **A block of the kind `search` that keeps a scope and the room or the page; the search API narrows to a page with `within` and `scope`; the browser draws a field with the hits below it.**
2. A search field that opens the search page with the scope in its address.
3. A query (ADR-0136) in a view of items (ADR-0203), with a condition `ancestor = …` and the words typed by the reader.

## Decision outcome

Chosen option: **1**, because it keeps nothing that can leak or go stale, narrows by containment on the server, where readability is checked, and shows the hits where the reader typed.

- **The block.** Kind `search`, with no text, mention or mark:
  - `Scope`: none for a room, `page` for the page alone, `tree` for the page and every page below it, `below` for the pages below it only (`item.SearchScopes`);
  - with no scope, `Space`: a room's key, or none for the room of the item the text is on;
  - with a scope, `Ref`: the page, by its ID, or none for the item the text is on, so a page moved or copied takes its search along.
    A field does not name its own item; it names it as none.
  The server checks each and refuses a room with a page or a page with a room, so a body says it one way.
  The block is part of the sealed body (ADR-0026).
- **The page is a mention.** A field that names another page mentions it (`RelMentions`, derived from the body, D4): the page must exist, be live and be readable by the writer when the block is written, the context of the page lists the text that searches it, a backup restore brings it along, and it is not deleted while a field names it (D8), as an embedded or mentioned item is not.
  A protected block's page is told only to the protection's readers (ADR-0160).
- **Format.** Codec version 73 adds the kind and the block field `Scope`; `Space` and `Ref` gain a meaning for it.
  Versions 1 to 72 are still read.
- **The search.** `GET /api/search` takes `within`, a page by its ID or key, with `scope` `page`, `tree` or `below` (`Query.Within`, `Query.Scope`).
  A scope without a page and a page without a scope are refused; a page the caller does not read is not found (D10).
  An item is below the page when the page is among its ancestors, every level deep, whatever the caller reads of the items between; each hit is still checked on its own (D9, D10).
  The words, the stars, the order and the snippets are the search's own (ADR-0072, ADR-0083).
  The MCP tool `search_items` names both parameters.
- **Drawing.** `searchBlockElement` in `assets/searchblock.js` draws a field, a choice of scope and a button; the hits appear below, ten at a time with **More**, each with its stars, key, title, room, type and snippet, as on the search page.
  The search runs on Enter, on the button, on a change of scope and 300 ms after typing stops.
  What needs no browser — the scopes, the block a choice makes, what a reader's scope searches and the search address — is in `assets/searchparts.js`, tested in Node.
- **Reading.** Readers may choose another scope; nothing of that is kept.
  A field of a room searches the item itself when a page is chosen; a field of a page searches the room of the item when the room is chosen.
- **A click** on a hit opens it.
- **Editing.** **Insert…** → **Search field…** opens a dialog with the scope, the room (for a room) and the page (for a page), found by its key or a word of its title and suggested while typing; **Change the search field…** changes it.
  A line above the field says what it searches; a page the reader may not read is named as such, without its title.
- **Exports** name the field (`export.SearchName`), as they name a view: "Search field: the pages below another page".
  Another page is not named by its title, which the document's reader may not read.

### Consequences

- **Positive:** a handbook carries its own search on its start page; each reader finds what they read; the search is the search's own, with its stars and snippets.
- **Negative / trade-offs accepted:** a page named by a field cannot be deleted while the field names it; narrowing walks each candidate's ancestors on every search, which costs the depth of the tree per item; a field in a text names no type, state or label.
- **Follow-ups / risks to watch:** filters by type, state and label in the block; keeping a reader's scope; an end-to-end step in `e2e/smoke.cjs`.

## Pros and cons of the options

### Option 2: a field that opens the search page

Strongest form: the search page already has every filter, the CSV export, the people and the labels; a field that leads there with the scope in its address adds only the scope to one screen and no second rendering of hits.

- Good: one place that shows hits; every filter reaches the field without a new block field.
- Bad: the reader leaves the page they were reading, which the request asked to avoid; the search page would still need the narrowing to a page, so the server change is the same; and the address would carry a page's key, which changes when the page moves to another room (D2).

### Option 3: a query in a view of items

Strongest form: the query language already has `ancestor` and `text ~`, a view already computes per reader and is placed in a text, so a search field is a view whose words come from the reader; nothing new in the format.

- Good: no new block kind; the narrowing by ancestor is already there.
- Bad: a query matches every word and orders by a field, not by how well an item answers, so a reader loses the stars, the snippets and the words found and missing (ADR-0072, ADR-0083); a view cannot take words from its reader without a new field in the block anyway; and a page named in the query's text, by key or ID, is no reference the server checks: it is not mentioned, may be deleted while named, and a key written there changes when the page moves to another room (D2, D4).

## Links

- relates to ADR-0033 (full-text search), ADR-0072 and ADR-0083 (stars and words of a hit), ADR-0203 (views in a text), ADR-0252 (a starmap in a text), ADR-0160 (protected blocks), ADR-0021 (deletion only when unreferenced), ADR-0025 (codec), ADR-0026 (sealed content)
