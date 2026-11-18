# ADR-0236: A table is copied and pasted with its formats

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-17
- **Deciders:** Project initiators (request of 2026-11-17 that tables in pages can be copied and pasted again in Orbithek with the same formatting, and, the same day, that chosen cells can be copied and that several cells, columns or rows can be formatted at once: font, border and fill); proposed for the maintainer's acceptance
- **Open question:** whether the colours of a table from elsewhere should be taken to the nearest of the eight tones (ADR-0214); whether stylesheet rules other than single classes should be read; whether one pasted cell should fill every chosen cell, as some spreadsheets do; whether several separate ranges of cells should be chosen at once
- **Question checked:** 2026-11

## Context and problem statement

A table is one block whose text holds its cells, with marks over that text and the block fields `Widths`, `Heights` and `Cells` (ADR-0042, ADR-0214).
In the editor it is an atom node drawn by a node view.
What a copy carried was the node's DOM output: a `pre` element with the table's text.
Pasted back, the editor's parser took that `pre` for a code block, so a copied table came back as a code block with tabs: its formats, sizes, colours and its very kind were lost.
A page being read is copied by the browser, not by the editor, and carried the drawn table without the block.
A table copied from a spreadsheet, a word processor or a web page was split into loose paragraphs.

People asked on 2026-11-17 that a table copied in a page can be pasted again in Orbithek formatted as it was.
How does a copied table carry everything the block holds, and what does a table pasted from elsewhere become, without weakening what the editor admits (ADR-0014)?

## Decision drivers

- **A copy is the block:** text, marks, column widths, row heights and coloured cells come back unchanged, whether the table was copied while reading or while editing.
- **Pasted content is untrusted:** links keep only the targets the model allows (ADR-0014), marks only the kinds a cell takes, sizes and colours only what the model accepts (ADR-0214); the server validates the body again (D3).
- **Other programs read the copy:** a word processor, a spreadsheet or a mail program should receive a table, not preformatted text.
- **A part is a part:** a selection over some rows of a table must not paste the whole table.
- **No new dependency and no codec change:** the body's format stays as ADR-0214 set it.

## Considered options

1. **An HTML table that carries the block in a data attribute, taken only when its cells show the same text; any other table read from its cells**
2. **A clipboard type of its own beside the HTML**
3. **The `pre` element kept, with a parse rule of its own**
4. **Pasted tables converted to Markdown and read by the Markdown conversion (ADR-0133)**

## Decision outcome

Chosen option: **1**, because it carries the whole block where Orbithek reads the copy, gives every other program a real table, and reads tables from elsewhere through the same checked path.

- **Copy:** a table block's DOM output is an HTML table: the header row as `th` in `thead`, the formats as elements (`strong`, `em`, `u`, `s`, `mark`, `code`, `a` with a target the model allows), the widths as `col width`, and `data-pm-table` holding the block's text, marks, widths, heights and coloured cells as JSON (`tableDOM`, `tableData` in `assets/tableparts.js`).
  The table the node view draws carries the same attribute, so the browser's own copy of a page being read holds the block too.
  As plain text a table is its cells separated by tabs and line breaks (`leafText`), as spreadsheets take it.
- **Paste:** the editor's schema reads every pasted `table` element into a table block (`tableFromDOM`).
  It first reads the cells: their text with white space collapsed as a browser shows it, paragraphs and line breaks as spaces; the formats their tags, inline styles and the single-class rules of a pasted stylesheet state (`classRules`), since spreadsheets copy a cell's formats as classes; merged cells split into a cell and empty places; short rows filled up; at most 100 columns; the widths of its column groups in pixels.
  When the table carries `data-pm-table` and the cells it holds show the same text as the cells read, the carried block is taken whole, checked as the cells are; otherwise the cells read are taken, so a part of a table, or a table another program changed, pastes as what was copied.
- **Chosen cells:** the cells chosen in a table (ADR-0214) are copied as a table of their own — their text, formats, the widths of their columns, the heights of their rows and their colours — as HTML with `data-pm-table` and as text separated by tabs (`subTable`, `specHTML`); cut copies them and empties their text, Delete empties it, their colours staying.
  A paste while cells are chosen, or of several cells into a cell being edited, writes the cells from the chosen or edited cell on: their text and formats replace what is there, a pasted cell's fill and border replace the cell's where it has them, and the table grows by the rows and columns the cells need, at most 100 columns, new columns taking the pasted widths (`pasteCells`).
  Text from a spreadsheet is read as cells separated by tabs and rows by line breaks, a cell in double quotes holding tabs, line breaks and doubled quotes (`gridFromText`).
  While cells are chosen the table holds the focus and no text of the page is selected, so the browser sends copy, cut and paste to it; the editor's history takes each change, so Ctrl+Z undoes a paste.
