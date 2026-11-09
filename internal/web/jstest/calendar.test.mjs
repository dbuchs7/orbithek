// Unit tests of the calendar of the items' dates (calendar.js, ADR-0166):
// the days of a month, a week and an agenda, moving by periods, the fields
// each type is placed by, the query that reads the period, the entries
// items become, the rows of a week, the days of an agenda, the query of
// the tasks page's tabs and the address. They run in Node.js without a
// browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The view itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { VIEWS, address, agendaDays, entries, listQuery, onDay, overdue, period, periodCondition, quote, readState, shift, tasksQuery,
  typeFields, weekRows, weekStart, withoutOrder } from "../static/assets/calendar.js";
import { dayString, parseDay } from "../static/assets/timeline.js";

const d = (s) => parseDay(s);
const ds = (n) => dayString(n);
const it = (key, type, props, labels = []) => ({ id: "id-" + key, key, title: key, type, properties: props, labels });
const types = [
  { name: "task", fields: [{ name: "assignee", kind: "user" }, { name: "due", kind: "date" }] },
  { name: "epic", fields: [{ name: "start", kind: "date" }, { name: "due", kind: "date" }] },
  { name: "decision", fields: [{ name: "decided", kind: "date" }] },
  { name: "page", fields: [{ name: "owner", kind: "user" }] },
];
const state = (over = {}) => ({ view: "month", at: d("2026-11-09"), space: "", type: "", labels: [], q: "", start: "", end: "", ...over });

test("weeks begin on Monday", () => {
  assert.equal(ds(weekStart(d("2026-11-09"))), "2026-11-09");
  assert.equal(ds(weekStart(d("2026-11-15"))), "2026-11-09");
  assert.equal(ds(weekStart(d("2026-11-01"))), "2026-10-26");
});

test("a month is drawn in whole weeks, a week is seven days, an agenda its month", () => {
  const m = period("month", d("2026-11-09"));
  assert.deepEqual([ds(m.first), ds(m.last), ds(m.from), ds(m.to)], ["2026-11-01", "2026-11-30", "2026-10-26", "2026-12-06"]);
  assert.equal(m.weeks.length, 6);
  assert.deepEqual(m.weeks.map(ds).slice(0, 2), ["2026-10-26", "2026-11-02"]);
  // February 2027 begins on a Monday and ends on a Sunday: four rows.
  assert.equal(period("month", d("2027-02-14")).weeks.length, 4);
  const w = period("week", d("2026-11-12"));
  assert.deepEqual([ds(w.from), ds(w.to), w.weeks.length], ["2026-11-09", "2026-11-15", 1]);
  const a = period("agenda", d("2026-11-12"));
  assert.deepEqual([ds(a.from), ds(a.to), a.weeks.length], ["2026-11-01", "2026-11-30", 0]);
});

test("moving by a period keeps the date where the month has it", () => {
  assert.equal(ds(shift("week", d("2026-11-09"), 1)), "2026-11-16");
  assert.equal(ds(shift("month", d("2026-11-09"), -1)), "2026-10-09");
  assert.equal(ds(shift("month", d("2027-01-31"), 1)), "2027-02-28");
  assert.equal(ds(shift("agenda", d("2026-12-15"), 1)), "2027-01-15");
});

test("each type is placed by its own guessed fields, or by the fields chosen", () => {
  assert.deepEqual(typeFields(types, state()), [
    { type: "task", start: "", end: "due" },
    { type: "epic", start: "start", end: "due" },
    { type: "decision", start: "", end: "decided" },
  ]);
  assert.deepEqual(typeFields(types, state({ type: "epic" })), [{ type: "epic", start: "start", end: "due" }]);
  // A chosen field holds for every type that has it; others are left out.
  assert.deepEqual(typeFields(types, state({ end: "due" })), [{ type: "task", start: "", end: "due" }, { type: "epic", start: "", end: "due" }]);
  assert.deepEqual(typeFields(types, state({ start: "decided" })), [{ type: "decision", start: "decided", end: "" }]);
});

test("the query reads the period's items only, its own order left out", () => {
  assert.equal(quote('a "b" \\c'), '"a \\"b\\" \\\\c"');
  const p = period("week", d("2026-11-09"));
  const fields = [{ type: "task", start: "", end: "due" }, { type: "epic", start: "start", end: "due" }];
  assert.equal(periodCondition(fields, p.from, p.to),
    '(type = "task" AND ("due" >= 2026-11-09 AND "due" <= 2026-11-15)) OR ' +
    '(type = "epic" AND (("start" <= 2026-11-15 AND "due" >= 2026-11-09) OR ("start" >= 2026-11-09 AND "start" <= 2026-11-15) OR ("due" >= 2026-11-09 AND "due" <= 2026-11-15)))');
  const q = listQuery(state({ labels: ["plan-q1", "ops"], q: "state != done ORDER BY due" }), fields.slice(0, 1), p);
  assert.equal(q, 'label IN ("plan-q1", "ops") AND (state != done) AND ((type = "task" AND ("due" >= 2026-11-09 AND "due" <= 2026-11-15)))');
  assert.equal(listQuery(state(), [], p), "", "nothing is read without a date field");
  assert.equal(withoutOrder('title ~ "order by" order by key'), 'title ~ "order by"');
  assert.equal(withoutOrder("(a = 1)ORDER BY key"), "(a = 1)");
  assert.equal(withoutOrder("ORDER BY key"), "");
  assert.equal(withoutOrder("border = x"), "border = x");
});

