# ADR-0172: Calendars of other services in the calendar

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09 to show feeds of Google Calendar or Apple Calendar in the calendar; the three choices below were answered the same day)
- **Open question:** whether a room should have calendars of its own that all its readers see, whether Microsoft 365 accounts should be connected through Microsoft Graph rather than a published address, and whether the server should ever write back to another service; all three are left out here
- **Question checked:** 2026-11

## Context and problem statement

The calendar of ADR-0166 shows items, and ADR-0168 lets a calendar program subscribe to it. The other direction is missing: what people keep in Google Calendar, Apple iCloud, Outlook or a Nextcloud is not beside their tasks and meetings in the wiki, so they compare two calendars by eye.

How does a person see their calendars of other services in the wiki's calendar, without the wiki becoming a copy of them, without their credentials leaking, and without the server becoming a tool to reach addresses it should not?

The requester chose on 2026-11-09:

1. **Who sees a calendar:** the person who added it, nobody else (*personal*).
2. **How calendars are connected:** both ways: an iCalendar address, and an account at Google or a CalDAV server such as Apple iCloud.
3. **Whether it is allowed:** a switch in the administration.

## Decision drivers

- D10: what one person adds is theirs; nobody else reads it, and nothing of it reaches a room, a listing, a search or a feed.
- D1 and D3: an entry of another service is not content of the wiki and is not changed through it; it is shown, never made an item.
- A Google "secret address in iCal format", a CalDAV password and a Google refresh token are each a credential that reads a whole calendar. They are kept sealed, as engine grants are (ADR-0151), and never answered again.
- The server reads addresses people type in. It must not become a way to reach the server's own network (as for links, ADR-0039, and connectors, ADR-0127).
- Google's and Apple's servers do not let a browser read a calendar of another site (no CORS), and a password belongs on the server, not in a page.
- The calendar stays responsive: a slow or broken service delays and spoils nothing but its own entries.
- No new dependency (AGENTS.md); `golang.org/x/oauth2` is already one.

## Considered options

1. **The server reads, per person: iCalendar addresses, Google accounts by OAuth and the Calendar API, CalDAV accounts by discovery and calendar queries; credentials sealed in a journal of their own; entries shown beside the items, read only; turned on by an administrator.**
2. The browser reads the calendars itself.
3. The server copies other calendars into items, kept in step on a schedule.
4. iCalendar addresses only.
5. Accounts only (OAuth for Google, CalDAV for Apple and others).

## Decision outcome

Chosen option: **1**, because it is the only one that reaches Google and Apple at all (option 2 fails on CORS and would put passwords into pages), keeps the wiki free of copies it would have to keep in step and protect (option 3), and serves both kinds of people the requester named: those who paste an address and those who sign in to their account (options 4 and 5 each leave one out).

