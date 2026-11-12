# ADR-0207: A command palette, an item beside another, and the context of an item

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 for keyboard navigation across the product, reading two items side by side, and seeing what refers to an item)
- **Open question:** whether the context should also list items that mention an item's title without linking it (unlinked mentions), and whether the palette should search the server with the same letters-in-order matching it uses for commands; this record decides neither
- **Question checked:** 2026-11

## Context and problem statement

Three things slow people down in a knowledge base that grows:

- **Getting somewhere.** Items are found with the search field, modules with the nine dots, rooms with the start page, and actions with their buttons. Each needs the mouse or several steps; nothing reaches all of them from the keyboard.
- **Reading two things at once.** Writing a page while consulting another means two browser tabs, or going back and forth and losing one's place.
- **Seeing what refers to an item.** Relations show the item's typed relations and, marked "from the text", derived mentions, without the sentence they stand in. Links in texts to the item's address are not relations and are not shown. What shares its labels is a click away on each label. Nothing shows the daily notes (ADR-0205) that name it.

How are these met without new stored objects, without revealing what a reader may not read, and within bounded work per request?

## Decision drivers

- D10: nothing shown names an item the reader may not read; a protected passage stays veiled in any sentence quoted (ADR-0088, ADR-0160).
- D4: references are derived from bodies on every request; nothing is stored beside them.
- ADR-0015: every list a request builds is bounded by a budget.
- ADR-0096: the editor keeps Ctrl+K for links; keys must not collide with the editor or the browser's own.
- ADR-0116: everything fits a phone.
- ADR-0017 and ADR-0014: every word from the catalogue; elements made with `h()`.
- Accessibility: keyboard alone, with the roles assistive technology knows (WAI-ARIA combobox and listbox).

## Considered options

1. **A palette in the browser over the search API, the shell's commands and the recent items of the browser; a pane in the same page whose item is in the address; a context derived per request by a new read route.**
2. **A palette with a server index of everything** (items, commands, rooms), searched by the server.
3. **Two items side by side as a split of two full item pages**, each with its own side column.
4. **Backlinks stored as relations**: links and label sharing kept as derived relations in the store.
5. **The context computed in the browser** from the relations, the labels and the search API.

## Decision outcome

Chosen option: **1**, because each part reuses what exists (search, the item API, the editor and live session), adds one read route, and keeps all state either in the address or, for conveniences, in the browser.

### The command palette

