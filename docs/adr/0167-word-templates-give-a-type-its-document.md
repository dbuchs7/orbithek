# ADR-0167: Word templates give a type its document

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-09
- **Deciders:** Project initiators (the maintainer asked on 2026-11-09 that a type such as "Word report" give its pages the structure, header and footer of a Word template and export them as a finished Word document; the editor stays as it is; the reading view shows the whole document with header and footer; only administrators keep templates; every page remembers the template version it was made with)
- **Open question:** whether pages should follow a new template version on their own after a grace period, whether the PDF export should be drawn as the template too, and whether section breaks, footnotes and landscape pages inside a text are needed; this record decides none of them
- **Question checked:** 2026-11

## Context and problem statement

Organisations write reports, concepts and memos in their corporate design: a Word template with its styles, page, header with logo, footer with classification and page numbers, and a cover. A page exported as Word today (ADR-0071) has the product's own styles and none of these; someone copies it into the template by hand. A page's look in the wiki says nothing of the document it becomes.

The editor is to stay as it is. What should a type with a template change — in storage, when a page is made, when it is read, when it is exported — and what keeps an exported document stable when the template changes?

## Decision drivers

- One binary, pure Go, no converter beside it (ADR-0009, ADR-0024); the standard library only if it can be done with it.
- A template is a file from outside: nothing in it may run or reach out when a document made from it is opened, and nothing in it becomes markup in the browser (ADR-0014).
- The template's own markup must survive: Word templates hold far more than any writer of this product understands.
- The body stays the block document of ADR-0001; queryable data stays in properties (D4).
- A document exported once and exported again later should look the same unless someone decides otherwise.

## Considered options

1. **The template's package as the base of the export**, filled at a marked place, its content controls filled from the page's fields; the reading view frames the page as the template, drawn from the same parts; templates kept with the corporate design.
2. **Styles only**: copy the template's styles into the product's own generated document.
3. **A word-processing service beside the server** (a document server speaking WOPI or similar), the page edited and stored as a `.docx`.
4. **Templates as attachments of items** in a room, a type naming the item.

## Decision outcome

Chosen option: **1**, with the template kept in the site's settings (not option 4).