test("items become entries on their days, a span over its days", () => {
  const fields = typeFields(types, state());
  const list = entries([
    it("T-1", "task", { due: "2026-11-10T00:00:00Z" }),
    it("E-1", "epic", { start: "2026-11-08", due: "2026-11-12" }),
    it("E-2", "epic", { start: "2026-11-20", due: "2026-11-18" }),
    it("D-1", "decision", {}),
    it("P-1", "page", {}),
  ], fields);
  assert.deepEqual(list.map((e) => [e.item.key, ds(e.from), ds(e.to), e.inverted]), [
    ["E-1", "2026-11-08", "2026-11-12", false],
    ["T-1", "2026-11-10", "2026-11-10", false],
    ["E-2", "2026-11-18", "2026-11-18", true],
  ]);
  assert.equal(list[0].end, "due", "an entry knows the fields it stands by");
  assert.deepEqual(onDay(list, d("2026-11-10")).map((e) => e.item.key), ["E-1", "T-1"]);
  assert.deepEqual(agendaDays(list, d("2026-11-11"), d("2026-11-30")).map((x) => [ds(x.day), x.entries.length]),
    [["2026-11-11", 1], ["2026-11-12", 1], ["2026-11-18", 1]]);
});

test("a week's entries are laid into rows, a span marked where it goes on", () => {
  const fields = typeFields(types, state());
  const list = entries([
    it("E-1", "epic", { start: "2026-11-08", due: "2026-11-11" }),
    it("T-1", "task", { due: "2026-11-10" }),
    it("T-2", "task", { due: "2026-11-12" }),
    it("E-2", "epic", { start: "2026-11-13", due: "2026-11-20" }),
    it("T-3", "task", { due: "2026-11-30" }),
  ], fields);
  const rows = weekRows(list, d("2026-11-09"));
  assert.deepEqual(rows.map((r) => r.map((s) => [s.entry.item.key, s.col, s.span, s.before, s.after])), [
    [["E-1", 0, 3, true, false], ["T-2", 3, 1, false, false], ["E-2", 4, 3, false, true]],
    [["T-1", 1, 1, false, false]],
  ]);
});

test("an unfinished entry before today is overdue", () => {
  const e = { from: d("2026-11-01"), to: d("2026-11-05") };
  assert.equal(overdue(e, d("2026-11-09"), false), true);
  assert.equal(overdue(e, d("2026-11-09"), true), false);
  assert.equal(overdue(e, d("2026-11-05"), false), false);
});

test("the tasks page's tabs become queries of open tasks", () => {
  const tt = [{ name: "task", assignee: "assignee" }, { name: "bug", assignee: "" }];
  assert.equal(tasksQuery("mine", tt), 'finished = false AND ((type = "task" AND "assignee" = currentUser()))');
  assert.equal(tasksQuery("unassigned", tt), 'finished = false AND ((type = "task" AND "assignee" IS EMPTY))');
  assert.equal(tasksQuery("all", tt), 'finished = false AND type IN ("task", "bug")');
  assert.equal(tasksQuery("mine", [{ name: "bug", assignee: "" }]), 'finished = false AND type IN ("bug")');
  assert.equal(tasksQuery("mine", []), "");
});

test("the address holds what differs from the defaults", () => {
  const today = d("2026-11-09");
  const s = readState(new URLSearchParams("view=week&at=2026-12-01&labels=a,b,a&q=x%20%3D%201&space=PLAT"), today);
  assert.deepEqual({ ...s, at: ds(s.at) }, { view: "week", at: "2026-12-01", space: "PLAT", type: "", labels: ["a", "b"], q: "x = 1", start: "", end: "" });
  assert.equal(address(s, today), "#/calendar?view=week&at=2026-12-01&space=PLAT&q=x+%3D+1&labels=a%2Cb");
  const plain = readState(new URLSearchParams("view=year&at=2026-02-30"), today);
  assert.equal(plain.view, VIEWS[0]);
  assert.equal(plain.at, today);
  assert.equal(address(plain, today), "#/calendar");
});
