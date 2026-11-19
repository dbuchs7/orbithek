# ADR-DRAFT: Planning and My journal are reached at #/planning and #/journal; the old addresses send on

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-19
- **Deciders:** Project initiators (the maintainer had the module *Tasks* renamed *Planning* and *My day* renamed *My journal* in #380, and asked for their addresses to follow, with old addresses sent on, 2026-11-19)

## Context and problem statement

ADR-0049 built the task inbox at `#/tasks`, with `#/tasks/mine`, `#/tasks/unassigned` and `#/tasks/all` and its filters in the query.
ADR-0205 built the daily notes at `#/daily`, today's note, and `#/daily/<date>`, a day's note.
Pull request #380 renamed the module *Tasks* to *Planning* (*Planung*) and the daily note's tile and module entry *My day* / *Today* to *My journal* (*Mein Journal*); their addresses stayed `#/tasks` and `#/daily`.
The address a person sees and bookmarks now names a module that no longer exists by that name.
Links to both exist outside the code: bookmarks, links pasted into pages and comments, and the address of a browser tab left open.
The question is whether the addresses follow the names, and what becomes of the old ones.

## Decision drivers

- An address names what it shows, as every other module's address does (`#/album`, `#/whiteboards`, `#/starmap`).
- No link or bookmark to either view breaks, including the inbox's tab and filters and a day's note.
- The server builds no link to `#/tasks` or `#/daily`; mails and notifications lead elsewhere (`#/notifications`, `#/admin`, `#/account`), so nothing stored by the server holds an old address.
- `#/tasks` in the links to an atlas engine's task app (`internal/engine`) is that application's address, not Orbithek's, and stays as it is.

## Considered options

1. New addresses `#/planning` and `#/journal`; the old ones send on to the same place
2. Keep `#/tasks` and `#/daily`
3. New addresses only; the old ones no longer known

## Decision outcome

Chosen option: **"New addresses `#/planning` and `#/journal`; the old ones send on to the same place"**, because it lets the addresses name the modules without breaking a single existing link.

- The inbox is at `#/planning`, `#/planning/mine`, `#/planning/unassigned` and `#/planning/all`; the module switcher, the start page's tile, its count of tasks, the notice of waiting account requests and the trails of kanbans, the roadmap and the calendar link there.
- The daily notes are at `#/journal` (today) and `#/journal/<date>`; the start page's tile, the module switcher, the top bar's *Today* button, Alt+Shift+D, the command palette, the view's own *Today* and its days before and after link there.
- An address whose first part is `tasks` or `daily` is replaced by the same address with `planning` or `journal` in its place, the rest of the path and the query kept (`#/tasks/unassigned?space=PLAT` becomes `#/planning/unassigned?space=PLAT`, `#/daily/2026-11-12` becomes `#/journal/2026-11-12`).
  The replacement uses `location.replace`, so the old address leaves no step in the browser's history and *Back* does not lead into it again.
  One table in the router (`movedSections` in `assets/app.js`) holds the old and new first parts.
- The modules keep their internal IDs `tasks` and `daily` (their symbols), the views keep their element IDs (`#tasks`, `#daily-note`) and the type keeps its name `daily-note`; only the addresses change.

### Consequences

- **Positive:** the address, the switcher and the heading say the same; every old link, bookmark and open tab keeps working, filters and dates included.
- **Negative / trade-offs accepted:** the section names `tasks` and `daily` stay reserved for the forwarding and cannot be given to another view; a link to an old address costs one extra round through the router.
- **Follow-ups / risks to watch:** the forwarding is kept as long as old links may exist, which has no natural end; whether it may ever be dropped is not decided here.

## Pros and cons of the options

### Option 1: new addresses, the old ones send on

- Good: the addresses follow the names; nothing breaks.
- Bad: two reserved section names and a few lines in the router, kept indefinitely.

### Option 2: keep `#/tasks` and `#/daily`

Strongest form: an address is an identifier, not a label; identifiers that never change are what makes links last, and the hash part of an address is rarely read by people.
Many applications keep an internal path long after renaming a feature, and this costs nothing.

- Good: no code, no forwarding, no risk.
- Bad: the address shown, copied into pages and read aloud in support names a module that no longer exists by that name, unlike every other module; the maintainer asked for the change.

### Option 3: new addresses only

Strongest form: no forwarding to keep forever; a link to an unknown section shows the start page, which is where both modules' tiles are anyway.

- Good: the least code.
- Bad: every bookmark and every link in a page to the inbox, a tab, a filtered list or a day's note silently lands on the start page instead; the person loses what they meant to open.

## Links

- refines the address of the inbox in ADR-0049 and of the daily notes in ADR-0205, whose decisions are otherwise unchanged
- follows the renaming of the modules in #380
