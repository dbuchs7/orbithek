# ADR-0205: A daily note per person and day

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 for a daily note per person, a quick way to it, and its place among the day's work)
- **Open question:** whether a person may give their daily notes a start text of their own (a template per person), and whether weeks or months want notes of their own; this record decides neither
- **Question checked:** 2026-11

## Context and problem statement

People keep a running log of their day: what they planned, whom they spoke to, what they decided, what to follow up. In the product today such a log is a page someone makes by hand each morning, in some room, under some title, open to everyone who reads the room unless they protect it (ADR-0078). Finding yesterday's log means remembering where it went. Nothing ties the log to the day's other work: the tasks due that day (ADR-0166) and the pages created or changed that day.

How does each person get one note per day, private by default, found again by its day, and seen beside the day's work, without a second kind of content?

## Decision drivers

- D1: a daily note is user content, so it is an item. No `DailyNote` object, no separate store.
- D3 and D4: the day is a validated property, never read from a title or a body.
- D6: no type-specific code in search, permissions, history, comments or notifications. The mechanism may know field and label names the maintainer chose (as ADR-0162 knows `owner` and `assignee`), not depend on a type's name for what it finds.
- D9 and D10: a note is the author's own; restricted to them before anyone can read it.
- Exactly one note per person and day, also when two tabs open "today" at the same moment.
- ADR-0163: at critical storage nothing new is made, a note included.
- The person decides where their notes go, among rooms where they may create items; administrators define no extra room.
- Other features (a quick capture dialog) add text to today's note: a small function and route for it.

## Considered options

1. **An item of a ready-made type, labelled and dated, made in the person's chosen room, restricted to its author in the same batch; found by author, label and day.**
2. **A personal space per person**, holding their daily notes and nothing else.
3. **Personal notes as ADR-0107 keeps them**, outside items, one per day.
4. **An index of notes kept with the account**: the accounts journal maps each person's days to item IDs.
5. **Option 1, found by the type's name** instead of the label.

## Decision outcome

Chosen option: **1**, because it adds no stored object and no codec change, keeps the note an item with everything items have (history, comments, protections, relations, search, export), lets a person keep notes where their team works or in a room of their own, and makes "one per day" a question the store's single writer answers in one command.

