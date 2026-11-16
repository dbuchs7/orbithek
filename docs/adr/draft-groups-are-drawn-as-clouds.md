# ADR-DRAFT: The groups of the starmap of all rooms are drawn as clouds

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16: when many rooms are shown and the starmap of all rooms can only show bubbles, show clouds instead of bubbles. The requester chose the same day: only the shape changes, the grouping per room and type stays; clouds appear when bubbles did, beyond the budget `StarmapNodes`.

## Context and problem statement

Beyond the budget the starmap of all rooms holds a node per room and type, drawn as a circle whose area grows with its items (ADR-0191).
Items are circles too (ADR-0157), so a small group and an item look alike but for size, and a map of groups reads as a map of items.
The requester wants these groups drawn as clouds.

What does a group look like, and what else changes with it?

## Decision drivers

- A group should not be mistaken for an item at a glance.
- The grouping, the sizes, the layout and the links of ADR-0191 keep working as they are.
- Every colour stays a token of the look (ADR-0156); no picture is added.
- The shape should be testable without a browser.

## Considered options

1. **A cloud: lobes along the top and a flat bottom, an SVG path within the circle the bubble took, sized by `groupRadius`.**
2. Keep the circle and mark it otherwise, with a double outline or a pattern.
3. A cloud per room enclosing its groups, the groups staying circles.

## Decision outcome

Chosen option: **1**, because it is what was asked for and changes nothing but the shape.

- A group is drawn as `path.star-shape.star-cloud` with `cloudPath(groupRadius(count))` from `assets/starmap-graph.js`: five arcs from the bottom left round the top to the bottom right and a straight bottom at `CLOUD_BOTTOM` (half the radius below the middle), every end point within the circle of radius `groupRadius(count)`.
- The caption stands below the cloud's bottom.
  Fill, outline, hover, focus and match keep the styles of `.star-shape`.
- The server, the grouping per room and type, the budget, the edges and the simulation are unchanged.
- The UI and the manual call groups clouds (de: Wolken).

### Consequences

- **Positive:** groups and items differ by shape, not only by size; the change is confined to the drawing and has unit tests.
- **Negative / trade-offs accepted:** a cloud covers less of its circle than a disc, so a small group is a little smaller to the eye and to the pointer; its area still grows with its items.
- **Follow-ups / risks to watch:** the legend lists no entry for clouds, as it listed none for bubbles; whether it should is left open.

## Pros and cons of the options

### Option 2: keep the circle and mark it otherwise

Strongest form: a disc keeps the largest target for the pointer and the exact area-to-count relation, and a second ring is enough to tell it from an item.

- Good: no new shape; size reads exactly.
- Bad: not what was asked for; a ring is easy to miss at a low zoom, where groups are seen most.

### Option 3: a cloud per room around its groups

Strongest form: it shows which groups belong to one room, which a map of many rooms needs most.

- Good: rooms read as regions.
- Bad: not what was asked for; it needs a hull computed from the moving layout on every frame and changes how the map is laid out; the requester chose the shape alone.

## Links

- amends ADR-0191 (how a group is drawn)
- relates to ADR-0157 (shapes of the starmap), ADR-0156 (looks), ADR-0228 (a double-click opens a group's room)
