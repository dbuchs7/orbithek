# ADR-0232: The clouds of the starmap of all rooms take the tone of their room

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: the clouds of one room should share a colour.

## Context and problem statement

Beyond the budget the starmap of all rooms shows a cloud per room and type (ADR-0191, ADR-0230).
Every cloud had the same colour, so which clouds belong to one room was told only by their captions, such as "API · page" and "API · task".
Items take a tone by their type (ADR-0157), from six tones of the look that work in light, dark and cockpit (ADR-0156).

How is a cloud coloured, and what happens when there are more rooms than tones?

## Decision drivers

- The clouds of one room should be seen together at a glance.
- Colours stay tokens of the look (ADR-0156); no new palette.
- The same map should get the same colours every time.
- The rule should be testable without a browser.

## Considered options

1. **A tone per room, 0 to 5, by the place of the room's key among the rooms with clouds on the map, sorted; beyond six rooms the tones repeat.**
2. A tone per room from a hash of its key, stable across maps.
3. A larger palette, one colour per room up to its size.

## Decision outcome

Chosen option: **1**, because it uses the tones the starmap has, gives neighbouring keys different colours and is the same for the same map.

- `roomToneOf(nodes)` in `assets/starmap-graph.js` maps a room to its tone; a cloud carries `tone-<n>` as an item carries the tone of its type.
- The legend's entry for clouds and the manual say that the clouds of one room share a colour, and the manual that the colours repeat beyond six rooms.
- Nothing else changes: shapes, sizes, grouping, edges, simulation.

### Consequences

- **Positive:** the clouds of a room read as one; the colours stay within the look's tokens.
- **Negative / trade-offs accepted:** with more than six rooms on the map two rooms share a tone; the caption still names the room.
  A room's tone depends on the rooms on the map, so a label narrowing the map can change it.
  On a grouped map colour now tells the room, while on a map of items it tells the type; the legend says which.
- **Follow-ups / risks to watch:** whether many rooms need more tones or another cue, such as a hull around a room's clouds.

## Pros and cons of the options

### Option 2: a tone from a hash of the key

Strongest form: a room keeps its colour on every map and after a label narrows it, which helps people who learn a room by its colour.

- Good: stable across maps.
- Bad: two rooms side by side may hash to one tone even when only two rooms are shown; the colours spread no better than by order.

### Option 3: a larger palette

Strongest form: every room gets its own colour up to the palette's size, so rooms never share one in a typical installation.

- Good: fewer repeats.
- Bad: new colours outside the look's tokens, each to be checked for contrast in light, dark and cockpit; many hues are hard to tell apart anyway.

## Links

- amends ADR-0230 (how a cloud is coloured)
- relates to ADR-0191 (the groups), ADR-0157 (tones of types), ADR-0156 (looks)
