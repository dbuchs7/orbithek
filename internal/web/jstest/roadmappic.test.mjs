// Unit tests of the picture of a roadmap and a Gantt chart (roadmappic.js,
// ADR-0218): where the title, the axis, the heads, the bars and
// milestones and the line of today stand in pixels, how far a picture is
// scaled before a browser would refuse its canvas, and what its file is
// called. They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The drawing itself, which needs a browser, is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { LIMITS, SIZES, pictureFormats, pictureLayout, pictureName, pictureScale } from "../static/assets/roadmappic.js";
import { frame, parseDay } from "../static/assets/timeline.js";

const d = (s) => parseDay(s);
// A frame of the months of 2026, from December 2025 to January 2027.
const fr = frame({ from: d("2026-01-10"), to: d("2026-12-20") }, "month", d("2026-03-01"));
const spec = (rows, gantt = false) => ({ title: "Roadmap · PLAT", date: "12 Nov 2026", frame: fr, unit: 96, gantt, ticks: fr.ticks.map((t) => "m" + t.month), rows });

test("the picture is as wide as the head and every unit, and as high as the title, the axis and every row", () => {
  const L = pictureLayout(spec([{ first: true, head: { label: "q1" }, bars: [] }, { first: false, head: null, bars: [] }, { first: true, head: { none: "Without a label" }, bars: [] }]));
  const { head, row, axis, title, margin } = SIZES;
  assert.equal(fr.ticks.length, 14);
  assert.equal(L.width, margin + head + 14 * 96 + margin);
  assert.equal(L.height, margin + title + axis + 3 * row + margin);
  assert.equal(L.axisY, margin + title);
  assert.equal(L.bodyY, L.axisY + axis);
  assert.deepEqual(L.ticks.map((t) => t.text).slice(0, 3), ["m11", "m0", "m1"], "each tick has its caption");
  assert.equal(L.ticks[0].x, L.left);
  assert.ok(Math.abs(L.ticks.at(-1).x + L.ticks.at(-1).w - (L.left + L.track)) < 1e-9, "the last unit ends where the tracks end");
  assert.deepEqual(L.rows.map((r) => r.rule), [false, false, true], "a roadmap parts its lanes, not the rows of one lane");
  assert.deepEqual(L.rows.map((r) => r.y), [L.bodyY, L.bodyY + row, L.bodyY + 2 * row]);
  assert.equal(L.title, "Roadmap · PLAT");
  assert.equal(L.date, "12 Nov 2026");
});

test("a Gantt chart parts every row", () => {
  const L = pictureLayout(spec([1, 2, 3].map((n) => ({ first: true, head: { key: "K-" + n, text: "t" }, bars: [] })), true));
  assert.deepEqual(L.rows.map((r) => r.rule), [false, true, true]);
  assert.deepEqual(L.rows[1].head, { key: "K-2", text: "t" });
});

test("a bar covers its days, both included, at least a few pixels; a milestone stands in the middle of its day", () => {
  const L = pictureLayout(spec([{ first: true, head: null, bars: [
    { from: d("2026-01-01"), to: d("2026-01-31"), text: "K-1 Plan", done: true },
    { from: d("2026-02-01"), to: d("2026-02-01"), text: "", waiting: true },
    { from: d("2026-03-10"), to: d("2026-03-10"), milestone: true, inverted: true, text: "K-3" },
  ] }]));
  const dayW = L.track / fr.days;
  const [bar, day, ms] = L.bars;
  assert.ok(Math.abs(bar.x - L.ticks[1].x) < 1e-9, "January starts with the tick of January");
  assert.ok(Math.abs(bar.w - L.ticks[1].w) < 1e-9, "and the bar covers its whole month");
  assert.deepEqual([bar.done, bar.waiting, bar.milestone, bar.text], [true, false, false, "K-1 Plan"]);
  assert.equal(day.w, Math.max(SIZES.minBar, dayW), "a bar of one day is no thinner than the least bar");
  assert.equal(day.waiting, true);
  assert.equal(ms.milestone, true);
  assert.equal(ms.inverted, true);
  assert.ok(Math.abs(ms.x - (L.left + (d("2026-03-10") - fr.from + 0.5) * dayW)) < 1e-9);
  assert.equal(bar.y, L.bodyY + 4);
  assert.equal(bar.h, SIZES.row - 8);
  assert.ok(Math.abs(L.today - (L.left + (d("2026-03-01") - fr.from + 0.5) * dayW)) < 1e-9, "today is a line in the middle of its day");
});

test("today outside the frame draws no line", () => {
  const outside = frame({ from: d("2026-01-10"), to: d("2026-02-20") }, "month", d("2030-01-01"));
  assert.equal(pictureLayout({ ...spec([]), frame: outside, ticks: [] }).today, null);
});

test("a picture is drawn twice as sharp where it fits, smaller where a browser would refuse it, and not when its text could not be read", () => {
  assert.equal(pictureScale(2000, 1500), LIMITS.sharp);
  assert.equal(pictureScale(3000, 16384), 1, "a side of 16384 at most");
  assert.ok(Math.abs(pictureScale(12000, 12000) - Math.sqrt(LIMITS.area) / 12000) < 1e-12, "the area bounds a picture large both ways");
  assert.equal(pictureScale(2000, 45000), 0, "a Gantt chart of 1500 rows is too high to read");
  assert.equal(pictureScale(2000, 32768), LIMITS.least, "half as sharp as the page is the least");
});

test("the file is named after what the view shows, with the format's extension", () => {
  assert.deepEqual(pictureFormats, ["png", "jpeg"]);
  assert.equal(pictureName("Gantt chart PLAT release 2026-11-12", "png"), "Gantt chart PLAT release 2026-11-12.png");
  assert.equal(pictureName("Roadmap by label v1.2 2026-11-12", "jpeg"), "Roadmap by label v1.2 2026-11-12.jpg", "a dot in a label is no extension");
  assert.equal(pictureName("Roadmap a/b: c? 2026-11-12", "png"), "Roadmap a-b- c- 2026-11-12.png", "characters file systems refuse become a dash");
  assert.equal(pictureName("", "png"), "picture.png");
  assert.throws(() => pictureName("x", "svg"), /unknown format/, "the chart exports no SVG");
});
