# ADR-0177: Photo management in the album, with imports from Google Photos and Apple

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 to manage photos in the album, covering what photo libraries commonly offer, and to bring in Google Photos and Apple Photos so photos are managed in one place; the four choices below were answered the same day; the option to remove the place on import was asked for the same day; accepted 2026-11-10)
- **Open question:** whether a shared iCloud album should be followed on a schedule rather than updated on request; whether older pictures should have their photo data recorded in one run; whether places should be told only to the person who added the picture, beyond removing them on import as one may now; whether videos, faces and similar pictures belong in the album
- **Question checked:** 2026-11

## Context and problem statement

The album (ADR-0054) lists the pictures of every item one may read; folders hold pictures of their own, pictures carry labels (ADR-0090), the versions of a file form one stack (ADR-0103) and every picture exports (ADR-0173). It is a place for the pictures of pages and tasks. It is not a place for photos: it knows nothing of when or where a photo was taken, it loads every picture whole, it has no way to mark, choose or download many at once, and photos kept in Google Photos or Apple Photos stay there.

How does the album become a place to manage photos — by time, by place, with favourites and choices of many — and how do photos from Google Photos and Apple Photos get into it, without a second model of access, without long-lived credentials of other services on the server, and without breaking the single binary?

The requester chose on 2026-11-10:

1. **Google Photos:** through the picker, where one chooses photos at Google and the server brings them in.
2. **Apple Photos:** both files exported from Photos or an iPhone (also as a ZIP archive) and the public address of a shared iCloud album.
3. **The first step:** a timeline with photo data and thumbnails; favourites and choosing many; a map and memories of the same day in earlier years.
4. **HEIC:** not taken; JPEG, PNG, GIF and WebP only.

What the services allow, as checked on 2026-11-10:

- Since 31 March 2025 the Google Photos Library API reaches only media an application created itself; reading a person's library is done with the Photos Picker API, where the person chooses media in a Google page and the application reads what was chosen for a limited time. A whole library cannot be followed.
- Apple has no public interface to iCloud Photos. A shared album with a public website is read by web pages through an interface Apple does not document (`sharedstreams`); it holds pictures at a reduced size and may change without notice.
- The free, self-hosted photo libraries found are licensed AGPL-3.0, which would fit, but each needs its own database and services; none can become part of a pure-Go single binary (ADR-0009, ADR-0024).

## Decision drivers

- **One model (D1, D9, D10):** a photo is a file of an item — a folder of the album or any page — read, encrypted, backed up and erased as every file (ADR-0045, ADR-0026). Nothing about a photo, not its place, reaches someone who may not read its item.
- **Queryable data is recorded (D4):** when and where a photo was taken is told by the file, but the album sorts and filters by it, so it is recorded with the file, not read from every file on every listing.
- **No long-lived credential of another service:** a Google grant that reads a library is not kept beyond the import that uses it.
- **One binary, no new dependency (ADR-0009):** the standard library reads JPEG, PNG and GIF; EXIF is read by own code; the map is drawn by the UI itself.
- **Untrusted input:** photos and the answers of other services are read with bounds; the server reaches only the services' own hosts for imports, and public addresses for map tiles.
- **Privacy:** a place a photo was taken can be someone's home. Map tiles are fetched by the server, so readers' browsers do not contact a tile service, and only where the operator names one.

## Considered options

1. **Photo data recorded with the file at upload, thumbnails made on demand and kept in memory, favourites per person, a timeline, a map and memories as views of the album; Google Photos through the picker without keeping its grant, Apple through files and through shared albums read on request; turned on by the operator.**
2. Run a free, self-hosted photo library beside the wiki and link it.
3. Read photo data from the file on every listing; record nothing.
4. Thumbnails made at upload and stored as files of their own.
5. Favourites as a mark on the picture, the same for every reader.
6. Follow the whole Google Photos library, or iCloud Photos with an Apple ID.

## Decision outcome

Chosen option: **1**.

### Photo data

- **Recorded at upload:** when a file is stored, the server reads what the picture tells about itself (`internal/photo`): from EXIF in JPEG (APP1), PNG (`eXIf`) and WebP (`EXIF`), the time of capture (`DateTimeOriginal` with its offset when given), the place (GPS latitude, longitude and height), camera, lens, exposure, aperture, ISO, focal length and orientation; the pixel size from the image header. It is kept on the attachment as `Photo`, sealed with the rest of the attachment (codec version 45) and erased with it. An import that knows a time the file does not tell (Google's creation time, iCloud's date) gives that time.
- **Pictures stored before:** have no recorded photo data. The album sorts them by the day they were added; the picture's information reads its data from the file when asked (`GET /api/items/{ref}/attachments/{id}/photo`) without recording it.
- **HEIC:** a file the server cannot show is refused by the album's imports with a message saying how to export JPEG; it may still be attached to a page as any file.

