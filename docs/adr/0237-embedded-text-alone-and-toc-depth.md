# ADR-0237: An embed may show its item's text alone, and a table of contents lists headings down to a chosen level

- **Status:** Proposed
- **Implementation:** Not started
- **Date:** 2026-11-17
- **Deciders:** Project initiators (the maintainer asked that a page can be embedded with its content only, without its heading, and that inserting a table of contents asks how many heading levels it lists, 2026-11-17; then that such content, for reading, looks exactly as if written in the page, 2026-11-17)
- **Open question:** whether the import of ADR-0023 should carry a source's table of contents depth, and whether a type's start text should name one (`[toc]` has no level)
- **Question checked:** 2026-11

## Context and problem statement

An embed shows another item whole (ADR-0106): a heading with the item's key and title that leads to it, its kind, presence and state, its fields, and its text, framed.
That suits a task or a status page.
It does not suit a page that gathers parts kept elsewhere, such as a common safety notice, a legal paragraph or a chapter shared by several manuals: there the reader should read the part as part of the text, without a frame, a key or a list of fields.

A table of contents (ADR-0042, ADR-0119) lists every heading of the body, levels 1 to 3.
A long text with many level-3 headings gets a table of contents longer than is useful, and the author cannot shorten it.

How does an embed show its item's text alone, and how does a table of contents list fewer levels, without changing what either block is?

## Decision drivers

- An embed still never grants access and never copies its target (ADR-0020, ADR-0106, D4, D10): a reader who may not read the target learns nothing of it.
- Bodies written before stay as they are and mean what they meant.
- The editor, the reading view, the live session and the Word, PDF and Markdown exports show the same.
- Agents write both blocks over MCP without reading a page first.

## Considered options

1. **Fields on the existing blocks:** an embed carries `View`, as diagrams do (ADR-0143), with `text` for its item's text alone; a table of contents carries `Level`, the deepest level of the headings it lists, 0 for every level.
2. **New block kinds:** `embedtext` beside `embed`, and one table of contents kind per depth or a `tocN` kind.
3. **Only in the browser:** a setting of the reader, or a style per page, that hides the heading of every embed and cuts every table of contents.

## Decision outcome

Chosen option: **1**, because it reuses two fields whose meaning already fits — what a block shows, and a heading level — and adds no kind that every reader of the API would have to learn.

### The embed of its text alone

- **What it shows:** the target's text, read-only, with its pictures and its own embeds; no heading, no key and title, no kind, presence, state or fields, and no frame. A web link shows its card, which is its content (ADR-0208).
- **For reading, as if written in the page:** the same spacing, fonts, pointer and selection as the text around it, nothing that tells it is embedded; a table of contents and the lists of figures and tables of the page count its headings, pictures and tables as the page's own, once it is loaded, in the reading view, the editor and every export (`wovenBlocks`, `woven`). Only what the reader may read is counted (D10).
- **Who sees what:** unchanged. The text is loaded as the reader, protected passages stay veiled, and a reader who may not read the target sees the same note as for a whole embed, "an item you cannot see": an embed never vanishes without a word.
- **Notes stand with a name:** where the text cannot be shown — the item is already shown around the place, or it is embedded too deep (ADR-0106) — the note names the item.
- **While editing:** a slim line names the item and offers the choice *The whole item* or *The content only*; the editor inserts either from the menu *Insert…* (*Embed an item…*, *Embed an item's content…*).
- **Exports:** the body is written as part of the text around it, its headings at the host's levels, without the facts; the name is written only where the body cannot be.
- **Validation:** an embed's `View` is empty or `text`; `both` is refused.

### The depth of a table of contents

- **What it lists:** the headings down to `Level`, 1 to 3; 0, the default, lists every level as before.
- **When inserted:** the editor asks for the depth — *Heading 1 only*, *Headings 1 to 2* or *Headings 1 to 3 (all)* — and offers the same choice on the block while editing.
- **Exports:** the HTML (PDF), Word and Markdown exports list the same headings; under a Word template, Word's own field is written with `\o "1-N"`.
- **Validation:** `Level` stays the field of headings and of tables of contents only, 0 to 3.

### Codec

Codec version 63 records that a table of contents may carry `Level` and an embed `View`.
Versions 1 to 62 are still read: their tables of contents list every level and their embeds show their items whole.
A server before this one refuses a version 63 log rather than misreading it.

### Consequences

- **Positive:** pages can be assembled from shared parts and read as one text; long texts get a short table of contents.
- **Negative / trade-offs accepted:**
  - A reader of an embed of the text alone does not see where the part comes from, by intent; the editor's line and the starmap and impact view (ADR-0157, ADR-0159) still show the link.
  - The text is a read-only editor of its own inside the page: a selection does not run across the boundary, and a numbered list does not continue its numbers across it.
  - A comment or a protection acts on the part's own page, as for a whole embed (ADR-0106).
  - The depth is a block's, not a page's: two tables of contents of one body may list different levels.
- **Follow-ups / risks to watch:** the import's and the start text's table of contents, under *Open question*; if headings gain levels beyond 3, `Level` on a table of contents keeps its meaning and only the choices grow.

## Pros and cons of the options

### Option 2: new block kinds

Strongest form: a kind says everything; a reader of the API, an export or a diff needs no second look at a field, and old servers cannot misread a new kind.

- Good: the old servers' refusal comes for free from the unknown kind.
- Bad:
  - The two embeds are one thing shown two ways; two kinds double every place that handles embeds — relations, copies, restores, protections, the impact view — or leave one of them out by accident.
  - One kind per depth multiplies the table of contents, which ADR-0119 kept as one block with good reason.
  - A codec version is needed all the same, and the codec version already makes an old server refuse a newer log.

### Option 3: only in the browser

Strongest form: nothing stored changes, and each reader picks the look they like.

- Good: no codec version.
- Bad:
  - What a text shows is the author's decision, not the reader's: a legal paragraph meant to read as part of a contract must not show a frame and a key to some readers.
  - Exports and agents would not know it, so a Word file and the page would differ.

## Links

- amends [ADR-0106](0106-an-embedded-item-is-shown-whole.md) (an embed may show its item's text alone)
- amends [ADR-0042](0042-tasks-panels-tables-and-computed-blocks.md) and [ADR-0119](0119-lists-of-figures-and-tables.md) (a table of contents lists headings down to a level)
- relates to ADR-0020 (embedding), ADR-0025 (codec), ADR-0071 (exports), ADR-0131 (Markdown export), ADR-0143 (`View`), ADR-0167 (Word templates)
