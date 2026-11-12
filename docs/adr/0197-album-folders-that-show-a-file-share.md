# ADR-0197: Album folders that show a file share, as previews, with the share as master

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-11
- **Deciders:** Project initiators (request of 2026-11-11 to name file shares — S3 and Samba — for album folders, so that photos are shown only as previews while they stay stored on the share; the four choices below were answered the same day)
- **Open question:** whether the link to open an original may, for S3, be a presigned address the server signs, which would hand out the original to every reader of the folder; whether pictures the server cannot scale (HEIC, RAW, WebP) should get a preview by other means; whether a share should be read for what changed (S3 event notifications, SMB change notifications) rather than listed on a schedule; whether a native SMB client should replace the mount
- **Question checked:** 2026-11

## Context and problem statement

The album holds pictures as files of items (ADR-0045, ADR-0054, ADR-0090): every picture is uploaded, encrypted under its item's key, backed up and erased with it. Photo libraries of households and teams often live elsewhere already — on a NAS shared over SMB (Samba) or in an S3-compatible bucket — and are kept there by other tools: cameras, phones, scanners, backup jobs. Bringing them into the album today means copying them in (ADR-0177), which doubles the storage, and the copies drift from the share.

How does an album folder show the photos of a share — found by time, place, labels and faces like every other picture — while the share keeps the originals and stays where they are managed, without a second model of access and without breaking the single binary?

The requester chose on 2026-11-11:

1. **Direction:** the wiki only reads. The share is the master; nothing in the wiki writes, renames, moves or deletes a file on the share.
2. **What is shown:** previews only, with a link to open the original where it lies.
3. **Access:** the wiki's rights decide. Who may read the folder sees its previews, whatever the share's own permissions say; the share's permissions are not consulted.
4. **Kinds:** both S3-compatible storage and Samba.

What is already in the code base, as checked on 2026-11-11:

- `internal/backup/dest` signs S3 requests (signature version 4, path-style), lists a bucket with continuation tokens (ListObjectsV2) and reads an object, on the standard library. It does not yet read an object's size, ETag or time from the listing, nor a range of an object.
- A directory of the server, such as a mounted file server, is a backup destination (ADR-0073), with the rule that a missing mount fails instead of writing to the server's own disk.
- Connectors run on a schedule with a dry run, a report and an audit, credentials from the environment and public addresses only unless the operator allows more (ADR-0077, ADR-0127).
- A video's poster is a sealed file beside the video's own file, backed up and removed with it, without an attachment of its own (ADR-0189).
- Photo data are read by own code (`internal/photo`) and recorded with the file (ADR-0177).

## Decision drivers

- **One model (D1, D9, D10):** a picture of a share is a file of its folder — an item — and is read with the folder's access. Nothing about it, not its path, reaches someone who may not read the folder.
- **Queryable data is recorded (D4):** the time, place and size by which the album sorts and filters are recorded with the entry, not read from the share on every listing.
- **The share is the master:** the wiki never changes it; a picture that leaves the share leaves the album.
- **Work done in the wiki survives the share's housekeeping:** labels, favourites, named faces, places set by hand and notes are not lost when a file is renamed or moved on the share, as far as the file can be recognised.
- **A failing share empties nothing:** a mount that is gone, a bucket that answers an error or an empty listing must not read as "every picture was deleted".
- **One binary, no new dependency (ADR-0009):** S3 reuses the signing already written; Samba comes through a directory the operator mounts.
- **Untrusted input:** files of a share are read with the same bounds as uploads; administrators bind only directories the operator allows and only public addresses unless the operator allows more.
- **Honest guarantees:** what the wiki can no longer promise for these pictures — erasure of the original, its backup, its encryption — is stated where the share is bound.

## Considered options

1. **Linked entries:** a folder bound to a share holds one record per file of the share, with recorded photo data and a sealed preview beside it; the original stays on the share and is opened by a link; read on a schedule; the folder's access decides.
2. **Import by copy on a schedule:** the server copies new files of the share into the folder as ordinary attachments.
3. **Linked entries without a stored preview:** thumbnails made on demand from the original on the share, kept in memory as today (ADR-0177).
4. **The share's permissions decide:** each person's own credential reads the share, so a person sees only what the share lets them open.
5. **Two-way:** renaming, moving and deleting in the album act on the share.
6. **A native SMB client** in the binary instead of a mounted directory.

## Decision outcome