- **Opening:** Ctrl+K where no text is being edited, and Ctrl+Shift+P everywhere (Cmd on a Mac). In a text being edited Ctrl+K stays the link dialog; while another dialog is open the keys stay with it.
- **What it finds:** items through `GET /api/search` (the server's search, permissions included, ADR-0033); the items opened recently in this browser (`localStorage`, at most twelve, never sent to the server); and commands: new item where one is, today's daily note, each module, each room one holds a role in, the query, the calendar, the light or dark look, reading mode and the manual. Commands and recent items match when the letters typed appear in their order, word starts and runs counting more (`assets/paletteparts.js`, tested by `internal/web/jstest/palette.test.mjs`).
- **Keyboard and roles:** a modal dialog; the field is a combobox controlling a listbox, the active option named by `aria-activedescendant`, groups as presentation headers, the number of results said politely. Arrows choose, Enter opens, Escape closes and gives the focus back.

### An item beside another

- **Opening:** Shift+click on a link to an item on an item's page or a daily note, or **Beside** in the context. The address keeps it (`?beside=KEY`), so the pair can be shared and the browser's history brings it back. A text being edited keeps Shift+click for its selection.
- **The pane (`assets/beside.js`):** the item's title (a link to it), key, type, state, filled fields, labels and text, read through the item API like any view (an unreadable item is not found). Where the reader may edit its text, **Edit** joins its live session in the pane, with the same writing aids, creating from a mention included (ADR-0206); **Done** leaves it. Fields, comments and the rest are on its own page (**Open on its own**). Escape (outside a text or field) or × closes the pane.
- **Layout:** the page's main column and the pane side by side, the item's side column then below its text; the pane stays in view while the page scrolls. Below 900 px the pane follows the page (ADR-0116).

### The context of an item

- **Route:** `GET /api/items/{ref}/context`, derived per request by `Session.Context` from the live state as the reader sees it; MCP `get_context` (a read).
- **Referred to by:** readable live items that relate to the item by hand (by the relation's type), embed it, mention it (with the first sentence the mention stands in), or link to its address in their text: an http or https address ending in `#/item/` and the item's ID or one of its keys, current or former (D2), with the sentence the link stands in. Sentences are taken from the text as the reader reads it (`veiled`), so a protected passage shows its placeholder; a mention only in protected blocks the reader may not read is not told at all (ADR-0160). One entry per item and kind, with how often; newest change first; daily notes are listed in their own part instead.
- **Sharing labels:** readable live items that carry the item's labels, the most shared first; the label `daily-note` is left out, as notes are not related by being notes.
- **In daily notes:** the reader's readable daily notes (ADR-0205) that mention or link it, newest day first, with the sentence.
- **Bounds:** each list holds at most the new budget `ContextEntries` (50) and says when more were left out. The work is a scan of the readable items' bodies, like the search (ADR-0033) and the starmap (ADR-0157).
- **UI (`assets/context.js`):** a section **Context** in the item's side column, each entry a link to its item and a button to open it beside.

### Consequences

- **Positive:** every module, room, command and item is a few keys away; two items are read and written side by side without a second tab; an item shows where it is used, in context, which is how people decide whether a change is safe or a page is still needed (beside the impact view, ADR-0159).
- **Negative / trade-offs accepted:** the palette's item search is the server's word search, not letters-in-order matching; the recent items are per browser, not per person. Links are recognised by the item's address alone: an address of another installation that names an item of the same key counts too, as the link alone cannot tell which site it names. A sentence ends at a full stop, question or exclamation mark followed by a space, so "e.g. this" splits early; long sentences are cut around the reference. The pane shows one item; its fields are read-only there. The context scans every readable item's text per request; at the sizes of ADR-0081 this is the cost of a search.
- **Follow-ups / risks to watch:** unlinked mentions (titles in texts without a link) as a fourth part; letters-in-order matching on the server; keeping the pane's scroll position when the main item changes.

**Implementation so far:** everything above; tested by `internal/item/context_test.go`, `internal/server/api_daily_test.go`, `internal/web/jstest/palette.test.mjs` and `e2e/smoke.cjs`. Not built: unlinked mentions, editing fields in the pane.

## Pros and cons of the options

### Option 2: a server index for the palette

Strongest form: one index on the server with items, rooms and commands, ranked by a single fuzzy scorer and by what the person uses most, gives the best results and works the same in every client, MCP included.

- Good: one ranking for everything; usage-based ordering.
- Bad: commands are the shell's (the look, reading mode, the module switcher) and mean nothing to the server; a usage history per person is data about people to keep, back up and erase. The search already finds items with permissions applied (ADR-0033).

### Option 3: two full item pages split

Strongest form: the second item gets everything its own page has — fields, comments, history, access — with no reduced view to maintain.

- Good: nothing missing on either side.
- Bad: two side columns, two trees and two toolbars do not fit a laptop's width, let alone a phone (ADR-0116); the item page's code assumes it owns the whole view (one live session per page, one address). A focused pane with **Open on its own** gives the full page when it is wanted.

### Option 4: backlinks stored as relations

Strongest form: links to item addresses could be derived relations like mentions (D4 allows relations that are a function of the body), and listing them would then be an index lookup instead of a scan.

- Good: cheaper reads; links on the starmap and in the impact view too.
- Bad: a new derived relation type changes what every body implies, so the event codec and every derived-relation test would change (ADR-0025), and existing data would have to be re-derived. Sentences would still be read from the bodies. The scan is bounded and costs what a search costs.

### Option 5: the context computed in the browser

Strongest form: the browser already has the incoming relations, the labels and the search; joining them needs no new route.

- Good: no server code.
- Bad: links to item addresses are not relations and the browser cannot find them without reading every body it may read; sentences would need each referring item's body (one request per reference); a label's items need a request per label. The server has all of it in memory and applies the permission check once.

## Links

- relates to ADR-0033 (search), ADR-0081 (reads side by side and the sizes the store is built for), ADR-0088 and ADR-0160 (protected passages and blocks), ADR-0096 (writing aids, Ctrl+K), ADR-0101 (reading mode), ADR-0116 (phones), ADR-0157 (starmap), ADR-0159 (impact), ADR-0205 (daily notes), ADR-0206 (creating from a mention)
