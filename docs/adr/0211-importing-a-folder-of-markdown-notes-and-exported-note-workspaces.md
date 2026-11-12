# ADR-0211: Importing a folder of Markdown notes and exported note workspaces

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (the maintainer asked that notes kept in other applications — a folder of Markdown files such as an Obsidian vault, a Notion workspace export, the Markdown export of other notes applications — can be brought into a room by its administrators, with a dry run, links between notes kept and re-runs that make nothing twice, 2026-11-12)
- **Open question:** Should a re-run remove the labels a source no longer names, and should it ever move a page back to where its note now lies, when people may have changed both since?
- **Question checked:** 2026-11

## Context and problem statement

People who come to the wiki often keep years of notes elsewhere: a folder of Markdown files edited with Obsidian or another editor, a workspace in Notion, notes in an application that exports Markdown. ADR-0023 and ADR-0128 import from Atlassian's products, ADR-0138 mirrors Markdown from Git repositories, and ADR-0133 converts Markdown pasted into one text. None of them takes a folder of notes as people have it: on their own computer, zipped, with links between the notes written as `[[wiki links]]`, pictures beside them, YAML front matter, tags inline, and — for a Notion export — IDs in every file name and databases as CSV files.

How does a room administrator bring such a collection into a room as pages and items, with its tree, its links and its metadata, without a second renderer, without anyone but the room's administrators deciding, and so that the next export can be imported again without making anything twice?

## Decision drivers

- Content is items (D1); a link between notes becomes a reference by ID (D2), which needs every page to exist first.
- One renderer (ADR-0014): Markdown is converted on the server into blocks, as ADR-0138 does with `internal/markdown`.
- What ADR-0023 decided for an import holds: a dry run that writes nothing and reports, re-runs matched by origin that change only what changed, nothing deleted (D8), whatever cannot be converted reported.
- The import acts with a person's rights, through their session (D10), and only room administrators write a whole tree into a room.
- An uploaded archive is untrusted input: its names must not reach outside it, and its size, its number of files and what it unpacks into are budgets (ADR-0015).
- Other products are named only where the subject is importing from them (AGENTS.md).

## Considered options

1. **An upload of a ZIP archive, converted on the server into pages through the room administrator's session**, with a dry run, origins per note and a name that a re-run repeats.
2. **A connector** (ADR-0077) that reads the notes from where they live.
3. **Conversion in the browser**, which creates the pages through the existing item API one by one.
4. **An import command** on the server's command line, as `import -from atlassian-cloud`.

## Decision outcome

Chosen option: **1**, because the notes are on the person's computer, not at an address a server can read; the server already converts Markdown (ADR-0138) and holds the validated, idempotent import machinery (ADR-0023); and a room administrator — not only an operator — can then do it.

### What is imported

- **A folder of Markdown files.** Every file ending in `.md` or `.markdown`. This covers an Obsidian vault and the Markdown export of any notes application that writes a folder of Markdown files with YAML front matter: such a source is supported as far as its export is such a folder. Folders and files starting with a dot (`.obsidian`, `.trash`), `__MACOSX` and `Thumbs.db` belong to the applications and are skipped.
- **A Notion workspace export** in the format *Markdown & CSV*, recognised by the 32 hexadecimal digits of the page's ID that end its files' and folders' names.
- An archive whose files all lie in one folder has that folder cut off; its name is the import's default name.

### Pages

- **Tree.** Every folder that holds notes, directly or below, becomes a page; every note a page below its folder's page. A note named like its folder, `index.md`, `_index.md` or `README.md` is the folder page's own text, which a list of its pages (block `children`) follows; a folder without one shows only the list. In a Notion export, the folder `Name <id>` holds the subpages of the page `Name <id>.md`, which is their parent.
- **Title:** the front matter's `title`, else a leading heading of level 1 (which then leaves the text), else the file name; in a Notion export without the ID.
- **Text:** `internal/markdown` converts the note (ADR-0138), after the wiki syntax is rewritten as CommonMark outside code: `[[Note]]`, `[[Note|alias]]` and `[[Note#Heading]]` become links, `![[picture.png]]` a picture, a block's anchor `^id` and comments `%%…%%` are dropped.
- **Identity:** a note is `md/<path>`, a folder `dir/<path>`, a Notion page `notion/<id>`, a database row without a page `notion/<database id>/row/<title>`. A Notion page keeps its identity when renamed or moved within the export; a note of a folder, which has no other identity, does not.

