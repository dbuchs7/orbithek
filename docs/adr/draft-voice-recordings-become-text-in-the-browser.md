# ADR-DRAFT: Voice recordings become text in the browser, the recording kept only on request

- **Status:** Proposed
- **Implementation:** Not started
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: voice recordings as content of pages and tasks, the transcript as the content and the recording as an optional source ("variant C"), everything processed locally, and the function available in **Today**, the daily note (ADR-0205). The rest are defaults proposed in this record, to be confirmed by the requester.
- **Open question:** which Whisper model and size is the default, measured on phones and on Swiss German before it is built; whether browsers apply the worker's own content security policy, or the UI's policy must admit `'wasm-unsafe-eval'`; whether an installation-local speech service should later transcribe for devices too slow to do it themselves; whether paragraphs of a transcript should link to their time in the recording; whether recordings of several people (meetings) need a consent step; whether agents should be able to transcribe over MCP
- **Question checked:** 2026-11

## Context and problem statement

People think faster than they type, and on a phone typing is slow.
A spoken note — a thought on the way, a task after a call, the day's log in the evening — is today kept only as a file attachment: any file can be attached (ADR-0045), but a recording is sent as `application/octet-stream` and can only be downloaded.
The body has no block that plays audio, and nothing turns speech into text.
A recording that only exists as sound is invisible to everything the product does with content:

- full-text search scans text (ADR-0033);
- the governance guard restricts an item that brings in sensitive data, read from its text (ADR-0040);
- data protection holds (ADR-0058), exports (ADR-0071, ADR-0131) and agents over MCP (ADR-0012) read text.

A dictated password would pass the guard that a typed one triggers.

The binary is pure Go without C (ADR-0004, ADR-0009, ADR-0024): it cannot run a speech model as a library does, and a second executable is excluded.
Nothing is loaded from a CDN, and the requester asks that no audio and no text leave the installation for transcription.
Daily notes (ADR-0205) are edited in place in their live session (ADR-0010), are made on first open, restricted to their author in the same batch, and need a room the person chose; the capture view (ADR-0209) adds to today's note through `POST /api/daily/{date}/paragraphs`.

How does a spoken note become content of a page, a task or the daily note — searchable, checked, exportable, private where the item is private — within these limits, with the recording kept only where it is wanted?

## Decision drivers

- **D1:** a transcript is a body's ordinary blocks; a recording is an attachment of the item and shown by a block kind of its own. No `VoiceNote` object.
- **Local only:** audio and transcript reach no third party; the model runs on the person's device, served by this server.
- **One binary, nothing from a CDN** (ADR-0009): the runtime is embedded; what is too large to embed is supplied by the administrator, not fetched by the product.
- **The text is the content:** search, the governance guard, protections (ADR-0088, ADR-0160), exports and MCP work on the transcript without change (D6).
- **The recording is personal data:** kept only when the person asks and the installation allows it, destroyed with its attachment (ADR-0045) and its item (ADR-0026).
- **Today first:** dictating into today's note works from the daily view, the command palette (ADR-0207) and the installed app (ADR-0209), on a phone too, including when the note does not exist yet.
- **Live editing is respected** (ADR-0010): a transcript is inserted by the editor while a live session holds the body, never by overwriting it.
- **The closed renderer** (ADR-0014) and the UI's content security policy are not weakened beyond what running the model needs.

## Considered options

1. **Whisper in the browser:** a Whisper model run by a vendored WebAssembly runtime in a worker, the model file served by this server; the transcript inserted as paragraphs; the recording kept as an attachment with a block kind `audio` only on request.
2. **The browser's own speech recognition** (Web Speech API).
3. **An installation-local speech service** that the server calls with the recording (`OT_SPEECH_URL`).
4. **Transcription in the server's Go process.**
5. **An audio block only, without transcription** (variant A).
6. **Option 1 without ever keeping the recording.**

## Decision outcome

