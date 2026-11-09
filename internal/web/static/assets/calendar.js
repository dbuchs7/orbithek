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
// A date field names a day wherever the reader is. A datetime field holds
// a point in time, in UTC (ADR-0166); the calendar places it on its day in
// the reader's time zone and shows its time, and moving it by days keeps
// its time of day there.
//
// Plain functions over plain data, so node --test runs them
// (internal/web/jstest/calendar.test.mjs).
import { byStart, dateFields, dayString, guessFields, parseDay, schedule, todayDay } from "./timeline.js";
import { expand, localIn, parseRule, shifted, withException, zoneTime, zoned } from "./recurrence.js";

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
// is left out. startTimed and endTimed tell a field that holds a point in
// time; assignee names the user field the type is assigned by, as the task
// list finds it (ADR-0049), empty for none.
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
    const timed = (name) => (ty.fields || []).some((x) => x.name === name && x.kind === "datetime");
    // A type repeats by its first field of the kind recurrence (ADR-0168).
    const recurrence = (ty.fields || []).find((x) => x.kind === "recurrence")?.name || "";
    if (f.start || f.end) out.push({ type: ty.name, start: f.start, end: f.end, startTimed: timed(f.start), endTimed: timed(f.end), assignee: assigneeOf(ty), recurrence });
  }
  return out;
}

// assigneeOf is the user field a type is assigned by: one named assignee
// (or zugewiesen, bearbeiter), else its first user field, as the task list
// reads it (ADR-0049); empty when the type has no user field.
export function assigneeOf(ty) {
  const users = (ty.fields || []).filter((f) => f.kind === "user");
  return (users.find((f) => /^(assignee|zugewiesen|bearbeiter)/i.test(f.name)) || users[0])?.name || "";
}

// WHO are the choices of whom the items are assigned to: anyone, the
// reader, or nobody. STATUS are the choices of their state: any, not
// finished, or finished (ADR-0082). The first of each is the default.
export const WHO = ["", "me", "none"];
export const STATUS = ["", "open", "done"];