### Links and pictures

- Pass one writes every page, pass two the pages whose links name pages made after them. A link to another note becomes a **mention** of its page (D2): `[[Note]]` by path, file name or front matter `aliases`, nearest first (the path that ends as written, the same folder, the shortest path, as the notes applications resolve); a relative Markdown link by path; a link into the Notion workspace by the ID that ends it. A link to the page itself, to nothing in the archive or to another file stays text and is reported.
- **Pictures** of the archive, embedded with `![[…]]` or a relative picture link, are attached to the page (ADR-0045) and shown; only PNG, JPEG, GIF and WebP are kept, at most 50 per page, each within `AttachmentBytes`, a picture the page already has with the same content not again. An SVG and other files stay their names, reported. Pictures on the web stay links: the server fetches nothing for an import, unlike a repository's pictures (ADR-0138), since the archive is the source.
- **Unknown embeds** — an embedded note `![[Note]]` (a mention of its page), a PDF or a recording — are reported.

### Front matter, tags and databases

- `tags` in the front matter and inline `#tags` become **labels** (ADR-0044); a tag's levels `a/b` become `a.b`; a tag that is no label even then is reported.
- A key named like a field of the chosen **type** (exactly, else in any case) fills the field when its value fits the field's kind — text, number, yes or no, date, point in time with an offset, a value of an enum, a web address; a user field is never filled from a name.
- `status` or `state`, when the type's workflow has that state, sets the state directly, as an import does (ADR-0023, D3's exception).
- Everything else — keys without a field, values that do not fit — is kept in a small **table at the top of the page**, two columns *Property* and *Value* in the importer's language; a value that does not fit its field, and a column without a field, is reported once.
- A Notion **database** (its CSV; of the two the export writes, the one ending `_all`) becomes a page listing its rows, and each row an item of the chosen **type of rows**, its first column the title, its other columns as above, a column `Tags` as labels. A row is matched with its page in the export by title, and the lines `Column: value` the export writes below a row's title are dropped from its text; a row without a page is written from the CSV alone. A CSV file outside a Notion export is reported and left out.

### Running

- **Who.** A room administrator of the target room, or an installation administrator, through their session: `Session.Import` (in `internal/item`) upserts by origin as the principal, only into a room the principal administers, only with an origin system starting `import:` so no connector's, migration's or mirrored file's item can be taken over, a new item below an item the principal may edit, an item imported before only where it may still be edited, left where people moved it. Labels, attachments and texts go through the session's own commands; the governance check runs as on every save (ADR-0040).
- **Where.** *Import notes* in a room's views (`#/import/{room}`) and *Administration → Import*, which chooses the room. `POST /api/spaces/{space}/imports/notes` takes the archive as `application/octet-stream`, which a cross-site form cannot send (ADR-0027), with `name`, `type`, `rowType`, `dryRun` and `lang`; `GET /api/spaces/{space}/imports` lists the room's imports by name. Both are routes of the table, authenticated, the check of the room in the handler.
- **Dry run** first: the same plan, compared with what earlier runs made, nothing written; the report gives the counts of new, changed and unchanged pages, the pictures, the pages of an earlier run no longer in the archive, the first 50 pages and the problems, up to 200.
- **Re-runs.** The origin is `import:notes`, the instance `<room>/<name>`, the external ID the page's identity, the external key its path, and the note as read is kept as the origin's raw content (sealed, ADR-0026). A run under the same name in the same room updates what an earlier one made: the title, the fields it names and the state are written when they changed; the text is written only when the note changed, so a page whose note is unchanged keeps what people wrote in it, and a changed note replaces its page's text, which the history keeps (ADR-0043). Labels are only added. A page whose note is gone is kept and counted, never deleted (D8). A page imported as one type is not made another.
- **Safety and budgets.** The archive is read in memory up to `NoteImportBytes` (100 MB by default), at most `NoteImportFiles` files (5000), whose reads together hold at most `NoteUnpackedBytes` (512 MB) unpacked, each read bounded whatever the archive's directory claims; a note or table beyond 2 MB or 8 MB is left out and reported. A name that is absolute, holds `..`, a drive letter or NUL, a link or a name given twice is not read and is reported; nothing of the archive is ever written to disk but the attachments, sealed. Reading and writing may take up to 15 minutes, past the server's usual minute.
- **Audit.** A run, not a dry run, is recorded as `notes.import` with the room, the name, the format and the counts.
- **MCP.** Not offered: the body is an archive no tool argument carries, and writing a tree of pages is a room administrator's decision; agents create pages one by one (`internal/mcp/tools.go`).
- **Nothing new at critical storage** (ADR-0163): the store refuses the new pages, and the run reports that it stopped making them.