- **Choosing many cells:** a drag of the mouse across cells chooses the rectangle between them; **Row**, **Column** and **All** among the table's tools widen the choice to whole rows, whole columns or every cell, and a click with Shift then keeps them whole. The keyboard chooses as the mouse does: Shift and an arrow at the edge of a cell's text leave the cell and choose the rectangle up to the cell beside, Escape leaves the cell with it chosen and a second Escape the table, the arrows then choose the cell beside and with Shift widen the choice, Shift+Space chooses whole rows, Ctrl+Space whole columns, Ctrl+A every cell, and Enter or F2 edits the last chosen cell (`stepCells`, `wholeOf`). A polite live region says each new choice to a screen reader — one cell with its row, column and text, more cells by their number and corners, whole rows, columns or all by their numbers (`chosenSaid`) — and the table keeps its role as a table: a grid role with `aria-selected` would say the same, but would change what every reader hears of every table in a page. The chosen cells stay chosen after a format, a fill or a border is applied, so several apply one after the other; the formats of ADR-0214 (bold, italic, underline, strike-through, highlight, code), the fill and the border then act on all of them.
- **What stays out:** colours and row heights of a table from elsewhere, a protection of the block (ADR-0160, as for every other block a copy carries), a header column and merged cells (ADR-0214), font families, sizes and colours, which the model does not hold.

This amends nothing of ADR-0214: the block, its validation and its format stay as they are; choosing cells is widened, not changed.

### Consequences

- **Positive:** a table copied in a page, read or edited, pastes back with its formats, sizes and colours; tables from spreadsheets, word processors and web pages become tables with their formats and column widths; other programs receive a copied table as a table.
- **Positive:** chosen cells move between tables and spreadsheets as cells, and whole rows and columns are formatted and coloured in a few clicks.
- **Negative / trade-offs accepted:** the clipboard holds a table's text twice, once in its cells and once in `data-pm-table`; the drawn table carries the attribute in the page as well.
- **Negative / trade-offs accepted:** the browser's copy of a page carries inline styles of its own, which the UI's content security policy refuses to apply while the paste is read; the browser reports each refusal in its console.
  This held for every paste of styled text before and is a sign the policy works.
- **Negative / trade-offs accepted:** a layout table of a web page also becomes a table block.
- **Follow-ups / risks to watch:** the open questions above; the version comparison (ADR-0043) still compares a table's text only.

## Pros and cons of the options

### 1. An HTML table that carries the block

Strongest form: one representation serves Orbithek, which reads the block, and every other program, which reads the table; the check against the cells keeps a partial copy partial.

- Good: lossless between pages, read or edited.
- Good: other programs and tables from elsewhere go through standard HTML.
- Bad: the block travels twice in the clipboard.

### 2. A clipboard type of its own

Strongest form: the block travels once, in a type no other program misreads, and the HTML stays as clean as other programs expect.

- Good: no data attribute in the HTML.
- Bad: the editor's copy writes only `text/html` and `text/plain`; a type of its own needs the asynchronous clipboard interface with web custom formats, which not every browser offers, and the browser's own copy of a page being read cannot carry it without taking over every copy on the page.
- Bad: tables from elsewhere still need option 1's reading of cells.

### 3. The `pre` element with a parse rule of its own

Strongest form: the smallest change; a rule for `pre.pm-table` before the code block's rule, with the block in attributes.

- Good: a few lines.
- Bad: other programs receive preformatted text, not a table; the browser's copy of a page being read still carries the drawn table, which this rule does not read; tables from elsewhere stay loose paragraphs.

### 4. Pasted tables converted to Markdown

Strongest form: the Markdown conversion already makes tables whose cells keep their formats (ADR-0133, ADR-0214); one converter would serve every source.

- Good: one path for tables written as text and tables pasted as HTML.
- Bad: an HTML-to-Markdown converter is a second conversion that loses what Markdown cannot hold — widths, heights, colours — so a copy within Orbithek would still lose them.

## Links

- relates to ADR-0014 (closed renderer), ADR-0032 (rich text), ADR-0042 (tables), ADR-0133 (Markdown pasted), ADR-0160 (protected blocks), ADR-0214 (tables with formatted cells, sizes and colours)