- **Storage:** `calendars.wal` in the data directory (`internal/extcal`, `extcal.Store`): per person their calendars (`Source`: kind `ics`, `google` or `caldav`, name, colour, hidden, an account and its calendar's ID) and accounts (`Account`: kind `google` or `caldav`, the account's name, a CalDAV server's address). An iCalendar address, a refresh token and a password are sealed with a key of their own in the keys file (prefix `extcal:`), destroyed when the calendar or account is removed; a key without its calendar is destroyed at the next start. Listings answer the host of an address, never the address. At most 30 calendars and 5 accounts per person.
- **Not backed up:** `calendars.wal` is left out of backups, as `connections.wal` is (ADR-0151): a restored or copied server must not read people's calendars with their credentials. People add them again.
- **iCalendar addresses:** `https://` or `webcal://` (read as `https://`); plain `http://` and addresses with a user name are refused, as the address is the credential. An address is read once when it is added, so a wrong one is told at once; a calendar that names itself (`X-WR-CALNAME`) names the calendar when the person gave no name.
- **Google:** connected by OAuth 2.0 with PKCE and offline access, asking only for `calendar.readonly`, through the Google client of single sign-on (`OT_GOOGLE_CLIENT_ID`, `OT_GOOGLE_CLIENT_SECRET`, ADR-0066) with the redirect address `{OT_PUBLIC_URL}/api/calendar-accounts/google/callback`, bound to the browser by a state cookie as engine connections are. The account is named by its primary calendar. Events are read with `singleEvents`, so Google expands repetitions itself. A refresh token Google no longer accepts is told as withdrawn; disconnecting revokes it at Google.
- **CalDAV:** an account is a server address, a user name and a password: for Apple iCloud `https://caldav.icloud.com`, the Apple ID and an app-specific password. Its calendars are found by `current-user-principal` at the address or at `/.well-known/caldav`, then `calendar-home-set` (RFC 6764); only collections that hold events are offered. A calendar is read by a `calendar-query` for the window (RFC 4791). The password goes only over `https`, also after a redirect.
- **Only what an account lists:** a calendar of an account is shown only if the account lists it, as its address is where the account's credential goes.
- **Reading iCalendar:** `internal/extcal` reads RFC 5545 without a dependency: folded and escaped lines, all-day spans, `DURATION`, zones by IANA name, by a prefixed name, by Outlook's Windows names (the common ones) or by the standard offset of the calendar's own `VTIMEZONE`, and repetitions with `FREQ` (daily to yearly), `INTERVAL`, `COUNT`, `UNTIL`, `BYDAY` with ordinals, `BYMONTHDAY`, `BYMONTH`, `BYSETPOS` and `WKST`, with `EXDATE`, `RDATE` and changed occurrences (`RECURRENCE-ID`). A rule with a part it does not know (`BYYEARDAY`, `BYWEEKNO`, `BYHOUR` …) shows its entry at its start only, never on days it does not repeat on. A time without a zone is taken in the calendar's zone (`X-WR-TIMEZONE`), else in the reader's.
- **Reading:** the server reads at most four calendars of a person at once, each at most 8 MB in 20 seconds, keeps what it read for 15 minutes and a failure for two, and reads Google and CalDAV by the window's months with a week on either side. Errors name the status, never the address. Only title, place and time are kept; descriptions, attendees and attachments are not read into the calendar.
- **The network:** requests go through the environment's proxy and only to public addresses (`connector.PublicOnlyUnless`); `OT_CALENDAR_ALLOW_PRIVATE=on` lets the server read calendars in its own network, such as an organisation's Nextcloud. That is the operator's decision, so it comes from the environment (ADR-0127).
- **Turning it on:** administrators turn calendars of other services on under *Administration → Calendars of other services* (`site.Calendars`, `CalendarsSet` in `site.wal`, `PUT /api/settings/calendars`). They are **off until turned on**, as the server then reads addresses people give it and keeps their credentials. While off, nothing is added, connected or read; calendars kept before stay and can be removed. The page says whether Google is set up, the redirect address to register, and whether the own network is allowed.
- **API:** `GET`, `POST /api/me/calendars`, `PUT`, `DELETE /api/me/calendars/{id}`, `GET /api/me/calendars/events?from&to&zone` (at most 400 days; `problems` names calendars that could not be read), `POST /api/me/calendar-accounts` (CalDAV), `GET /api/me/calendar-accounts/{id}/calendars`, `DELETE /api/me/calendar-accounts/{id}`, `GET /api/me/calendar-accounts/google/connect` and the public callback.
- **In the calendar:** a folded part *Other calendars* lists one's calendars, each in its colour, shown or hidden, removable; the accounts, whose calendars one chooses and which one disconnects; and the three ways to add one. Their entries stand beside the items in their colour, cannot be dragged, and a click tells their time, place and calendar. They are never part of a saved view's feed (ADR-0168).
- **Audit:** turning the setting on or off, adding and removing a calendar, connecting and disconnecting an account (`calendars.change`, `calendar-source.add`, `calendar-source.remove`, `calendar-account.connect`, `calendar-account.disconnect`).

### Consequences

- **Positive:** people see their days in one place; the wiki holds no copy of other calendars and has nothing of them to protect in rooms, search or backups; Google, Apple, Outlook, Nextcloud and any publisher of iCalendar work without a dependency.
- **Negative / trade-offs accepted:** the server keeps credentials that read whole calendars, sealed but available to the running server; a restored server has lost them; iCalendar repetitions are read in the parts calendar programs write, not in every part RFC 5545 allows; an address published by Outlook or Apple is public to whoever holds it, which the wiki cannot change; reading happens on the server, so a service sees the server's address, not the person's.
- **Follow-ups / risks to watch:** a calendar shared with a room; Microsoft 365 accounts through Microsoft Graph; a person's accounts when the person's account is deleted (their records stay sealed in `calendars.wal` until removed); an upper bound on what all people together make the server read, should many add calendars.

## Pros and cons of the options

### Option 1 — the server reads, per person (chosen)

Strongest form: the server is the only party that can reach every service, it can keep a credential sealed instead of handing it to a page, guard the addresses it reaches, keep what it read for a while, and give the browser one answer for all calendars.

- Good: works with Google, Apple and every other service; credentials never reach a page; one guard for every request; cached.
- Bad: the server keeps credentials and makes requests on people's behalf.

### Option 2 — the browser reads

Strongest form: nothing is kept on the server, the server reaches nothing, and each person's browser reads with their own network and sign-in.

- Good: no credential on the server; no request from the server.
- Bad: Google and Apple answer no browser of another site (CORS), so it does not work for the two services asked for; a CalDAV password would be held in a page; nothing is cached.

### Option 3 — copy into items

Strongest form: entries become items, so they are searchable, linkable, shown on boards and in feeds, and the calendar needs no second kind of entry.

- Good: one kind of entry; search and links for free.
- Bad: the wiki becomes a copy of private calendars that it must keep in step, protect per person (D10) and back up; deleted or changed events linger until the next round; a person's private appointments become content.

### Option 4 — iCalendar addresses only

Strongest form: one protocol every service can publish, no account, no OAuth, no password: the smallest thing that works.

- Good: simple; no client to register at Google.
- Bad: Apple publishes only public calendars, so private iCloud calendars need CalDAV; a secret Google address reads the whole calendar and cannot be limited or revoked from the wiki; the requester asked for both.

### Option 5 — accounts only

Strongest form: an account connection can be revoked at the service, reads exactly what the person allows, and needs no secret address.

- Good: revocable; Google's own expansion of repetitions.
- Bad: needs a registered Google client and an app-specific password at Apple; leaves out every service that only publishes an address (Outlook's published calendars, public holiday calendars, booking tools).

## Links

- relates to ADR-0166 (the calendar) and ADR-0168 (saved views and the feed, which other calendars never enter)
- relates to ADR-0066 (the Google client of single sign-on), ADR-0151 (sealed grants that are not backed up), ADR-0039 and ADR-0127 (public addresses only unless the operator allows the own network)
