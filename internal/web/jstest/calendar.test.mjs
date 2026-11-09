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
import { VIEWS, address, agendaDays, dayAt, entries, listQuery, localDayOf, moves, onDay, overdue, period, periodCondition, quote, readState, shift,
  shiftValue, tasksTab, typeFields, weekRows, weekStart, withoutOrder, assigneeOf, timedSpan, dayColumns, minutesAt, retime,
  occurrenceChanges, seriesChanges, cancelChanges } from "../static/assets/calendar.js";

// Points in time are placed where the reader is; these tests read them in
// Zurich, one hour ahead of UTC in winter and two in summer.
process.env.TZ = "Europe/Zurich";
import { dayString, parseDay } from "../static/assets/timeline.js";

const d = (s) => parseDay(s);
const ds = (n) => dayString(n);
const it = (key, type, props, labels = []) => ({ id: "id-" + key, key, title: key, type, properties: props, labels });
const types = [
  { name: "task", fields: [{ name: "assignee", kind: "user" }, { name: "due", kind: "date" }] },
  { name: "epic", fields: [{ name: "start", kind: "date" }, { name: "due", kind: "date" }] },
  { name: "decision", fields: [{ name: "decided", kind: "date" }] },
  { name: "page", fields: [{ name: "owner", kind: "user" }] },
  { name: "meeting", fields: [{ name: "starts", kind: "datetime" }, { name: "ends", kind: "datetime" }] },
];
const untimed = (f) => ({ startTimed: false, endTimed: false, assignee: "", recurrence: "", ...f });
const state = (over = {}) => ({ view: "month", at: d("2026-11-09"), space: "", type: "", labels: [], who: "", status: "", text: "", q: "", start: "", end: "", ...over });

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
    untimed({ type: "task", start: "", end: "due", assignee: "assignee" }),
    untimed({ type: "epic", start: "start", end: "due" }),
    untimed({ type: "decision", start: "", end: "decided" }),
    { type: "meeting", start: "starts", end: "ends", startTimed: true, endTimed: true, assignee: "", recurrence: "" },
  ]);
  assert.deepEqual(typeFields(types, state({ type: "epic" })), [untimed({ type: "epic", start: "start", end: "due" })]);
  // A chosen field holds for every type that has it; others are left out.
  assert.deepEqual(typeFields(types, state({ end: "due" })), [untimed({ type: "task", start: "", end: "due", assignee: "assignee" }), untimed({ type: "epic", start: "", end: "due" })]);
  assert.deepEqual(typeFields(types, state({ start: "decided" })), [untimed({ type: "decision", start: "decided", end: "" })]);
});

