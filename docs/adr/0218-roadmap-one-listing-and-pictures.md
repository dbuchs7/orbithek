# ADR-0218: The roadmap and the Gantt chart read their items in one listing and export as PNG and JPEG

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12: the roadmap and the Gantt chart build up slowly, their loading time is to be shortened, and both are to be exported as PNG or JPEG). Accepted 2026-11-12.
- **Open question:** whether the picture should always be drawn in the light look, whatever look the reader uses, and whether a chart too large for one picture should be split into several pages instead of being refused
- **Question checked:** 2026-11

## Context and problem statement

The roadmap and the Gantt chart (ADR-0164) are computed in the browser from the item listing.
Measured on a room of 1500 tasks with dates and 3000 pages without, the view took about 430 ms to its first paint on the same machine, and about 1400 ms with 40 ms of network latency per request.
Three causes add up:

- **Requests in a row.** The view read the types, rooms and labels first, and only then the items, in pages of 100, one after the other: 20 or more round trips for a room of a few thousand items.
- **Items it does not show.** Without a type in the address it read every item, pages without dates included, and dropped them in the browser. Those items also counted against the boards' limit of 2000 (`boardMax`), so in a large room tasks could be cut off by pages that never appear on the chart.
- **Dates written slowly.** Each bar's accessible name writes its dates with `toLocaleDateString` and options, which builds a new date format per call; for 1800 bars this took 200–280 ms of the main thread, more than everything else the view draws.

Changing the layout, the scale or the date fields also read every item again, although only the drawing changes.

People also want the chart outside the wiki, on a slide or in a document, as the album's pictures can be exported (ADR-0173).
How does the view load faster without a stored object or a server route of its own, and how does the chart become a picture?

## Decision drivers

- **ADR-0164 unchanged in substance:** nothing is stored; the view's state is its address; a link shows another reader only what that reader may read (D10).
- **One binary, no new dependency (ADR-0009, ADR-0024),** and the UI's content security policy unchanged (ADR-0014, ADR-0031).
- **The picture holds what the chart holds:** every row and the whole frame, not only what is scrolled into sight.
- **It works in every browser the UI supports,** and fails with a clear note rather than an empty file.

## Considered options

Loading:

1. **One listing through the query language (ADR-0136), read as soon as the types are known, pages of 500, kept in the tab while one stays in the view.**
2. **A server route that computes bars and lanes.**
3. **Only larger pages,** the reads otherwise unchanged.

Picture:

A. **Draw the chart's model on a canvas in the browser** (PNG and JPEG through `canvas.toBlob`).
B. **Copy the page's elements into an SVG `foreignObject`** and rasterise that.
C. **Render the picture on the server,** in Go.

## Decision outcome

Chosen options: **1** and **A**.

**Loading.**

- The items are read with `query=type in (…) AND label in (…)`: the types that have a date field (or, when the address names a type, that type through the `type` parameter), and any of the chosen labels.
  Every value is quoted, so a name with spaces, commas or quotes stays one value.
  Pages without dates are no longer read and no longer count against the boards' limit.
- The listing is read in pages of 500, the default largest page (`OT_BUDGET_PAGE_ITEMS`).
- The rooms and labels are read while the types are read; the items as soon as the types are known.
- Where the server refuses the query with 400 — a deployment that lowered `OT_BUDGET_QUERY_TERMS` or `OT_BUDGET_QUERY_BYTES` below what the types and labels need — the view reads as before, per label and every type.
  A deployment that lowered `OT_BUDGET_PAGE_ITEMS` below 500 is not served by this fallback; the view then reports the error as other views do.
- What the view read is kept in the tab for at most five minutes, and only while the address stays in the view: changing the layout, the scale or the fields draws again without a request; leaving the view forgets it, so coming back shows what changed meanwhile.
  This is a cache of reads already permitted, not stored state; a reload starts empty.
- Dates are written with one `Intl.DateTimeFormat` per language and format.
- Rows out of sight are neither laid out nor painted until they scroll near (`content-visibility: auto`); a row's height is fixed, so the scroll bar stays true.
  The lines of the units are placed before the rows so that rows, now each a stacking context, are drawn over them.