- **Keeping templates.** Administrators upload a `.dotx` or `.docx` under *Administration → Word templates* (`POST /api/word-templates/{id}`, at most the budget `WordTemplateBytes`, its parts unpacking to at most eight times as much). `internal/wordtemplate` checks the package and cleans it; the cleaned file is kept in `site.wal` as a new version (`WordTemplateSet`), so it is backed up and restored with the corporate design. Every version is kept. A template is removed (`WordTemplateRemoved`) only when no type names it and no live or trashed item was made with one of its versions, as an item is deleted only when nothing refers to it (D8). Changes are audited as `design.change`.
- **What a template may not hold.** Macros (macro-enabled content types, `vbaProject.bin`), ActiveX controls, embedded objects, alternative-format chunks, sub-documents and frames are refused, and so is every relationship to a target outside the package other than a hyperlink — an external picture or template would make Word fetch it when the document opens. The reference to the template a document was made from, which Word writes and which names a file on the author's computer or a share, is removed with its element. A document type declaration is refused; every XML part must be well-formed.
- **Keeping the template's markup.** Each part is read into a tree that knows each element's byte span; a change replaces exactly that span. No part passes through an XML encoder, whose namespace handling would rewrite Word's markup.
- **A type names its template and its start text.** `TypeDef.Template` names the template by its ID; `TypeDef.Start` is the text a new page begins with when it is made without one (`POST /api/items` without a body). Start blocks are text and the blocks that show what the page itself holds (headings, panels, lists, tables, a table of contents, child pages); blocks that name an item, a file, an engine's process or a protection belong to one page and are refused. The type API also takes and shows the start text as Markdown (`startMarkdown`), with lines `[toc]`, `[children]`, `[figures]` and `[tables]` for the blocks Markdown has no form for. Both are system data of the type, kept in plaintext like its field names.
- **A page remembers its version.** `ItemCreated` carries `template`, the newest version of the type's template when the page is made, decided when the command runs (the store asks the site through a hook) and recorded in the event, so replaying gives the same item. A copy keeps its original's version. `TemplateSet` moves a page to another version (`POST /api/items/{ref}/template`, editors; the newest by default). Codec version 41.
- **The export.** A page of such a type exports as the template's package: the text where the content control tagged `orbithek:body` stands (taking its paragraph's place when the control is inside one), else after the template's content; every control tagged `orbithek:<name>` in the text, the header and the footer shows the page's title, key, type, state, room, labels or the property of that name, in the formatting of the control's first paragraph and run, no longer as a placeholder and no longer bound to the document's data, which Word would write over it. The product's styles are found among the template's by their names — Word names its built-in styles in English in every language, while their IDs follow the language a template was made in — and only what the template lacks is added; lists, links and pictures get numbering and relationship IDs the template does not use. The facts table of ADR-0071 gives way to the controls. A table of contents is Word's own field, marked as changed so that Word offers to bring it up to date with page numbers when the document opens. The main part's content type becomes a document's. The export uses the version the page remembers; a page made before its type had a template uses the newest.
- **The reading view.** `GET /api/items/{ref}/document` answers, through the reader's session (D10), the page framed as its template's version: the template's content split into sheets at its page and section breaks, the place of the text, each sheet's header and footer (the first sheet's own when the template has a title page), the controls filled in, page fields as fields the browser numbers by sheet, pictures (PNG, JPEG, GIF) by the address of the version's media, text boxes after their paragraph, a background picture left out. Every value is checked; the browser builds every element with `h()` and sets only checked style values. `GET /api/word-templates/{id}/versions/{v}/style.css` draws the sheet with the page's size and margins, a class per paragraph style for the template's content, and the template's styles over the page's text under a scope of that version, heading numbers by CSS counters where the heading styles number theirs. While a page is edited, the editor is the same as for every page. A reader sees which version the page is laid out after; an editor is offered the newest.
- **MCP.** Not offered: keeping templates is the administration of the corporate design, and the frame is drawn for people. An agent creating a page of such a type gets its start text, and reads it as Markdown.

### Consequences

- **Positive:** a page leaves the wiki as a finished document in the organisation's design; the template's own markup — headers, footers, fields, cover pages, numbering, building blocks — passes through untouched.
- **Positive:** an exported document stays the same when the template changes, until someone moves the page; the version is part of the page's history.
- **Negative / trade-offs accepted:** the reading view approximates Word: it does not break pages, wrap text around pictures or run fields other than page numbers; a font the reader's computer lacks shows as another; pictures in other formats (EMF, WMF, SVG without a fallback) are left out of the view, not of the export.
- **Negative / trade-offs accepted:** the block document limits what a text can use of a template: three heading levels, tables without merged cells, one level of lists, no footnotes, section breaks or landscape pages within a text.
- **Negative / trade-offs accepted:** every version of every template stays in `site.wal` and in memory; templates are rare and small, as the design's files are (ADR-0052).
- **Negative / trade-offs accepted:** the PDF export still prints the product's printable form (ADR-0071); a PDF in the template's design is made from the Word document.
- **Follow-ups / risks to watch:** templates that rely on content controls bound to custom XML parts, or on fields other than page numbers, may need more of their behaviour mapped.

## Pros and cons of the options

### Option 1: the template's package as the base

Strongest form: the template is used as Word uses it — as the document's starting point — so whatever it holds keeps working, and the product writes only what it understands.

- Good: header, footer, cover, numbering and fields of any complexity survive; no dependency.
- Bad: a careful merge of styles, numbering, relationships and content types; a template is untrusted input and needs checking.

### Option 2: styles only

Strongest form: the smallest change to ADR-0071 — the export already writes styles; taking the template's makes the text look right.

- Good: little code.
- Bad: no header, footer, logo, page numbers or cover — what makes a document the organisation's is exactly what is left out; someone still copies the text into the template.

### Option 3: a word-processing service

Strongest form: only a real word processor shows a document exactly as Word does, page breaks included; anything else is an approximation people will notice.

- Good: exact layout while editing.
- Bad: the body becomes a binary file: no blocks, embeds, mentions, protected passages, search by structure or derived relations (ADR-0001, D4); a second server beside the binary; licences to settle. It would be another product, not an extension.

### Option 4: templates as attachments of items

Strongest form: templates become content with everything content has — versions, comments, permissions, the trash — and anyone with a room keeps their own.

- Good: no administration needed; history and comments come for free.
- Bad: a type is global but an item lives in a room with its restrictions; a reader of a page who may not read the template's item would get a document made from something they cannot see (D10), or the export would have to read past the permission check. The maintainer chose administrators as the only keepers, as for the corporate design.

## Links

- extends ADR-0071 (exporting pages), ADR-0052 (corporate design in a journal of its own)
- relates to ADR-0001 (types are data), ADR-0014 (closed renderer), ADR-0015 (budgets), ADR-0021 (deletion only when unreferenced), ADR-0025 (persisted format), ADR-0067 (copies)