test("the query reads the period's items only, its own order left out", () => {
  assert.equal(quote('a "b" \\c'), '"a \\"b\\" \\\\c"');
  const p = period("week", d("2026-11-09"));
  const fields = [untimed({ type: "task", start: "", end: "due" }), untimed({ type: "epic", start: "start", end: "due" })];
  // A point in time is read a day wider on either side, for every zone.
  assert.equal(periodCondition([{ type: "meeting", start: "starts", end: "", startTimed: true, endTimed: false }], p.from, p.to),
    '(type = "meeting" AND ("starts" >= 2026-11-08 AND "starts" <= 2026-11-16))');
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

test("the tasks page's tabs become choices a person reads", () => {
  assert.deepEqual(tasksTab("mine"), { status: "open", who: "me" });
  assert.deepEqual(tasksTab("unassigned"), { status: "open", who: "none" });
  assert.deepEqual(tasksTab("all"), { status: "open", who: "" });
});

test("the choices in words become the query", () => {
  assert.equal(assigneeOf({ fields: [{ name: "owner", kind: "user" }, { name: "Bearbeiter", kind: "user" }] }), "Bearbeiter");
  assert.equal(assigneeOf({ fields: [{ name: "owner", kind: "user" }] }), "owner");
  assert.equal(assigneeOf({ fields: [] }), "");
  const p = period("week", d("2026-11-09"));
  const fields = typeFields(types, state({ type: "" })).filter((f) => f.type === "task" || f.type === "decision");
  // Assigned to me: only the types assigned by a field are read.
  assert.equal(listQuery(state({ who: "me", status: "open", text: 'say "hi"' }), fields, p),
    'finished != true AND text ~ "say \\"hi\\"" AND ((type = "task" AND "assignee" = currentUser() AND ("due" >= 2026-11-09 AND "due" <= 2026-11-15)))');
  assert.equal(listQuery(state({ who: "none", status: "done" }), fields, p),
    'finished = true AND ((type = "task" AND "assignee" IS EMPTY AND ("due" >= 2026-11-09 AND "due" <= 2026-11-15)))');
  assert.equal(listQuery(state({ who: "me" }), fields.filter((f) => f.type === "decision"), p), "", "no type assigned by a field: nothing to read");
});

test("the address holds what differs from the defaults", () => {
  const today = d("2026-11-09");
  const s = readState(new URLSearchParams("view=week&at=2026-12-01&labels=a,b,a&q=x%20%3D%201&space=PLAT&who=me&status=open&text=offer"), today);
  assert.deepEqual({ ...s, at: ds(s.at) }, { view: "week", at: "2026-12-01", space: "PLAT", type: "", labels: ["a", "b"], who: "me", status: "open", text: "offer", q: "x = 1", start: "", end: "" });
  assert.equal(address(s, today), "#/calendar?view=week&at=2026-12-01&space=PLAT&who=me&status=open&text=offer&q=x+%3D+1&labels=a%2Cb");
  const plain = readState(new URLSearchParams("view=year&at=2026-02-30&who=boss&status=maybe"), today);
  assert.equal(plain.who, "");
  assert.equal(plain.status, "");
  assert.equal(plain.view, VIEWS[0]);
  assert.equal(plain.at, today);
  assert.equal(address(plain, today), "#/calendar");
});

test("a point in time stands on its day where the reader is, with its time", () => {
  assert.equal(ds(localDayOf("2026-11-09T23:30:00Z")), "2026-11-10", "half past midnight in Zurich");
  assert.equal(localDayOf("soon"), null);
  const list = entries([
    it("M-1", "meeting", { starts: "2026-11-09T13:30:00Z" }),
    it("M-2", "meeting", { starts: "2026-11-09T07:00:00Z", ends: "2026-11-10T16:00:00Z" }),
    it("M-3", "meeting", { starts: "2026-11-09T23:30:00Z" }),
    it("T-1", "task", { due: "2026-11-09" }),
  ], typeFields(types, state()));
  assert.deepEqual(list.map((e) => [e.item.key, ds(e.from), ds(e.to)]), [
    ["T-1", "2026-11-09", "2026-11-09"],
    ["M-2", "2026-11-09", "2026-11-10"],
    ["M-1", "2026-11-09", "2026-11-09"],
    ["M-3", "2026-11-10", "2026-11-10"],
  ], "a day without a time first, then by time");
  assert.equal(list[0].at.from, null);
  assert.equal(list[2].at.from, Date.parse("2026-11-09T13:30:00Z"));
  assert.deepEqual(list[1].at, { from: Date.parse("2026-11-09T07:00:00Z"), to: Date.parse("2026-11-10T16:00:00Z") });
});

test("moving an entry shifts its dates by days, a span both, a time kept where the reader is", () => {
  assert.equal(shiftValue("2026-11-09", false, 3), "2026-11-12");
  assert.equal(shiftValue("2026-11-09T00:00:00Z", false, -9), "2026-10-31");
  assert.equal(shiftValue("nothing", false, 1), null);
  // 09:00 in Zurich stays 09:00 across the end of summer time.
  assert.equal(shiftValue("2026-10-24T07:00:00Z", true, 1), "2026-10-25T08:00:00.000Z");
  assert.equal(shiftValue("soon", true, 1), null);
  const fields = typeFields(types, state());
  const [span] = entries([it("E-1", "epic", { start: "2026-11-08", due: "2026-11-12" })], fields);
  assert.deepEqual(moves(span, 2), [{ field: "start", value: "2026-11-10" }, { field: "due", value: "2026-11-14" }]);
  assert.deepEqual(moves(span, 0), []);
  const [one] = entries([it("E-2", "epic", { due: "2026-11-12" })], fields);
  assert.deepEqual(moves(one, -1), [{ field: "due", value: "2026-11-11" }], "only what the item holds moves");
  const [meeting] = entries([it("M-1", "meeting", { starts: "2026-11-09T13:30:00Z" })], fields);
  assert.deepEqual(moves(meeting, 7), [{ field: "starts", value: "2026-11-16T13:30:00.000Z" }]);
});

test("the day under a point of a week's row", () => {
  const monday = d("2026-11-09");
  assert.equal(ds(dayAt(monday, 0, 700)), "2026-11-09");
  assert.equal(ds(dayAt(monday, 350, 700)), "2026-11-12");
  assert.equal(ds(dayAt(monday, 699, 700)), "2026-11-15");
  assert.equal(ds(dayAt(monday, 900, 700)), "2026-11-15");
  assert.equal(ds(dayAt(monday, -5, 700)), "2026-11-09");
  assert.equal(ds(dayAt(monday, 10, 0)), "2026-11-09");
});

test("a timed entry of one day stands on the axis of hours, overlapping ones side by side", () => {
  const fields = typeFields(types, state());
  const list = entries([
    it("M-1", "meeting", { starts: "2026-11-09T08:00:00Z", ends: "2026-11-09T09:30:00Z" }),
    it("M-2", "meeting", { starts: "2026-11-09T09:00:00Z" }),
    it("M-3", "meeting", { starts: "2026-11-09T12:00:00Z", ends: "2026-11-09T13:00:00Z" }),
    it("M-4", "meeting", { starts: "2026-11-09T08:00:00Z", ends: "2026-11-10T08:00:00Z" }),
    it("M-5", "meeting", { starts: "2026-11-09T22:30:00Z", ends: "2026-11-09T22:00:00Z" }),
    it("T-1", "task", { due: "2026-11-09" }),
  ], fields);
  const span = (k) => timedSpan(list.find((e) => e.item.key === k));
  assert.deepEqual(span("M-1"), { from: 9 * 60, to: 10 * 60 + 30 }, "in Zurich, an hour ahead");
  assert.deepEqual(span("M-2"), { from: 10 * 60, to: 11 * 60 }, "a point in time alone takes an hour");
  assert.equal(span("M-4"), null, "over two days it stands above the hours");
  assert.equal(span("T-1"), null, "a day without a time stands above the hours");
  assert.deepEqual(span("M-5"), { from: 23 * 60 + 30, to: 24 * 60 }, "a misdated end gives an hour, never past midnight");
  const cols = dayColumns(list.filter((e) => e.from === d("2026-11-09")));
  assert.deepEqual(cols.map((x) => [x.entry.item.key, x.col, x.cols]), [["M-1", 0, 2], ["M-2", 1, 2], ["M-3", 0, 1], ["M-5", 0, 1]]);
});

test("a drop on the axis of hours is a time of day, rounded to a quarter of an hour", () => {
  assert.equal(minutesAt(0, 960), 0);
  assert.equal(minutesAt(480, 960), 12 * 60);
  assert.equal(minutesAt(487, 960), 12 * 60 + 15);
  assert.equal(minutesAt(-30, 960), 0);
  assert.equal(minutesAt(2000, 960), 24 * 60 - 15);
  assert.equal(minutesAt(10, 0), 0);
});

test("retiming moves a meeting to another day and hour, keeping its length", () => {
  const fields = typeFields(types, state());
  const [m] = entries([it("M-1", "meeting", { starts: "2026-11-09T08:00:00Z", ends: "2026-11-09T09:30:00Z" })], fields);
  // From 09:00 on Monday to 14:15 on Wednesday, in Zurich.
  assert.deepEqual(retime(m, 2, 14 * 60 + 15), [
    { field: "starts", value: "2026-11-11T13:15:00.000Z" },
    { field: "ends", value: "2026-11-11T14:45:00.000Z" },
  ]);
  assert.deepEqual(retime(m, 0, 9 * 60), [], "the same place is no change");
  const [task] = entries([it("T-1", "task", { due: "2026-11-09" })], fields);
  assert.deepEqual(retime(task, 1, 600), [{ field: "due", value: "2026-11-10" }], "a day without a time moves by days");
});

const series = [{ name: "standup", fields: [{ name: "starts", kind: "datetime" }, { name: "repeats", kind: "recurrence" }] },
  { name: "review", fields: [{ name: "due", kind: "date" }, { name: "every", kind: "recurrence" }] }];

test("a repeating type is read whenever it began, and gives an entry per occurrence", () => {
  const fields = typeFields(series, state());
  assert.deepEqual(fields.map((f) => [f.type, f.recurrence]), [["standup", "repeats"], ["review", "every"]]);
  const p = period("week", d("2026-11-09"));
  assert.equal(periodCondition(fields.slice(1), p.from, p.to),
    '(type = "review" AND (("due" >= 2026-11-09 AND "due" <= 2026-11-15) OR ("every" IS NOT EMPTY AND "due" <= 2026-11-15)))');
  const items = [
    it("S-1", "standup", { starts: "2026-11-02T08:00:00Z", repeats: "FREQ=WEEKLY;BYDAY=MO,WE;TZID=Europe/Zurich;EXDATE=2026-11-11;MOVE=2026-11-16@2026-11-17T10:00" }),
    it("R-1", "review", { due: "2026-10-01T00:00:00Z", every: "FREQ=MONTHLY" }),
    it("B-1", "review", { due: "2026-11-10T00:00:00Z", every: "FREQ=NEVER" }),
  ];
  const list = entries(items, fields, { from: d("2026-11-09"), to: d("2026-11-22") });
  assert.deepEqual(list.map((e) => [e.item.key, ds(e.from), e.occurrence || "", e.moved || false]), [
    ["S-1", "2026-11-09", "2026-11-09", false],
    ["B-1", "2026-11-10", "", false],
    ["S-1", "2026-11-17", "2026-11-16", true],
    ["S-1", "2026-11-18", "2026-11-18", false],
  ], "a cancelled one is gone, a moved one on its new day, a monthly one outside the days left out, a broken rule placed once");
  const plain = entries(items.slice(0, 1), fields);
  assert.equal(plain.length, 1, "without a range a series stands once, on its first day");
});

test("an occurrence moves alone, or the whole series with its exceptions", () => {
  const fields = typeFields(series, state());
  const [mon] = entries([it("S-1", "standup", { starts: "2026-11-09T08:00:00Z", repeats: "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;EXDATE=2026-11-23" })], fields,
    { from: d("2026-11-09"), to: d("2026-11-15") });
  // One day on, at the same time in Zurich.
  assert.deepEqual(occurrenceChanges(mon, 1), [{ field: "repeats", value: "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;EXDATE=2026-11-23;MOVE=2026-11-09@2026-11-10T09:00" }]);
  // To Wednesday at 14:15 where the reader is (Zurich in these tests).
  assert.deepEqual(occurrenceChanges(mon, 2, 14 * 60 + 15), [{ field: "repeats", value: "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;EXDATE=2026-11-23;MOVE=2026-11-09@2026-11-11T14:15" }]);
  assert.deepEqual(occurrenceChanges(mon, 0, 9 * 60), [], "the same place is no change");
  assert.deepEqual(cancelChanges(mon), [{ field: "repeats", value: "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;EXDATE=2026-11-09,2026-11-23" }]);
  assert.deepEqual(seriesChanges(mon, 7), [
    { field: "starts", value: "2026-11-16T08:00:00.000Z" },
    { field: "repeats", value: "FREQ=WEEKLY;BYDAY=MO;TZID=Europe/Zurich;EXDATE=2026-11-30" },
  ]);
  assert.deepEqual(seriesChanges(mon, 0, 10 * 60), [{ field: "starts", value: "2026-11-09T09:00:00.000Z" }], "a new time keeps the exceptions' days");
  assert.deepEqual(seriesChanges(mon, 0), []);
  const [day] = entries([it("R-1", "review", { due: "2026-11-10T00:00:00Z", every: "FREQ=WEEKLY" })], fields, { from: d("2026-11-09"), to: d("2026-11-15") });
  assert.deepEqual(occurrenceChanges(day, -1), [{ field: "every", value: "FREQ=WEEKLY;MOVE=2026-11-10@2026-11-09" }]);
});
