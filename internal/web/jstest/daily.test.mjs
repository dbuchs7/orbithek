// Unit tests of the parts of a person's daily notes (dailyparts.js,
// ADR-0203): the day an address names, the month beside a note, the
// bounds of a day where the reader is and the query of what was made or
// changed on it, and the names of days and months. They run in Node.js
// without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The view itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";

// Days are read where the reader is; these tests read them in Zurich.
process.env.TZ = "Europe/Zurich";
const { activityQuery, dailyHref, dayBounds, dayOfAddress, month, monthTitle, noteTitle, weekdayNames } = await import("../static/assets/dailyparts.js");
const { parseDay } = await import("../static/assets/timeline.js");

test("an address names a day, or today without one", () => {
  const now = new Date(2026, 10, 12, 23, 30); // late on 12 November in Zurich
  assert.equal(dayOfAddress("", now), parseDay("2026-11-12"));
  assert.equal(dayOfAddress(undefined, now), parseDay("2026-11-12"));
  assert.equal(dayOfAddress("2026-02-28"), parseDay("2026-02-28"));
  assert.equal(dayOfAddress("2026-02-30"), null);
  assert.equal(dayOfAddress("2026-2-3"), null);
  assert.equal(dayOfAddress("yesterday"), null);
  assert.equal(dailyHref(parseDay("2026-11-12")), "#/daily/2026-11-12");
});

test("the month beside a note is drawn in whole weeks from Monday", () => {
  const day = parseDay("2026-11-12");
  const m = month(day, ["2026-11-02", "2026-11-12", "2026-12-01"], parseDay("2026-11-13"));
  // November 2026 begins on a Sunday: its first week starts on 26 October.
  assert.equal(m.weeks.length, 6);
  assert.equal(m.weeks[0][0].day, parseDay("2026-10-26"));
  assert.equal(m.weeks[0][6].date, 1);
  assert.equal(m.weeks[0][0].inMonth, false);
  const all = m.weeks.flat();
  // 1 December stands in the last week drawn.
  assert.deepEqual(all.filter((d) => d.noted).map((d) => d.date), [2, 12, 1]);
  assert.equal(all.find((d) => d.today).date, 13);
  assert.equal(all.find((d) => d.shown).date, 12);
  assert.equal(all.filter((d) => d.inMonth).length, 30);
});

test("a day is bounded where the reader is, even when the clocks change", () => {
  assert.deepEqual(dayBounds(parseDay("2026-11-12")), { from: "2026-11-11T23:00:00.000Z", to: "2026-11-12T23:00:00.000Z" });
  // On 29 March 2026 the clocks go forward: the day has 23 hours.
  assert.deepEqual(dayBounds(parseDay("2026-03-29")), { from: "2026-03-28T23:00:00.000Z", to: "2026-03-29T22:00:00.000Z" });
  assert.equal(activityQuery(parseDay("2026-11-12")),
    '(created >= "2026-11-11T23:00:00.000Z" AND created < "2026-11-12T23:00:00.000Z") OR ' +
    '(updated >= "2026-11-11T23:00:00.000Z" AND updated < "2026-11-12T23:00:00.000Z") ORDER BY updated DESC');
});

test("days and months are named in the language of the interface", () => {
  const day = parseDay("2026-11-12");
  assert.match(noteTitle(day, "en-GB"), /^Thursday,? 12 November 2026$/);
  assert.match(noteTitle(day, "de-CH"), /^Donnerstag, 12\. November 2026$/);
  assert.equal(monthTitle(day, "en-GB"), "November 2026");
  assert.equal(weekdayNames("en-GB")[0], "Mon");
  assert.equal(weekdayNames("en-GB").length, 7);
  // A language the browser does not know falls back.
  assert.equal(noteTitle(day, "!!"), "2026-11-12");
});
