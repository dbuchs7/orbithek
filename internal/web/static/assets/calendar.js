// The calendar of the items' dates (ADR-0166) without a browser: the days
// a month, a week or an agenda shows, the period before and after it, the
// date fields each type is placed by, the query that reads only the items
// of the period, the entries items become, how the entries of a week are
// laid into rows, the days of an agenda, and the view's state as an
// address.
//
// Days are whole days counted from 1970-01-01 in UTC, as on the roadmap
// (timeline.js), whose reading of dates, guessing of fields and turning of
// items into bars this module shares rather than copies: a bar of the
// roadmap is an entry of the calendar.
//
// Plain functions over plain data, so node --test runs them
// (internal/web/jstest/calendar.test.mjs).
import { byStart, dateFields, dayString, guessFields, parseDay, schedule } from "./timeline.js";

const DAY = 86400000;

// VIEWS are the layouts of the calendar; the first is the default.
export const VIEWS = ["month", "week", "agenda"];

// parts splits a day number into its calendar parts; weekday 0 is Monday.
function parts(day) {
  const d = new Date(day * DAY);
  return { year: d.getUTCFullYear(), month: d.getUTCMonth(), date: d.getUTCDate(), weekday: (d.getUTCDay() + 6) % 7 };
}

const dayOf = (year, month, date = 1) => Date.UTC(year, month, date) / DAY;

// weekStart is the Monday on or before a day (ISO 8601).
export function weekStart(day) {
  return day - parts(day).weekday;
}

// period is what a view shows around the day at: first and last are the
// days it is about (the month, or the week), from and to the days it
// draws, both included, and weeks the Mondays of its rows. A month is
// drawn in whole weeks, so it begins and ends with days of the months
// beside it; an agenda lists the days of its month only.
export function period(view, at) {
  if (view === "week") {
    const from = weekStart(at);
    return { view, first: from, last: from + 6, from, to: from + 6, weeks: [from] };
  }
  const { year, month } = parts(at);
  const first = dayOf(year, month);
  const last = dayOf(year, month + 1) - 1;
  if (view === "agenda") return { view, first, last, from: first, to: last, weeks: [] };
  const from = weekStart(first);
  const to = weekStart(last) + 6;
  const weeks = [];
  for (let w = from; w <= to; w += 7) weeks.push(w);
  return { view, first, last, from, to, weeks };
}

// shift is the day at, moved by n periods of a view: by weeks for a week,
// by months otherwise, the date kept where the month has it (31 January
// and one month on is 28 or 29 February).
export function shift(view, at, n) {
  if (view === "week") return at + 7 * n;
  const { year, month, date } = parts(at);
  const last = dayOf(year, month + n + 1) - dayOf(year, month + n);
  return dayOf(year, month + n, Math.min(date, last));
}

// typeFields is, for every type with date fields that the view shows, the
// fields its items are placed by. Without a choice each type's are guessed
// from their names, as the roadmap guesses them, so a task stands on its
// due date and a decision on the day it was decided. A chosen start or end
// applies to every type that has it as a date field; a type with neither
// is left out.
export function typeFields(types, state) {
  const chosen = Boolean(state.start || state.end);
  const out = [];
  for (const ty of types) {
    if (state.type && ty.name !== state.type) continue;
    const names = dateFields([ty]);
    if (!names.length) continue;
    const f = chosen
      ? { start: names.includes(state.start) ? state.start : "", end: names.includes(state.end) ? state.end : "" }
      : guessFields(names);
    if (f.start || f.end) out.push({ type: ty.name, start: f.start, end: f.end });
  }
  return out;
}

// quote writes a name or a value as a quoted text of the query language
// (ADR-0136): a quoted name always names a property, never a field every
// item has, and a quoted value is taken as it is written.
export function quote(text) {
  return '"' + String(text).replace(/["\\]/g, "\\$&") + '"';
}

// periodCondition selects the items of the types that have a date in the
// days from to to: one of their dates in the days, or a span from the
// start to the end that covers them. A day of the query names the whole
// day, so the last day is included.
export function periodCondition(fields, from, to) {
  const first = dayString(from), last = dayString(to);
  const within = (f) => `${quote(f)} >= ${first} AND ${quote(f)} <= ${last}`;
  return fields.map((f) => {
    const cond = f.start && f.end
      ? `(${quote(f.start)} <= ${last} AND ${quote(f.end)} >= ${first}) OR (${within(f.start)}) OR (${within(f.end)})`
      : within(f.start || f.end);
    return `(type = ${quote(f.type)} AND (${cond}))`;
  }).join(" OR ");
}

// withoutOrder is a query without its ORDER BY: the calendar orders its
// entries by their days, and an order inside the parentheses the calendar
// puts around the query would not parse. A quoted text that holds the
// words is left alone.
export function withoutOrder(q) {
  const text = String(q || "");
  let quoteChar = "";
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoteChar) {
      if (c === "\\") i++;
      else if (c === quoteChar) quoteChar = "";
    } else if (c === '"' || c === "'") {
      quoteChar = c;
    } else if ((i === 0 || /[\s)]/.test(text[i - 1])) && /^ORDER\s+BY\b/i.test(text.slice(i))) {
      return text.slice(0, i).trim();
    }
  }
  return text.trim();
}

