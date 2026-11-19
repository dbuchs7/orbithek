# ADR-DRAFT: The module Planning is reached at #/planning; #/tasks sends on to it

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-19
- **Deciders:** Project initiators (the maintainer had the module *Tasks* renamed *Planning* in #380 and asked for its address to follow, with old addresses sent on, 2026-11-19)

## Context and problem statement

ADR-0049 built the task inbox at `#/tasks`, with `#/tasks/mine`, `#/tasks/unassigned` and `#/tasks/all` and its filters in the query.
Pull request #380 renamed the module *Tasks* to *Planning* (*Planung*) in the module switcher, on its tile and as its heading; its address stayed `#/tasks`.
The address a person sees and bookmarks now names a module that no longer exists by that name.
Links to `#/tasks` exist outside the code: bookmarks, links pasted into pages and comments, and the address of a browser tab left open.
The question is whether the address follows the name, and what becomes of the old one.

## Decision drivers

- The address names what it shows, as every other module's address does (`#/album`, `#/whiteboards`, `#/starmap`).
- No link or bookmark to the inbox breaks, including its tab and filters.
- The server builds no link to `#/tasks`; mails and notifications lead elsewhere (`#/notifications`, `#/admin`, `#/account`), so nothing stored by the server holds the old address.
- `#/tasks` in the links to an atlas engine's task app (`internal/engine`) is that application's address, not Orbithek's, and stays as it is.

## Considered options

1. `#/planning`, and `#/tasks…` sends on to the same place
2. Keep `#/tasks`
3. `#/planning` only, `#/tasks` no longer known

## Decision outcome

Chosen option: **"`#/planning`, and `#/tasks…` sends on to the same place"**, because it lets the address name the module without breaking a single existing link.

- The inbox is at `#/planning`, `#/planning/mine`, `#/planning/unassigned` and `#/planning/all`; the module switcher, the start page's tile, its count of tasks, the notice of waiting account requests and the trails of kanbans, the roadmap and the calendar link there.
- An address whose first part is `tasks` is replaced by the same address with `planning` in its place, the rest of the path and the query kept (`#/tasks/unassigned?space=PLAT` becomes `#/planning/unassigned?space=PLAT`).
  The replacement uses `location.replace`, so the old address leaves no step in the browser's history and *Back* does not lead into it again.
- The module keeps its internal ID `tasks` (its symbol, `module-tasks`) and the view keeps its element IDs (`#tasks`); only the address changes.

### Consequences

- **Positive:** the address, the switcher and the heading say the same; every old link, bookmark and open tab keeps working, filters included.
- **Negative / trade-offs accepted:** the section name `tasks` stays reserved for the forwarding and cannot be given to another view; a link to `#/tasks` costs one extra round through the router.
- **Follow-ups / risks to watch:** the forwarding is kept as long as old links may exist, which has no natural end; whether it may ever be dropped is not decided here.

## Pros and cons of the options

### Option 1: `#/planning`, and `#/tasks…` sends on

- Good: the address follows the name; nothing breaks.
- Bad: one reserved section name and a few lines in the router, kept indefinitely.

### Option 2: keep `#/tasks`

Strongest form: an address is an identifier, not a label; identifiers that never change are what makes links last, and the hash part of an address is rarely read by people.
Many applications keep an internal path long after renaming a feature, and this costs nothing.

- Good: no code, no forwarding, no risk.
- Bad: the address shown, copied into pages and read aloud in support names a module that no longer exists by that name, unlike every other module; the maintainer asked for the change.

### Option 3: `#/planning` only

Strongest form: no forwarding to keep forever; a link to an unknown section shows the start page, which is where the module's tile is anyway.

- Good: the least code.
- Bad: every bookmark and every link in a page to the inbox, a tab or a filtered list silently lands on the start page instead; the person loses the list they meant.

## Links

- refines the address of the inbox in ADR-0049, whose decision is otherwise unchanged
- follows the renaming of the module in #380
