# ADR-0208: A web link becomes an item with what its page says about itself

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 for web links as items, a preview of a page's metadata, a link card and taking over a page's text). Accepted by the requester on 2026-11-12, with the defaults proposed with the implementation.
- **Open question:** whether a link card should also exist without an item, as a block of its own; whether the server should read pages through the proxy the environment names, as connectors do, rather than only directly; whether a page's text taken over should be replaced when it is read again rather than added; whether the picture of a page should be scaled down before it is kept
- **Question checked:** 2026-11

## Context and problem statement

People collect pages of the web: articles to read, references of a decision, sources of a specification. A link in a text keeps the address and nothing else; what the page is about, who wrote it and when, is lost unless someone copies it by hand, and the link cannot be commented on, annotated, labelled, related or found by its fields.

How does a page of the web become something the wiki holds and works with, and how much does the server read of other people's pages to help, given that every outgoing call of the server is off until an operator switches it on (ADR-0039, ADR-0127) and that nothing a page holds may ever run in the UI (ADR-0014)?

## Decision drivers

- **One model for content (D1):** a page kept is content, so it is an item, not a parallel object.
- **No shadow data in the document (D4):** what is queryable — site, author, day — is a property, not text copied into a body.
- **The server's own network stays closed:** the server reads only public addresses, checked where a connection opens and for every redirect, unless an operator allows the own network.
- **Bounded work:** the bytes of a page and of its picture, the time of a request and the readings at once are budgets (ADR-0015).
- **Nothing runs:** a page is parsed, never executed; a picture is kept only when its bytes are a format a browser shows without running anything (ADR-0045).
- **The picture is a file of the item:** it obeys the attachment budget, the critical storage rule (ADR-0163), versions (ADR-0103) and erasure with the item (ADR-0026).
- **Useful without the outgoing call:** a web link is saved with the call off; only its fields stay empty.
- **One binary, no CDN (ADR-0009).**

## Considered options

1. **A ready-made type `weblink`; the link card is the embed of the web link item;** the server reads a page's head on request (`internal/linkpreview`), with `golang.org/x/net/html`.
2. **A link card block of its own** (codec version 57) holding title, site, description and picture, with or without an item.
3. **Metadata read in the browser** from the page directly.
4. **Unfurling in the server automatically** whenever an address is saved in any text.

## Decision outcome

Chosen option: **1** (defaults proposed with the implementation, confirmed by the requester), because it keeps every part of a web link in the item model the rest of the wiki already understands, needs no change of the persisted format, and confines the outgoing call to explicit actions of a person.

