// Unit tests of repeating items in the browser (recurrence.js, ADR-0167).
// The cases are shared with the server's tests (internal/item/testdata/
// recurrence-cases.json), so both expand a rule alike. They run in Node.js
// without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { expand, localIn, normalRule, parseRule, shifted, validZone, withException, zoneTime, zoned } from "../static/assets/recurrence.js";

const cases = JSON.parse(readFileSync(new URL("../../item/testdata/recurrence-cases.json", import.meta.url), "utf8"));
const iso = (ms) => new Date(ms).toISOString().replace(".000Z", "Z");

test("a rule is read and written in one form, as the server does", () => {
  for (const c of cases.parse) {
    if (c.error) {
      assert.throws(() => normalRule(c.in), Error, c.in);
      continue;
    }
    assert.equal(normalRule(c.in), c.norm, c.in);
    assert.equal(normalRule(c.norm), c.norm);
  }
});

test("a rule is expanded into the occurrences the server finds", () => {
  for (const c of cases.expand) {
    const got = expand(c.rule, c.start, c.end, c.timed, Date.parse(c.from), Date.parse(c.to))
      .map((o) => [o.original, iso(o.start), iso(o.end), o.moved]);
    assert.deepEqual(got, c.want.map((w) => [w.original, w.start, w.end, w.moved]), c.name);
  }
  assert.deepEqual(expand("FREQ=DAILY", "not a time", "", false, 0, 1), []);
});

test("times of a zone, both ways", () => {
  assert.equal(iso(zoned(2026, 11, 9, 9, 0, 0, "Europe/Zurich")), "2026-11-09T08:00:00Z");
  assert.equal(iso(zoned(2026, 7, 9, 9, 0, 0, "Europe/Zurich")), "2026-07-09T07:00:00Z");
  assert.equal(iso(zoned(2026, 7, 9, 9, 0, 0, "UTC")), "2026-07-09T09:00:00Z");
  assert.deepEqual(localIn(Date.parse("2026-11-09T23:30:00Z"), "Europe/Zurich"), { year: 2026, month: 11, day: 10, hour: 0, minute: 30, second: 0 });
  assert.equal(zoneTime(Date.parse("2026-11-17T09:00:00Z"), "Europe/Zurich"), "2026-11-17T10:00");
  assert.equal(validZone("Europe/Zurich"), true);
  assert.equal(validZone("Mars/Olympus"), false);
});

test("an occurrence is cancelled or moved, and moved back", () => {
  const rule = "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich";
  const moved = withException(rule, "2026-11-16", "2026-11-17T10:00");
  assert.equal(moved, "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;MOVE=2026-11-16@2026-11-17T10:00");
  assert.equal(withException(moved, "2026-11-16", null), "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;EXDATE=2026-11-16");
  assert.equal(withException(moved, "2026-11-16", "2026-11-16"), rule, "back on its day, no exception is left");
  assert.equal(shifted("FREQ=DAILY;EXDATE=2026-11-10;MOVE=2026-11-11@2026-11-12T09:00", 2), "FREQ=DAILY;EXDATE=2026-11-12;MOVE=2026-11-13@2026-11-14T09:00");
  assert.equal(shifted("FREQ=DAILY", 0), "FREQ=DAILY");
  assert.deepEqual(parseRule("FREQ=DAILY").moves, {});
});