- **Type:** a ready-made template `daily-note` in `item.TypeTemplates`: a body and a field `date` of kind `date`. A person's first note defines it in the same batch when the installation has no type of that name (actor `daily-notes`, as ADR-0162 defines its task type). A type of that name that cannot hold notes (no body, no `date` field of kind `date`, or another required field) is refused with the reason; it is not replaced.
- **What a daily note is:** a live item, made by its author (the actor of its creation), labelled `daily-note`, whose type, at the version it was made with, holds notes, with a `date`. Author, label and day identify it (`Store.IsDailyNote`); no index is kept, so moving a note to another room keeps it found, and removing its label makes it an ordinary page (a new note for that day is then made on the next open). A note someone else made is never one's own, even where one reads it.
- **The room:** each person chooses once the room their notes go to, among rooms where they hold Editor or higher; it is kept in the accounts journal (`DailyRoomSet`, `room`), cleared by an empty key. Changing it moves nothing: existing notes stay where they are.
- **Making a note:** `Session.OpenDailyNote(space, day, title)` returns the person's note of the day or makes it in their room: `ItemCreated` (title as given, else the day as `2026-11-12`; the UI names it in the person's language), `LabelsSet [daily-note]`, `RestrictionSet` read and edit `[author]` (ADR-0078) and `Watched`, in one batch. The single writer (`storage.DB.Write`) runs the lookup and the batch as one command, so concurrent requests make one note. Creation follows every rule of creating: the role in the room, ADR-0163 (the whole batch is refused at critical storage, the type's definition included), the governance guard (ADR-0040) and data protection holds (ADR-0058).
- **Adding text:** `Session.AppendToDailyNote(space, day, title, text)` appends a paragraph per non-empty line at the end of the note, making it first, in the same batch, when it is missing. This is the function a quick capture uses; the route is `POST /api/daily/{date}/paragraphs`, and the browser calls it through `appendToDailyNote(text, day)` in `assets/daily.js`.
- **Routes:** `GET /api/daily?from&to` (the room, whether one may still create there, the days with a note; at most `DailyNoteDays` days, a new budget), `PUT /api/daily/room`, `GET /api/daily/{date}`, `POST /api/daily/{date}` (open), `POST /api/daily/{date}/paragraphs`. A note to be made without a room chosen answers 409 with `needsRoom`. Days are `YYYY-MM-DD` in the caller's calendar; the server stores them at midnight UTC as every `date` value (ADR-0166).
- **MCP:** `open_daily_note` (opening never destroys anything; text is then written with `commit_body`). Choosing the room is a person's own setting and is not offered; nor are the days listed or text appended, which existing tools cover.
- **UI:** **Today** in the top bar, Alt+Shift+D and the command palette (ADR-0207) open `#/daily`, today where the browser is, opening the note at once. `#/daily/<date>` shows a day: the note's text in its live session (ADR-0010) in place, previous and next day, a month marking the days with a note (from `GET /api/daily`), the items created or last changed that day that the reader may read (one query of the query language, ADR-0136, with the day's bounds where the browser is), and the items whose dates fall on the day (the calendar's own fields and query, ADR-0166, daily notes left out). Another day's note is made only by **Start this day's note**, so browsing past days makes nothing. The view's parts are plain functions in `assets/dailyparts.js`, tested by `internal/web/jstest/daily.test.mjs`.

### Consequences

- **Positive:** a private log per day that is a full item: searchable by its author, linkable, exportable, commented on, protected further, mentioned in the context of the items it names (ADR-0207). Teams that want shared daily logs lift the restriction on a note as on any item.
- **Negative / trade-offs accepted:** relabelling or retyping a note by hand makes the mechanism lose it and make a new one; that is the price of keeping no index. A room administrator reads every note in their room (ADR-0022), as with any restricted item; a person who wants none to read them chooses a room they administer. "Changed that day" means "last changed that day": the query language knows an item's last change only, so an item changed that day and again later is listed on the later day. A note's day is the browser's: a person travelling across a date line names the day as they see it.
- **Follow-ups / risks to watch:** a start text for daily notes (the type's Word template, ADR-0167, already gives every new note one, which `AppendToDailyNote` refuses to mix with a first paragraph); a weekly review that gathers the week's notes; the start page could show today's note.

**Implementation so far:** everything above. Not built: a person's own start text, notes of a week or a month, today's note on the start page.

## Pros and cons of the options

### Option 2: a personal space per person

Strongest form: a room of one's own (*My space*) is a familiar idea; it gives daily notes, drafts and private pages one obvious home, permissions are the space's (no restriction per item), and nothing needs choosing.

- Good: no restriction per note; a place for other private pages too.
- Bad: spaces are system objects administrators create and name (ADR-0069); one per person multiplies them by the number of accounts, clutters every choice of a space (ADR-0147), and needs rules for leaving people, backups and quotas. It decides for teams that log in their own room that they may not. The restriction per item already makes a note private in any room.

### Option 3: personal notes outside items

Strongest form: personal notes on passages (ADR-0107) already exist outside items, sealed, seen by no one else; a daily note is personal too, and keeping it there avoids restrictions and room choices altogether.

- Good: private by construction; no room to choose.
- Bad: user content outside items breaks D1; such notes have no history, comments, relations, search by others when shared, export or MCP access, and could never become a team's page. Mentions from them would not be derived relations (D4), so the context of an item could not name them.

### Option 4: an index kept with the account

Strongest form: a map from day to item ID in the accounts journal finds a note at once, survives relabelling and retyping, and makes "one per day" a property of the index.

- Good: constant-time lookup; robust against edits to the note.
- Bad: a second record of what the item log already says, kept in a journal that is not replayed with the items; a restore of a space (ADR-0124), a purge or a move would leave it pointing at items that are gone or elsewhere, and keeping it in step is the kind of shadow data D4 forbids in spirit. A scan over the person's own items is cheap at the sizes the store is built for (ADR-0081).

### Option 5: found by the type's name

Strongest form: the type `daily-note` is the clearest marker; no label to remove by accident.

- Good: one marker fewer.
- Bad: D6's spirit — what an item is would hang on a type's name, which administrators may rename or reuse; a team that keeps notes in a type of its own could never have them found. The label is visible, queryable and chosen by nobody but the mechanism; the type is checked by its definition, not its name, once it exists.

## Links

- relates to ADR-0001 (one model), ADR-0022 and ADR-0078 (restrictions), ADR-0040 (governance guard), ADR-0046 (watching), ADR-0136 (query language), ADR-0145 (per-person settings in the accounts journal), ADR-0162 (a type defined in the batch that needs it), ADR-0163 (nothing new at critical storage), ADR-0166 (the calendar and `date` values), ADR-0207 (the palette, the item beside and the context)
