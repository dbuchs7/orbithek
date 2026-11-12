// Unit tests of drawing a presentation's charts from the values it keeps
// (pptxchart.js, the chart parts of pptxdraw.js, ADR-0186): reading a chart
// part's series, colours, labels, title and axes; round axis scales; the
// range values span, stacked or not; number formats; stacking; and where
// title, legend and plot go. They run in Node.js without a browser and
// without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// Drawing onto a canvas is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { parseXML } from "../static/assets/pptx.js";
import { colorOf, fillOf, lineOf, readTheme } from "../static/assets/pptxscene.js";
import { readChart, niceScale, valueRange, formatNumber } from "../static/assets/pptxchart.js";
import { chartLayout, legendEntries, stackOf } from "../static/assets/pptxdraw.js";

const NS = `xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`;
const theme = readTheme(parseXML(`<a:theme ${NS}><a:themeElements><a:clrScheme name="t">
  <a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
  <a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3>
</a:clrScheme><a:fontScheme name="f"><a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/></a:minorFont></a:fontScheme></a:themeElements></a:theme>`));
const ctx = { theme, clrMap: { tx1: "dk1", bg1: "lt1" } };
const helpers = { colorOf, fillOf, lineOf };
const rgb = (c) => [c.r, c.g, c.b].map((v) => Math.round(v * 255));

const strCache = (vals) => `<c:strRef><c:f>x</c:f><c:strCache><c:ptCount val="${vals.length}"/>${vals.map((v, i) => `<c:pt idx="${i}"><c:v>${v}</c:v></c:pt>`).join("")}</c:strCache></c:strRef>`;
const numCache = (vals, fmt = "General") => `<c:numRef><c:f>x</c:f><c:numCache><c:formatCode>${fmt}</c:formatCode><c:ptCount val="${vals.length}"/>${vals.map((v, i) => (v === null ? "" : `<c:pt idx="${i}"><c:v>${v}</c:v></c:pt>`)).join("")}</c:numCache></c:numRef>`;
const ser = (i, name, cats, vals, more = "") => `<c:ser><c:idx val="${i}"/><c:order val="${i}"/><c:tx>${strCache([name])}</c:tx>${more}<c:cat>${strCache(cats)}</c:cat><c:val>${numCache(vals)}</c:val></c:ser>`;
const space = (plot, chartMore = "", axes = "", after = "") => parseXML(`<c:chartSpace ${NS}><c:chart>${chartMore}<c:plotArea>${plot}${axes}</c:plotArea>${after}</c:chart>
  <c:txPr><a:bodyPr/><a:p><a:pPr><a:defRPr sz="1400"/></a:pPr></a:p></c:txPr></c:chartSpace>`);

test("a column chart's series, values, colours, title, legend and axes are read from its cached values", () => {
  const doc = space(`<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>
      ${ser(0, "Umsatz", ["Q1", "Q2", "Q3"], [10.5, 14, null])}
      ${ser(1, "Kosten", ["Q1", "Q2", "Q3"], [8, 9.5, 11], `<c:spPr><a:solidFill><a:srgbClr val="00B050"/></a:solidFill></c:spPr>`)}
      <c:gapWidth val="80"/><c:axId val="1"/><c:axId val="2"/></c:barChart>`,
  `<c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>Umsatz und Kosten</a:t></a:r></a:p></c:rich></c:tx></c:title>`,
  `<c:catAx><c:axId val="1"/><c:axPos val="b"/></c:catAx><c:valAx><c:axId val="2"/><c:scaling><c:max val="30"/></c:scaling><c:axPos val="l"/><c:majorGridlines/><c:numFmt formatCode="#,##0" sourceLinked="0"/></c:valAx>`);
  const ch = readChart(doc, ctx, helpers);
  assert.equal(ch.title.text, "Umsatz und Kosten");
  assert.equal(ch.title.size, 14 * 1.4);
  assert.equal(ch.font.size, 14);
  assert.equal(ch.font.font, "Calibri");
  assert.equal(ch.legend, null, "no legend element, no legend");
  const [p] = ch.plots;
  assert.deepEqual([p.kind, p.dir, p.grouping, p.gap, p.overlap], ["bar", "col", "clustered", 80, 0]);
  assert.deepEqual(p.series.map((s) => s.name), ["Umsatz", "Kosten"]);
  assert.deepEqual(p.series[0].categories, ["Q1", "Q2", "Q3"]);
  assert.deepEqual(p.series[0].values, [10.5, 14, null], "a missing point stays a gap");
  assert.deepEqual(rgb(p.series[0].color), [68, 114, 196], "the first accent");
  assert.deepEqual(rgb(p.series[1].color), [0, 176, 80], "its own fill");
  assert.equal(ch.valAx.max, 30);
  assert.equal(ch.valAx.gridlines, true);
  assert.equal(ch.valAx.format, "#,##0");
  assert.equal(ch.catAx.pos, "b");
});

