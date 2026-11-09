# ADR-0168: Saved calendar views, repeating dates with exceptions, and a calendar feed

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09 for saved views, repeating dates and a subscribable feed; the three choices below were answered the same day; accepted 2026-11-09)
- **Open question:** whether saved views should be shared with a room, and whether one occurrence of a series should carry its own title or fields; both are left out here
- **Question checked:** 2026-11

## Context and problem statement

The calendar of ADR-0166 keeps its whole state in its address. People who look at the same few calendars every day ("my open tasks", "the platform team's week") have to keep links. Dates happen once: a weekly meeting is one item per week, made by hand. And a calendar program (Outlook, Apple Calendar, Google Calendar, a phone) cannot show what Orbithek holds. ADR-0166 left repeating dates and a feed open as its question.

How does a person keep calendars under a name, how does an item repeat, with occurrences cancelled or moved, and how does a calendar program subscribe to a calendar, without giving away what its owner may not read?

## Decision drivers

- D1: a saved view is not content; it is a person's setting, like their favourite rooms (ADR-0145). An occurrence is not an item of its own.
- D3 and D4: a rule by which an item repeats is a validated property; anything the calendar places is read from properties, never from a body.
- D10: a feed shows only what its owner may read, now, and it opens nothing once its owner is gone or the feed is ended.
- ADR-0064: what leaves the product should be as little as serves its purpose; a calendar program needs a title and a time.
- A rule must give the same occurrences in the browser and in the feed.
- A time of day of a repeating meeting is kept in a time zone: 09:00 in Zurich stays 09:00 when summer time ends (ADR-0166 noted this).
- No new dependency.

## Considered options

1. **Views in the accounts journal; a field kind `recurrence` holding a rule with its exceptions; a feed per saved view, opened by a secret, as iCalendar with each occurrence an entry of its own.**
2. Views stored as system objects of the item store, like kanbans.
3. A repeating item as many items, one per occurrence, made by the server.
4. A feed with RRULE, EXDATE and RECURRENCE-ID, leaving the expansion to the calendar program.
5. A feed opened by an API token of scope `read` in its address.

## Decision outcome

Chosen option: **1**. The three choices were made by the requester on 2026-11-09:

- views are personal, to be shareable later;
- series have exceptions;
- a feed holds a title, a time and a link.

### Saved views

- **Model:** a person saves the calendar as it is shown under a name. A saved view keeps the parameters of the calendar's address, without the day shown, so it always opens on today. Saving under a name already used saves over that view.
- **Where kept:** `CalendarViewsSet` in the accounts journal, the whole list per user, as `FavouritesSet` keeps favourites (ADR-0145). At most 50 views per user, a name of at most 80 characters, parameters of at most 2 000.
- **API:** `GET`, `POST /api/me/calendar-views`, and `PUT`, `DELETE /api/me/calendar-views/{id}`. A token of scope `read` lists them only.
- **UI:**
  - a row *Saved view* above the filters, with a choice of the views, *Save view…* and *Remove view*;
  - the view shown is the one whose parameters equal the calendar's.

### Repeating dates

- **Field kind:** a field of the kind `recurrence` holds a rule as text (codec version 42). Its parts follow iCalendar where it has them:
  - `FREQ` (`DAILY`, `WEEKLY`, `MONTHLY`, `YEARLY`) and `INTERVAL` (1 to 999);
  - `BYDAY` (weekly only);
  - `UNTIL` (a day) or `COUNT` (1 to 1 000);
  - `TZID`, the zone a time of day is kept in;
  - `EXDATE` (days of cancelled occurrences) and `MOVE` (`day@new day` or `day@new dayTHH:MM`).
  An occurrence is named by the day it falls on by its rule.
- **Validation:** the server refuses a rule it cannot read (D3) and keeps it in one canonical form. At most 500 cancelled and 500 moved occurrences.
- **First occurrence:** the item's date fields give it, as ADR-0166 places any item. A month without the day (the 31st, 29 February) is skipped, as iCalendar skips it. A cancelled occurrence still counts towards `COUNT`.
- **Zones:** `time/tzdata` is embedded, because the container image has no zone database. This adds about 450 KB to the binary.
- **One rule, two implementations:** the browser (`assets/recurrence.js`) and the server (`item.Recurrence.Expand`) expand a rule alike. Both are tested with the same cases, `internal/item/testdata/recurrence-cases.json`.
- **In the calendar:**
  - A repeating item is read whenever it began by the last day shown; its occurrences are worked out in the browser.
  - An occurrence carries ↻, a moved one ↷, and says so in words.
  - Moving an occurrence asks whether only it moves, which writes a `MOVE` exception, or the whole series. A series move changes the item's dates, and its exceptions move with it.
  - Delete cancels one occurrence after asking, which writes an `EXDATE` exception.