- **The type.** `item.TypeTemplates` gains `weblink`: a body for one's notes and the fields `url` (kind url), `site`, `description`, `author` (text) and `published` (date). None is required. `POST /api/weblinks` saves a page as an item of it; when no type of that name exists, `Session.CreateFromTemplate` defines the ready-made type in the same batch as the item, as the task of an access request does (ADR-0162), so one who may create in a room saves a link before an administrator added the type. A refused item leaves no type behind. The setup card on the start page offers the type to administrators like every missing template.
- **Reading a page.** With `OT_LINK_PREVIEWS=on` (off by default) the server reads, for a person's request, the page's title, description, site name, Open Graph or card picture, canonical address, author and day of publication (`POST /api/linkpreview`, nothing kept), and fills a new web link's fields from them. `internal/linkpreview` connects through `linkcheck.Transport`: the name is resolved and every address checked before a connection opens, also for each of at most five redirects, and never through a proxy. `OT_LINK_PREVIEWS_ALLOW_PRIVATE=on` lets it read the own network (an intranet), the operator's decision. Budgets: `LinkPreviewPageBytes` (a larger page is read as far as the budget, its head comes first), `LinkPreviewImageBytes` (and `AttachmentBytes`), `LinkPreviewSeconds`, `LinkPreviewsAtOnce` (beyond it, 503 with Retry-After) and `LinkPreviewBlocks`. ISO 8859-1 and Windows-1252 pages are converted; other pages are read as UTF-8.
- **The picture** a page names is read, checked by its bytes against `item.ImageTypes` (no SVG), and stored as the attachment `preview.<ext>` of the item, through the session of the person, so the attachment budget, the critical storage rule and the item's key apply. A page that fails leaves the link saved and the answer names the problem.
- **Reading again.** `POST /api/items/{ref}/weblink` reads the page of any item whose type has a url field named `url`: it sets `site`, `description`, `author` and `published` where the type has them as validated changes (D3), keeps a changed picture as a new version of the previous one (`Source`, ADR-0055, ADR-0103) and the same picture once. It never renames the item: the title may have been chosen.
- **Taking over the page's text.** With `text`, the page's main text — its first `article`, else `main` or `role=main`, else `body`; navigation, headers, footers, forms, scripts and media left out — becomes blocks: headings (h1 and h2 to levels 1 and 2, the rest 3), paragraphs, bullet and ordered items, quotes, code, tables and dividers, with links (absolute http, https), bold, italic and code as marks. They follow the body after a divider, so the existing inline comments (ADR-0047) and personal notes (ADR-0107) work on them. It is refused while the body is edited live (ADR-0010). `golang.org/x/net/html` parses the page: it is the HTML5 parsing algorithm browsers use, maintained by the Go project, BSD-3-Clause, already in the module graph, and its tokenizer never runs anything; a hand-written scanner would misread the many pages that rely on the parser's error recovery.
- **The card.** A web link shows as a card — picture, site, author, day, its title linked to the page, its description — on its own page and wherever a text embeds it (`assets/weblink.js`). The card is how the UI draws any item whose url field holds a web address, from the item's properties: nothing of the page is copied into the text that shows it (D4), and the embed is a reference, so a web link in use is not deleted (D8).
- **In the editor.** The bubble under a link offers *Save as web link item*: below the page or in a chosen room; a paragraph that holds the address alone becomes the embed. Pasting an address alone leaves the cursor in the link, so the offer shows at once.
- **MCP** offers `save_web_link` and `read_web_link_again`, not the preview: an agent reads the web itself and would only borrow the server's address.

### Consequences

- **Positive:** a kept page is searchable, labelled, related, commented, annotated, restricted and exported as any item; its fields are queryable; the server reads pages only when asked and only within budgets and public addresses.
- **Negative / trade-offs accepted:** a card needs an item, so a page shown as a card is one more item in the room; a page whose server answers only through a proxy cannot be read; a picture is kept at its own size within the budgets; the template type appears in the start page's setup card of installations that never wanted it.
- **Follow-ups / risks to watch:** pages that hide their metadata behind scripts give nothing; the attachment name `preview.*` marks the page's picture by convention.

## Pros and cons of the options

### Option 1: an item type, the card as its embed

- Good: D1 and D4 hold without exception; no codec change; every capability of items applies; deleting a web link in use is refused (D8).
- Bad: one item per card.

### Option 2: a link card block

Strongest form: a card is a property of the text that shows it, like an image's caption; it needs no item, is placed and removed as fast as a link, does not crowd the room, and a new block kind under codec version 57 is a contained change.

- Good: lightweight; works where a person may edit the text but not create items.
- Bad: the title, description and site copied into a body are shadow data (D4): not queryable, stale the day the page changes, duplicated in every text that shows the page; the picture would be a file of each text; no comments or notes on the page itself; a persisted format change for what an embed already does.

### Option 3: in the browser

Strongest form: the reader's browser fetches the page itself, so the server makes no outgoing call at all and no address check is needed.

- Bad: the cross-origin policy of nearly every site forbids reading its HTML from another origin, and the UI's content security policy (`default-src 'self'`) forbids it too; a picture fetched by the browser would reveal readers to the page's owner each time.

### Option 4: automatic unfurling

Strongest form: every address in every text is previewed without anyone asking, as people expect from messengers.

- Bad: the server would call out on every save, also for addresses people did not mean to share, in texts of restricted items; the calls would scale with the wiki rather than with what people ask for.

## Links

- relates to ADR-0001, ADR-0009, ADR-0014, ADR-0015, ADR-0026, ADR-0039, ADR-0045, ADR-0047, ADR-0055, ADR-0074, ADR-0103, ADR-0106, ADR-0107, ADR-0162, ADR-0163, ADR-0209
