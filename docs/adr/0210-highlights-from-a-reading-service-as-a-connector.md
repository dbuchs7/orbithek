# ADR-0210: Highlights from a reading service as a connector

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (the maintainer asked that one's highlights from Readwise come into the wiki as a connector set up under Administration → Connectors like the others, with a schedule and a dry run, each book or article an item of a chosen type, updated incrementally, never deleting what people added below a marked section, 2026-11-12)
- **Open question:** Is one account per connector, the operator's token for it, the right model, or should each person connect their own account, as people connect theirs to a process engine (ADR-0151)?
- **Question checked:** 2026-11

## Context and problem statement

People who read on e-readers, in browsers and in apps collect highlights and notes in Readwise, which gathers them from Kindle, from read-later applications, from podcasts and from articles. What they highlighted is a part of what they know, and it belongs beside the pages and tasks it informs. Copying highlights by hand goes stale as reading goes on.

Connectors already read sources on a schedule into a room, with a dry run, a report, an audited run, credentials from the environment and public addresses only (ADR-0077, ADR-0127); a source read for what changed exists too (ADR-0140). But a connector's page is the source's: every run replaces it (ADR-0080), or writes it whenever the source changed (ADR-0138). A book's page of highlights is where its reader goes on to write their own thoughts. How do highlights come in as items that a run keeps current without ever taking what people wrote?

## Decision drivers

- What ADR-0077, ADR-0127 and ADR-0140 decided for connectors holds: set up by administrators, a schedule, a dry run, upsert by origin (ADR-0023), withdrawal instead of deletion, the token from the environment, public addresses only unless `OT_CONNECTOR_ALLOW_PRIVATE=on`.
- What people write in an item is never lost to a run.
- A run asks the service only for what changed, as its export allows, so a large library is not read every day.
- Items of a type the administrator chooses, with its fields: a book's author, its address and its cover are queryable properties (D4), its tags labels (ADR-0044).
- Other products are named only where the subject is importing from them (AGENTS.md); the kind is named for what it reads.

## Considered options

1. **A connector kind `highlights`** whose reader reads the service's export API, each book an entry with labels; its page is the highlights above a line, and a run keeps what is below the line.
2. **Highlights as comments** on the book's item, which people do not edit and runs add to.
3. **A page per highlight**, below a page per book.
4. **An import of the service's CSV export** through the import of notes (ADR-0209).

## Decision outcome

Chosen option: **1**, because it reuses the connectors' whole machinery and adds only what highlights need: labels from the source, a merge of what changed, and a part of the page that belongs to people.

### Reading

- **Kind and settings.** `highlights`, with the service's address (`https://readwise.io` in the template, so another installation of the same API can be named), the language of the page's own words, and the days between full runs (7 by default, 1 to 365). The type and the field mapping are those of every connector (ADR-0127); the values given are `author`, `category`, `cover` (the cover picture's address), `source` (where it was read, e.g. kindle), `url` (the book's or article's address), `link` (its page at the service) and `tags`. A new connector runs every 24 hours.
- **Export API, version 2.** `GET <address>/api/v2/export/` with `Authorization: Token <token>`, the token in `OT_CONNECTOR_<ID>_TOKEN` (ADR-0077). The list is read page by page by `nextPageCursor`, up to 500 pages. Asked too often, the service answers 429 with `Retry-After`; a run waits as asked, at most a minute each time and four times, then fails. A refused token fails the run with the variable's name, never its value.
- **What changed.** A run asks with `updatedAfter` for the books whose highlights changed since the last run that wrote began, an hour earlier (ADR-0140); a full run comes after a restart, a change of settings, on request (*Sync in full now*) and when the days between full runs have passed. Only a full run withdraws a book the service no longer gives.
- **Requests** keep to public addresses, redirects included, as every connector's (ADR-0127). The reader is the connector's guarded client.
- The source's code lives in `internal/connector/readwise.go`; the shared code names the kind `highlights`.

### Writing

- **An item per book or article**, identified by the service's `user_book_id` (`entry/<id>`, origin system the connector's ID, instance the service's address). Its title is the book's readable title.
- **Labels** from the book's tags, normalised as labels are (`Classic Books` becomes `classic-books`); a run adds them and never removes a label, so labels people set stay. This is a new field of an entry, `Entry.Labels`, which any reader may fill.
- **The page** is written from the book: a note that says where the highlights come from, the book's address and cover as links, its summary and note, then each highlight as a quote, its note below it in italics and its place — *Page 12*, *Location 1834*, *At 02:05* for a recording, else *Position* — linked to the highlight, with its tags. Then a line (a divider and a panel of the tone *note* that says to write below it).
- **What people write below the line stays.** When a run writes the page anew, it keeps every block below the line of the page as it stands (`Entry.Below`). If people removed the line, the run keeps their whole text below the new one and reports it, so nothing they wrote is dropped; the next run finds the line again.
- **Merging.** The book and its highlights as read are kept in the item's origin (`Origin.Raw`, sealed, ADR-0026). A run of what changed merges the highlights it hears of with those kept — new and changed ones by their ID, deleted or discarded ones taken out — through `Entry.Update`, which is given the kept content; a full run writes what the service gives. The page is written only when the merged book differs from the kept one, so a run that hears of nothing writes nothing.
- **Withdrawal** is that of every connector: a book a full run no longer finds is labelled withdrawn and keeps what was written here.

### Consequences

- **Positive:** a reader's highlights appear in the wiki, searchable, labelled and linked from pages, and stay current without manual copying; the item's text below the line is the reader's own.
- **Positive:** two general mechanisms for connectors — labels from the source, a part of the page that belongs to people — are available to every reader.
- **Negative / trade-offs accepted:**
  - One account per connector: the operator holds its token; every reader of the room sees that account's highlights. A person's own highlights need a connector of their own and a room for them.
  - A highlight edited here, above the line, is replaced at the next change of its book at the service; the history keeps it.
  - Pictures of covers stay links: the connector stores no files from the service.
  - Readwise's documentation was not reachable from where this was built; the endpoint, the parameters and the fields follow its published description of the export API version 2 as known to the authors and are tested against a fake. The first run against the real service is the check; an unexpected answer fails the run and says so.
- **Follow-ups / risks to watch:**
  - the open question: people connecting their own accounts;
  - the service's documents of its read-later application (another API) as a second source;
  - highlights' own tags as labels of the item, if readers want them queryable.

## Pros and cons of the options

### Option 2: highlights as comments

Strongest form: comments are part of their item (ADR-0035) and are never overwritten by a run; each highlight would be one comment, and the item's text would be entirely the reader's.

- Good: no line in the text, and no risk to what people wrote.
- Bad: comments are a discussion, ordered by time, not a book's passages in their order; a highlight changed or deleted at the service would mean a run editing or deleting comments, which is a person's act (ADR-0035).
- Bad: comments are not exported with the page (ADR-0071), so the highlights would be missing from a book's page as a document.

### Option 3: a page per highlight

Strongest form: each highlight an item could be labelled, related and embedded on its own, and a run would replace only that item's text.

- Good: the finest granularity for linking a passage.
- Bad: thousands of items for a library, each of a few words, in search, the starmap and every listing; reading a book's highlights would mean opening a tree.

### Option 4: the CSV export through the import of notes

Strongest form: the service exports CSV; ADR-0209 already turns a CSV into items, with no token in the server at all.

- Good: no credential and no outbound call.
- Bad: a person would download and upload again after every reading session; nothing would stay current, which is the point of the request.

## Links

- builds on [ADR-0077](0077-connectors-keep-catalogs-in-step-with-their-sources.md) (connectors, credentials from the environment), [ADR-0127](0127-connectors-read-any-json-source-into-a-type-of-ones-own.md) (a type and fields, public addresses only), [ADR-0140](0140-a-json-source-is-read-for-what-changed.md) (what changed, full runs)
- relates to [ADR-0080](0080-one-page-per-data-set-written-from-the-source.md) and [ADR-0138](0138-markdown-from-git-repositories.md) (pages written from the source, replaced or written when it changed; here a part of the page stays people's), [ADR-0023](0023-import-and-sync-from-jira-and-confluence.md) (origin), [ADR-0026](0026-crypto-shredding-of-item-content.md) (the kept book sealed), [ADR-0044](0044-labels.md) (labels), [ADR-0151](0151-user-tasks-of-an-engine-become-linked-tasks.md) (people connecting their own accounts)
- relates to [ADR-0209](0209-importing-a-folder-of-markdown-notes-and-exported-note-workspaces.md) (the other import of notes)