// listQuery is the query the view reads its items with: the chosen labels,
// the query typed, and the period of the view. It is empty when no type
// has a date field to place items by, and nothing is read then.
export function listQuery(state, fields, p) {
  const pc = periodCondition(fields, p.from, p.to);
  if (!pc) return "";
  const conds = [];
  if (state.labels.length) conds.push(`label IN (${state.labels.map(quote).join(", ")})`);
  const own = withoutOrder(state.q);
  if (own) conds.push(`(${own})`);
  conds.push(`(${pc})`);
  return conds.join(" AND ");
}

// entries turns items into entries, each by the fields of its type: an
// item with a start and an end is a span over both days and those
// between; an item with one date, or with an end before its start, stands
// on one day (the end, when it has both: the date that is promised), as a
// milestone stands on the roadmap.
export function entries(items, fields) {
  const byType = new Map(fields.map((f) => [f.type, f]));
  const out = [];
  for (const it of items) {
    const f = byType.get(it.type);
    if (f) out.push(...schedule([it], f).bars.map((b) => ({ ...b, start: f.start, end: f.end })));
  }
  return out.sort(byStart);
}

// weekRows lays the entries that touch the week beginning on monday into
// rows without overlapping: each becomes a segment from its first to its
// last day in the week (col 0 is Monday, span the number of days), marked
// where it goes on before or after the week. Longer segments come first,
// so a span keeps one row through the week.
export function weekRows(list, monday) {
  const sunday = monday + 6;
  const segs = list.filter((e) => e.from <= sunday && e.to >= monday).map((e) => {
    const a = Math.max(e.from, monday), b = Math.min(e.to, sunday);
    return { entry: e, col: a - monday, span: b - a + 1, before: e.from < monday, after: e.to > sunday };
  });
  segs.sort((x, y) => x.col - y.col || y.span - x.span || byStart(x.entry, y.entry));
  const rows = [];
  for (const s of segs) {
    const row = rows.find((r) => r[r.length - 1].col + r[r.length - 1].span <= s.col);
    if (row) row.push(s);
    else rows.push([s]);
  }
  return rows;
}

// onDay are the entries a day holds, in the order of their start.
export function onDay(list, day) {
  return list.filter((e) => e.from <= day && e.to >= day);
}

// agendaDays are the days from first to last that hold entries, each with
// them; a span is listed on each of its days.
export function agendaDays(list, first, last) {
  const out = [];
  for (let day = first; day <= last; day++) {
    const held = onDay(list, day);
    if (held.length) out.push({ day, entries: held });
  }
  return out;
}

// overdue tells an entry whose last day lies before today and whose item
// is not finished.
export function overdue(entry, today, done) {
  return !done && entry.to < today;
}

// tasksQuery is the query the tasks page hands to the calendar for one of
// its tabs (ADR-0049): the open tasks assigned to the reader, those
// assigned to nobody, or all of them. taskTypes name the task types and
// the field each is assigned by, empty for a type without one. When no
// type is assigned by a field, every tab shows all open tasks.
export function tasksQuery(tab, taskTypes) {
  if (!taskTypes.length) return "";
  const all = `type IN (${taskTypes.map((ty) => quote(ty.name)).join(", ")})`;
  const assigned = taskTypes.filter((ty) => ty.assignee);
  if (tab === "all" || !assigned.length || (tab !== "mine" && tab !== "unassigned")) return `finished = false AND ${all}`;
  const test = tab === "mine" ? "= currentUser()" : "IS EMPTY";
  return `finished = false AND (${assigned.map((ty) => `(type = ${quote(ty.name)} AND ${quote(ty.assignee)} ${test})`).join(" OR ")})`;
}

// readState reads the view's state from the address's parameters; what
// is missing or unknown takes its default. at is a day number, today's
// when the address names none or no real day.
export function readState(params, today) {
  const get = (k) => params.get(k) || "";
  const at = parseDay(get("at"));
  return {
    view: VIEWS.includes(get("view")) ? get("view") : VIEWS[0],
    at: at ?? today,
    space: get("space"),
    type: get("type"),
    labels: [...new Set(get("labels").split(",").map((s) => s.trim()).filter(Boolean))],
    q: get("q"),
    start: get("start"),
    end: get("end"),
  };
}

// address writes the view's state as its address, leaving out what holds
// its default, today included, so a shared link stays short and a link
// without a day always opens on the day it is opened.
export function address(state, today) {
  const p = new URLSearchParams();
  if (state.view && state.view !== VIEWS[0]) p.set("view", state.view);
  if (state.at !== undefined && state.at !== today) p.set("at", dayString(state.at));
  for (const k of ["space", "type", "start", "end", "q"]) if (state[k]) p.set(k, state[k]);
  if (state.labels?.length) p.set("labels", state.labels.join(","));
  const q = p.toString();
  return "#/calendar" + (q ? "?" + q : "");
}
