# ADR-DRAFT: The clouds of a room lie together and are outlined

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: an outline around the clouds of one room.

## Context and problem statement

Beyond the budget the starmap of all rooms shows a cloud per room and type (ADR-0191, ADR-0230), coloured by room (ADR-0232).
Beyond six rooms the tones repeat, and the layout placed clouds by the map's order and their links, so the clouds of one room could lie far apart and among other rooms' clouds.
An outline drawn around a room's clouds would then cross the map and other rooms' outlines.

How are a room's clouds kept together, and how is the outline drawn?

## Decision drivers

- An outline must enclose its room's clouds and, as far as the layout allows, no other room's.
- The links between groups (ADR-0191) keep pulling as they did; no new server data.
- Colours stay the starmap's tones (ADR-0156, ADR-0232).
- The geometry should be testable without a browser and the same for the same map.

## Considered options

1. **Tie a room's clouds by springs that are not drawn, start each room around a centre of its own, and draw the convex hull of its clouds, widened by a margin, as a dashed outline in the room's tone.**
2. Draw the hull only, the layout unchanged.
3. Lay rooms out as fixed regions (a grid or a treemap) and place their clouds inside.

## Decision outcome

Chosen option: **1**, because it keeps the live, force-directed map and still keeps rooms apart.

- `roomLinks(nodes)` ties each room's first cloud to its other clouds, as edges of the kind `room` whose rest is the two clouds side by side (`groupRadius` of both plus 8).
  They go to the seeded layout and the live simulation with the drawn edges, and are never drawn, filtered or counted.
- `roomStart(nodes, mid)` starts each room around a centre of its own, the centres on a spiral in the order of the rooms' keys; `layout(graph, { start })` takes it in place of the spiral's start for those nodes.
- A link of the layout and of the simulation may carry a rest of its own (the third element of a link in `simulation`); every other link rests as before.
- `roomHull(circles, pad)` is the convex hull of the clouds' circles, each widened by the margin and sampled at sixteen points.
  The map draws it as `path.star-hull.tone-<n>` below the edges, for every room that shows two clouds or more, and redraws it as the clouds move.
  A room with one cloud gets none: its cloud and caption name it already.
- Legend and manuals say that a dashed outline gathers a room's clouds.

### Consequences

- **Positive:** a room reads as one region even when tones repeat; outlines rarely cross.
- **Negative / trade-offs accepted:** the layout of a grouped map changes: links between rooms pull less against the ties, and a room's clouds may sit closer than before.
  A convex hull can still overlap another room when a room's clouds are linked to far rooms, and a dragged cloud stretches its room's outline.
  The simulation's collision radius stays the same, so two large clouds of one room may overlap.
- **Follow-ups / risks to watch:** whether outlines should name their room, and whether the collision radius should follow the cloud's size.

## Pros and cons of the options

### Option 2: the hull only

Strongest form: no change to the layout, so the map looks as before but for the outlines, and the change is smaller.

- Good: the least change.
- Bad: rooms whose clouds lie apart get outlines spanning the map and crossing each other, as a first try showed; the outline then hides more than it shows.

### Option 3: fixed regions

Strongest form: regions never overlap, and their size can show a room's share of items at a glance.

- Good: a clean partition of the map.
- Bad: the links between rooms lose their pull and become long crossing lines; the live layout of ADR-0158 would give way to a second layout engine for grouped maps.

## Links

- amends ADR-0230 and ADR-0232 (how the clouds of a room are placed and drawn)
- relates to ADR-0191 (the groups and their links), ADR-0158 (the live simulation), ADR-0156 (looks)
