# ADR-0214: Tables with formatted cells, sized columns and rows, and coloured cells

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 that tables can be edited further: cells and borders coloured, columns and rows made wider or narrower by drag and drop, text that overflows a cell wrapped, and text in cells formatted)
- **Open question:** whether a table needs merged cells, a header column or several header rows; whether colours beyond the eight named tones are wanted; whether the Markdown conversions (ADR-0133, ADR-0138) and the import (ADR-0023) should keep the formatting of cells they read; whether a dragged column should take its width from its neighbour instead of widening the table
- **Question checked:** 2026-11

## Context and problem statement

A table is one block whose text holds its cells, rows separated by line breaks and cells by tabs (ADR-0042). ADR-0042 decided that cells are plain text without marks and accepted as a consequence that "table cells have no formatting, links or mentions". The editor showed each cell as a one-line input field: long text did not wrap, columns took the width their text needed, and nothing could be coloured.

People keep overviews, matrices and status tables in pages and asked on 2026-11-12 for four things: cells and borders coloured, columns and rows sized by dragging their edges, text that does not fit a cell wrapped within it, and formatting inside cells.

How does a table carry formatting, sizes and colours without giving up what ADR-0042 kept: one flat block that locks, search, governance, derived relations and the link check read as text?

## Decision drivers

- **The body stays flat** (ADR-0042, ADR-0010): a table remains one block, locked as a whole while one person edits it; search, governance and the link check read its text without change.
- **What is formatted is the text that is searched:** marks lie over the block's text, counted in code points, as in every other block (ADR-0032, D4).
- **Content is sealed** (ADR-0026): sizes and colours travel inside the body, which is sealed with the item's key.
- **The persisted format is a contract** (ADR-0025): new fields raise the codec version; older logs are still read.
- **Pages read alike in a light and a dark theme and in every export:** the PDF export's policy refuses style attributes (`style-src 'self'`), and so does the policy of the UI for markup (ADR-0014, ADR-0031).
- **No new dependency:** ProseMirror (ADR-0032) already edits rich text.

## Considered options

1. **Marks over the table's text and three block fields: column widths, row heights and coloured cells with named tones**
2. **Nested blocks:** a table of rows of cells, each a list of blocks
3. **The same block fields with free colours (hexadecimal) instead of named tones**
4. **Formatting and colours only in the browser, as CSS of the page**

## Decision outcome

Chosen option: **1**, because it keeps the table one block whose text search, governance, the link check and every export already read, and adds only what a table needs to look as its author meant it.

