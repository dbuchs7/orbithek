# ADR-DRAFT: Radar and waterfall charts are drawn in an imported presentation

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-16
- **Deciders:** Project initiators (request of 2026-11-16 that radar and waterfall charts be drawn as well, answering part of the open question of ADR-0186)
- **Open question:** whether bubble, stock and surface charts and the other newer kinds (treemap, sunburst, funnel, histogram, box and whisker) should be drawn too; whether a waterfall should show the legend the presentation program draws for it (rise, fall, total) in the language of the album
- **Question checked:** 2026-11

## Context and problem statement

ADR-0186 imports a PowerPoint presentation into the album by drawing its slides in the browser. Its charts are drawn from the values the chart parts keep (`assets/pptxchart.js`): columns, bars, lines, areas, pies, doughnuts and scatter charts. Every other chart kind appears as a grey box, and ADR-0186 left open which further kinds should be drawn. The requester asked for radar (spider) charts and waterfall charts.

The two are stored differently:

- A **radar chart** is a classic chart part (`c:chartSpace` with `c:radarChart`): categories, series names and numbers are cached as for the kinds already drawn, with a style that marks the points (`marker`), only joins them (`standard`) or fills the area (`filled`).
- A **waterfall chart** is one of the newer kinds PowerPoint stores in a separate part (`cx:chartSpace`, "chartex", since Office 2016). On the slide it sits in markup compatibility content (`mc:AlternateContent`): the choice holds the chart frame, the fallback a shape with a notice for programs that cannot show it. The part keeps its data by data ID (`cx:chartData`: a dimension of categories, one of values), and a series names its layout (`layoutId="waterfall"`) with the points that are subtotals. ADR-0186's renderer takes the fallback, so it shows the notice, not the chart.

How are the two drawn, within the constraints of ADR-0186 — no dependency, nothing new on the server, and honest where the drawing differs?

## Decision drivers

- **Same model, same drawing.** A new kind should reuse the chart model, scales, number formats, title, labels and colours that the existing kinds use, so that it behaves like them and is tested the same way, in Node.
- **The values are there.** Both kinds keep every number they show in the presentation file; nothing has to be computed that the file does not say, except the running total of a waterfall, which follows from its points and subtotals.
- **No words the presentation does not hold.** The album draws what the presentation keeps; captions the presentation program adds in its own language are not in the file.

## Considered options

1. **Draw both from the values they keep, through the existing chart model** — radar charts read like the other classic kinds, waterfall charts read from the chartex part into the same model, the chartex choice taken over its fallback.
2. **Draw the radar chart only**, leaving the newer kinds to the exported pictures.
3. **Keep both as grey boxes** and point to the exported pictures, as ADR-0186 does.

## Decision outcome

Chosen option: **1**, because both kinds keep their values in the file in a form close to the kinds already drawn, and the newer kinds are where a grey box was most often seen: PowerPoint offers waterfall charts in its main chart menu.

### How it works

- **Radar charts** (`c:radarChart`) are read as a plot of kind `radar` with their style. They are drawn around a centre with one spoke per category, the first straight up and the rest clockwise; the web is drawn at the value axis's round steps (`niceScale`, with the bounds and step the chart sets), its step labels beside the first spoke. Each series is a closed polygon in its own colour or the theme's next accent: filled with three quarters of its opacity for the style `filled`, otherwise a line with markers where the style or the series asks for them. Category labels stand outside the spokes; labels are drawn after the series, so a filled series does not hide them. The legend shows a line per series, a box when the radar is filled.
- **Waterfall charts** (`cx:chartSpace`, series with `layoutId="waterfall"`) are read by `readChartEx`: categories and values from the data the series names, the subtotals from its layout properties, data labels and their visibility, connector lines, the gap between bars, the title and the value axis's bounds, step and gridlines. Each point is a bar from the running total to the total after it; a subtotal is a bar from zero to its own value, and the running total continues from it (`waterfallSteps`). Rises take the theme's first accent, falls the second and subtotals the third, as the presentation program colours them; a point's own fill wins. Connector lines join each bar's end to the next bar, labels stand above rises and below falls. The value axis always includes zero.
- **The slide's markup compatibility content** takes its choice when that holds a chartex frame, and its fallback otherwise, as before.
- **The legend of a waterfall is left out:** its entries (*Increase*, *Decrease*, *Total*) are words of the presentation program in its language, not text of the presentation.
- Charts of other layouts in a chartex part (treemap, sunburst, funnel, histogram, Pareto, box and whisker, region map), and bubble, stock and surface charts, remain grey boxes; the import still says how many slides hold what it could not draw, and the exported pictures still give exact slides.

### Consequences

- **Positive:** the two kinds asked for appear as charts rather than grey boxes; the chartex reading is a base the other newer kinds can follow; no new dependency, route or server change.
- **Negative / trade-offs accepted:** the layout of radar and waterfall charts follows rules of our own, as the other kinds do (ADR-0186); the waterfall's legend is missing; the chartex structure is read from the format's description and from files written for the tests, not yet from presentations saved by PowerPoint.
- **Follow-ups / risks to watch:** how real presentations' chartex parts differ from the test files (other data ID references, formulas instead of cached levels, several series); whether people miss the waterfall's legend.

## Pros and cons of the options

### Option 1: draw both through the existing chart model

Strongest form: both kinds keep their numbers in the file; one more plot kind and one reader for the chartex part reuse scales, formats, labels and colours that are already tested, and the chartex reader opens the way for the other newer kinds.

- Good: what was asked; small additions to existing modules; tested in Node like the other kinds.
- Bad: a second chart format to keep up with; its structure is less documented than the classic one.

### Option 2: draw the radar chart only

Strongest form: a radar chart is a classic chart part, so reading it is a few lines and certain; the chartex format is newer and less documented, and waiting for real files avoids reading it wrongly.

- Good: smallest change; no new format.
- Bad: leaves the waterfall, the more common of the two in business presentations, as a grey box, though the request named it.

### Option 3: keep both as grey boxes

Strongest form: ADR-0186 already says what it cannot draw and offers the exported pictures as the exact way; every kind drawn is more code that approximates.

- Good: no code.
- Bad: the request is not met; the import stays two steps for every deck with such a chart.

## Links

- amends ADR-0186 (a presentation is imported into the album as slides with their notes)
