# ADR-0181: A photo's place set by hand, with suggestions from a place search

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 to set the place of photos by hand, with suggestions after the first letters typed; asked where suggestions come from and where a place is set, the requester chose a Photon place search on OpenStreetMap's data, and setting the place in the viewer, for several chosen pictures at once and by clicking on a map; asked the same day that administrators can turn the place search off)
- **Open question:** whether a list of places built into the binary should answer when no place search is reachable; whether places already used in the album should be suggested first; whether a place set by hand should also be written into the file's EXIF
- **Question checked:** 2026-11

## Context and problem statement

The album reads where a photo was taken from its EXIF (ADR-0177). Many photos tell no place: scans, pictures from cameras without GPS, and Google Photos, whose downloads leave the place out. Others tell a wrong one. People want to say where such a photo was taken, quickly, by typing a few letters of a place's name.

How is a photo's place set by hand, where do the suggestions come from, and how does a place set by hand relate to the place the file itself tells?

The requester chose on 2026-11-10:

1. **Suggestions:** from a Photon place search on OpenStreetMap's data, asked by the server. Nominatim's usage policy forbids suggestions while typing.
2. **Where:** in the viewer, for several chosen pictures at once, and by clicking on a map.
3. **Switch:** administrators can turn the place search off.

## Decision drivers

- **Readers' browsers reach no other site:** the server asks the place search, as it fetches map tiles (ADR-0179).
- **What is typed is sent to a service:** the operator chooses the service, administrators may turn it off, and a place can always be set on the map without it.
- **One truth for the place:** once a place is set or removed by hand, the file's own place must not reach readers through a download.
- **Permissions as for labels:** whoever may edit a picture's item sets its place (ADR-0090), but never a place its uploader keeps to themselves (ADR-0178).

## Considered options

1. **Photon asked by the server (`OT_GEOCODER`, by default photon.komoot.io), turned off by administrators; places set by hand recorded in the photo data with a name, and the file given to everyone but the uploader without its own place.**
2. A list of places built into the binary.
3. Nominatim.
4. Write the place set by hand into the file's EXIF.

## Decision outcome

Chosen option: **1**.

### The place search

- **Searches:**
  - `GET /api/places?q=` suggests up to eight places for 2 to 100 characters typed, named in the reader's language where the search knows it (de, en, fr, it) and else in its own names.
  - `GET /api/places/reverse?lat=&lon=` names a point chosen on a map.
  - Each place answers its name, a label of where it lies, and its point. Answers are kept in memory up to `PlaceSearchCacheBytes`.
- **Requests:** made by the server through its proxy and kept to public addresses unless `OT_MAP_ALLOW_PRIVATE=on`. They name the server like tile requests do, and the suggestions carry OpenStreetMap's credit.
- **The operator:** `OT_GEOCODER` names a Photon service. Unset, it is the public one; `off` turns the search off for good.
- **Administrators:** turn the search off and on under *Administration → Album → Place suggestions* (`site.Places`, `PlacesSet` in `site.wal`, `PUT /api/settings/places`, audited as `places.change`). It is on until turned off. While off, nothing is asked, and places are still set on the map.

### Setting the place

- **Routes:**
  - `PUT /api/items/{ref}/attachments/{id}/geotag` with `lat`, `lon` and `name` sets a picture's place.
  - `DELETE` on the same address removes it.
- **Who may:** whoever may edit the picture's item, unless its uploader keeps its place from them.
- **What is recorded:** photo data carry `PlaceName` and `PlaceManual` (codec version 48), with `AttachmentPlaceSet` holding the photo data as they are afterwards.
  - The height the file told goes.
  - Whether the place is private stays.
  - A picture stored before photo data were recorded has its data read from the file and recorded with the place.
- **The file:** once a place is set or removed by hand, everyone but the uploader gets the file without the file's own place, as for a hidden place (ADR-0178). The uploader keeps the original.
- **Copies:** a copy takes the place set by hand on its original, as far as the copier may know it.
- **In the UI:**
  - The viewer has a field *Place* with suggestions after two letters typed (arrow keys, Enter, Escape), *Choose on the map* and *Remove the place*.
  - The bar of chosen pictures sets one place for all of them, typed or chosen on the map.
  - The map to choose on sets the point with a click, names it with the place search where it can, and has a field to move to a place.

### Consequences

- **Positive:** photos without a place, or with a wrong one, get one in a few keystrokes or a click; the place set by hand is the one every reader sees and downloads; the search is replaceable and switchable.
- **Negative / trade-offs accepted:**
  - What people type into the place field goes to the place search, by default a public service of a third party.
  - The public Photon service is meant for fair use; a busy installation names its own.
  - Without the place search, a place is set on the map alone and has no name.
- **Follow-ups / risks to watch:** the open questions above.

## Pros and cons of the options

### Option 1 — Photon asked by the server (chosen)

Strongest form: a search built for suggestions while typing, on open data, finding streets, mountains and sights, reached by the server alone and under the operator's and administrators' control.

- Good: precise; replaceable; switchable; no data in the binary.
- Bad: typed text leaves the server; a public service's limits.

### Option 2 — a list built into the binary

Strongest form: nothing leaves the server, it answers at once and offline.

- Good: private, fast, always there.
- Bad: towns only, no streets or sights; megabytes more in the binary; the requester chose Photon.

### Option 3 — Nominatim

Strongest form: OpenStreetMap's own search, well known and precise.

- Good: precise.
- Bad: its usage policy forbids suggestions while typing.

### Option 4 — write the place into the file

Strongest form: the file and the data agree, and the place travels with the file anywhere.

- Good: one place in both.
- Bad: rewrites the original, which ADR-0178 keeps whole for the uploader; every change of the place would rewrite and re-encrypt the file.

## Links

- amends ADR-0177 (photo management in the album)
- relates to ADR-0178 (a place kept to its uploader) and ADR-0179 (map tiles through the server)
