# ADR-0238: Text colour and size in table cells

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-17
- **Deciders:** Project initiators (request of 2026-11-17 that the text of table cells can take a font size and a font colour, and of 2026-11-18 that paragraphs can too, and then headings, lists, quotes and panels); proposed for the maintainer's acceptance
- **Open question:** whether a font family is wanted; whether more sizes or tones are wanted; whether the colours and sizes of a table pasted from elsewhere should be taken to the nearest tone and size; whether yellow text, which reads weakly on white, should be offered at all
- **Question checked:** 2026-11

## Context and problem statement

A table's cells carry formats as marks over the block's text: bold, italic, underline, strike-through, highlight, code and links (ADR-0214).
ADR-0214 rejected free colours and named eight tones instead, because a free colour chosen in a light theme can vanish in a dark one and because the export's content security policy refuses style attributes.
It fills and borders cells in those tones; the text itself has the theme's colour and the text's size.

People asked on 2026-11-17 that the text of cells can take a font size and a font colour, as in a spreadsheet.
How does a cell's text carry a colour and a size without free values, and without letting them into the rest of the body?

## Decision drivers

- **The table stays one flat block** whose text search, governance and the link check read (ADR-0042, ADR-0214).
- **Pages read alike in a light and a dark theme and in every export**, under policies that refuse style attributes (ADR-0014, ADR-0031).
- **Only what a model validates is stored** (D3): a value that cannot be checked would let anything into the log.
- **The persisted format is a contract** (ADR-0025): a new field raises the codec version.
- **The request is about cells:** what the rest of the body takes is not asked.

## Considered options

1. **Two marks with a named value: `color` with one of the eight tones, `size` with one of three named sizes; in tables only**
2. **The same marks with free values:** a colour as hexadecimal, a size in points
3. **The same marks for every block of the body**
4. **Colour and size as properties of a whole cell**, beside fill and border in `Block.Cells`

## Decision outcome

Chosen option: **1**, because it gives cells a colour and a size of their text with values every theme, export and validation can hold, and keeps the rest of the body as it is.

- **Marks:** `Mark.Value` holds a mark's value. A mark `color` colours its text in one of `item.TableTones`; a mark `size` sizes it as one of `item.TextSizes`: `small`, `large` or `larger`, the text's own size being no mark. Both lie inside one cell as every mark of a table does (ADR-0214), may overlap each other and the other formats, and a stretch of text has at most one of each where the editor writes them; the model accepts no value on another mark.
- **In table cells and every block of text but code:** paragraphs, headings, list items, tasks, quotes and panels take both marks, a code block, which takes no format, does not, and any other block refuses them. The body's toolbar offers **Text colour** and **Size** where the cursor stands in such a block: on the selected text, or, at the cursor, on what is typed next. Codec version 68 lets a paragraph carry them, codec version 69 a heading, a list item, a task, a quote and a panel (requests of 2026-11-18).
- **Format:** codec version 64 adds `Mark.Value`, sealed with the body; versions 1 to 63 are still read, their cells' text in the text's own colour and size.
- **API and agents:** a mark carries `value`; the MCP description of the body names the two marks (ADR-0012).
- **Editor:** the table's tools gain **Text colour** and **Size**. They act on the selected text of the cell being edited, on all of its text when nothing is selected, or on all of each chosen cell (ADR-0236); the selection of a cell's text is kept as a list takes the focus. The UI mixes a tone into the theme's text colour, so it stays readable in a light and a dark theme; a size is a class, relative to the text around.
- **Copy and paste (ADR-0236):** colour and size travel with a copied table and copied cells, as classes of their own and in the block a copy carries; a table from elsewhere brings neither.
- **Exports (ADR-0071, ADR-0131):** Word writes a colour as the tone's strong shade and a size in points (9, 14 and 18 beside the body's 11); the PDF export writes classes of its own stylesheet; Markdown has neither and leaves them out.

### Consequences

- **Positive:** a cell's text is coloured and sized as asked, alike in both themes, in Word and in PDF; validation knows every value.
- **Negative / trade-offs accepted:** no colour outside the eight tones and no size outside three; yellow text reads weakly on a light surface.
- **Negative / trade-offs accepted:** every data directory written by this version needs it or a newer one (codec version 64).
- **Follow-ups / risks to watch:** the open questions above; the version comparison (ADR-0043) still compares a table's text only.

## Pros and cons of the options

### 1. Two marks with named values, in tables only

Strongest form: the same reasoning that gave cells eight tones for fill and border (ADR-0214), applied to their text.

- Good: one validated list of values each; works under every policy and in both themes.
- Good: nothing changes for the other blocks.
- Bad: a colour or a size that is not among them cannot be had.

### 2. Free values

Strongest form: any colour and any size, as a word processor offers, so a table matches a corporate design exactly.

- Good: no limit.
- Bad: the reasons ADR-0214 gave against free colours hold for text even more, since text on the theme's own surface vanishes sooner than a fill; a size in points means nothing to a screen's text of its own size; the export's policy refuses the style attributes free values need.

### 3. The marks for every block

Strongest form: a heading or a warning in colour is wanted as often as a cell, and one rule for all blocks is simpler to explain.

- Good: one rule.
- Bad: every block kind and the Markdown conversions would change at once, and a page's look would drift from its headings and panels, which already carry meaning by their kind. Paragraphs, headings, lists, quotes and panels were asked for on 2026-11-18, one after the other, and take them: in the end the option was chosen block by block, as people asked for it.

### 4. Colour and size of a whole cell

Strongest form: a spreadsheet formats cells, not stretches of text; beside fill and border, a cell's text colour and size would be two more fields of `Block.Cells`.

- Good: no new mark kind.
- Bad: one word of a cell could not be coloured; a cell's look would be split between marks and cell fields, and copying a part of a cell's text would lose it.

## Links

- relates to ADR-0012 (MCP), ADR-0014 (closed renderer), ADR-0025 (persisted format), ADR-0042 (tables), ADR-0071 (exports), ADR-0131 (Markdown export), ADR-0214 (tables with formatted cells, sizes and colours), ADR-0236 (tables copied and pasted, chosen cells)
