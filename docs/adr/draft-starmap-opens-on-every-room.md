# ADR-DRAFT: The starmap opens on every room, and a double-click opens a bubble's room

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: a double-click on a bubble of the starmap of all rooms should open the map of its room directly, as its panel's button does; the starmap should always open on all rooms.

## Context and problem statement

The starmap opens on the first room of the room select, favourites first (ADR-0157), unless the address names a room.
The starmap of all rooms (ADR-0191) is reached only by choosing *All rooms*.

A double-click on a node, or Shift+Enter, separates it with its neighbourhood (ADR-0158).
On a bubble of the starmap of all rooms that is of little use: a bubble has no links within itself, so separating it shows the bubble alone, or with the few bubbles it is linked to.
What one wants from a bubble is its room, which so far only the panel's button *Open the room's map* gives.

Two questions: which room does the starmap open on, and what does a double-click on a bubble do?

## Decision drivers

- The person asking wants the overview first and to go down into a room in one gesture.
- A double-click should do the most useful thing for the node it hits.
- The address keeps naming a room when one is wanted: links to a room's starmap keep working.
- The rule should be testable without a browser.

## Considered options

1. **Open on every room; a double-click (and Shift+Enter) on a bubble opens its room's map, on any other node it separates as before.**
2. Keep the first room as the start; only change the double-click.
3. Remember the last room chosen in the browser and open on it.

## Decision outcome

Chosen option: **1**, because it is what was asked for and keeps every address that names a room.

- `#/starmap` without a room, and an address naming a room the reader does not read, open on every room (`*`), no longer on the first room of the select (`startRoom` in `assets/starmap-graph.js`).
  An address that names a readable room opens that room, as before.
- A double-click or Shift+Enter on a bubble (`kind: group`) goes to `#/starmap/<room>`, the address the panel's button links (`roomAddress`).
  On items, pictures and labels it separates as ADR-0158 has it; in an impact view (ADR-0159) it still does nothing on them.
- The panel keeps both buttons, *Open the room's map* and *Separate*.

### Consequences

- **Positive:** the overview is the first thing one sees, and a room is one double-click away from it.
  The rule lives in two pure functions with unit tests.
- **Negative / trade-offs accepted:** on a large installation the first view is the bubbles of every room, which costs more to compute than one room (ADR-0191 bounds it by the starmap budget).
  Separating a bubble is now reached only through the panel's *Separate*.
- **Follow-ups / risks to watch:** whether a double-click on a bubble should open only its type in its room (ADR-0191 leaves this open).

## Pros and cons of the options

### Option 2: keep the first room as the start

Strongest form: a single room is smaller, faster and readable at once, and most people work in one room most of the time; favourites first already put their room first.

- Good: a cheaper first view; no change to ADR-0157's start.
- Bad: not what was asked for; the overview stays one choice away, and the room that comes first is chosen by order, not by intent.

### Option 3: remember the last room

Strongest form: it returns people to where they were, which is often where they want to be.

- Good: fits a person who works in one room.
- Bad: state kept in the browser, different per device, and not what was asked for; the start would no longer follow from the address alone.

## Links

- amends ADR-0157 (the start room) and ADR-0158 (the double-click on a bubble)
- relates to ADR-0191 (the starmap of all rooms and its bubbles), ADR-0159 (impact view)