### Thumbnails

- `GET /api/items/{ref}/attachments/{id}/thumbnail?edge=` answers the picture scaled to 400 or 1600 pixels on its longer edge, turned upright by its orientation, as JPEG (PNG when it has transparency), with the access of the file itself. It is made on demand by at most `ThumbnailWorkers` at once, from pictures of at most `ThumbnailPixels`, and kept in memory up to `ThumbnailCacheBytes`; nothing is written to disk. A picture the server cannot scale (WebP, a larger one) is answered whole.

### Views of the album

- **Timeline:** `GET /api/album?order=taken` lists pictures by the time they were taken, newest first, with `from` and `to` (days) and `GET /api/album/months` counting them per month; the album groups them by day under the months.
- **Memories:** `day=MM-DD` lists the pictures taken on that day in earlier years; the album shows today's.
- **Map:** `GET /api/album/places` lists the readable pictures with a place, at most `AlbumPlaces`, which the album draws as clustered points on a map of its own (Web Mercator). Map tiles come through the server (`GET /api/map/tiles/{z}/{x}/{y}`) from the tile service the operator names in `OT_MAP_TILES` (a template with `{z}`, `{x}`, `{y}`), only public addresses, kept in memory up to `MapTileCacheBytes`; without it the map shows the points on a grid of degrees.
- **Favourites:** each person marks pictures as their own favourites (`PUT /api/album/favourites`, `PictureFavouriteSet` in the accounts journal, at most `PictureFavourites`); a favourite names the item and the first version of the file, so it stays through later versions. `favourites=on` lists them. A favourite of a picture one can no longer read is not shown.
- **Choosing many:** the album chooses pictures by click, Shift-click for a range, or all shown; the chosen ones are labelled, marked as favourites, moved to a folder, deleted or downloaded together. Moving copies with the source and deletes the original where one may (ADR-0054, ADR-0055), in the browser as before. `POST /api/album/zip` streams a ZIP archive of chosen pictures, each read with the caller's rights, at most `AlbumZipPictures` and `AlbumZipBytes`.

### Imports

- **Files and ZIP archives (Apple Photos, an iPhone, any camera):** chosen or dropped onto a folder; a ZIP archive is unpacked in the browser, so the server holds no archive and no unpacked file outside the item's encryption. Pictures of a format the album does not show are listed as skipped.
- **Google Photos:** turned on with `OT_PHOTOS_GOOGLE=on` and the Google client of single sign-on (ADR-0066) with the Photos Picker API enabled. One signs in at Google with the scope `photospicker.mediaitems.readonly`, online access only: the access token is held in memory for its hour and no refresh token is asked for or kept. The server opens a picker session; one chooses photos in Google's page; the album then brings each chosen photo into the folder (`=d`, the original with its EXIF), skipping videos and pictures the folder already holds (same SHA-256), and closes the session. Google says its download leaves out the place; such photos have no place.
- **Shared iCloud albums:** turned on with `OT_PHOTOS_ICLOUD=on`. One gives the address of a shared album with a public website; the server reads the album and brings its photos into the folder, skipping videos and pictures the folder holds. Updating is giving the address again: only new photos come. The album remembers the address per folder in the browser only. The interface is not documented by Apple; when it changes, imports fail with a message and nothing else is affected. Shared albums hold reduced copies (Apple's limit), not originals.
- **Leaving the place behind:** a folder's imports offer *Remove the place on import*, a choice the browser remembers. With it the server reads each picture whole, up to `AttachmentBytes`, empties the GPS directory of its EXIF (values and entries overwritten with zeros, a PNG's checksum written again) and drops its XMP metadata, which may repeat the place, before it stores the file (`photo.StripPlace`; `place=strip` on an upload, `strip` on an import from Google or iCloud). A picture that still tells a place afterwards is refused, never stored with it. Time, camera and exposure stay. Place names in IPTC or a camera maker's own notes are not looked at.
- **Rights and bounds:** every import stores into a folder the caller may add files to, as the caller; each file is bounded by `AttachmentBytes`; answers of the services are read with bounds; the server reaches Google's and Apple's hosts only, checked on every address an answer names. At most `PhotoImports` picker sessions and iCloud albums are held at once.
- **Audit:** connecting Google for an import (`photos.google.connect`); imports are recorded as the files they add.