// quote writes a name or a value as a quoted text of the query language
// (ADR-0136): a quoted name always names a property, never a field every
// item has, and a quoted value is taken as it is written.
export function quote(text) {
  return '"' + String(text).replace(/["\\]/g, "\\$&") + '"';
}

// periodCondition selects the items of the types that have a date in the
// days from to to: one of their dates in the days, or a span from the
// start to the end that covers them. A day of the query names the whole
// day, so the last day is included. The query counts a point in time by
// its day in UTC, the calendar by its day where the reader is; a day more
// on either side covers every time zone, and the days drawn pick out what
// belongs to them. With who, each type is narrowed by its assignee field
// to the reader ("me") or to nobody ("none"); a type without one is left
// out then, as the task list leaves it out of those tabs.
export function periodCondition(fields, from, to, who = "") {
  const first = (timed) => dayString(timed ? from - 1 : from), last = (timed) => dayString(timed ? to + 1 : to);
  const within = (name, timed) => `${quote(name)} >= ${first(timed)} AND ${quote(name)} <= ${last(timed)}`;
  const whom = who === "me" ? "= currentUser()" : who === "none" ? "IS EMPTY" : "";
  return fields.filter((f) => !whom || f.assignee).map((f) => {
    let cond = f.start && f.end
      ? `(${quote(f.start)} <= ${last(f.startTimed)} AND ${quote(f.end)} >= ${first(f.endTimed)}) OR (${within(f.start, f.startTimed)}) OR (${within(f.end, f.endTimed)})`
      : within(f.start || f.end, f.start ? f.startTimed : f.endTimed);
    // A repeating item is read whenever it began by the last day; its
    // occurrences are worked out in the browser (ADR-0168).
    if (f.recurrence) {
      const [anchor, timed] = f.start ? [f.start, f.startTimed] : [f.end, f.endTimed];
      cond = `(${cond}) OR (${quote(f.recurrence)} IS NOT EMPTY AND ${quote(anchor)} <= ${last(timed)})`;
    }
    return `(type = ${quote(f.type)}${whom ? ` AND ${quote(f.assignee)} ${whom}` : ""} AND (${cond}))`;
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

// listQuery is the query the view reads its items with, written from what
// was chosen in words: the state, the words of the text, the labels, whom
// the items are assigned to, the query typed in the advanced field, and
// the period of the view. It is empty when no type has a date field to
// place items by (or none is assigned by a field, when that is asked
// for), and nothing is read then. "Not finished" holds for items without
// a workflow too: they are never done.
export function listQuery(state, fields, p) {
  const pc = periodCondition(fields, p.from, p.to, state.who);
  if (!pc) return "";
  const conds = [];
  if (state.status === "open") conds.push("finished != true");
  if (state.status === "done") conds.push("finished = true");
  const words = String(state.text || "").trim();
  if (words) conds.push(`text ~ ${quote(words)}`);
  if (state.labels.length) conds.push(`label IN (${state.labels.map(quote).join(", ")})`);
  const own = withoutOrder(state.q);
  if (own) conds.push(`(${own})`);
  conds.push(`(${pc})`);
  return conds.join(" AND ");
}

// moment is a point in time as milliseconds, or null for none.
function moment(value) {
  const ms = typeof value === "string" ? Date.parse(value) : NaN;
  return Number.isNaN(ms) ? null : ms;
}

// localDayOf is the day a point in time falls on where the reader is.
export function localDayOf(value) {
  const ms = moment(value);
  return ms === null ? null : todayDay(new Date(ms));
}

// entries turns items into entries, each by the fields of its type: an
// item with a start and an end is a span over both days and those
// between; an item with one date, or with an end before its start, stands
// on one day (the end, when it has both: the date that is promised), as a
// milestone stands on the roadmap. An entry keeps the points in time it
// stands by, in at.from and at.to, null for a day without a time.
//
// A repeating item (ADR-0168) gives an entry per occurrence in the days
// from range.from to range.to, when a range is given; each knows the day
// it falls on by its rule (occurrence), whether it was moved, its rule and
// the field that holds it. A rule that cannot be read places the item once.
export function entries(items, fields, range) {
  const byType = new Map(fields.map((f) => [f.type, f]));
  const out = [];
  for (const it of items) {
    const f = byType.get(it.type);
    if (!f) continue;
    const timed = (name) => (name === f.start ? f.startTimed : name === f.end ? f.endTimed : false);
    const read = (value, name) => (timed(name) ? localDayOf(value) : parseDay(value));
    for (const b of schedule([it], f, read).bars) {
      const props = it.properties || {};
      const ms = (name) => (name && timed(name) ? moment(props[name]) : null);
      // A one-day entry stands by the end it has, else by its start.
      const one = f.end && read(props[f.end], f.end) !== null ? f.end : f.start;
      const at = b.milestone ? { from: ms(one), to: ms(one) } : { from: ms(f.start), to: ms(f.end) };
      const base = { ...b, start: f.start, end: f.end, startTimed: f.startTimed, endTimed: f.endTimed, at };
      const rule = f.recurrence ? props[f.recurrence] : "";
      let parsed = null;
      try { parsed = rule && range ? parseRule(rule) : null; } catch { parsed = null; }
      if (!parsed) {
        out.push(base);
        continue;
      }
      const [first, last] = b.milestone ? [one, ""] : [f.start, f.end];
      const isTimed = timed(first) || Boolean(last && timed(last));
      // A day wider on either side covers every zone; the days drawn pick
      // out what belongs to them.
      const occ = expand(parsed, props[first], last ? props[last] : "", isTimed, (range.from - 1) * DAY, (range.to + 2) * DAY);
      for (const o of occ) {
        const fromDay = isTimed ? localDayOf(new Date(o.start).toISOString()) : o.start / DAY;
        const toDay = isTimed ? localDayOf(new Date(o.end).toISOString()) : o.end / DAY;
        if (toDay < range.from || fromDay > range.to) continue;
        out.push({ ...base, from: fromDay, to: toDay, at: isTimed ? { from: o.start, to: o.end } : { from: null, to: null },
          occurrence: o.original, moved: o.moved, rule, recurrence: f.recurrence, tzid: parsed.tzid || "UTC", timedSeries: isTimed });
      }
    }
  }
  return out.sort(byDayAndTime);
}

// externalEntries turns the entries of a person's calendars of other
// services (ADR-0172), as /api/me/calendars/events answers them, into
// entries of the calendar: an all-day one over its days, its end being
// the day after its last, one with a time on the days it touches where
// the reader is. They stand beside the items but are none: their item has
// no key, they cannot be moved, and external names their calendar.
export function externalEntries(events, sources) {
  const byId = new Map((sources || []).map((s) => [s.id, s]));
  const out = [];
  for (const ev of events || []) {
    const src = byId.get(ev.source);
    if (!src) continue;
    let from, to, at;
    if (ev.allDay) {
      from = parseDay(ev.start);
      const end = parseDay(ev.end);
      if (from === null) continue;
      to = end !== null && end > from ? end - 1 : from;
      at = { from: null, to: null };
    } else {
      const start = moment(ev.start);
      if (start === null) continue;
      const stop = moment(ev.end);
      const end = stop !== null && stop > start ? stop : start;
      from = todayDay(new Date(start));
      // An end at midnight closes the day before, not the next one.
      to = end > start ? todayDay(new Date(end - 1)) : from;
      at = { from: start, to: end };
    }
    out.push({ item: { id: "", key: "", title: ev.title || "" }, from, to, at,
      external: { source: src.id, name: src.name, colour: src.colour, location: ev.location || "" } });
  }
  return out.sort(byDayAndTime);
}

// byDayAndTime orders entries by their first day, then those without a
// time before those with one, by their time, then as the roadmap orders.
export function byDayAndTime(a, b) {
  return a.from - b.from || (a.at?.from ?? -Infinity) - (b.at?.from ?? -Infinity) || byStart(a, b);
}

// shiftValue moves a value of a field by n days: a day to the day n days
// on, a point in time to the same time of day n days on where the reader
// is, so a change of daylight saving time does not move it by an hour.
export function shiftValue(value, timed, n) {
  if (timed) {
    const ms = moment(value);
    if (ms === null) return null;
    const d = new Date(ms);
    d.setDate(d.getDate() + n);
    return d.toISOString();
  }
  const day = parseDay(value);
  return day === null ? null : dayString(day + n);
}

// moves are the property changes that move an entry by n days: each of
// its start and end that the item holds a value in, both, so a span keeps
// its length. Nothing moves by 0 days.
export function moves(entry, n) {
  if (!n) return [];
  const props = entry.item.properties || {};
  const out = [];
  for (const [name, timed] of [[entry.start, entry.startTimed], [entry.end, entry.endTimed]]) {
    if (!name || out.some((m) => m.field === name)) continue;
    const value = shiftValue(props[name], timed, n);
    if (value !== null) out.push({ field: name, value });
  }
  return out;
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
  segs.sort((x, y) => x.col - y.col || y.span - x.span || byDayAndTime(x.entry, y.entry));
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

// dayAt is the day of a week under a point of its row: x is how far from
// the row's left edge, width the row's width; Monday is the first column.
export function dayAt(monday, x, width) {
  if (!(width > 0)) return monday;
  return monday + Math.min(6, Math.max(0, Math.floor((x / width) * 7)));
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

// tasksTab is what the tasks page hands to the calendar for one of its
// tabs (ADR-0049), as choices a person reads, not as a query: what is not
// finished, assigned to the reader, to nobody, or to anyone.
export function tasksTab(tab) {
  return { status: "open", who: tab === "mine" ? "me" : tab === "unassigned" ? "none" : "" };
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
    who: WHO.includes(get("who")) ? get("who") : "",
    status: STATUS.includes(get("status")) ? get("status") : "",
    text: get("text"),
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
  for (const k of ["space", "type", "start", "end", "who", "status", "text", "q"]) if (state[k]) p.set(k, state[k]);
  if (state.labels?.length) p.set("labels", state.labels.join(","));
  const q = p.toString();
  return "#/calendar" + (q ? "?" + q : "");
}

// DAY_MINUTES is a day's length on the week's axis of hours.
export const DAY_MINUTES = 24 * 60;

// minutesOf is how far into its day, where the reader is, a point in time
// lies, in minutes.
function minutesOf(ms) {
  const d = new Date(ms);
  return d.getHours() * 60 + d.getMinutes();
}

// timedSpan is where an entry stands on the week's axis of hours: from and
// to in minutes of its day. Only an entry of one day with a time stands
// there; it reaches to its end's time on that day, or an hour on when it
// has none, never past midnight. Others stand above the hours, with the
// days without a time. null means it has no place on the axis.
export function timedSpan(entry) {
  if (entry.from !== entry.to || entry.at?.from === null || entry.at?.from === undefined) return null;
  const from = minutesOf(entry.at.from);
  let to = entry.at.to !== null && entry.at.to > entry.at.from ? minutesOf(entry.at.to) : from + 60;
  if (to <= from) to = from + 60;
  return { from, to: Math.min(to, DAY_MINUTES) };
}

// dayColumns lays the timed entries of one day side by side where they
// overlap: each gets a column and the number of columns of the entries it
// overlaps with, so their widths share the day. Entries are taken in the
// order of their start, then the longer first.
export function dayColumns(list) {
  const items = list.map((entry) => ({ entry, span: timedSpan(entry) })).filter((x) => x.span)
    .sort((a, b) => a.span.from - b.span.from || b.span.to - a.span.to || byStart(a.entry, b.entry));
  const out = [];
  let group = [], ends = [], groupEnd = -1;
  const close = () => {
    for (const x of group) x.cols = ends.length;
    out.push(...group);
    group = [];
    ends = [];
  };
  for (const x of items) {
    if (x.span.from >= groupEnd) close();
    let col = ends.findIndex((end) => end <= x.span.from);
    if (col < 0) col = ends.length;
    ends[col] = x.span.to;
    x.col = col;
    group.push(x);
    groupEnd = Math.max(groupEnd, x.span.to);
  }
  close();
  return out;
}

// minutesAt is the time of day at a height of the axis of hours, rounded
// to a quarter of an hour and kept within the day.
export function minutesAt(y, height, step = 15) {
  if (!(height > 0)) return 0;
  const m = Math.round(((y / height) * DAY_MINUTES) / step) * step;
  return Math.min(DAY_MINUTES - step, Math.max(0, m));
}

// retime are the property changes that move a timed entry to the day n
// days on at a time of day, in minutes, where the reader is. The point in
// time it stands by goes there; every other date of the item moves by as
// much, so a meeting keeps its length.
export function retime(entry, n, minutes) {
  const anchor = entry.at?.from;
  if (anchor === null || anchor === undefined) return moves(entry, n);
  const target = new Date(anchor);
  target.setDate(target.getDate() + n);
  target.setHours(Math.floor(minutes / 60), minutes % 60, 0, 0);
  const delta = target.getTime() - anchor;
  if (!delta) return [];
  const props = entry.item.properties || {};
  const out = [];
  for (const [name, timed] of [[entry.start, entry.startTimed], [entry.end, entry.endTimed]]) {
    if (!name || out.some((m) => m.field === name)) continue;
    const value = props[name];
    if (timed) {
      const ms = typeof value === "string" ? Date.parse(value) : NaN;
      if (!Number.isNaN(ms)) out.push({ field: name, value: new Date(ms + delta).toISOString() });
    } else {
      const moved = shiftValue(value, false, n);
      if (moved !== null) out.push({ field: name, value: moved });
    }
  }
  return out;
}

// occurrenceChanges are the property changes that move one occurrence of a
// repeating entry by n days, and, for one with a time, to a time of day in
// minutes where the reader is: an exception in its rule (ADR-0168). The
// series stays as it is.
export function occurrenceChanges(entry, n, minutes) {
  let target;
  if (!entry.timedSeries) {
    target = dayString(entry.from + n);
  } else if (minutes === undefined) {
    // The same time of day in the rule's zone, n days on.
    const p = localIn(entry.at.from, entry.tzid);
    target = zoneTime(zoned(p.year, p.month, p.day + n, p.hour, p.minute, 0, entry.tzid), entry.tzid);
  } else {
    const d = new Date((entry.from + n) * DAY);
    const local = new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), Math.floor(minutes / 60), minutes % 60).getTime();
    target = zoneTime(local, entry.tzid);
  }
  // Back at the time its rule gives it, an occurrence is no exception.
  if (entry.timedSeries) {
    const props = entry.item.properties || {};
    const anchor = Date.parse(props[entry.start] || props[entry.end]);
    if (!Number.isNaN(anchor) && target === entry.occurrence + zoneTime(anchor, entry.tzid).slice(10)) target = entry.occurrence;
  }
  const value = withException(entry.rule, entry.occurrence, target);
  return value === entry.rule ? [] : [{ field: entry.recurrence, value }];
}

// seriesChanges are the property changes that move a whole series by n
// days, and, with minutes, to that time of day where the reader is: its
// dates as any entry's move them, and its exceptions with it, so they keep
// naming the same occurrences.
export function seriesChanges(entry, n, minutes) {
  const dates = minutes === undefined ? moves(entry, n) : retime(entry, n, minutes);
  if (!dates.length) return [];
  return n ? [...dates, { field: entry.recurrence, value: shifted(entry.rule, n) }] : dates;
}

// cancelChanges are the property changes that cancel one occurrence.
export function cancelChanges(entry) {
  return [{ field: entry.recurrence, value: withException(entry.rule, entry.occurrence, null) }];
}