- **On the item's page:** an editor in words chooses how often, every how many periods, the weekdays, the end, the zone, lists the cancelled and moved occurrences to take back, and cancels one by its day.
- **Queries:** a `recurrence` field takes `=`, `IN` and `IS EMPTY` (ADR-0136).

### The calendar feed

- **Address:** a saved view may have a feed. Its address `/calendar/{secret}.ics` holds a secret of 256 bits that starts `otcal_`, so secret scanners can find it. The secret is shown once and kept only as its hash.
- **Making and ending it:** `POST /api/me/calendar-views/{id}/feed` makes a new address and ends the old one; `DELETE` ends the feed. Both are audited (`calendar-feed.create`, `calendar-feed.revoke`). Removing the view ends its feed.
- **What it shows:** the feed reads as its owner reads (D10). The view's choices become the same query the browser writes (`internal/calendar`, a port of `calendar.js` tested with the same cases), over 90 days back and 400 ahead, at most 2 000 items.
- **Each entry:** the key and title, the start and end (a day as a whole-day entry, a time in UTC, an hour when it has no end) and a link back. The text and the comments are left out (ADR-0064).
- **Repeating items:** each occurrence is an entry of its own, with an identifier of the item and the occurrence's day. The feed carries no RRULE, so no time zone definition is needed and every program shows the same days.
- **A view that no longer reads:** when a field or type is gone, the feed answers 400 with the reason.
- **UI:** *Subscribe (iCal)* shows the address once, with a copy button and what holding it means; *New address* and *End feed* follow.
- **Turning feeds off:** `OT_CALENDAR_FEEDS=off` turns feeds off for a server whose data must not leave it this way, or whose gateway lets nothing through unsigned. No feed is made then (403), and every feed address answers 404, as an unknown one does; the secrets stay stored and work again when feeds are turned on, unless they were ended. Saved views and the calendar work as before; the calendar says that feeds are off and still offers to end a feed made before. Feeds are on by default; an unknown value stops the server at start.

### Consequences

- **Positive:**
  - people keep their calendars under names on every device;
  - a meeting is one item however often it takes place;
  - a calendar program shows Orbithek's dates without an account there;
  - the feed shows nothing its owner may not read.
- **Negative / trade-offs accepted:**
  - **The address is a credential:** whoever holds it reads titles and times until it is renewed or ended. Access logs may hold it, as with every secret in an address that calendar programs require. A server where that is not acceptable turns feeds off.
  - **Two implementations:** field guessing, query and expansion exist in the browser and on the server. Shared test cases keep them alike.
  - **Moving a series:** a series moves its exceptions by whole days; `UNTIL` stays.
  - **Moving by keyboard:** an occurrence moved by keyboard asks each time.
  - **Format version:** the codec version rises, so a data directory written by this build cannot be opened by an older one.
- **Follow-ups / risks to watch:**
  - sharing a view with a room;
  - an occurrence with a title or fields of its own;
  - `BYMONTHDAY` and "the second Tuesday";
  - writing back from a calendar program, which this design leaves out (D3).

## Pros and cons of the options

### Option 2: views as system objects of the item store

Strongest form: kanbans are stored there, with rooms and permissions; a view stored there could be shared with a room at once.

- Good: sharing comes with the room's permissions.
- Bad: a personal setting would enter the item store's log, its backups and its codec, and every room's members would see a list of others' private calendars to filter out. A view is a person's setting today; the accounts journal holds those. Sharing can later add room views beside personal ones.

### Option 3: one item per occurrence

Strongest form: every occurrence would have its own title, text, comments, state and history, and the calendar, search and boards would need nothing new.

- Good: an occurrence can differ in everything.
- Bad: a weekly meeting for two years is a hundred items made by the server, changing the series means changing all of them, and ending it means trashing them (D7, D8). The item model would hold derived copies (D4).

### Option 4: RRULE in the feed

Strongest form: a series stays a series in the calendar program, its future goes on without a window, and the feed is smaller.

- Good: one entry per series.
- Bad: a rule with a time of day needs its zone, and iCalendar requires a VTIMEZONE definition for every zone named, which the server would have to write from the zone database for every year. `MOVE` becomes RECURRENCE-ID entries. Programs differ in how they read all of it. Occurrences of their own, within a window renewed every hour, show the same in every program.

### Option 5: an API token in the address

Strongest form: tokens exist, have scopes and expiry, are audited, and a read token reads only.

- Good: no second kind of secret.
- Bad: a read token opens the whole API to whoever finds it in a calendar program's settings or a log, not one calendar. A feed secret opens one view's titles and times, nothing else, and is ended with it.

## Links

- answers the open question of ADR-0166 (calendar and datetime field kind)
- relates to ADR-0145 (favourites in the accounts journal), ADR-0027 (tokens and their hashes), ADR-0029 (audit trail), ADR-0064 (a message is a pointer, not a copy), ADR-0136 (query language), ADR-0022 (permissions, D10)
