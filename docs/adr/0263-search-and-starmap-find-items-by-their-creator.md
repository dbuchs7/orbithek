# ADR-0263: Search and starmap find items by their creator; a dropped node stays until released

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-27
- **Deciders:** Project initiators (request of 2026-11-27 that the search finds the items of an author by part of the author's name, also on the starmap, without searching mail addresses; that nodes of the starmap can be moved by mouse with "Fit" and "Release" as in atlas; and that the starmap's filters stand in a column at the right as in atlas). Built in #410 and accepted by the requester on 2026-11-27.
- **Open question:** whether the display name (ADR-0246) is searched too, which needs other people to see it and so a record of its own; and whether people who changed an item, not only its creator, find it
- **Question checked:** 2026-11

## Context and problem statement

The search at the top (ADR-0033, ADR-0083, ADR-0092) finds an item by words of its title, key, text, properties, labels and comments.
It does not find an item by who made it.
The query language does (`creator = <username>`, ADR-0136), but only with the whole username, and one has to know the query exists.
The starmap's search (ADR-0157) compares title, key, name and labels in the browser and knows nothing of an item's creator either.

Who made an item is not stored on the item.
It follows from the log: the actor of the event that created it.

Asked at the same time: on the starmap a dragged node springs back into the simulation when it is let go (ADR-0158), so an arrangement made by hand is lost at once;
and its filters stand in a row above the map, the focused node's panel over the map's right edge.
Atlas's starmap keeps a dropped node where it is put, offers "Fit" and "Release", and has its filters and panel in a column at the right.

## Decision drivers

- One search box finds what one knows of an item; who made it is part of that.
- Who made an item is a fact of the log; it must not become a value anyone can change, and it must stay correct when nothing is rewritten.
- Nothing a caller receives reveals an item the caller may not read (D10), nor lists the accounts.
- Mail addresses are not searched (the request), and a display name is seen by its owner alone (ADR-0246).
- Search stays type-agnostic (D6).
- An arrangement made by hand on the starmap is kept while one looks at the map, and can be undone.

## Considered options

1. **Derive the creator when asked**: the search matches the creator's username as one more region of an item, the suggestions offer people, the starmap's nodes carry the creator's username.
2. **A label with the username**, set automatically when an item is made.
3. **A search syntax** such as `by:anna`.
4. **Store the creator on the item** as a property.

## Decision outcome

Chosen option: **1**, because the creator stays what the log says, no item is written to make it findable, and older items are found at once.

### Search

- **Matching:** `item.Query.Names` returns the words a user is found by; `Session.Search` adds the creator's words as one region of the item, scored like a property, below the title (ADR-0033, ADR-0092).
  Without `Names` nobody is found by who made an item.
- **Which words:** the username only.
  The account's mail address (a channel of messages, ADR-0061) is never searched;
  the display name is not either, because others do not see it (ADR-0246) and a search that found it would reveal it piece by piece and let anyone appear as another person by choosing that person's name.
  Under single sign-on the username is often a mail address (ADR-0030); it is searched then, as it is already shown beside comments and in histories.
- **Server:** the handler looks each creator up once per request (`itemHandlers.usernames`), so the item package keeps knowing no accounts, as with the query's users (ADR-0136).
  `GET /api/search` and `GET /api/search.csv` find by creator.
- **People:** `GET /api/people?q=` lists, per username holding `q`, how many readable live items that person made (`Session.Creators`), most first.
  Only makers of items the caller reads are named, so the answer neither lists the accounts nor reveals an unreadable item.
- **Suggestions:** the search at the top offers a group **People** between labels and items, at most four, each with how many items they made;
  choosing one opens the query `creator = "<username>" ORDER BY created DESC`.

### Starmap

- **Creator:** `StarNode.Creator` is the creator's user ID, derived from the log; the API gives it as `creator`, the username.
  The map's search finds an item by it, and an item's panel says who made it.
