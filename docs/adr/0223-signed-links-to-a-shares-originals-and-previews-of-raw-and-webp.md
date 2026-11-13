# ADR-0223: Signed links to an S3 share's originals, and previews of RAW and WebP

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 to settle the open questions of ADR-0197; the four choices below were answered the same day; accepted 2026-11-12)
- **Open question:** whether a HEIC should get a preview once a decoder of HEVC exists that keeps the binary pure Go; whether a reader's browser that decodes HEIC (Safari) should make the preview, as it makes a video's poster (ADR-0189), which needs the original in that browser
- **Question checked:** 2026-11

## Context and problem statement

ADR-0197 shows the pictures of a file share in an album folder as previews; the originals stay on the share and open through a link template, with the reader's own access to the share. It left four questions open: whether the link to an S3 share's original may be one the server signs; whether HEIC, RAW and WebP should get previews; whether a share should be read for what changed rather than listed; whether a native SMB client should replace the mount.

The requester chose on 2026-11-12:

1. **Signed links:** offered per S3 share, off unless an administrator turns it on.
2. **Previews:** for RAW and WebP; HEIC keeps its symbol.
3. **Reading changes:** not built.
4. **Native SMB client:** not built.

What was checked on 2026-11-12:

- An S3 store accepts a GET signed in its query (signature version 4, `X-Amz-Expires` up to seven days); the signing of `internal/s3sig` makes it with the keys the server already holds for the share.
- The RAW of cameras holds a JPEG the camera made: TIFF-based RAW (DNG, CR2, NEF, ARW and others) in an IFD or SubIFD, CR3 in a box of its own, RAF at an offset in its header, ORF and RW2 as TIFF variants. The standard library decodes it; the RAW data themselves, often lossless JPEG (SOF3), it does not.
- The standard library has no WebP decoder; `golang.org/x/image/webp` is one in pure Go, BSD-3-Clause, maintained by the Go project, like `golang.org/x/crypto` already in use.
- A HEIC's picture, and its thumbnail, are coded in HEVC; no decoder of HEVC in pure Go was found (as far as checked), and the decoders in use are C libraries.

## Decision drivers

- **The requester's choice of ADR-0197 stays the default:** previews only; a signed link is a decision per share, made by an administrator, never the default.
- **Access is the folder's (D9, D10):** whoever may read the folder may follow a signed link; the link is made when it is followed, for that reader, and holds for minutes.
- **One binary (ADR-0009):** no C library; a dependency only from the Go project.
- **Bounded work:** a RAW is read whole once to make its preview, within a budget.

## Considered options

1. **Signed links per S3 share, made when followed; previews from the JPEG inside a RAW and from WebP decoded in Go; changes not read; no SMB client.**
2. Signed links in every listing of the album.
3. Previews of RAW by decoding the RAW data.
4. Read a share for what changed: S3 event notifications, SMB change notifications.
5. A native SMB client.

## Decision outcome

Chosen option: **1**.

### Signed links

- **Per share:** an S3 share carries `signedLinks`, off by default; the administration's form offers it for S3 shares only, and the settings refuse it for a directory.
- **Made when followed:** an entry of such a share links to `GET /api/items/{ref}/attachments/{id}/original`, not to the store. The route checks that the caller reads the entry (D10), that its share signs links and has its keys, and answers `302` to a GET of the object signed in its query for `signedLinkTTL` (15 minutes), with `Cache-Control: no-store` and `Referrer-Policy: no-referrer`. Otherwise it answers `404`. The link template, when set, is not used for such a share.
- **What it means, stated:** every reader of the folder gets the originals of the share, as long as the link holds; whoever holds a signed link may use it until it expires, without signing in.
- **Not offered to agents:** the route hands out originals to a browser.

### Previews of RAW and WebP

- **RAW:** a run reads a RAW whole, within the new budget `SharePreviewBytes` (128 MiB), looks for the JPEGs it holds, and makes the preview from the largest the standard library decodes, turned upright by the orientation of a TIFF-based RAW's first IFD. A RAW beyond the budget, or one holding no JPEG it decodes, keeps its symbol.
- **WebP:** `internal/photo` decodes WebP with `golang.org/x/image/webp`, so a WebP of a share gets a preview, and every WebP of the album gets thumbnails scaled as a JPEG or PNG is (ADR-0177 answered them whole) and a fingerprint (ADR-0196).
- **HEIC** keeps its symbol (see *Open question*).

### Reading changes: not built

Shares are listed on their schedule, as ADR-0197 decided.

### A native SMB client: not built

Samba is reached through a directory the operator mounts, as ADR-0197 decided.

### Consequences

- **Positive:** readers of a folder may open an S3 share's originals where an administrator wants it, without accounts at the store; RAW and WebP show a picture.
- **Positive:** WebP pictures of the album load as thumbnails rather than whole.
- **Negative / trade-offs accepted:** with signed links on, the share's own permissions do not narrow who gets the originals, and a link passed on works for its minutes.
- **Negative / trade-offs accepted:** a RAW is held whole in memory while its preview is made, at most `ShareWorkers` at once, each within `SharePreviewBytes`; a RAW's embedded JPEG may be smaller than the picture.
- **Negative / trade-offs accepted:** a dependency of the Go project is added (`golang.org/x/image`).
- **Follow-ups / risks to watch:** HEIC; a share read for what changed, should the stores in use offer it reliably.

## Pros and cons of the options

### Option 2 — signed links in every listing

Strongest form: no route of the wiki between the reader and the store; a link copied from the album works like any address.

- Good: simpler; no redirect.
- Bad: every listing signs a link per entry, valid for its minutes whether followed or not, and puts it into every page and log that records the listing; the reader is not checked when the link is followed.

### Option 3 — decode the RAW data

Strongest form: the full picture, with the camera's colours developed as the wiki wants, independent of what the camera embedded.

- Good: the largest possible preview.
- Bad: every camera's RAW format and lossless JPEG to decode and develop; far more code than reading the JPEG the camera already made, for a preview of 1600 pixels.

### Option 4 — read a share for what changed

Strongest form: a change on the share reaches the album within seconds, and a run never lists millions of files to find one new.

- Good: fresh at once; little work per change.
- Bad: S3 event notifications need an address of the wiki the store can reach, or a queue, configured per provider; changes on a CIFS mount are not told reliably to the host that mounted it; the schedule is still needed for what was missed.

### Option 5 — a native SMB client

Strongest form: no mount on the host, no privileges for the container; the share set up entirely in the wiki.

- Good: one place to configure.
- Bad: a dependency outside the Go project for SMB 2/3 with NTLM and Kerberos, the credentials held by the wiki, and a protocol surface to keep secure, for what a mount already does.

## Links

- amends [ADR-0197](0197-album-folders-that-show-a-file-share.md) (its open questions settled) and [ADR-0177](0177-photo-management-in-the-album.md) (WebP thumbnails scaled)
- relates to ADR-0009 (one binary), ADR-0012 (agents), ADR-0189 (posters), ADR-0196 (fingerprints)
