# ADR-0192: Videos as slides, and from Google Photos and shared iCloud albums

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: two open questions of ADR-0189 are to be built, videos as slides of a presentation and videos brought in from Google Photos and shared iCloud albums. The requester chose the same day that a video slide shows its poster until the space bar or a click starts it, controlled from both windows, with sound in the presentation window only.

  The rest are defaults proposed with the implementation, confirmed by the requester; accepted 2026-11-10.
- **Open question:** whether the presenter's window should follow the presentation window's playing time exactly rather than play alongside it; whether WebM's metadata should be read
- **Question checked:** 2026-11

## Context and problem statement

ADR-0189 took videos into the album and handles them as pictures, with two exceptions it left open: a presentation (ADR-0176) skipped videos, because a slide is a picture to zoom into and point at; and the imports from Google Photos and shared iCloud albums (ADR-0177) skipped videos, because every imported file was bounded by `AttachmentBytes`.

How does a video become a slide, and how does it come in from the two services, without a new route, budget or dependency?

## Decision drivers

- **Same handling** (ADR-0189): whatever the album does with a picture it does with a video, unless the picture's nature is what the action is about.
- **Nothing new on the server for what is momentary** (ADR-0176): whether a video plays is part of the presentation's state between two windows, not content.
- **Bounded resources** (ADR-0015): an imported video is streamed into storage within `VideoBytes` and `VideoTransferSeconds`, as an uploaded one.
- **One binary, no new dependency** (ADR-0009): the server still decodes no video.

## Considered options

1. **A video slide plays in both windows from the presentation's state; the imports download videos within the video budget and keep the service's picture of a video as its poster.**
2. **Video slides as their poster only**, and the imports leaving videos out as before.
3. **The presentation window alone plays the video**, the presenter's window showing its poster.

## Decision outcome

Chosen option: **1** (defaults proposed with the implementation, confirmed by the requester).

### Videos as slides

- `slidesOf` keeps videos. A video slide is ordered by name as every slide.
- The presentation's state carries `playing`. The space bar and K on a video slide start and pause it, in either window; a click on the presentation window does the same. The arrow keys, page up and down and the strip move on as on every slide. Moving to another slide stops the video.
- The presentation window plays the video with sound. A browser that refuses sound before a click in that window plays it without and says *Click to turn the sound on*; the click turns it on.
- The presenter's window plays it alongside, without sound. When either reaches the end, the state says it no longer plays.
- A video slide is not zoomed, and the laser pointer is put away on it: both work in the picture's coordinates, which a playing video does not have.
- The next slide and the strip show the video's poster, or a symbol for a video without one.
- No route, event or budget is new.

### Videos from Google Photos and iCloud

- **Google Photos:** a picked video is downloaded at its base address with `=dv`, the picture Google makes of it with `=w1600-h1600`. The server asks the picked item's kind first (`Google.Picked`), without a request to Google, to choose the limit.
- **Shared iCloud albums:** of a video, the largest derivative that is not the poster frame is downloaded; the poster frame (`PosterFrame`) becomes its poster. A video Apple offers only a poster frame of is listed and refused with a reason.
- **On the server:** an imported video may have `VideoBytes` and takes `VideoTransferSeconds`. What comes must be MP4, WebM or QuickTime by its own bytes (`photo.VideoType`), or it is discarded. It is read for when and where it was taken (ADR-0189); `place=strip` stores it again without its place. Its poster is kept as `PUT …/poster` keeps one: JPEG, PNG or WebP within `AttachmentBytes`, sealed beside the video.
- The answer says whether the video has a poster. Without one, the browser takes one as after an upload (ADR-0189).
- `photosource.ErrVideo` is gone; nothing refuses a video for being one.

### Consequences

- **Positive:** a talk can show a clip between its slides; phone videos kept in Google Photos or a shared iCloud album come into the album with a poster, whatever browser imports them.
- **Positive:** no new route, event, budget or dependency.
- **Negative / trade-offs accepted:** the two windows play the same video side by side; they may drift apart by a fraction of a second. The presenter's window is a monitor, not the source.
- **Negative / trade-offs accepted:** a video from iCloud is the derivative Apple keeps for shared albums, which may be smaller than the original. A video from Google is what `=dv` hands out.
- **Negative / trade-offs accepted:** importing a large video holds one request for as long as the service takes to send it, up to `VideoTransferSeconds`.
- **Follow-ups / risks to watch:**
  - What `=dv`, the size parameters and the iCloud derivative names return was taken from the services' documented behaviour and the stream format already read; it was not checked against the live services in this change. A service that answers otherwise makes the import fail with a reason rather than store something else.
  - Exact time sync between the windows.

## Pros and cons of the options

### Option 2: poster only, imports as before

Strongest form: nothing plays in a presentation that could go wrong in front of an audience, and the imports stay small and quick.

- Bad: it leaves both open questions open; a video slide that cannot play is a picture of a video.

### Option 3: only the presentation window plays

Strongest form: one source of playback, no drift, and the presenter's computer decodes one video, not two.

- Good: simpler and lighter.
- Bad: the presenter, facing the console, does not see where the video is. Playing it muted alongside costs little on a computer that plays it once anyway.

## Links

- amends [ADR-0189](0189-videos-in-the-album.md) (videos are no slides; the imports skip them) and [ADR-0176](0176-pictures-of-the-album-are-presented-as-slides-with-speakers-notes.md) (slides are pictures)
- relates to ADR-0015 (budgets), ADR-0177 (the imports of the album)