- **Formats in cells:** a table block may carry marks: bold, italic, underline, strike-through, highlight, code and links (ADR-0032, ADR-0059), counted over the block's text in code points like the marks of a paragraph. A mark lies inside one cell: it never covers the tab or line break between two cells. Mentions and protected passages (ADR-0088) are not taken in a cell: a mention is an inline node a cell's editor does not hold, and a protected passage would replace text inside the table's shape; a whole table is protected as a block (ADR-0160). `item.TableCells` splits a table into its cells with their marks, so that every export writes a cell as it writes a paragraph.
- **Column widths and row heights:** `Block.Widths` holds one width per column and `Block.Heights` one height per row (named so beside the field `Columns` of a view, ADR-0203), in CSS pixels, 0 for the size the text takes, or none when every one is 0. A width lies between 24 and 2000 pixels, a height between 16 and 1000: smaller, a column or row could no longer be grasped to make it larger again. A height is the least a row takes; text that does not fit wraps within its cell and makes the row taller.
- **Coloured cells:** `Block.Cells` lists the coloured cells, each with its row and column counted from 0 with the header row, a fill and a border, each one of eight named tones (`item.TableTones`, the tones of a kanban's cards, ADR-0134), in the order of rows and columns, each once. The UI mixes a fill's tone into the theme's surface, so the theme's text stays readable on it in a light and a dark theme; a border is drawn in the tone itself.
- **Validation:** sizes and coloured cells only on tables; one size per column or row or none; sizes within their bounds; coloured cells inside the table, once each, in order, each with a tone; marks of the kinds a cell takes, inside one cell. The API turns sizes that are all 0 and an empty list of cells into none, so a block decoded from the log holds what the live one held.
- **Format:** codec version 59 adds the block fields `Widths`, `Heights` and `Cells` and lets a table carry marks; they are sealed with the body. Versions 1 to 58 are still read: their tables are plain and sized by their text.
- **Editor:** a table shows its cells with their formats; text wraps within a cell. An editor clicks a cell to edit it in place, in a small ProseMirror editor of its own whose schema holds text with the marks a cell takes: Ctrl+B, Ctrl+I, Ctrl+U and the other keys of the body format the selection, Ctrl+K links it, Tab and Shift+Tab move to the next and previous cell, Enter to the cell below, Escape leaves it. Shift and a click choose a rectangle of cells. The table's tools format the chosen cells as a whole, fill them and colour their borders, add a row below or a column right of the chosen cell, remove its row or column, and reset the sizes. A drag of a cell's right edge sizes its column, a drag of its lower edge its row. The first drag of a column fixes every column at the width it is shown with, so the others keep theirs; a table then wider than the text column scrolls sideways (ADR-0116). The live session locks the table while one of its cells has the focus (ADR-0010).
- **Exports (ADR-0071, ADR-0131):** Word writes the formats as runs, the widths as the grid and cell widths with a fixed layout, the heights as least row heights, a fill as cell shading in the tone's soft shade and a border in the tone. The PDF export writes the formats as elements, widths and heights as the `width` and `height` attributes of `col` and `tr`, and the colours as classes of its own stylesheet, since its policy refuses style attributes. Markdown keeps the formats; it has no colours or sizes, so those are left out. CSV writes the text, as before.
- **Agents:** the MCP description of the body names the marks, sizes and coloured cells of a table (ADR-0012).

This amends the sentence of ADR-0042 that cells are plain text without marks, and its consequence that cell text cannot be formatted or linked; everything else of ADR-0042 stands. A table is still locked as a whole.

### Consequences

- **Positive:** tables can be formatted, sized and coloured as people asked, in the page and in Word, PDF and Markdown alike; search, governance and the link check read them as before; a link in a cell is clickable.
- **Positive:** named tones read alike in both themes and in the PDF export, and an author cannot colour a cell so that its text vanishes.
- **Negative / trade-offs accepted:** no free colours; no merged cells, no header column; mentions and protected passages stay out of cells.
- **Negative / trade-offs accepted:** sizes are pixels of the screen; a sized table may be wider than a phone's screen and scrolls sideways there, as every table already does (ADR-0116).
- **Negative / trade-offs accepted:** every data directory written by this version needs it or a newer one (codec version 59).
- **Follow-ups / risks to watch:** pasted Markdown (ADR-0133), Markdown from Git repositories (ADR-0138) and the import (ADR-0023) still write plain cells; whether they should keep a cell's formatting is open. The version comparison (ADR-0043) compares a table's text, not its formats, sizes or colours.

## Pros and cons of the options

### 1. Marks over the table's text and three block fields with named tones

Strongest form: the smallest change that gives a table everything asked for, kept in the one block every part of the system already understands.

- Good: locks, search, governance, the link check and derived relations see the table as before.
- Good: every export already writes marks; a cell is written as a paragraph is.
- Good: named tones work in both themes and under every content security policy.
- Bad: no colour outside the eight tones.

### 2. Nested blocks

Strongest form: a cell holding blocks can hold lists, pictures, several paragraphs and mentions, as rich tables do in word processors; formatting and protection of passages would come for free with the blocks.

- Good: the richest cells.
- Bad: ADR-0042 rejected it for the reasons that still hold: every consumer of a body must learn nesting, a lock on a cell, a table and a page become three different things, and the format changes deeply. What was asked for, formatted text, does not need it.

### 3. Free colours

Strongest form: any colour, as a word processor offers, so a table matches a corporate design or a colour code exactly.

- Good: no limit on colours.
- Bad: a free fill chosen in a light theme can make the text unreadable in a dark one; the PDF export's policy refuses the style attributes a free colour would need; validation could only check the syntax. Named tones can grow by a decision when one is missing.

### 4. Formatting and colours only in the browser

Strongest form: nothing in the model changes; the editor keeps a stylesheet per page.

- Good: no codec change.
- Bad: what is not in the body is lost on the next device, in every export and for agents; it would be shadow data outside the document (D4).

## Links

- amends [ADR-0042](0042-tasks-panels-tables-and-computed-blocks.md) (cells are plain text)
- relates to ADR-0010 (live editing), ADR-0012 (MCP), ADR-0014 (closed renderer), ADR-0025 (persisted format), ADR-0026 (sealed content), ADR-0032 (rich text), ADR-0059 (more marks), ADR-0071 (exports), ADR-0088 (protected passages), ADR-0116 (phones), ADR-0131 (Markdown export), ADR-0134 (kanban tones), ADR-0160 (protected blocks)
