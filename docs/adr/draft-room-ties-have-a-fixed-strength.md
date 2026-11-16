# ADR-DRAFT: A room's ties keep a fixed strength, links between rooms pull weakly, and rooms push each other off by a bounded amount

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-16
- **Deciders:** Project initiators. Request of 2026-11-16, after a load test of the starmap of all rooms: room ties of a fixed strength, so that links between rooms no longer pull a room's clouds apart.

## Context and problem statement

A load test of the starmap of all rooms (100 rooms, 300 clouds, 300 relations between items of different rooms) showed rooms torn apart: single clouds followed their links into other rooms, outlines (ADR-0233) ran across the map, and the farthest two clouds of one room lay up to 1163 apart beyond their reaches (median 106).
The assumed cause was that a room's ties are weakened, as d3 weakens every link, by one over the smaller degree of its ends.

Measuring each part offline, on the load test's own map, showed more:

- A fixed tie strength alone made it worse (median 223), because the ties no longer counted toward the degree and so the links between rooms pulled harder.
- Weaker ties made the map explode (a cloud flung 31 000 away): the push that keeps rooms apart (ADR-0235) grew without bound with a room's spread, flung rooms against their links and spread them further.
- The links between rooms, not the weakness of the ties, were what tore rooms apart.

## Decision drivers

- A room's clouds stay together whatever links they have.
- Outlines of different rooms do not overlap.
- Links between rooms still pull rooms towards each other, so linked rooms lie nearer.
- Maps without clouds behave exactly as before; the measures are repeatable.

## Considered options

1. **Ties of a fixed strength that still count toward the degree; links between clouds of two rooms at a tenth of their usual strength; the push between rooms bounded per tick; ties four times as strong in the seeded layout.**
2. Ties of a fixed strength alone, as requested.
3. A push between rooms that cools with the simulation.

## Decision outcome

Chosen option: **1**, because it is the only one measured that keeps rooms whole and outlines apart at once.

- A link of the simulation is `[a, b, distance, strength, weight]`.
  With a `strength` it keeps it whatever degree its ends have; otherwise it takes d3's strength times its `weight` (1 if none).
  Every link counts toward its ends' degree, so links without a strength weaken as before.
- `roomLinks` gives each tie `strength: ROOM_TIE_STRENGTH` (1); the seeded layout pulls a tie four times as hard as a link.
- `linkWeight(edge, byID)` gives an edge between clouds of two different rooms `ROOM_LINK_WEIGHT` (0.1), every other edge 1; on a map of items every edge keeps 1.
- `keepApart` pushes two overlapping rooms apart by a fifth of their overlap, at most `DEFAULTS.clusterPush` (120) per pair and tick, at every temperature, so rooms part at rest too.

### Measurements

Spread is, per room, the farthest two of its clouds less their reaches (median / p90 / max); outlines overlapping by a separating-axis test on the drawn outlines; in the browser after the map rests.

| Map | Before (main) | After |
|---|---|---|
| 100 rooms, 300 relations | 106 / 191 / 1163, 2–4 outlines overlapping | 34 / 44 / 153, 0 |
| 100 rooms, no relations | 51 / 75 / 110, 0 | 50 / 93 / 112, 0 |
| 200 rooms, 600 relations | 103 / 297 / 1909, 9; zoom at its floor 0.1 | 48 / 65 / 180, 2; zoom 0.175 |
| 50 rooms, no relations | spread not measured, 0 | 53 / 78 / 113, 0 |

Frame times are unchanged (median 16.7 ms up to 300 clouds, 33 ms at 600).

### Consequences

- **Positive:** rooms stay whole with many links between them; outlines no longer run across the map; a large map no longer spreads to the zoom's floor.
- **Negative / trade-offs accepted:** links between rooms pull weakly, so linked rooms lie less near each other than before; their lines are longer.
  With 600 clouds and 600 links a few outlines may still touch.
- **Follow-ups / risks to watch:** the values were found on generated maps (even room sizes, three types, links in a fixed pattern); a real installation may want other ones.

## Pros and cons of the options

### Option 2: fixed ties alone

Strongest form: the smallest change, and it addresses the assumed cause directly.

- Good: one parameter.
- Bad: measured worse than before (median 223 against 106), since uncounted ties strengthen every other link, and counted ones change little where ties already had strength 1.

### Option 3: a cooling push between rooms

Strongest form: forces in a simulation usually cool with it, and a cooling push cannot fling rooms late.

- Good: rooms tore less (median 61).
- Bad: rooms no longer parted at rest; outlines overlapped (up to 159 pairs), and even without links room circles overlapped.

## Links

- amends ADR-0233 (ties), ADR-0235 (the push between rooms)
- relates to ADR-0158 (the live simulation), ADR-0191 (the groups and their links)
