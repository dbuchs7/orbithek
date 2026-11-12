# ADR-DRAFT: Sound in the text

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: implement the audio block in the text, the next step ADR-0219 proposed. The rest are defaults proposed with the implementation, to be confirmed by the requester.
- **Open question:** whether a sound should have a width or a place beside the text, as pictures and videos have; whether removing the last block that plays a recording should offer to delete its file; whether the daily view should offer recording without opening the editor, as ADR-0220 proposes for dictation
- **Question checked:** 2026-11

## Context and problem statement

ADR-0219 records sound into an item's files and plays it from the file list.
The recording stands beside the text, not where the text speaks of it, and the daily view (ADR-0205), whose text is edited in place, shows no file list at all: a daily note's recordings are made and played on its item page.
ADR-0219 left open whether a recording should be placed in the text as a block of its own, and named it the next step (its option 2).

The body has blocks that show a file of the item — `image` (ADR-0045), `video` (ADR-0097), `bpmn` (ADR-0129) — but none that plays sound; a `video` block refuses a recording, which ADR-0219 tells as sound.

How does a sound play in a text, and how is a recording made into one, on pages, tasks and in the daily view, without a second kind of content?

## Decision drivers

- **D1:** a sound in a text is a block of the body that names a file of the item, as an image does; no new object.
- **The model stays explicit:** a block says what it shows; a sound is not a video that happens to have no pictures (ADR-0097's reason for its own kind).
- **Told by its bytes:** a block plays only a file the server told as sound (`item.AudioTypes`, ADR-0219).
- **Where text is edited:** the daily view edits its note in the item's editor, so whatever the editor offers works there.
- **Everything that reads a body knows it:** copies, exports, restores, protected blocks, agents.

## Considered options

1. **A block kind `audio`** with a reference and a plain caption, offered by the editor's *Insert…* as a sound file and as a recording.
2. **The `video` block for sound too**, its player showing no picture.
3. **A link to the file in a paragraph**, played from the file list.

## Decision outcome

Chosen option: **1, a block kind `audio`**, because it plays sound where the text speaks of it, in the daily view as on every page, and keeps the body's model saying what it holds.

- **The block:** `audio` names an attachment of the item in `Ref` and holds a plain caption, like `video` without width and alignment.
  It is valid only for a file of `item.AudioTypes` (`audio/webm`, `audio/ogg`, `audio/mp4`, `audio/mpeg`, `audio/wave`), as the server told it (ADR-0219).
  As for images and videos, a new item gets its sounds after it is created.
  Codec version 61.
- **In the editor:** *Insert…* offers **Sound file**, which uploads a file and places it after the cursor, and, where the browser can record (ADR-0219), **Record sound**: a dialog records at once with the recorder of ADR-0219, **Stop and attach** uploads the recording and places it, **Cancel** or leaving discards it.
  The block plays with the browser's own player, which keeps its clicks; while editing, its caption is edited in place and **Remove** takes the block out of the text.
  The file stays a file of the item until it is deleted from the file list.
- **In the daily view:** today's note is edited in the same editor in its live session (ADR-0010, ADR-0205), so a sound is recorded into it and played there; it is the note's file and as private as the note.
- **Elsewhere:**
  - a copy of an item points its sounds at their copies (ADR-0055);
  - the page printed as PDF plays the sound (`<audio>`, `media-src 'self'`); a Word file names it, `[Audio: name]`, with its caption; Markdown links it by its name, `♪ name`, and an archive does not copy it, as for videos (ADR-0071, ADR-0131);
  - a restore brings the block back where its file came back, and leaves it out otherwise (ADR-0124);
  - a protected block (ADR-0160) protects a sound and its file as every block that shows a file;
  - agents write it over MCP as `audio` with the attachment's ID.
- **Code:** `item.Audio` in `internal/item` (model, validation, protected blocks, codec 61, golden files `events.current.audio*`), the server's body, copy and export, `internal/export`, `internal/restore`, `internal/mcp`; `audioView` and the dialog in `assets/editor.js`, the recorder's ID prefix and autostart in `assets/recorder.js`.

### Consequences

- **Positive:** a recording stands in the text where it belongs, in a page, a task or today's note, and plays while reading.
- **Positive:** the daily view gets sound without a view of its own.
- **Negative / trade-offs accepted:**
  - an older build cannot read a log that has an `audio` block (codec version 61);
  - removing a block leaves its file in the item, as removing a picture does; files are deleted from the file list;
  - a sound has no width and no place beside the text.
- **Follow-ups / risks to watch:** the open questions above; transcription as ADR-0220 proposes would place its text after such a block.

**Implementation:** everything above is built and tested in Go, Node.js and Chromium, the daily view included. Not built: what the open questions ask.

## Pros and cons of the options

### Option 1: a block kind `audio`

Strongest form: the pattern of `image` and `video`, each a kind that says what it shows and is checked against the file's type; the editor that places them is the one the daily view uses.

- Good: explicit model; works wherever text is edited.
- Bad: a codec version, and every reader of bodies learns one more kind.

### Option 2: the `video` block for sound

Strongest form: no new kind and no codec change; a browser's video element plays sound alone.

- Bad: the model would say "video" for a sound, and every reader — export, the album, a list of figures — would have to look at the file to know what it shows, the reason ADR-0097 gave a video its own kind. ADR-0219 already keeps recordings from being taken for videos.

### Option 3: a link in a paragraph

Strongest form: nothing new at all; a link to the file opens it.

- Bad: the sound does not play in the text, the daily view has no file list to play it from, and a link to a file is not checked against its type or kept by a copy.

## Links

- answers the open question of [ADR-0219](0219-recording-sound-into-an-items-files.md) on a block in the text (its option 2), and amends its consequence that the daily view cannot play a recording
- relates to [ADR-0097](0097-videos-in-the-text-the-instance-in-the-tab-done-beside-the-text.md) (a media block), [ADR-0205](0205-a-daily-note-per-person-and-day.md) (the daily view), [ADR-0160](0160-protected-blocks-and-pictures-of-a-page.md) (protected blocks), [ADR-0220](0220-voice-recordings-become-text-in-the-browser.md) (transcription, proposed)