Chosen option: **1, Whisper in the browser**, because it is the only option that keeps audio and text on the person's device until they save, needs neither C in the binary nor a second service, and makes the transcript the content that every existing capability already reads.

### Recording and transcribing

- **Recording:** the browser's `MediaRecorder` records from the microphone (`getUserMedia`) in the format it offers: WebM or Ogg with Opus in Chromium-based browsers and Firefox, MP4 with AAC in Safari.
  A recording is at most `SpeechSeconds` long, a new budget (default 600 seconds; ADR-0015).
  The microphone needs a secure context (HTTPS or `localhost`); elsewhere the function is not offered and says why.
  The server sends `Permissions-Policy: microphone=(self)`.
- **Transcribing:** a dedicated worker (`assets/speechworker.js`) decodes the recording, resamples it to 16 kHz mono and runs Whisper through a WebAssembly build of whisper.cpp (MIT), vendored under `vendor/whisper/` like every other bundle (ADR-0032).
  Language: the UI's language, changeable in the dialog; Whisper's own detection on request.
  Nothing of the recording or the text leaves the browser while it is transcribed.
- **The worker's policy:** the worker script is served with its own content security policy, `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'`, which a dedicated worker obeys instead of the document's (Content Security Policy Level 3).
  The UI's policy stays as it is.
  *That every supported browser applies the worker's own policy is an assumption to be checked before the build;* where one does not, the UI's policy would need `'wasm-unsafe-eval'`, which needs its own amendment of ADR-0014 (open question).
- **The model:** too large to embed (a quantised Whisper *base* is in the order of 60 MB, *small* in the order of 180 MB — *figures from memory, to be measured*).
  The administrator places a ggml model file in the data directory's `speech/` folder or names its path in `OT_SPEECH_MODEL`; the product never downloads one.
  The server serves it to signed-in callers at `GET /api/speech/model`, with its SHA-256 as ETag and `Cache-Control: private, max-age=31536000, immutable`, so a browser loads it once per model.
  The model's name, size and hash are shown under the administration's new section *Speech*.
  Whisper's weights are released under the MIT licence (*from memory, to be checked against the model's own card before it is listed in `THIRD_PARTY_NOTICES.md`*).
- **Turned on:** the function exists only while administrators turn it on (`site.Speech`, kept in `site.wal`, ADR-0052) and a model is present.
  `GET /api/speech` tells the UI whether it is on, the model's size and hash, and whether recordings may be kept.

### What is saved

- **The transcript is the content.** The dialog shows the text, editable, before anything is saved.
  *Insert* adds it as paragraphs, one per segment Whisper ends with a pause, at the cursor of the body being edited.
  From then on it is text like typed text: searched, scanned by the governance guard in the same batch, protectable, exported, read over MCP.
- **The recording is kept only on request.** Administrators choose under *Speech* whether recordings may be kept at all (`text only`, the default, or `text and recording`).
  Where they may, the dialog offers *Keep the recording*, unticked by default.
  A kept recording is uploaded as an attachment of the item (ADR-0045), and a block of the new kind `audio` is placed before the transcript.
  A recording that is not kept never reaches the server.
- **The block kind `audio`** names an attachment of the item in `Ref`, with a plain caption (default: "Recording, <date> <time>, <duration>"), like `video` (ADR-0097) without width.
  It plays with the browser's own controls, in the editor and while reading.
  It is valid only for a file whose bytes read as audio a browser plays (`item.AudioTypes`: `audio/webm`, `audio/ogg`, `audio/mp4`, `audio/mpeg`, `audio/wav`), told by `photo.AudioType` from the container's header and tracks, as `photo.VideoType` tells videos (ADR-0189).
  `VideoType` is not changed; an audio-only WebM is offered as audio where the recorder made it.
  The next codec version.
- **Serving:** audio of these types is sent inline with its type, in parts (HTTP Range), with `nosniff` and the sandboxing policy, as videos are (ADR-0097).
- **Transcribing later:** on a device too slow to wait for, or when the person chooses *Transcribe later*, the recording is kept regardless of the default (the dialog says so) and the `audio` block offers *Transcribe* to every editor of the item on any device; the text is inserted after the block.
  Where administrators allow `text only`, *Transcribe later* is not offered: the person transcribes now or discards.
- **Removing the recording:** the block's menu offers *Delete recording*, which deletes the attachment and the block in one change; the file is destroyed as every deleted attachment is (ADR-0045).
  Backups taken before keep it until they are rotated (ADR-0073).

### In Today

- **In the note:** the daily view edits today's note in its live session in place (ADR-0205); the editor's *Dictate* action (see below) is the same component there, and inserts through the live session at the cursor, or at the end when the cursor is elsewhere.
- **Without opening the editor:** the daily view's header carries **Dictate**, the command palette offers *Dictate into today's note*, and Alt+Shift+M does the same anywhere.
  The installed app (ADR-0209) gets a manifest shortcut *Dictate* to `/#/daily?dictate=1`, so a phone starts it from the app's icon.
