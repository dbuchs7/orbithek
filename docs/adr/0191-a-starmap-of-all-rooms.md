# ADR-0191: A starmap of all rooms, in bubbles per room and type when it is too large

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: a starmap of all items across every room; where they are too many, bubbles sized by the number of pages and tasks; then a label shown across every room from the search field. The requester chose the same day:
  - bubbles per room and type;
  - a label shows the items that carry it, in bubbles when they are too many.

  The rest are defaults proposed with the implementation and await the requester's confirmation.
- **Open question:** whether pictures belong on the starmap of all rooms; whether a bubble should open as the items of its type only, not its whole room; whether agents should read the map of all rooms over MCP
- **Question checked:** 2026-11

## Context and problem statement

A room's starmap (ADR-0157, ADR-0158) draws its items, pictures and shared labels, with related items of other rooms at its edge, within the budget `StarmapNodes` (600). The labels' starmap (ADR-0165) draws labels across rooms, but not the items. Nothing shows the items of every room together. An installation with thousands of items cannot draw them one by one, neither within the budget nor legibly.

How does a map show every room, and a label across them, at any size?

## Decision drivers

- **Unreadable is non-existent (D10):** nothing on the map names or counts an item the reader may not read.
- **Bounded** (ADR-0015): the map is derived per request within `StarmapNodes`; nothing is stored.
- **One view:** the map is the room's starmap with a choice more, so filters, panel, drilling and the address behave alike.

## Considered options

1. **All rooms as items within the budget; beyond it, a bubble per room and type sized by its items, with edges counting the links between them. A label narrows either.**
2. **Always bubbles per room.**
3. **The items most connected first, cut at the budget.**

## Decision outcome

Chosen option: **1** (bubbles per room and type, as the requester chose).

- **Route:** `GET /api/starmap` with an optional `label` (`Session.StarmapAll`).
- **Within the budget**, the map holds every readable live item of every room the reader reads, with containment, the relations the reader sees, and the labels at least two of its items carry, as a room's starmap does. Pictures are left out: the album shows them across rooms, and they would take half the budget.
- **Beyond the budget**, the map holds a node per room and type (`group`, with `count` its readable items), and an edge between two groups for the containment and relations between their items (`count` how many). Links within a group are not drawn. The answer says `grouped` and how many items it covers; groups beyond the budget are cut and said so.
- **Label:** `label` narrows the map to the items that carry it, in every room. Within the budget they are drawn one by one, beyond it in bubbles.
- **UI:**
  - *All rooms* heads the room chooser of the starmap (`#/starmap/*`).
  - A bubble is drawn with an area that grows with its items, up to a bound, and is filtered by its type as items are. Its panel tells the room, type and count and opens the room's map.
  - *Show a label in every room* suggests the labels in use (`/api/labels`) and keeps the label in the address (`?label=`).
- No format, event or budget is new.
- **MCP:** not offered; `get_starmap` keeps answering one room.

### Consequences

- **Positive:** every room on one map, at any size, and a label followed across rooms.
- **Positive:** the same view, filters and panel as a room's map.
- **Negative / trade-offs accepted:**
  - A bubble opens its whole room, not only its type.
  - A large installation shows bubbles until a label narrows it.
  - Pictures are not on this map.

## Pros and cons of the options

### Option 2: always bubbles per room

Strongest form: one picture of the installation, the same at any size, and the room maps for detail.

- Bad: a small installation, where every item fits, would hide the items it could show; the requester asked for the items first.

### Option 3: the most connected items first

Strongest form: the map always shows items, the important ones.

- Bad: what is left out depends on a ranking the reader cannot see, and a room without relations vanishes from the map.

## Links

- amends [ADR-0157](0157-a-starmap-of-a-space.md) (the starmap of a room)
- relates to ADR-0015 (budgets), ADR-0158 (moving and drilling), ADR-0165 (the labels' starmap), ADR-0022 (permissions)
