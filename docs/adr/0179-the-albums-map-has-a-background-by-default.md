# ADR-0179: The album's map has a background by default, OpenStreetMap's

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 that the album's map always loads a background, for instance OpenStreetMap's)
- **Open question:** whether an installation with many readers should be warned that OpenStreetMap's tile servers are meant for light use; whether tiles should be kept on disk, as tile services recommend, rather than in memory only
- **Question checked:** 2026-11

## Context and problem statement

The album's map (ADR-0177) draws its points over tiles the server fetches from the service named in `OT_MAP_TILES`; without it, the map shows a grid of degrees. Few operators set the variable, so most people see a grid. The map also showed no credit, which tile services ask for.

Should the map have a background without the operator naming one, and which?

## Decision drivers

- People recognise a place on a map, not on a grid.
- Readers' browsers still reach no other site: the server fetches the tiles (ADR-0177).
- The tile service's terms: a credit shown with the map, an application that names itself, tiles kept a while.
- The operator can still choose another service, or none, for privacy, load or contract.

## Considered options

1. **OpenStreetMap's tile service unless the operator names another or `off`, with its credit, the server named in the User-Agent with its public address, tiles kept a week by browsers.**
2. Keep the grid until the operator names a service.
3. Ship tiles in the binary.

## Decision outcome

Chosen option: **1**.

- **Default:**
  - Without `OT_MAP_TILES` the map draws `https://tile.openstreetmap.org/{z}/{x}/{y}.png`, with the credit «© OpenStreetMap contributors» shown in its corner.
  - `OT_MAP_TILES` names another service.
  - `OT_MAP_ATTRIBUTION` gives that service's credit.
  - `OT_MAP_TILES=off` draws no tiles, as before.
- **Naming the server:** tile requests carry `Orbithek/<version> (album map; +<OT_PUBLIC_URL>)`, as OpenStreetMap's usage policy asks of applications.
- **Caching:** tiles are kept in memory up to `MapTileCacheBytes` and in browsers for a week.
- **Documentation:** README and `compose.yaml` say that OpenStreetMap's servers are meant for light use and how to name another service.

### Consequences

- **Positive:** a map with a background everywhere, without setup; the credit is shown as asked.
- **Negative / trade-offs accepted:** every installation's server now contacts OpenStreetMap when someone opens the map, unless turned off; a busy installation must name its own or a paid service to stay within OpenStreetMap's policy.
- **Follow-ups / risks to watch:** a cache of tiles on disk; a warning to administrators of heavy use.

## Pros and cons of the options

### Option 1 — OpenStreetMap by default (chosen)

Strongest form: the free, open map most people know, reached by the server alone, with its terms met.

- Good: works without setup; open data; replaceable.
- Bad: an outbound request by default; OpenStreetMap's servers are not meant for heavy use.

### Option 2 — the grid until named

Strongest form: no request to a third party unless the operator decided it.

- Good: nothing leaves the server by default.
- Bad: most people see no map they can read; the requester asked for a background always.

### Option 3 — tiles in the binary

Strongest form: no request at all, offline.

- Good: private and offline.
- Bad: tiles of the world at useful zooms are hundreds of gigabytes; even a few zoom levels bloat the single binary (ADR-0024).

## Links

- amends ADR-0177 (photo management in the album), whose map showed a grid without a named tile service
- relates to ADR-0024 (one binary)