test("a pie's slices take the accents in turn, a point its own colour; a single series names the chart", () => {
  const pie = `<c:pieChart><c:varyColors val="1"/>${ser(0, "Anteil", ["CH", "DE", "AT"], [0.5, 0.3, 0.2],
    `<c:dPt><c:idx val="2"/><c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></c:spPr></c:dPt>`)}
    <c:dLbls><c:numFmt formatCode="0%" sourceLinked="0"/><c:dLblPos val="outEnd"/><c:showVal val="1"/></c:dLbls><c:firstSliceAng val="90"/></c:pieChart>`;
  const ch = readChart(space(pie, "", "", "<c:legend><c:legendPos/></c:legend>"), ctx, helpers);
  assert.equal(ch.title.text, "Anteil", "a single series names a chart without a title");
  assert.equal(ch.legend.pos, "r", "a legend without a position stands right");
  const [p] = ch.plots;
  assert.equal(p.kind, "pie");
  assert.equal(p.firstAngle, 90);
  assert.deepEqual(p.series[0].colors.map(rgb), [[68, 114, 196], [237, 125, 49], [255, 0, 0]]);
  assert.deepEqual(p.series[0].labels, { value: true, percent: false, category: false, series: false, format: "0%", pos: "outEnd" });
  assert.deepEqual(legendEntries(ch).map((e) => e.name), ["CH", "DE", "AT"], "a varying pie's legend lists its categories");
  assert.equal(readChart(space(pie, '<c:autoTitleDeleted val="1"/>'), ctx, helpers).title, null, "a deleted title stays away");
});

test("line, stacked area and scatter plots are read; kinds that are not read make no chart", () => {
  const line = readChart(space(`<c:lineChart><c:grouping val="standard"/>${ser(0, "A", ["x", "y"], [1, 2], `<c:marker><c:symbol val="none"/></c:marker>`)}<c:marker val="1"/></c:lineChart>`), ctx, helpers);
  assert.equal(line.plots[0].series[0].marker, false);
  assert.equal(line.plots[0].markers, true);
  const area = readChart(space(`<c:area3DChart><c:grouping val="stacked"/>${ser(0, "A", ["x"], [1])}${ser(1, "B", ["x"], [2])}</c:area3DChart>`), ctx, helpers);
  assert.deepEqual([area.plots[0].kind, area.plots[0].grouping], ["area", "stacked"]);
  const xy = readChart(space(`<c:scatterChart><c:scatterStyle val="lineMarker"/><c:ser><c:idx val="0"/><c:tx>${strCache(["M"])}</c:tx>
    <c:spPr><a:ln><a:noFill/></a:ln></c:spPr><c:xVal>${numCache([1, 2])}</c:xVal><c:yVal>${numCache([3, 4])}</c:yVal></c:ser></c:scatterChart>`), ctx, helpers);
  assert.deepEqual(xy.plots[0].series[0].categories, ["1", "2"]);
  assert.equal(xy.plots[0].series[0].line.color, null);
  assert.equal(legendEntries(xy)[0].kind, "dot", "points without a line show a dot in the legend");
  assert.equal(readChart(space(`<c:radarChart>${ser(0, "A", ["x"], [1])}</c:radarChart>`), ctx, helpers), null);
  assert.equal(readChart(parseXML(`<c:chartSpace ${NS}/>`), ctx, helpers), null);
  assert.equal(readChart(null, ctx, helpers), null);
});

test("axes take round ends and steps, or the fixed ones", () => {
  assert.deepEqual(niceScale(0, 21), { min: 0, max: 25, step: 5, ticks: [0, 5, 10, 15, 20, 25] });
  assert.deepEqual(niceScale(0, 0.45, 5).ticks, [0, 0.1, 0.2, 0.3, 0.4, 0.5]);
  assert.deepEqual(niceScale(-3, 7, 5).ticks, [-4, -2, 0, 2, 4, 6, 8]);
  assert.deepEqual(niceScale(0, 21, 5, { max: 30, unit: 10 }).ticks, [0, 10, 20, 30]);
  assert.deepEqual(niceScale(5, 5).ticks.at(0), 0);
  assert.deepEqual(niceScale(NaN, NaN).ticks, [0, 0.2, 0.4, 0.6, 0.8, 1]);
});

