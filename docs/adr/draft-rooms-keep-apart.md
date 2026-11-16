# ADR-DRAFT: Rooms on the starmap of all rooms keep apart as wholes

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: rooms should push each other off, so their outlines no longer overlap.

## Context and problem statement

The clouds of a room are tied together and outlined (ADR-0233), and each cloud keeps others off by its own size (ADR-0234).
Nothing kept rooms apart as wholes: the simulation's forces act between nodes, so the outline of a room with large clouds overlapped its neighbours' outlines while the clouds stayed apart.

How does the simulation keep rooms apart, and what else must follow so the result can be seen?

## Decision drivers

- Outlines of different rooms should not overlap.
- A room should move as a whole and keep the shape its ties give it.
- Maps without clouds behave exactly as before.
- The rule should be testable without a browser.

## Considered options

1. **Clusters in the simulation: each room's clouds taken as a circle round their middle, reaching their farthest cloud's collision radius; two circles closer than their reaches and a gap are pushed apart, every cloud of a room by the same amount.**
2. Stronger charge between clouds of different rooms.
3. Collide by the outlines themselves (convex polygons).

## Decision outcome

Chosen option: **1**, because it acts on rooms as wholes, keeps each room's shape and costs one pass over pairs of rooms per tick.

- `simulation({ clusters, clusterGap })` takes lists of node indices; each tick, after the collision, `keepApart` takes the active nodes of each cluster as a circle and pushes overlapping circles apart along the line between their middles, by a fifth of the overlap, the smaller cluster giving way more; pinned nodes stay.
- `roomClusters(nodes)` gives each room's clouds, rooms in the order of their keys; `null` on a map without clouds.
- `ROOM_GAP` is the outlines' margins beyond the clouds' collision on both sides, `2 × (HULL_PAD − CLOUD_MARGIN)`, and 6 of air; `HULL_PAD` (14) names the outline's margin `roomHull` uses.
- A circle encloses a convex outline, so circles kept apart keep outlines apart.

Two things followed, without which the result could not be seen:

- **Frame:** the view framed node middles with a fixed margin, so a large cloud with its outline could reach beyond the canvas.
  `fitView` now takes a point's reach `r`; a cloud is framed with its reach and `HULL_PAD`.
- **Captions:** at a middle zoom only items kept their captions (ADR-0158).
  Framed wholly, a grouped map often lies at a middle zoom, where clouds lost theirs.
  A cloud's caption names its room, type and count, so it now shows at a middle zoom as an item's does (`captionShown`, and the same rule in the style sheet).

### Consequences

- **Positive:** outlines no longer overlap; rooms read as separate regions; every cloud and outline lies within the frame with its caption.
- **Negative / trade-offs accepted:** a grouped map spreads wider and is framed at a lower zoom; a circle is a loose fit for a long, thin room, which keeps more room around it than its outline needs.
  The pass is quadratic in the number of rooms with clouds on the map (bounded by the budget of nodes).
- **Follow-ups / risks to watch:** whether outlines should name their room; whether a tighter fit than a circle is worth its cost.

## Pros and cons of the options

### Option 2: stronger charge between rooms

Strongest form: one more term in the existing repulsion, with no new structure in the simulation.

- Good: smallest change to the simulation.
- Bad: charge acts between clouds, not rooms; a large room still overlaps a small one whose clouds sit beside its edge, and a strong charge tears rooms' own clouds apart against their ties.

### Option 3: collide by the outlines

Strongest form: the exact shapes, so rooms pack as tightly as their outlines allow.

- Good: no wasted room around long rooms.
- Bad: polygon collision each tick needs the hulls inside the simulation and a push direction that jitters as hulls change; far more cost and code for a modest gain.

## Links

- amends ADR-0233 and ADR-0234 (rooms as wholes), ADR-0158 (captions at a middle zoom, the frame)
- relates to ADR-0191 (the groups)
