# ADR-0189: Videos in the album, handled as its pictures are

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: films are uploaded into the album and handled as photos are. The requester chose the same day:
  - MP4, WebM and also MOV are taken;
  - a budget of its own allows videos of up to 2 GiB, while every other file keeps its limit;
  - the browser takes a picture from the video as its poster when it uploads it.

  The rest are defaults proposed with the implementation, confirmed by the requester; accepted 2026-11-10.
- **Open question:** whether videos should become slides of a presentation; whether Google Photos and shared iCloud albums should bring their videos too; whether WebM's metadata should be read; whether a poster should be made for videos that did not come through the album's upload
- **Question checked:** 2026-11

## Context and problem statement

The album is a photo library (ADR-0177): pictures are uploaded into folders, sorted by when and where they were taken, marked, labelled, moved (ADR-0184, ADR-0188) and downloaded. Videos play in a text (ADR-0097), but the album shows images and drawings only. Its imports skip videos, and every file is bounded by `AttachmentBytes`, 25 MiB, which a phone video exceeds in seconds. The server also ends every request after a minute.

The binary is pure Go, without C libraries (ADR-0009, ADR-0024). It cannot decode a video to make a thumbnail.

How does the album take videos and handle them as pictures, within these limits?

## Decision drivers

- **One model (D1):** a video is a file of an item, as a picture is; access, encryption, backup and erasure stay those of files (ADR-0026, ADR-0045).
- **Same handling:** whatever the album does with a picture it does with a video, unless the picture's nature is what the action is about (exporting as PNG, zooming into a slide).
- **One binary, no new dependency:** no ffmpeg, no C, no codec in the server.
- **Bounded resources** (ADR-0015): a video is never held in memory; its upload and download are streamed, within budgets.
- **Privacy** (ADR-0177, ADR-0178, ADR-0181): a video's place is removed, kept private or set by hand as a photo's is.

## Considered options

1. **Videos as files of the album: the server reads their header in Go and keeps a poster the browser takes from them.**
2. **The same, with posters made on the server by an external program (ffmpeg).**
3. **Videos linked from another service (a photo library or a video host).**

## Decision outcome

Chosen option: **1** (defaults proposed with the implementation, confirmed by the requester).

### What is taken

- **Formats:** MP4, WebM and QuickTime (MOV), told by their own bytes (`photo.VideoType`): Go's detection reads QuickTime, and MP4 of some brands, as an octet stream. An audio file or a still picture in the same container (M4A, HEIF) is no video.
- **QuickTime in texts:** QuickTime joins `item.VideoTypes`, so a MOV also plays in a text where the browser plays it. Safari always does; other browsers often only H.264, not HEVC.
- **Size and time:** an upload with `video=1` may store up to the new budget `VideoBytes` (2 GiB). What it stores beyond `AttachmentBytes` must turn out to be a video, or it is discarded. Uploading and downloading a video may take `VideoTransferSeconds` (an hour) instead of the server's minute (`http.ResponseController`). The browser asks for a video's budget in the album and in a text alike.

### What a video tells

`photo.ReadVideo` reads a few small boxes of the movie header, seeking past the media data, so a video of gigabytes is read in kilobytes. It reads:
- when it was taken: Apple's `creationdate` with its offset, else the movie header in UTC;
- how long it plays;
- its size and turn;
- the camera: Apple's make and model;
- where it was taken: `©xyz` or Apple's location, ISO 6709.

It is recorded as photo data, sealed as a picture's (ADR-0177). The new field `Duration` raises the codec to version 50. WebM is recognised but not read.

### The poster

- When the album uploads a video, the browser plays it unseen, takes a picture a second in (a tenth of a shorter video) and sends it: `PUT /api/items/{ref}/attachments/{id}/poster`, JPEG, PNG or WebP, up to `AttachmentBytes`, for whoever may add files to the item.
- The server seals it with the item's key beside the video's own file (`<id>.poster`). It is backed up, kept while the video is, moved with it (ADR-0188) and removed with it. It is not an attachment of its own, so it shows in no list and holds no event.
- The thumbnail of a video is its poster's. A video without one, such as a video the browser cannot play (HEVC outside Safari) or one attached elsewhere, shows a symbol.

### How it is handled

| Action | Videos |
|---|---|
| Folders, collections, timeline, months, favourites, memories, map, search, labels, rename, move, delete, ZIP download | as pictures |
| Viewer | plays it, with its poster until it starts |
| Kind filter | `video` beside `image` and `drawing` |
| Inserting into a text, from the album or its clipboard | a video block (ADR-0097) |
| Export | none; the card offers *Download* instead (ADR-0173 exports pictures) |
| Presentation | not a slide (ADR-0176 zooms and points into pictures) |
| Imports from Google Photos and iCloud | still skipped |

### The place

- **Removed on import** (`place=strip`): the stored video is stored again with the texts that hold its place read as zeros (`photo.VideoWithoutPlace`). It keeps its size and plays as before; a video whose place cannot be told apart is refused.
- **Kept to the uploader, or set by hand** (ADR-0178, ADR-0181): everyone else gets the video as it is read, its place read as zeros, streamed and seekable, never held in memory.

### Consequences

- **Positive:** phone and camera videos live in the album beside the photos, found by when and where they were taken, with the access of their folder.
- **Positive:** no new dependency; the binary stays pure Go.
- **Negative / trade-offs accepted:**
  - Codec version 50: a data directory written by this build cannot be opened by an older one.
  - A poster exists only when the album's upload made it in a browser that plays the video. HEVC videos outside Safari have none, and do not play there either.
  - Up to 2 GiB per video fills disks quickly; the storage watch (ADR-0150, ADR-0163) applies, and `VideoBytes` can be lowered.
  - A restored single space (ADR-0124) comes back without posters.
- **Follow-ups / risks to watch:** videos as slides; videos from Google Photos and iCloud; WebM metadata; posters for videos that came another way, made when a browser plays them.

## Pros and cons of the options

### Option 2: posters made on the server

Strongest form: every video gets a poster, whoever uploaded it and in whatever browser, and the server could read every format's metadata and even transcode HEVC to H.264 so every browser plays it.

- Bad: a second executable or a C library, against ADR-0009 and ADR-0024. Transcoding a 2 GiB video costs minutes of CPU per upload, a resource no budget bounds today.

### Option 3: videos linked from elsewhere

Strongest form: a video host streams in every quality to every device, and a photo library handles videos already.

- Bad: the video would leave the access, encryption and erasure of its folder (D1, ADR-0026), and the album would show what another service decides, as ADR-0177 found for photo libraries.

## Links

- amends [ADR-0177](0177-photo-management-in-the-album.md) (the album as a photo library) and [ADR-0097](0097-videos-in-the-text-the-instance-in-the-tab-done-beside-the-text.md) (videos in the text: QuickTime, a larger budget)
- relates to ADR-0015 (budgets), ADR-0026 (encryption), ADR-0045 (attachments), ADR-0173 (export), ADR-0176 (presentation), ADR-0178 and ADR-0181 (places), ADR-0188 (moving files)