- **The day** is the browser's day when the recording *started*, so a note dictated over midnight goes to the day it was begun.
- **Saving there:**
  - while the note is open in a live session in this browser, the editor inserts the text (and the block);
  - otherwise the text alone goes through `POST /api/daily/{date}/paragraphs`, which makes the note when it is missing (ADR-0205);
  - with the recording kept, the browser opens the note (`POST /api/daily/{date}`), uploads the recording to it and appends the `audio` block and the paragraphs with `POST /api/items/{ref}/body/blocks` (ADR-0209).
  - A 409 because a live session elsewhere holds the body leaves the transcript in the dialog, to copy or to save again.
- **No room yet:** a 409 with `needsRoom` opens the room choice of ADR-0205; the recording and the transcript stay in the dialog's memory until saved or discarded.
  At critical storage (ADR-0163) nothing is made and the dialog says why.
- **Private by construction:** a daily note is restricted to its author (ADR-0205); its recording is an attachment of it and is read by exactly those who read the note (ADR-0022, ADR-0045).
- **Other days:** dictating into a past day's note is offered only where that note exists or after *Start this day's note*, so browsing makes nothing (ADR-0205).
- **The capture view** (ADR-0209) offers *Dictate* beside its text field; the transcript fills the field and goes to whichever target is chosen, today's note first. Recordings are not kept from the capture view.

### Elsewhere

- **Pages and tasks:** every body editor offers *Insert… → Dictation* and a microphone button in its bar; a new task's description is dictated in the task once it is created, as images and videos are added after creation (ADR-0097).
- **Word export:** the block exports as `[Audio: name]` with its caption, the transcript as text; Markdown export and `get_item_markdown` link the recording as an attachment; recordings are not copied into exported archives, as videos are not.
- **Copying an item** copies its recordings with it (ADR-0055).
- **Protecting a block** (ADR-0160) protects an `audio` block and its file as every block that shows a file.
- **The album** does not list recordings.
- **MCP** offers no recording and no transcription; agents read the transcript.

### Consequences

- **Positive:** a spoken note becomes searchable, checked and exportable text in the item it belongs to; a dictated password restricts its item as a typed one does.
- **Positive:** audio and text stay on the device until saved, and a recording reaches the server only when the person and the installation want it.
- **Positive:** Today becomes a place to speak the day's log into, on a phone through the installed app, private to its author.
- **Negative / trade-offs accepted:**
  - every device downloads the model once (tens to hundreds of MB), and transcription speed depends on the device; old phones may need *Transcribe later* on another device;
  - the quality of the transcript depends on the model; Swiss German is transcribed into written German with errors that are not yet measured;
  - an edited transcript no longer matches its recording: removing a password from the text leaves it in a kept recording, which the guard cannot read; the dialog says so when a kept recording's text is changed;
  - the administrator has to supply a model file; without one the function does not appear;
  - an older build cannot read a log that has an `audio` block (codec version).
