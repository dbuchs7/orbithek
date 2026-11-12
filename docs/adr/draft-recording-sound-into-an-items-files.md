# ADR-DRAFT: Recording sound into an item's files

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: for now only recording sound, attached as a file of the page or task and played from it where possible; no transcription for the time being. The rest are defaults proposed with the implementation, to be confirmed by the requester.
- **Open question:** whether a recording should also be placed in the text as a block of its own, so that the daily view (ADR-0205), which shows no file list, plays it too; whether the longest recording should follow the installation's size of a file instead of a fixed 30 minutes; whether recordings should be transcribed later (ADR-draft-voice-recordings-become-text-in-the-browser)
- **Question checked:** 2026-11

## Context and problem statement

Any file can be attached to an item (ADR-0045), so a recording made elsewhere can be uploaded today, but recording one needs another application first.
Once attached, sound is sent as `application/octet-stream` with `Content-Disposition: attachment` and can only be downloaded: only pictures and the videos a text plays are sent inline (ADR-0045, ADR-0097).

The browser records from the microphone itself (`getUserMedia`, `MediaRecorder`), in a format of its own choosing: WebM with Opus in Chromium-based browsers, Ogg or WebM with Opus in Firefox, MP4 with AAC in Safari.
The standard detection, which names every stored file (`http.DetectContentType`), calls WebM `video/webm` whatever its tracks hold, and the album's own detection calls most MP4 files videos by their brands (ADR-0189).
A recording uploaded as it is would therefore be taken into the album as a video without a picture.

How does a person record sound into a page's or task's files, and play it from the item, without a new kind of content, without a second detection that guesses, and without weakening what files may do in the browser (ADR-0014)?

## Decision drivers

- **D1:** a recording is a file of its item, as every attachment is; no new object, no new block kind, no codec change.
- **Told by its bytes:** a file's type comes from what it holds, never from its name or what the browser claims (ADR-0045, ADR-0189).
- **Files run nothing:** what is sent inline keeps `nosniff` and the sandboxing policy; the UI's content security policy is unchanged.
- **Nothing kept in the browser:** a recording not attached is gone, as captures are not kept either (ADR-0209).
- **Small:** what already exists — the file list, uploads, serving in parts — carries it.

## Considered options

1. **A recorder in the file list; sound told by its tracks and sent inline; a player in the file list.**
2. **A block kind `audio` in the text**, as `video` (ADR-0097), with a recorder in the editor.
3. **Recording only**, the file downloaded as today.
4. **Telling a recording by its name or the type the browser gives**, instead of its tracks.

## Decision outcome

Chosen option: **1**, because it records and plays sound where files already are, on every item and with the item's rights, and changes neither the model nor the log.

- **Telling sound:** `photo.AudioType` reads a stored file's container: a WebM file all of whose tracks are sound (its Tracks element, read up to the first Cluster), an MP4 file all of whose tracks have the handler `soun` (`moov/trak/mdia/hdlr`), or an Ogg stream whose first packet is Opus or Vorbis.
  Such a file is `audio/webm`, `audio/mp4` or `audio/ogg`; MP3 and WAV keep the standard detection's `audio/mpeg` and `audio/wave`.
  The upload tells sound before videos, so a recording is no video: the album does not take it, and a `video` block refuses it.
  Files stored before keep the type they were stored with.
  A sound file has the size of every file other than a video (`AttachmentBytes`), also when it is sent as a video.
- **Serving:** files of `item.AudioTypes` (`audio/webm`, `audio/ogg`, `audio/mp4`, `audio/mpeg`, `audio/wave`) are sent inline with their type, in parts (HTTP Range), with `nosniff` and `default-src 'none'; sandbox`, as videos are (ADR-0097).
- **Playing:** the file list of an item (`attachmentsSection`) shows the browser's own player under every sound file, loading nothing before it is played (`preload="none"`); the list's kind filter has *Sound*.
- **Recording:** whoever may edit the item finds **Record sound** under **Upload files** where the browser can record: a secure context (HTTPS or `localhost`), `getUserMedia` and `MediaRecorder`.
  The recorder asks for Opus in WebM, then in Ogg, then MP4, at 64 kbit/s; the recording runs with its time shown and is ended by **Stop and attach** or **Discard**.
  It stops by itself after 30 minutes, about 14 MB at that rate, and is attached.
  It is named by when it started, in the person's language, without a colon: *Recording 2026-11-12 08-14.webm*.
  Attaching uploads it as any file (`POST /api/items/{ref}/attachments`), so the rights, the size limit, the governance check of the name and critical storage apply as to every upload.
  Leaving the view, or the view being drawn again, discards the recording and frees the microphone; nothing is kept in the browser.
  A microphone that is refused, missing or in use says so.
