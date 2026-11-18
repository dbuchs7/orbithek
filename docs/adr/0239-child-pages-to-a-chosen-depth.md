# ADR-0239: A list of child pages shows the levels chosen

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-17
- **Deciders:** Project initiators. Request of 2026-11-17: a list of child pages embedded in a text should not show only the first level of child pages; the person editing chooses how deep it goes. The offered depths, the budget and the import of the macro's depth are defaults proposed with the implementation, to be confirmed by the requester. Leaving out types was asked for later and is decided in ADR-0240. Request of 2026-11-18: a type's start text may name a list's depth.
- **Open question:** whether five levels by number are enough before *All levels*
- **Question checked:** 2026-11

## Context and problem statement

ADR-0042 made the list of child pages (`children`) an empty block that the editor fills when it shows it: the item's readable child pages, in the order of the page tree (ADR-0041), one level deep.
A page that heads a handbook, a project or a catalog wants to show what lies below it further down: the chapters and their sections, a project's work packages and their tasks.
One level shows only the first; the reader opens each child to see what is below it, and an export lists only titles of the first level.

How does a list of child pages show more than one level, chosen per list, without content of its own and without revealing what the reader may not read?

## Decision drivers

- **D4:** the list holds no copy of the tree; it is computed for each reader, as before.
- **D10:** nothing it shows, nor how many pages it says are left out, names a page the reader may not read.
- **The format is a contract (ADR-0025):** a body written before keeps meaning what it meant: one level.
- **Bounded work (ADR-0015):** a list of every level of a large tree must not make the server hold or the browser ask without limit.
- **One answer for every reader of a body:** the editor, the reading view, the beside pane and the daily view, the Word, PDF and Markdown exports and agents list the same pages.

## Considered options

1. **A block field `Depth`** on `children`, chosen in the editor, and one request to the page tree's API that answers the levels asked for, within a budget.
2. **The depth as the reader's choice**, a control in the reading view kept in the browser, the block unchanged.
3. **The browser walks the tree itself**, one request per page with children, as the page tree does when one opens its nodes.
4. **A fixed deeper default**, every list showing two levels or all.

## Decision outcome

Chosen option: **1, a block field `Depth`**, because the depth is part of what the author means the text to show, as a heading's level is, and only a field of the block reaches every reader of the body, the exports and agents alike.

- **The field:** `Block.Depth` on a block of the kind `children`: 0 shows one level, 2 to `MaxChildrenDepth` (5) that many, `ChildrenAll` (-1) every level.
  One level is written only as 0, so a body says it one way; the API takes 1 and writes 0.
  Any other value, and a depth on any other kind, is refused.
  Raising the codec to version 65 lets an older binary refuse a log it would read without the depths instead of losing them silently; older versions are still read, their lists showing one level.
- **The levels:** `Session.ChildPages` lists the readable children of a page and theirs to the depth, in sibling order, gathering the space's children once (ADR-0081) and asking whether one may read a page only when the walk reaches it.
  A page below an unreadable one is unreadable itself (D9), so the walk never passes through a page the reader may not read.
- **The bound:** at most the new budget `ChildPages` (500) pages per list, taken level by level, so a cut list keeps its upper levels whole.
  The answer says `truncated`, and a page whose readable children are left out, below the depth or beyond the budget, carries `more`.
  Neither counts nor names what the reader may not read.
  `MaxChildrenDepth` is a format of the block, not a budget, and is classified so.
- **The API:** `GET /api/spaces/{space}/tree` takes `depth` (1 to 5 or `all`); with it, each entry holds its `children`, unpaged within the budget.
  Without it the listing is paged as before (ADR-0041).
  MCP's `get_tree` offers the parameter, and its description of blocks names the field.
- **The editor:** *Insert…* asks for the depth first in a dialog, and the list's title then carries the same choice, *Depth*, both as a table of contents asks for and carries its depth (ADR-0237): *1 level*, *2 levels* to *5 levels*, *All levels*; a change is a change of the block's attributes, saved and shared in the live session (ADR-0010) as every other.
  Readers see the list without the choice, each deeper page indented below its parent, and a line when the list was cut.
- **Exports:** Word, PDF and Markdown list each list to its own depth: nested lists in PDF and Markdown, and a deeper indent per level in Word, whose list numbering keeps a single level.
  The server reads the children once per page, to the depth of its deepest list.
- **Import (ADR-0023):** the children macro brings its `depth` along, and `all` or a depth beyond five becomes every level, so no page the macro listed is left out.
- **Start text (ADR-0167):** the marker line of a list of child pages in a type's start text (`startMarkdown`) may name its depth, `[children:2]` to `[children:5]` or `[children:all]`, and the types it leaves out (ADR-0240), `[children:all without task, decision]`; the keyword and `all` are read in any letter case, the type names as written, sorted and named once each.
  `[children:1]` is one level, written back as `[children]`.
  A start text written from a type's blocks names both, so editing it as Markdown keeps them.
  A depth out of range, or a depth or types on another marker, leaves the line as text, as any other line that is no marker.

### Consequences

- **Positive:** a page shows the tree below it as deep as its author wants, the same in every view, in exports and to agents; bodies written before keep their meaning.
- **Negative / trade-offs accepted:** a list of every level asks the server to walk the space's tree for each reader each time it is shown, bounded by the budget; a body with a depth needs a binary that reads version 65.
  The depth is the author's, not the reader's: one who would rather see less opens the page tree.
- **Follow-ups / risks to watch:** the open question above; a table of contents' level (ADR-0237) has no form in a start text yet; Word lists use indents, not list levels of their own, so a template's list styles apply to the first level only.

ADR-0237 gives a table of contents its depth in the existing field `Level`, where 0 lists every level.
A list of child pages takes a field of its own instead, because its default, kept by every body written before, is one level, not all, and a heading's level counts from the top of the page, not from the page itself.

## Pros and cons of the options

### Option 1: a block field `Depth`

Strongest form: the depth is part of what the text says, kept with the body and versioned with it, read the same by every view, export and agent, and bounded on the server where the tree is.

- Good: one meaning for every reader; exports and agents see what the page shows; older bodies unchanged.
- Bad: a change of the persisted format and a new budget.

### Option 2: the depth as the reader's choice

Strongest form: no change of the format at all; each reader sees as much as they like, and the author need not foresee it.

- Good: nothing persisted, nothing for exports to learn.
- Bad: the author cannot make a page show its structure; exports and agents would still list one level, so the page and its document differ; a choice kept per browser is lost on another device.

### Option 3: the browser walks the tree itself

Strongest form: the page tree's API already answers one level with whether each page has children; the browser could ask level by level, and the server would need nothing new.

- Good: no new route parameter.
- Bad: one request per page with children, each scanning the space (ADR-0081), and no bound but the browser's patience; exports, which run on the server, would still need the walk there.

### Option 4: a fixed deeper default

Strongest form: no choice to make and no field: lists simply show more.

- Good: simplest.
- Bad: changes what every existing page shows; a page above a large tree becomes a long list its author did not want; no way back to one level.

## Links

- extends ADR-0042 (the list of child pages as a computed block)
- relates to ADR-0237 (a table of contents' depth), ADR-0041 (page tree and sibling order), ADR-0081 (listing without copying content), ADR-0015 (budgets), ADR-0025 (persisted format), ADR-0023 (import), ADR-0012 (MCP)
