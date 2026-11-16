# ADR-DRAFT: A cloud keeps other nodes off by its own size

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: the simulation's collision should follow the size of the clouds.

## Context and problem statement

The live simulation keeps active nodes at least two radii apart, one radius for every node (`DEFAULTS.radius`, 18; ADR-0158).
Clouds of the starmap of all rooms are drawn far larger, sized by `groupRadius` up to 64 (ADR-0191, ADR-0230), so large clouds lay on each other.

Measuring that showed a second fault.
ADR-0230 says a cloud lies within the circle the bubble took; that holds for the end points of its arcs, which its test checked, but its outer lobes bulge beyond them.
A cloud reaches 1.3 times its radius to its sides (0.77 up, 0.62 down).
The room's ties and outlines (ADR-0233) used the radius, so clouds could touch with their ties at rest and large clouds could reach beyond their outline.

How far does a cloud keep other nodes off, and by what size do the ties and outlines reckon?

## Decision drivers

- Clouds should not lie on each other; items, pictures and labels keep their spacing.
- One size for a cloud everywhere it matters: collision, ties, outline.
- The rule should be testable without a browser, against the drawn shape, not a stated one.
- Maps without clouds behave exactly as before.

## Considered options

1. **A radius per node for the collision: a cloud its reach (`CLOUD_REACH` 1.3 times its radius) and a margin of 6, every other node the default; ties and outlines reckon with the same reach.**
2. Shrink the cloud's shape to fit its radius, keeping one collision radius for all.
3. Collide by each node's drawn bounding box instead of a circle.

## Decision outcome

Chosen option: **1**, because it fixes the overlap where it arises and keeps the clouds as they look.

- `simulation({ radii })` takes a radius per node; two active nodes keep the sum of their radii apart.
  The grid that finds pairs is one largest diameter wide.
  Without `radii` the collision is unchanged.
- `nodeRadii(nodes, base)` gives a cloud `cloudReach(count) + CLOUD_MARGIN` and every other node `base`; on a map without clouds it gives `null`.
- `CLOUD_REACH` (1.3) and `cloudReach(count)` name how far a cloud reaches; a test computes the arcs of `cloudPath` as SVG defines them and checks the reach, no wider and not much narrower.
- A room's ties rest at the two clouds' reaches and margins plus 2, just outside their collision; a room's outline encloses circles of the clouds' reach.
  This replaces the radius ADR-0233 used for both.

### Consequences

- **Positive:** clouds, small or large, no longer lie on each other; the outline encloses its clouds wholly; the cloud's true width is pinned by a test.
- **Negative / trade-offs accepted:** a grouped map needs more room; the outline of a room with large clouds grows, and neighbouring rooms' outlines may overlap at their edges while their clouds stay apart.
  The circle is wider than a cloud is tall, so clouds keep more space above and below than they need; captions use part of it.
- **Follow-ups / risks to watch:** whether rooms should push each other off as wholes, so their outlines no longer overlap.

## Pros and cons of the options

### Option 2: shrink the cloud to its radius

Strongest form: the statement of ADR-0230 becomes true, and nothing in the simulation changes.

- Good: no new parameter in the simulation.
- Bad: clouds shrink by almost a quarter and still lie on each other, since the collision radius stays 18 while clouds reach up to 64.

### Option 3: collide by bounding boxes

Strongest form: boxes fit the wide, low cloud better than a circle and would pack the map tighter.

- Good: less wasted room above and below a cloud.
- Bad: box collision has no single push direction and jitters in a force simulation; it costs more per pair and changes the behaviour of every node, not only clouds.

## Links

- amends ADR-0230 (the cloud's width) and ADR-0233 (the reach the ties and outlines use)
- relates to ADR-0158 (the live simulation), ADR-0191 (the groups)