- **Elsewhere:** a recording is a file like any other: backed up, deleted and erased as files are (ADR-0045, ADR-0026).
  A copy of its item stores it again and tells it as sound again, so the copy is no video either (ADR-0055).
  Nothing is transcribed.
- **Code:** `photo.AudioType` (`internal/photo/audio.go`), `item.AudioTypes`, the upload and serving in `internal/server`; `assets/recorder.js` with its plain parts in `assets/recorderparts.js`, tested by `internal/web/jstest/recorder.test.mjs`.

### Consequences

- **Positive:** a spoken note is attached to a page or task in two clicks, on a phone too, and played from the item.
- **Positive:** recordings and other sound in WebM or MP4 no longer appear in the album as videos without a picture.
- **Negative / trade-offs accepted:**
  - the daily view shows no file list, so a recording of a daily note is made and played on the note's item page (*As an item*);
  - a recording in the file list is not placed in the text, so it is not seen where the text speaks of it;
  - recording needs HTTPS; an installation served over plain HTTP shows no button;
  - whether Safari's MP4 recording has the handler `soun` is assumed from the format, not tested on a device: should it not, it stays `video/mp4` as before;
  - an installation whose `AttachmentBytes` is below about 14 MB refuses its longest recordings when they are attached.
- **Follow-ups / risks to watch:** a block kind `audio` in the text (option 2) as the next step; transcription as proposed in ADR-draft-voice-recordings-become-text-in-the-browser.

**Implementation:** everything above is built, tested in Go and Node.js, and tried in Chromium with a simulated microphone. Not built: option 2.

## Pros and cons of the options

### Option 1: a recorder and a player in the file list

Strongest form: no new model, no codec change, every item has a file list already, and rights, limits and checks are the upload's.

- Good: small; works on pages, tasks and every other item alike (D6).
- Bad: the recording stands beside the text, not in it, and the daily view does not show it.

### Option 2: a block kind `audio`

Strongest form: a recording placed where the text speaks of it, played while reading, and shown in the daily view, whose text is edited in place; the pattern of `video` (ADR-0097) already exists, and the transcription draft builds on it.

- Good: the best place to play a spoken note; the daily view gets it for free.
- Bad: a new block kind is a codec version and touches export, copy, restore, protected blocks and MCP; the requester asked for files first. Not ruled out: the detection and serving decided here are what it needs.

### Option 3: recording only

Strongest form: the smallest change; downloading plays the file in the system's player.

- Good: no change to what is sent inline.
- Bad: the person leaves the page to listen, and on a phone a download often opens nothing; the request asked to play it from the item.

### Option 4: telling a recording by its name or the browser's type

Strongest form: the recorder knows it recorded sound, and the name it gives ends in `.webm` or `.m4a`; no parser on the server.

- Good: no reading of containers.
- Bad: what a client says is not what a file holds; a video renamed would be served as sound and kept out of the album, and every other way a file arrives (uploads, imports, copies) would be told differently. The bytes decide, as for videos (ADR-0189).

## Links

- amends [ADR-0045](0045-attachments-encrypted-files-beside-the-log.md) (sound is sent inline) and [ADR-0189](0189-videos-in-the-album.md) (a file of sound alone is no video)
- relates to [ADR-0097](0097-videos-in-the-text-the-instance-in-the-tab-done-beside-the-text.md) (serving in parts), [ADR-0205](0205-a-daily-note-per-person-and-day.md) (the daily view), ADR-0014 (files run nothing), ADR-draft-voice-recordings-become-text-in-the-browser (transcription, a later step)
