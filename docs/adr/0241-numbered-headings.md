# ADR-0241: Headings numbered by their level, switched per text

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-17
- **Deciders:** Project initiators (request of 2026-11-17 that headings can be numbered automatically by their formatting, heading 1 to x, the numbering switched on and off and shown or hidden in the table of contents accordingly; asked where the switch belongs, the requester chose one per page; on 2026-11-17 the requester asked for a type's default as well)
- **Open question:** whether other forms of numbers (a closing dot, letters, Roman numerals) are wanted; whether the Word export should write Word's own list numbering instead of the numbers as text; whether a text should refer to a heading by its number ("see 2.1") that follows the heading
- **Question checked:** 2026-11

## Context and problem statement

A body has headings of three levels (ADR-0032) and can hold a table of contents that lists them when shown (ADR-0042, ADR-0119).
Concepts, specifications and manuals number their headings: 1, 1.1, 1.1.1.
Today a person has to type those numbers into the heading's text and renumber every heading after one is added, moved or removed; the table of contents then shows whatever was typed.

The question is where the decision to number lives, how the numbers are computed, and how they reach the editor, the reading view, the table of contents, the exports, a live session and agents.

## Decision drivers

- Numbers are derived from the headings, so, like the table of contents, they are computed when shown and cannot go stale (ADR-0042).
- One switch: the text and its table of contents show numbers, or neither does.
- The same numbers in the editor, in reading, in the Word, PDF and Markdown exports, and for agents.
- The body stays a flat list of blocks; live editing keeps locking one block at a time (ADR-0010).
- Nothing a reader receives reveals a heading the reader may not read (D10, ADR-0160).
- An agent that writes a body without knowing the switch must not turn it off by accident.

## Considered options

1. **A field of the body**, `Doc.Numbered`, stored with every version of the body.
2. **A field of the table of contents block**: a numbered table of contents numbers the headings of its body.
3. **A setting of the item** outside the body, recorded by an event of its own.
4. **A setting of the type**, the same for every item of it.
5. **A setting of the viewer**, kept in the browser.
6. **Numbers typed into the headings' text**, written by the editor.

## Decision outcome

Chosen option: **1, a field of the body**, because the numbering is a property of the document, as a word processor keeps it in the file: it travels with each version of the text, is restored with it, and every consumer that reads a body already has it.

### The numbers

- `item.HeadingNumbers` computes them from a body, `assets/numbering.js` the same in the browser; `internal/web/jstest/heading-numbers-vector.json` holds both to one set of cases.
- Every heading with text is numbered by its level, joined with dots: 1, 1.1, 1.1.1.
- The shallowest level the body uses counts first, so a body that starts at heading 2 numbers its level 2 headings 1, 2, 3.
- A level skipped below another counts as 0 (1.0.1), as word processors do; a deeper heading before the first of the top level is numbered 0.1.
- Headings without text are not counted, as the table of contents does not list them.
- The numbers are computed over the blocks as the reader reads them: headings in a protected section the reader may not read are a placeholder and are not counted (ADR-0160).
  Readers of different rights may therefore see different numbers after such a section.
- An item embedded whole keeps its own numbering; its headings do not count in the text that embeds it.
- The text of an embed of its text alone reads as written in the page (ADR-0237): its headings count as the page's own, take the page's numbers and stand numbered in its table of contents, whether or not the embedded item numbers its own headings.

### Where they show

- **Editor and reading view:** the number stands before each heading as a widget that is not part of the text, so it is neither edited, copied nor searched.
- **Table of contents:** each entry shows its heading's number; without the numbering the table of contents is unchanged.
- **Exports:** HTML (PDF) writes `<span class="num">`, Markdown and Word write the number before the heading's text, so Word's own table of contents, when updated, lists it (ADR-0071, ADR-0131, ADR-0167).
- **Versions:** the history's comparison says when a version numbered the headings or stopped numbering them, which changes no block.

### How it is switched

- **Editor:** the toolbar button **Number headings** switches it for the whole text.
- **Live session:** the operation `number` names no block and takes no lock; the server stores the body at once with the blocks as they were last released, and tells every client with the event `numbered`.
  A snapshot carries the setting; drafts others hold stay drafts.
- **API:** the item carries `numbered`; `PUT /api/items/{ref}/body` takes an optional `numbered`, and a body sent without it keeps what it had; appending blocks keeps it; a version carries it.
- **MCP:** `commit_body` takes the optional `numbered`; `get_item_markdown` shows the numbers through the Markdown export.
- **Server-side rewrites** of a body — copies, web links, whiteboard checkpoints, a connector's withdrawal note, the daily note — keep the setting.

### A type's default

- A type may say that its new items begin with their headings numbered: **Number the headings of new items** in the type editor, `numbered` on the type in the API, for the installation's types and a room's alike (ADR-0201).
- It is kept as the numbering of the type's start text (`TypeDef.Start.Numbered`, ADR-0167): a type's start is the document a new item begins with, its text and whether it is numbered, with or without start text.
- An item made through the API — the creation view, **Create and mention**, the calendar's new entry, agents through MCP `create_item` — begins numbered when its type says so, whether a text is given or not; a request's own `numbered` takes precedence.
- It is a default, not a link: the item's own switch decides from then on, and items made before keep their choice; changing the type changes no item.
- A copy keeps its original's numbering, even of a body without blocks; imports, connectors, linked tasks and daily notes make their items as before, as they do not take a start text either.
- A type without a body refuses it.

### Codec

Codec version 66 adds the body field `Numbered`, sealed with the body.
Versions 1 to 65 are still read; their headings are not numbered.
A server before this one refuses a version 66 log rather than dropping the setting.
A type's default needs no version of its own: it is the same field in the type's start text, which version 66 already reads.

**Implementation:** everything above is built and tested in Go, Node.js and Chromium. Not built: what the open questions ask.

### Consequences

- **Positive:**
  - Numbers follow every added, moved or removed heading without anyone renumbering.
  - The table of contents, the text and every export agree.
  - Restoring an earlier version restores whether it was numbered.
- **Negative / trade-offs accepted:**
  - Switching the numbering writes a new version of the body, so it shows in the history and the change feed.
  - Numbers are text in the Word export, not Word's list numbering: renumbering in Word after editing there is manual.
  - Only three levels exist, so numbers have at most three parts.
  - Readers who may not read a protected section see numbers that skip its headings.
- **Follow-ups / risks to watch:** references to a heading by its number.

## Pros and cons of the options

### Option 2: a field of the table of contents block

Strongest form: the table of contents already lists the headings, a block carries attributes such as a panel's tone, and live editing changes it as any block, under its lock, with no new operation.

- Good: no change to the body's shape or to the live session.
- Bad:
  - A text without a table of contents could not be numbered, and the request numbers the headings themselves.
  - Two tables of contents could disagree; one would have to win.
  - The setting would vanish when the table of contents is deleted.

### Option 3: a setting of the item outside the body

Strongest form: like the order of a folder's slides (ADR-0195), an event of its own leaves every path that rewrites a body untouched, and nothing could drop it by rebuilding a body.

- Good: no body rewrite can lose it.
- Bad:
  - Restoring an earlier version would not restore whether it was numbered, though the numbers are part of how it read.
  - It needs an event kind, a route, its own MCP tool and its own place in the change feed, for one bit of the document.
  - The exports and the live session would read the setting from somewhere other than the body they render.

### Option 4: a setting of the type

Strongest form: an organisation numbers all its concepts or none, set once by an administrator.

- Good: consistent across a type without anyone deciding per text.
- Bad: the request asks for a switch per text; a type cannot hold one text numbered and another not.
  It is built as a default on top of option 1 (see *A type's default*).

#### A type's default: copied at creation or followed

Strongest form of following: an item without a choice of its own would follow its type, so switching a type numbers or unnumbers all its items at once, as a style sheet does.

- Good: one change reaches every item of the type.
- Bad:
  - An item would need a third state, «as its type», in the body, the API, the live session and the editor's switch.
  - A text would change its numbers, its exports and its table of contents without a version of its own, when someone redefines a type.
  - Chosen instead: the default is copied when an item is made, as the start text is.

### Option 5: a setting of the viewer

Strongest form: no stored data at all, nothing to migrate.

- Bad: other readers, the exports and agents would not see the numbers; a printed document would differ from the one on screen.

### Option 6: numbers typed into the headings

Strongest form: the numbers are plain text, so every consumer shows them without knowing anything.

- Bad:
  - Every change of a heading would rewrite the text of every heading after it, a lock on each in a live session.
  - Search, mentions and the table of contents would carry the numbers as content; switching off would have to strip them reliably.
  - Numbers would be data in the document that a function of the document can compute (D4).

## Links

- relates to [ADR-0032](0032-prosemirror-rich-text-editor.md) (headings), [ADR-0042](0042-tasks-panels-tables-and-computed-blocks.md) and [ADR-0119](0119-lists-of-figures-and-tables.md) (table of contents), [ADR-0010](0010-real-time-collaboration-with-block-locks.md) (live editing), [ADR-0160](0160-protected-blocks-and-pictures-of-a-page.md) (protected blocks), [ADR-0071](0071-exporting-pages-and-lists.md) (exports), [ADR-0237](0237-embedded-text-alone-and-toc-depth.md) (embeds of their text alone, depth of a table of contents)