Chosen option: **1**.

### Shares

- **Definition:** administrators define shares under *Administration → Album → Shares*, kept in `site.wal` as connectors are: an ID (lower-case letters, digits, dots and hyphens, at most 40, fixed once made), a name, a kind, an address, the folder it fills (by the folder's ID, D2), a link template, whether it runs on its schedule, and its interval (one hour to seven days, six hours by default). A site keeps up to 20 of them, like connectors.
- **One folder, one share:** a bound folder shows its share and nothing else. Uploading into it, and moving or copying pictures into it, is refused with a message that names the share. Its subfolders follow the share's directories; they are folders of the album like any other, made, renamed and removed by the runs, never by hand. A folder holding pictures of its own cannot be bound.
  As built, the folders of directories are items of the album's folder type with the origin `share`, the share's ID and the directory's path (ADR-0023), upserted once a run, so a title changed by hand is set back; an entry whose file moves to another directory moves with its preview to that folder (`MoveLinkedFile`), and the folder of a directory gone is removed to the trash once it holds nothing, restored when the directory comes back.
  Folders are not made, moved or trashed by hand inside a bound folder, and the bound folder is not trashed while it is bound.
- **Kind `dir` (Samba and every mounted file system):** the address is an absolute path of the server. The operator lists the directories administrators may bind in `OT_SHARE_ROOTS` (separated by the system's list separator); a path outside them, inside the data directory, or containing the data directory, is refused. The share is mounted by the operator (CIFS on the host, a volume of the container); its credentials, Kerberos included, stay with the operating system. Files are read through `os.Root`, so no symbolic link or `..` leads out of the bound directory.
- **Kind `s3`:** the address is `s3://bucket/prefix?endpoint=https://…&region=…`, as for backups. The credentials come from `OT_SHARE_<ID>_ACCESS_KEY` and `OT_SHARE_<ID>_SECRET_KEY`, the ID in capitals with every other character as `_`; the page shows whether they are set and their names, never their values. The signing of `internal/backup/dest` moves into a package both use; the listing learns each object's size, ETag and time, and reading learns ranges. Every request, redirects included, goes to public addresses only (`linkcheck.Public`) unless the operator sets `OT_SHARE_ALLOW_PRIVATE=on`, which a bucket on the own network needs.
- **What is taken:** files whose leading bytes show a picture or video the album takes (ADR-0177, ADR-0189), told by content, never by name. Names starting with a dot and the directories `@eaDir` and `#recycle` (housekeeping of common NAS systems) are skipped. A share holds at most `ShareFiles` entries; a run that finds more takes the first in path order and reports the rest.
  As built, the album takes JPEG, PNG, GIF and WebP pictures, MP4, WebM and QuickTime videos, and HEIC, AVIF and the RAW of cameras (TIFF-based such as DNG, CR2, NEF and ARW, and CR3, RAF, ORF and RW2), told by their leading bytes; other files are left out and counted as skipped.

### Entries

- **A record per file:** `AttachmentLinked` records, on the folder that holds the file's directory, an attachment with no content of its own and a `Share`: the share's ID, the path below its address and a tag (the ETag of S3, the modification time of a directory), besides the size, the SHA-256 of the content, the media type and the photo data (ADR-0177).
  The path and the name are content, sealed with the item's key and erased with it (ADR-0026).
  `AttachmentRelinked` records new content or a new path, `AttachmentWithdrawn` (the attachment's ID only, unsealed) a file gone from the share, `AttachmentRestored` its return.
  Only a share's run records them, as the trusted store; the kinds raise the codec to version 60.
- **The preview:** the server reads the original once, scales it to 1600 pixels on its longer edge, turned upright, as JPEG (PNG with transparency), and seals it with the item's key beside the record as `<id>.preview`, as a poster is kept (ADR-0189). It is backed up, moved with the record between the share's folders, and removed with it. The album's 400-pixel thumbnails are made from it on demand, as today. Reading the original for the hash and the preview are one pass.
- **What the album hands out is the preview:** the viewer, the slides (ADR-0176), the export (ADR-0173), the ZIP download (ADR-0177), the clipboard (ADR-0108) and *Insert → From the album* (ADR-0054) all use the preview. Inserting copies the preview into the edited item as an ordinary attachment, with the entry as its source (ADR-0055). No route serves the original.
  Storage gives out an entry's preview wherever an attachment's content is read, since only such an entry has one.
- **Copies and exports** (the requester's choice of 2026-11-12): copying an entry into another item, alone or with its whole item (ADR-0067), and exporting it, take its preview as an ordinary file, with the entry as its source and no link to the share.
  The item model accepts such a copy only with the preview as its content, which the server marks (`FromPreview`).
  A restore of single items from a backup leaves the entries out; the share's next run links them again.
- **Pictures the server cannot scale** (HEIC, RAW, WebP, pictures beyond `ThumbnailPixels`) and videos are listed with a symbol, their name, the link and what `internal/photo` can read of them, without a preview. A video's photo data are read through ranges, as an upload's are (ADR-0189). The run's report counts them by format.
  As built, a picture the server cannot scale (HEIC, RAW, WebP, one beyond `ThumbnailPixels`) is linked without a preview and shows a symbol; it and a video are streamed for their hash within `VideoBytes`, as a RAW often exceeds `AttachmentBytes`, and a video's photo data are read through ranges.
  Such an entry offers only the link to its original, in the album and in the viewer.

### The link to open

- Each share has a link template with `{path}`, the entry's path with each segment escaped, for example `smb://nas/photos/{path}` or `https://nas.example/photo/?p={path}`. Only `https:` and `smb:` are taken; `file:` is not, because browsers refuse to open it from a page served over HTTPS (assumption checked against current browsers when implemented). Without a template there is no link.
- The link is given to whoever may read the folder, as everything else about the entry (D10). Whether it opens is the share's decision: the wiki grants nothing on the share, and a reader the share does not know sees the share's refusal.

### Access

- **The wiki decides:** who may read the folder sees its previews, paths and photo data; who may edit it sets labels, places, faces and notes on its entries. The share's own permissions are not read, as the requester chose.
- **Binding is a disclosure:** the server reads the share with one credential — the mount's or the S3 keys — so every file that credential can read below the address becomes visible to every reader of the folder. The page says so when a share is bound, and the folder's restrictions (ADR-0022) are the means to narrow it.

### What the wiki keeps and what follows the share

| On an entry | Allowed | Where it is kept |
|---|---|---|
| Labels, favourites, collections, notes, places set by hand, faces | yes | in the wiki, as for any picture; nothing is written to the file |
| Rename, move to a folder, delete, new version, upload into the folder | no | the share is the master; the album says so |
| Timeline, months, memories, map, search, people | yes | from the recorded photo data |

### Runs

- **Schedule:** one runner ticks every ten minutes and runs each share that is switched on and due, one run per share at a time; a share is run or dry-run on request. Runs are recorded as `sync:share:<ID>`; the last ten are kept in memory with their reports, as for connectors (ADR-0077).
- **Unchanged files are not read:** a file whose size and ETag or modification time match its record is skipped. A new or changed file is read whole once, within `AttachmentBytes` (`VideoBytes` for videos, of which only the header is read), by at most `ShareWorkers` at once.
- **Recognising a file again:** within a run, a path that is gone and a new path with the same size and SHA-256 are the same file: the record takes the new path (moving between the share's folders as ADR-0188 moves a file) and keeps everything set in the wiki. A file changed at the same path keeps its record, with new hash, photo data and preview; faces found on the earlier content are kept, marked as found before the change.
- **Gone from the share:** an entry whose file is gone is withdrawn (`AttachmentWithdrawn`): hidden from the album, its preview kept. If a file with the same SHA-256 appears in a later run, at any path, the entry comes back with what was set on it (`AttachmentRestored`). After 30 days a withdrawn entry is deleted with its preview, as a deleted file is (ADR-0045).
- **A failing share empties nothing:** a run that cannot read the address — the mount is missing, the directory is not the mount point the share was bound to (its device changed), the bucket answers an error — fails and withdraws nothing. A run that would withdraw more than half of a share's live entries, and more than 20, withdraws none and reports how many it would; an administrator repeats it with *Confirm withdrawals*.
- **Storage:** at critical storage (ADR-0163) runs record nothing new and make no previews; they still withdraw.

### Removing a share and erasure

- Removing a share, after a confirmation that names how many entries go, deletes its entries and previews and what was set on them in one batch, and unbinds the folder; the folder stays with its subfolders, now empty. A running share cannot be removed.
- Trashing the folder needs the share removed first. A purge or redaction of the folder (ADR-0026) destroys its key and with it every record and preview.
- **What the wiki cannot do, stated:** it does not erase, encrypt or back up the originals. A purge leaves them on the share; a backup holds the previews and records only; a lost share leaves the previews alone.

### Audit and agents

- Audited: `share.add`, `share.change`, `share.remove`, `share.confirm` (withdrawals confirmed). Runs are recorded as the records they write.
- The routes under `/api/shares` are for administrators and are not MCP tools (ADR-0012): binding discloses a share. Entries are read by agents as every picture of the album is.

### Consequences

- **Positive:** a NAS or bucket appears in the album with time, place, labels and faces, without a copy of its originals; the share stays managed by the tools that fill it.
- **Positive:** no new dependency; Samba needs no protocol code in the binary; S3 reuses the backup's signing.
- **Positive:** work done in the wiki survives renames and moves on the share, and short absences of a file.
- **Negative / trade-offs accepted:** every file the server's credential can read below the address is shown to every reader of the folder; the share's own permissions do not narrow it (the requester's choice).
- **Negative / trade-offs accepted:** the originals are outside the wiki's encryption, erasure and backup.
- **Negative / trade-offs accepted:** the first run reads every file of the share whole, over the network; a file both renamed and changed between two runs is not recognised and its wiki annotations are withdrawn with the old entry.
- **Negative / trade-offs accepted:** previews take space in the data directory (as an assumption, roughly 200 to 500 KB per picture at 1600 pixels); HEIC, RAW and WebP show a symbol only; videos have no poster.
- **Negative / trade-offs accepted:** whether a link opens depends on the reader's device and the share (an `smb:` link needs a handler of the operating system).
- **Follow-ups / risks to watch:** presigned S3 links; previews of HEIC; reading changes instead of listing; a native SMB client; WebDAV and SFTP as further kinds, whose clients exist for backups.

## Pros and cons of the options

### Option 1 — linked entries with stored previews (chosen)

Strongest form: every album feature works on pictures that stay where they are kept; only what the album needs — a record and a preview — enters the wiki.

- Good: one model of access; no copy of originals; no dependency; previews survive an absent share.
- Bad: a new kind of attachment, a codec version, and guarantees that no longer reach the original.

### Option 2 — import by copy on a schedule

Strongest form: nothing new in the model; every guarantee of attachments holds, and the share could vanish without loss. The iCloud import (ADR-0177) already works this way.

- Good: encryption, erasure and backup as for every file.
- Bad: doubles the storage of the whole library; the share is no longer the master — a deletion or edit on the share does not reach the copy; the requester asked for previews only.

### Option 3 — thumbnails on demand from the share

Strongest form: nothing but records enters the data directory; the album always shows the share's current content.

- Good: no stored previews; no space taken.
- Bad: after every restart each thumbnail reads a whole original over the network; an absent share empties the album's pictures; the hash and photo data still need one full read per file.

### Option 4 — the share's permissions decide

Strongest form: nothing becomes visible through the wiki that the share would not show its reader; the share's administrators keep control.

- Good: no disclosure by binding.
- Bad: rejected by the requester (choice 3); the server would hold a credential per person for the share; records and previews kept once would be visible per person, so the folder would no longer decide who reads its files (D9, D10).

### Option 5 — two-way

Strongest form: the album becomes the place to tidy the share; nothing has to be done twice.

- Good: one place to manage.
- Bad: rejected by the requester (choice 1); the server would need write access to the share, and a mistake in the album would delete originals the wiki does not back up.

### Option 6 — a native SMB client

Strongest form: no mount on the host, so a container needs no privileges or host configuration; the share is set entirely in the wiki.

- Good: one place to configure.
- Bad: a new dependency for SMB 2/3 with NTLM and Kerberos, credentials held by the wiki, and a protocol surface to keep secure; a mounted directory serves Samba and every other file system the host can mount.

## Links

- amends [ADR-0090](0090-album-folders-uploads-and-picture-labels.md) (a folder may be bound to a share) and [ADR-0177](0177-photo-management-in-the-album.md) (pictures whose original is on a share)
- relates to ADR-0026 (erasure), ADR-0045 (attachments), ADR-0054 (the album), ADR-0055 (copies carry their source), ADR-0073 (backup destinations and credentials), ADR-0077 and ADR-0127 (connectors, public addresses only), ADR-0163 (critical storage), ADR-0188 (a moved file keeps its identity), ADR-0189 (posters beside a file), ADR-0194 (faces)