- **Follow-ups / risks to watch:** the worker's content security policy in every supported browser; the model's licence; memory use of the worker on phones; whether `SpeechSeconds` suits the use; the legal reading of voice data below.

**Data protection.** A voice recording is personal data of the speaker.
It is biometric data under the revDSG and the GDPR only where it is processed to identify a person, which this record does not do.
*This reading is taken from memory, as the readings of ADR-0194 were; it is to be checked against the primary texts on fedlex and EUR-Lex before the record is accepted.*

## Pros and cons of the options

### Option 1: Whisper in the browser

Strongest form: the same pattern as faces (ADR-0194) — a model run where the data already is, served by the installation, nothing leaving the device — applied to speech; the text then needs no new path anywhere.

- Good: local by construction; no second service; the transcript is ordinary content.
- Bad: a large one-time download per device; speed and memory depend on the device; a WebAssembly policy question to settle.

### Option 2: the browser's own speech recognition

Strongest form: nothing to download, text appears while one speaks, and the browser maintains the model; on some systems recognition runs on the device.

- Good: no model to supply, live text, no WebAssembly.
- Bad: Chromium-based browsers send the audio to their vendor's servers, and which systems recognise on the device is not something the page can know or require; that breaks "local only". Firefox does not offer it.

### Option 3: an installation-local speech service

Strongest form: a Whisper server in the installation's own network runs a large model on a GPU, transcribes a phone's recording in seconds, gives the same quality on every device, and spares each device the download; it stays within the installation.

- Good: fast and uniform; works on weak devices.
- Bad: a second service to run, secure, update and back up beside the single binary (ADR-0009); every recording travels to the server even when only the text is wanted, so "text only" no longer means the sound never leaves the device.
- Not ruled out for later as a second transcriber behind the same dialog, for devices that cannot do it themselves (open question).

### Option 4: transcription in the server's Go process

Strongest form: one binary, no client load, any device; the server transcribes what it receives.

- Good: no download per device; no WebAssembly in the browser.
- Bad: no mature speech model runs in pure Go, and C is excluded (ADR-0004); a transcription takes minutes of a CPU per recording on the server every user shares; the recording reaches the server always.

### Option 5: an audio block only

Strongest form: the smallest change — one block kind and a codec version, no model, no WebAssembly — and a recording keeps what text loses: tone, emphasis, a name spelled out.

- Good: simple; nothing to measure.
- Bad: the content stays invisible to search, the governance guard, data protection holds, exports and agents; a dictated password restricts nothing.

### Option 6: never keep the recording

Strongest form: the strongest privacy — no voice data on the server at all — no block kind, no codec change, no question of a recording and its text drifting apart.

- Good: nothing to delete, nothing biometric to discuss.
- Bad: a transcription error cannot be checked against what was said, and a device too slow to transcribe has no way to hand the recording to another.
- Kept as the installation's default: `text only` is exactly this option, and administrators choose whether recordings may be kept at all.

## Links

- relates to [ADR-0045](0045-attachments-encrypted-files-beside-the-log.md) (attachments), [ADR-0097](0097-videos-in-the-text-the-instance-in-the-tab-done-beside-the-text.md) (a media block and serving in parts), [ADR-0189](0189-videos-in-the-album.md) (telling media by their bytes), [ADR-0194](0194-faces-and-person-albums.md) (a model run in the browser), [ADR-0205](0205-a-daily-note-per-person-and-day.md) (Today), [ADR-0207](0207-a-command-palette-an-item-beside-another-and-the-context-of-an-item.md) (the palette), [ADR-0209](0209-capturing-into-the-wiki-from-anywhere.md) (capture and appending)
- relates to ADR-0004 and ADR-0009 (no C, one binary), ADR-0010 (live sessions), ADR-0014 (closed renderer and policy), ADR-0015 (budgets), ADR-0026 (erasure), ADR-0033 (search), ADR-0040 (governance guard), ADR-0052 (site settings), ADR-0160 (protected blocks), ADR-0163 (critical storage)