test("values span from zero, stacked by their sums, unless they lie far from it", () => {
  const plot = (kind, grouping, ...values) => ({ kind, grouping, series: values.map((v) => ({ values: v, categories: v.map((_, i) => String(i + 1)) })) });
  assert.deepEqual(valueRange([plot("bar", "clustered", [10, 21], [8, 12])]), { lo: 0, hi: 21 });
  assert.deepEqual(valueRange([plot("bar", "stacked", [10, 21], [8, 12], [-2, 0])]), { lo: -2, hi: 33 });
  assert.deepEqual(valueRange([plot("bar", "percentStacked", [10, 21])]), { lo: 0, hi: 1 });
  assert.deepEqual(valueRange([plot("line", "standard", [95, 99, 100])]), { lo: 95, hi: 100 }, "a line far from zero keeps its range");
  assert.deepEqual(valueRange([plot("line", "standard", [20, 99])]), { lo: 0, hi: 99 });
  assert.deepEqual(valueRange([plot("pie", "standard", [1, 2])]), { lo: 0, hi: 1 });
  assert.deepEqual(valueRange([plot("scatter", "standard", [3, 4])], "x"), { lo: 0, hi: 2 }, "an x axis near zero starts there");
});

test("numbers are written in the presentation's formats", () => {
  assert.equal(formatNumber(10.5, "General", "en-US"), "10.5");
  assert.equal(formatNumber(1234567, "", "en-US"), "1234567");
  assert.equal(formatNumber(0.1 + 0.2, "General", "en-US"), "0.3");
  assert.equal(formatNumber(12.345, "0.0", "en-US"), "12.3");
  assert.equal(formatNumber(8, "0.0", "en-US"), "8.0");
  assert.equal(formatNumber(1234567.8, "#,##0", "en-US"), "1,234,568");
  assert.equal(formatNumber(0.452, "0%", "en-US"), "45%");
  assert.equal(formatNumber(0.4525, "0.0%", "en-US"), "45.3%");
  assert.equal(formatNumber(1500, '"CHF "#,##0', "en-US"), "CHF 1,500");
  assert.equal(formatNumber(2500000, '#,##0.0,," Mio."', "en-US"), "2.5 Mio.");
  assert.equal(formatNumber(-5, "0.0", "en-US"), "-5.0");
  assert.equal(formatNumber(-5, "0.0;(0.0)", "en-US"), "(5.0)", "a section of its own for negative numbers");
  assert.equal(formatNumber(null, "0"), "");
  assert.equal(formatNumber(1.5e12, "General", "en-US"), "1.50e+12");
});

test("stacked series stand on the ones before them, percent stacks as shares", () => {
  const plot = (grouping) => ({ grouping, series: [{ values: [10, 20] }, { values: [5, -5] }, { values: [null, 5] }] });
  assert.deepEqual(stackOf(plot("clustered"), 2)[1], [{ from: 0, to: 5, v: 5 }, { from: 0, to: -5, v: -5 }]);
  const st = stackOf(plot("stacked"), 2);
  assert.deepEqual(st[1], [{ from: 10, to: 15, v: 5 }, { from: 0, to: -5, v: -5 }], "a negative value stacks below zero");
  assert.deepEqual(st[2], [null, { from: 20, to: 25, v: 5 }]);
  const pct = stackOf({ grouping: "percentStacked", series: [{ values: [30] }, { values: [10] }] }, 1);
  assert.deepEqual(pct.map((s) => s[0].to), [0.75, 1]);
});

test("title, legend and plot area share a chart's frame", () => {
  const chart = { title: { text: "T", size: 20 }, legend: { pos: "r", size: 10 }, font: { size: 10 },
    plots: [{ kind: "bar", series: [{ name: "Umsatz", color: null }, { name: "Kosten", color: null }] }] };
  const measure = (font, text) => text.length * 5;
  const px = (pt) => pt;
  const lay = chartLayout(chart, 400, 200, measure, px);
  assert.equal(lay.title.size, 20);
  assert.ok(lay.plot.y > 20, "the plot stands below the title");
  assert.ok(lay.plot.x + lay.plot.w < lay.legend.rows[0].x, "a legend on the right stands beside the plot");
  assert.equal(lay.legend.rows.length, 2);
  const bottom = chartLayout({ ...chart, title: null, legend: { pos: "b", size: 10 } }, 400, 200, measure, px);
  assert.ok(bottom.plot.y + bottom.plot.h < bottom.legend.rows[0].y, "a legend below stands under the plot");
  assert.equal(new Set(bottom.legend.rows.map((r) => r.y)).size, 1, "short entries share one row");
  assert.ok(bottom.legend.rows[0].x > 0 && bottom.legend.rows[1].x > bottom.legend.rows[0].x);
});