Measured on the same data and machine after the change: about 105 ms to the first paint instead of 430 ms, and about 360 ms instead of 1400 ms with 40 ms of latency; changing the scale takes about 100 ms instead of 420 ms.

**Picture.**

- The view offers **Export** with *PNG* and *JPEG* above the chart.
- The picture is drawn from the same model the chart is drawn from (`chartModel` in `roadmap.js`): a title naming the layout, room, type and labels, the day of the export, the axis with its captions, the heads (a label as a chip, the lane without a label in italics, an item by key and title), the bars and milestones with the chart's marks (hatched when finished, dashed when waiting or inverted) and the line of today.
  Its colours are the look's tokens as the browser computes them, so the picture looks as the chart does, in the light or the dark look.
- Each unit of the axis is as wide as the chart's least width for its scale; the picture is drawn twice as sharp as the page where the canvas fits.
  Browsers refuse a canvas beyond about 16384 pixels a side or 125 million pixels; the picture is scaled down to stay within 16384 a side and 120 million pixels, and halved again where a browser still refuses it, but never below half the page's sharpness.
  Beyond that the export is refused with a note to narrow the view.
- JPEG is written with the album's quality of 0.92; both formats are opaque, on the look's surface colour.
- The file is named after the view and the day, such as `Gantt chart PLAT release 2026-11-12.png`, with the album's rules for characters file systems refuse.
- No route, no permission, nothing persisted and no MCP tool is added: the picture holds exactly the items the reader's chart holds.
- Code: `assets/roadmappic.js`; its decisions (layout in pixels, scale, file name) are tested in Node (`jstest/roadmappic.test.mjs`), the query in `jstest/timeline.test.mjs`, the drawing in `e2e/smoke.cjs`.

### Consequences

- **Positive:** a few requests instead of dozens; a large room no longer loses tasks to the boards' limit because of pages without dates.
- **Positive:** the picture has every row and the whole frame, is the same in every browser and needs no library.
- **Negative / trade-offs accepted:** the drawing of the chart exists twice, as elements and as canvas commands; a change to the chart's look has to be made in both.
- **Negative / trade-offs accepted:** for up to five minutes in the view, the chart can show an item as it was when the view was entered; leaving the view or reloading shows the current state.
- **Negative / trade-offs accepted:** the picture is a raster image; text in it cannot be selected, and the links of the bars are lost.
- **Negative / trade-offs accepted:** a Gantt chart of many hundreds of rows does not fit in one readable picture and is refused.
- **Follow-ups / risks to watch:** an SVG export drawn from the same layout; reading the page size from the server's budgets instead of assuming 500.

## Pros and cons of the options

### Option 2: a server route that computes bars and lanes

Strongest form: the server filters, schedules and packs in one request; the browser receives only what it draws, and an agent could use the same route.

- Good: one request, whatever the number of items.
- Bad: the timeline's logic would exist in Go and in JavaScript, or move to the server and leave ADR-0164's decision of a view computed in the browser.
- Bad: the existing listing with the query language already narrows to the items shown; the remaining cost is a few pages.

### Option 3: only larger pages

Strongest form: the smallest change, one number.

- Bad: pages without dates would still be read and still count against the boards' limit.
- Bad: the dates would still be written slowly, which cost more than the requests on a fast network.

### Option B: copying the page into an SVG `foreignObject`

Strongest form: the picture is the page itself, with every style, without drawing the chart a second time.

- Bad: every computed style has to be copied inline onto thousands of elements, which is slower than the drawing it replaces.
- Bad: some browsers taint a canvas that drew a `foreignObject`, and then refuse to write it as a file.
- Bad: the sticky head column and the scroll container would have to be undone for the picture to hold every row.

### Option C: rendering on the server

Strongest form: deterministic, testable in Go, available to agents.

- Bad: text layout and fonts in Go need a new dependency, and the look's colours and the reader's language live in the browser.
- Bad: the server would rebuild a view that ADR-0164 computes in the browser.

## Links

- relates to ADR-0164 (the roadmap and the Gantt chart), ADR-0136 (the query language), ADR-0173 (picture export of the album), ADR-0015 (budgets)