### Not offered to agents

The new routes are not MCP tools: imports act with a person's sign-in at another service, thumbnails, tiles and ZIP archives are for browsers, and favourites are a person's own.

### Consequences

- **Positive:** photos are found by time, place, camera and favourites; the album loads thumbnails; photos from Google Photos, Apple Photos and any camera come into the album with the access of their folder; no credential of another service outlives an import; nothing new is written to disk but the photo data and the favourites.
- **Positive:** no dependency and no runtime file is added; the binary stays one.
- **Negative / trade-offs accepted:** unless removed on import, the place a photo was taken is told to every reader of its item, as the file itself tells it to whoever downloads it; removing it holds the whole picture in memory while it is rewritten; Google's downloads carry no place; shared iCloud albums are reduced copies read through an undocumented interface; WebP pictures and HEIC files get no thumbnail; pictures stored before this change have no recorded data until they are added again.
- **Negative / trade-offs accepted:** thumbnails are made again after every restart; a server with many readers of large libraries spends processor time on them.
- **Follow-ups / risks to watch:** recording the data of older pictures in one run; following a shared iCloud album on a schedule; videos in the timeline; similar or duplicate pictures beyond the same content; faces and objects, which would need recognition the binary does not hold.

## Pros and cons of the options

### Option 1 — photo data with the file, views in the album, imports on request (chosen)

Strongest form: everything a photo library is used for daily — time, place, favourites, many at once, the photos of other services — built on the files the wiki already keeps safe, with nothing new to protect but a person's favourites.

- Good: one model of access; no credential kept; no dependency; works offline but for imports and tiles.
- Bad: no faces or objects; reduced copies from iCloud; a codec version.

### Option 2 — a free photo library beside the wiki

Strongest form: the free photo libraries already offer faces, objects, videos, mobile apps with automatic upload and years of work on large libraries; linking them would bring all of it at once.

- Good: far more features than this record can build.
- Bad: a second server with its own database, accounts and access, so a photo's access is no longer its item's (D9, D10); not part of the single binary; the requester asked for photos managed in the wiki.

### Option 3 — read photo data on every listing

Strongest form: no change to the persisted format, and every picture, old or new, has its data.

- Good: no codec version; older pictures included.
- Bad: every listing of the timeline would decrypt and read every picture; a library of thousands becomes slow, and the data the album sorts by would be a shadow of the file (D4).

### Option 4 — thumbnails stored at upload

Strongest form: made once, served at once after a restart, no processor time for readers.

- Good: fast after a restart.
- Bad: a second file per picture to encrypt, back up, erase and keep in step with versions and protections (ADR-0160); older pictures would need a run of their own.

### Option 5 — favourites the same for every reader

Strongest form: a team marks its best pictures once, and everyone sees them, as a label does.

- Good: no new record in the accounts journal.
- Bad: one person's favourite is not another's; a mark everyone sees needs edit rights to set, and labels already serve a team's choice (ADR-0090).

### Option 6 — follow whole libraries

Strongest form: photos taken anywhere appear in the wiki without anyone doing anything.

- Good: no step to forget.
- Bad: Google no longer allows it; iCloud would need the Apple ID's password and second factor on the server, through an undocumented interface, a credential that reads every photo of a person.

## Links

- relates to ADR-0054 (the album), ADR-0055 (copies carry their source), ADR-0090 (folders and labels), ADR-0103 (versions stack), ADR-0173 (export), ADR-0175 (collections, which narrow the timeline, the months and the map as they narrow the album), ADR-0176 (slides with speaker's notes, which the album presents from any of its views)
- amended by [ADR-0178](0178-the-place-of-a-photo-for-its-uploader-alone.md) (the place kept to the uploader), [ADR-0179](0179-the-albums-map-has-a-background-by-default.md) (a map background by default) and [ADR-0181](0181-a-photos-place-set-by-hand-with-suggestions.md) (a place set by hand)
- relates to ADR-0026 (erasure), ADR-0045 (attachments), ADR-0066 (the Google client of single sign-on), ADR-0127 and ADR-0174 (public addresses only, other services read per person)