- **Dropped nodes stay:** a node one drags is pinned where it is dropped and marked by a glow;
  **Release this node** in its panel lets it go, **Release** on the map lets every one go, disabled while none is pinned.
  Pins outlast filters, drills and resizes and are forgotten when the map is left; nothing is stored.
  This amends ADR-0158, where a node was free again when let go.
- **Fit:** the zoom buttons, **Fit** (which frames what is shown, pinned nodes included) and **Release** float over the canvas's lower right corner.
- **A column at the right:** the search, the room, the label of every room, the depth or the impact's controls, the count and the filters stand in a column at the right of the map, with the focused node's panel and the impact list below them;
  the legend is under the map.
  On a narrow screen the column stands above the map, and the panel is a sheet at the bottom as before.
  A starmap in a text (ADR-0252) keeps its tools above the map and its filters folded below it, with the same controls on the canvas.

### Consequences

- **Positive:** typing part of a username finds what that person made, in the search and on the starmap, for every item made so far, without a field anyone can change.
- **Positive:** an arrangement made by hand stays on the map until one releases it.
- **Negative / trade-offs accepted:** substring matching finds `ann` in `johann` too (ADR-0033); such hits rank below hits in the title.
- **Negative / trade-offs accepted:** an item imported or synchronised is found by the account that imported it, not by the author in the other system.
- **Negative / trade-offs accepted:** each search looks up the creators of its candidates in the accounts, once per user and request.
- **Follow-ups / risks to watch:** finding by display name needs the display name shown to others first, with the username beside it and an audit of changes (ADR-0246); finding by contributors needs the walk over each item's log that `contributor` in the query language does, measured on large rooms first.

## Pros and cons of the options

### Option 1: derive the creator when asked

Strongest form: the creator is a fact of the log, read where it is; nothing is duplicated, so nothing drifts or can be forged.

- Good: works for every existing item at once; no write, no history noise.
- Good: no new kind of label or property; D4 and D6 hold.
- Bad: a lookup in the accounts per creator and request.

### Option 2: an automatic label with the username

Strongest form: labels are already searched (ADR-0092) and drawn on the starmap, so nothing in search or starmap has to change; the starmap would even show every person's items around one diamond.

- Bad: a label is a claim any editor can add or remove, so a search by author would find what someone labelled, not what someone made.
- Bad: every person becomes a label with a page, a board by label (ADR-0087), a place in the roadmap and the labels' starmap (ADR-0164, ADR-0165); a prolific author becomes a hub that bends the starmap's layout.
- Bad: older items lack the label; adding it writes an event to every item, which ADR-0258 would not do unasked.
- Bad: the label keeps a username that is later changed.

### Option 3: a search syntax

Strongest form: precise, no hits by accident.

- Bad: a syntax has to be known before it helps, which ADR-0092 rejected for labels; the query language already offers `creator`, and the suggestions now lead there.

### Option 4: store the creator as a property

Strongest form: a field like any other, shown, sorted and filtered with no special case.

- Bad: a second copy of a fact the log holds; changeable through the item's properties unless made a special read-only field, which no type has; older items lack it.

## Links

- amends [ADR-0033](0033-full-text-search-by-scanning-the-live-state.md) and [ADR-0092](0092-labels-in-search-and-task-counts-on-label-boards.md) (what the search finds and suggests)
- amends [ADR-0157](0157-a-starmap-of-a-space.md) (layout of the starmap) and [ADR-0158](0158-the-starmap-moves-live-physics-and-separating-a-neighbourhood.md) (a dropped node stays until released)
- respects [ADR-0246](0246-a-display-name-each-person-chooses.md) (the display name is seen by its owner alone) and [ADR-0061](0061-request-an-account.md) (mail addresses are channels)
- uses `creator` of [ADR-0136](0136-a-query-language-selects-items-by-their-fields.md); relates to [ADR-0261](0261-a-query-field-for-who-made-or-changed-an-item.md) (`contributor`)