### Consequences

- **Positive:** a vault or a workspace moves into a room with its tree, its links as references by ID, its pictures, labels and fields, rehearsed by a dry run; the next export updates it without duplicates and without losing what people wrote here.
- **Positive:** one converter serves repositories (ADR-0138) and imports; the import machinery of ADR-0023 serves people as well as connectors.
- **Negative / trade-offs accepted:**
  - The archive is held in memory while it is read, bounded by its budget.
  - A link to a page that a run failed to write stays text until the note changes; a page written before the target existed is not revisited.
  - A note's text written here is replaced when its note changes in the source; only the history keeps it. Labels a source drops stay.
  - A note of a plain folder that is renamed or moved is a new page; the old one is kept and reported. Notion pages keep their identity by ID.
  - What the block model cannot hold is lost as for ADR-0138: nested lists flatten, a table's cells are plain text. A picture inside a line of text stays its name, linked nowhere.
  - Notion's other export formats (HTML, PDF), its relations between databases and its formulas, rollups and people are not imported beyond the text of their cells.
- **Follow-ups / risks to watch:**
  - the open question: labels and places a re-run does not take back;
  - choosing a page below which an import goes, instead of the top of the room;
  - relations between databases as typed relations (ADR-0001);
  - a mapping of a database's columns to fields of another name, offered after the dry run;
  - attachments other than pictures (PDF, recordings) as files of the page.

## Pros and cons of the options

### Option 2: a connector

Strongest form: connectors already have a schedule, a dry run, a report and an audited run (ADR-0077); a vault kept in a Git repository is already mirrored by ADR-0138, and Notion has an API, so a connector would keep a room in step without anyone uploading anything again.

- Good: no upload, and updates arrive on their own.
- Bad: a vault on someone's computer has no address; most notes are not in a repository. Notion's API needs an integration token per workspace, which only the operator sets (ADR-0077), and its pages and blocks map differently from its export.
- Bad: connectors are set up by installation administrators, not room administrators, and write as `sync:<id>`, not as the person who brought the notes.

### Option 3: conversion in the browser

Strongest form: the browser already converts pasted Markdown (ADR-0133); converting an archive there needs no new route, and every page is created through the API as the person, so permissions hold by construction.

- Good: no server-side archive handling.
- Bad: a vault of thousands of notes would be thousands of requests from one tab, interrupted by a closed laptop and leaving half an import behind; the two passes for links, the comparison with earlier runs and the dry run would be rebuilt in JavaScript.
- Bad: the server converts Markdown in Go for repositories (ADR-0138); a second converter of a whole file format in the browser would drift from it.
- Bad: the item API keeps no origin for a person's item, so a re-run could not find what it made before.

### Option 4: an import command

Strongest form: the command of ADR-0023 exists; an operator copies the archive to the server and runs it, with no upload route and no budgets for it.

- Bad: only operators can run it, the server must be stopped (ADR-0128), and room administrators — the people who own the notes — cannot do it themselves.

## Links

- builds on [ADR-0023](0023-import-and-sync-from-jira-and-confluence.md) (origin, upsert, dry run, report), [ADR-0138](0138-markdown-from-git-repositories.md) (`internal/markdown`, the title rule, pictures)
- relates to [ADR-0133](0133-markdown-is-pasted-into-the-editor-and-converted-in-the-browser.md) (pasted Markdown), [ADR-0014](0014-closed-renderer-for-user-content.md) (one renderer), [ADR-0015](0015-resource-budgets-and-paged-listings.md) (budgets), [ADR-0022](0022-permission-model.md) (sessions), [ADR-0026](0026-crypto-shredding-of-item-content.md) (the raw note sealed), [ADR-0044](0044-labels.md) (labels), [ADR-0045](0045-attachments-encrypted-files-beside-the-log.md) (pictures), [ADR-0163](0163-at-critical-storage-nothing-new-is-created.md) (nothing new at critical storage)
- relates to [ADR-0212](0212-highlights-from-a-reading-service-as-a-connector.md) (highlights from a reading service, the other import of notes)
