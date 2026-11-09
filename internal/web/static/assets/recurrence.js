// Repeating items in the browser (ADR-0167): reading and writing the rule
// of a field of the kind recurrence, and the occurrences it gives in a
// stretch of time. The server reads the same rule (internal/item/
// recurrence.go) for the calendar feed; both are tested with the same
// cases (internal/item/testdata/recurrence-cases.json), so a calendar in
// the browser and one subscribed to show the same occurrences.
//
// A rule is parts separated by semicolons: FREQ (DAILY, WEEKLY, MONTHLY,
// YEARLY), INTERVAL, BYDAY (MO … SU, weekly only), UNTIL (a day) or COUNT,
// TZID (the zone a time of day is kept in), EXDATE (days of cancelled
// occurrences) and MOVE (day@new day, or day@new dayTHH:MM).
//
// Plain functions over plain data, so node --test runs them
// (internal/web/jstest/recurrence.test.mjs).

const DAY = 86400000;
const FREQS = ["DAILY", "WEEKLY", "MONTHLY", "YEARLY"];
export const WEEKDAYS = ["MO", "TU", "WE", "TH", "FR", "SA", "SU"];
const MAX_INTERVAL = 999, MAX_COUNT = 1000, MAX_EXCEPTIONS = 500, MAX_STEPS = 100000;

const isDay = (s) => /^\d{4}-\d{2}-\d{2}$/.test(s) && dayMs(s) !== null;
const isDayTime = (s) => /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(s) && isDay(s.slice(0, 10)) && Number(s.slice(11, 13)) < 24 && Number(s.slice(14, 16)) < 60;

// dayMs is a day YYYY-MM-DD as milliseconds at midnight UTC, or null.
function dayMs(s) {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(s));
  if (!m) return null;
  const ms = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  return new Date(ms).toISOString().slice(0, 10) === m[0] ? ms : null;
}
const dayText = (ms) => new Date(ms).toISOString().slice(0, 10);

// validZone tells whether the browser knows a time zone by that name.
export function validZone(tz) {
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: tz });
    return tz !== "Local";
  } catch {
    return false;
  }
}

// parseRule reads a rule; it throws an Error naming what is wrong, as the
// server refuses it.
export function parseRule(text) {
  const r = { freq: "", interval: 1, byDay: [], until: "", count: 0, tzid: "", exdates: [], moves: {} };
  const seen = new Set();
  for (const raw of String(text || "").trim().split(";")) {
    const part = raw.trim();
    if (!part) continue;
    const at = part.indexOf("=");
    const name = (at < 0 ? part : part.slice(0, at)).trim().toUpperCase();
    const value = at < 0 ? "" : part.slice(at + 1).trim();
    if (at < 0 || !value) throw new Error(`${part} is not NAME=value`);
    if (seen.has(name)) throw new Error(`${name} is given twice`);
    seen.add(name);
    switch (name) {
      case "FREQ":
        r.freq = value.toUpperCase();
        if (!FREQS.includes(r.freq)) throw new Error(`FREQ ${value} is none of ${FREQS.join(", ")}`);
        break;
      case "INTERVAL":
        r.interval = Number(value);
        if (!/^\d+$/.test(value) || r.interval < 1 || r.interval > MAX_INTERVAL) throw new Error(`INTERVAL ${value} is not a whole number from 1 to ${MAX_INTERVAL}`);
        break;
      case "BYDAY":
        for (const d of value.split(",")) {
          const wd = d.trim().toUpperCase();
          if (!WEEKDAYS.includes(wd)) throw new Error(`BYDAY ${d} is not a weekday (MO to SU)`);
          if (!r.byDay.includes(wd)) r.byDay.push(wd);
        }
        r.byDay.sort((a, b) => WEEKDAYS.indexOf(a) - WEEKDAYS.indexOf(b));
        break;
      case "UNTIL":
        if (!isDay(value)) throw new Error(`UNTIL ${value} is not a day YYYY-MM-DD`);
        r.until = value;
        break;
      case "COUNT":
        r.count = Number(value);
        if (!/^\d+$/.test(value) || r.count < 1 || r.count > MAX_COUNT) throw new Error(`COUNT ${value} is not a whole number from 1 to ${MAX_COUNT}`);
        break;
      case "TZID":
        if (!validZone(value)) throw new Error(`TZID ${value} is no known time zone`);
        r.tzid = value;
        break;
      case "EXDATE":
        for (const d of value.split(",").map((x) => x.trim())) {
          if (!isDay(d)) throw new Error(`EXDATE ${d} is not a day YYYY-MM-DD`);
          if (!r.exdates.includes(d)) r.exdates.push(d);
        }
        break;
      case "MOVE":
        for (const m of value.split(",").map((x) => x.trim())) {
          const [from, to, rest] = m.split("@");
          if (rest !== undefined || !isDay(from || "") || !(isDay(to || "") || isDayTime(to || ""))) throw new Error(`MOVE ${m} is not YYYY-MM-DD@YYYY-MM-DD or YYYY-MM-DD@YYYY-MM-DDTHH:MM`);
          r.moves[from] = to;
        }
        break;
      default:
        throw new Error(`${name} is not a part of a rule`);
    }
  }
  if (!r.freq) throw new Error("FREQ is missing");
  if (r.byDay.length && r.freq !== "WEEKLY") throw new Error("BYDAY belongs to a weekly rule only");
  if (r.until && r.count) throw new Error("UNTIL and COUNT cannot both end a rule");
  if (r.exdates.length > MAX_EXCEPTIONS || Object.keys(r.moves).length > MAX_EXCEPTIONS) throw new Error(`at most ${MAX_EXCEPTIONS} cancelled and ${MAX_EXCEPTIONS} moved occurrences`);
  for (const from of Object.keys(r.moves)) if (r.exdates.includes(from)) throw new Error(`the occurrence of ${from} is both cancelled and moved`);
  r.exdates.sort();
  return r;
}

// ruleText writes a rule in its one canonical form, as the server keeps it.
export function ruleText(r) {
  const parts = ["FREQ=" + r.freq];
  if (r.interval > 1) parts.push("INTERVAL=" + r.interval);
  if (r.byDay?.length) parts.push("BYDAY=" + [...r.byDay].sort((a, b) => WEEKDAYS.indexOf(a) - WEEKDAYS.indexOf(b)).join(","));
  if (r.until) parts.push("UNTIL=" + r.until);
  if (r.count) parts.push("COUNT=" + r.count);
  if (r.tzid) parts.push("TZID=" + r.tzid);
  if (r.exdates?.length) parts.push("EXDATE=" + [...r.exdates].sort().join(","));
  const moves = Object.keys(r.moves || {}).sort();
  if (moves.length) parts.push("MOVE=" + moves.map((k) => `${k}@${r.moves[k]}`).join(","));
  return parts.join(";");
}

// normalRule is a rule in its canonical form; it throws as parseRule does.
export function normalRule(text) {
  return ruleText(parseRule(text));
}

const formatters = new Map();
function partsIn(ms, tz) {
  let f = formatters.get(tz);
  if (!f) {
    f = new Intl.DateTimeFormat("en-US", { timeZone: tz, hourCycle: "h23", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" });
    formatters.set(tz, f);
  }
  const p = Object.fromEntries(f.formatToParts(new Date(ms)).map((x) => [x.type, x.value]));
  return { year: Number(p.year), month: Number(p.month), day: Number(p.day), hour: Number(p.hour) % 24, minute: Number(p.minute), second: Number(p.second) };
}

// localIn is the day and time a point in time shows in a zone.
export function localIn(ms, tz) {
  return partsIn(ms, tz || "UTC");
}

// zoned is the point in time a day and time of a zone names.
export function zoned(year, month, day, hour, minute, second, tz) {
  const wall = Date.UTC(year, month - 1, day, hour, minute, second);
  if (!tz || tz === "UTC") return wall;
  const offset = (ms) => {
    const p = partsIn(ms, tz);
    return Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute, p.second) - Math.floor(ms / 1000) * 1000;
  };
  let ms = wall - offset(wall);
  const second2 = offset(ms);
  if (wall - second2 !== ms) ms = wall - second2;
  return ms;
}

// zoneTime writes a point in time as the day and time a MOVE names it by
// in a zone: YYYY-MM-DDTHH:MM.
export function zoneTime(ms, tz) {
  const p = localIn(ms, tz);
  const two = (n) => String(n).padStart(2, "0");
  return `${p.year}-${two(p.month)}-${two(p.day)}T${two(p.hour)}:${two(p.minute)}`;
}

// days lists the days (milliseconds at midnight UTC) a rule falls on from
// its first day, up to horizon, in order.
function days(r, anchor, horizon) {
  const out = [];
  const step = r.interval;
  const a = new Date(anchor);
  if (r.freq === "DAILY") {
    for (let k = 0; k < MAX_STEPS; k++) {
      const d = anchor + k * step * DAY;
      if (d > horizon) break;
      out.push(d);
    }
  } else if (r.freq === "WEEKLY") {
    const wd = (a.getUTCDay() + 6) % 7;
    const chosen = r.byDay.length ? r.byDay.map((d) => WEEKDAYS.indexOf(d)) : [wd];
    const monday = anchor - wd * DAY;
    for (let k = 0; k * 7 < MAX_STEPS; k++) {
      const week = monday + 7 * step * k * DAY;
      if (week > horizon) break;
      for (const i of chosen) {
        const d = week + i * DAY;
        if (d >= anchor && d <= horizon) out.push(d);
      }
    }
  } else {
    const months = r.freq === "YEARLY" ? 12 * step : step;
    for (let k = 0; k < MAX_STEPS; k++) {
      const d = Date.UTC(a.getUTCFullYear(), a.getUTCMonth() + k * months, a.getUTCDate());
      if (d > horizon) break;
      // A month without the day is skipped, as iCalendar skips it.
      if (new Date(d).getUTCDate() === a.getUTCDate()) out.push(d);
    }
  }
  return out;
}

// expand lists the occurrences of a series that touch the stretch of time
// from (included) to to (excluded), both in milliseconds. start and end
// are the first occurrence as the API gives them (RFC 3339): for a series
// of days (timed false) each spans as many days as the first; for a series
// of points in time each keeps its time of day in the rule's zone (UTC
// when it names none). An occurrence has the day it falls on by the rule
// (original), its start and end in milliseconds, and whether it was moved.
export function expand(rule, start, end, timed, from, to) {
  const r = typeof rule === "string" ? parseRule(rule) : rule;
  const tz = timed ? r.tzid || "UTC" : "UTC";
  const s = Date.parse(start);
  if (Number.isNaN(s)) return [];
  const e = end ? Date.parse(end) : NaN;
  const length = !Number.isNaN(e) && e > s ? e - s : 0;
  const first = timed ? localIn(s, tz) : (() => { const d = new Date(s); return { year: d.getUTCFullYear(), month: d.getUTCMonth() + 1, day: d.getUTCDate(), hour: 0, minute: 0, second: 0 }; })();
  const anchor = Date.UTC(first.year, first.month - 1, first.day);
  const until = r.until ? dayMs(r.until) : null;
  const at = (dayStart, clock) => {
    const d = new Date(dayStart);
    if (!timed) return dayStart;
    let [h, m, sec] = [first.hour, first.minute, first.second];
    if (clock) [h, m, sec] = [Number(clock.slice(0, 2)), Number(clock.slice(3, 5)), 0];
    return zoned(d.getUTCFullYear(), d.getUTCMonth() + 1, d.getUTCDate(), h, m, sec, tz);
  };
  const out = [];
  let n = 0;
  for (const day of days(r, anchor, to + 366 * DAY)) {
    if (until !== null && day > until) break;
    n++;
    if (r.count && n > r.count) break;
    const original = dayText(day);
    if (r.exdates.includes(original)) continue;
    let startMs, moved = false;
    if (r.moves[original]) {
      const target = r.moves[original];
      const d = dayMs(target.slice(0, 10));
      if (d === null) continue;
      startMs = at(d, target.length > 10 ? target.slice(11) : "");
      moved = true;
    } else {
      startMs = at(day, "");
    }
    const endMs = startMs + length;
    // A day's end is the last day it covers; a time's end is when it is over.
    if (startMs >= to || endMs < from || (timed && length > 0 && endMs <= from)) continue;
    out.push({ original, start: startMs, end: endMs, moved });
  }
  return out.sort((x, y) => x.start - y.start);
}

// withException is a rule with one occurrence cancelled (to null) or moved
// to a new day, or day and time, YYYY-MM-DD[THH:MM]; a move back to the day
// it falls on takes the exception away.
export function withException(rule, original, to) {
  const r = typeof rule === "string" ? parseRule(rule) : structuredClone(rule);
  delete r.moves[original];
  r.exdates = r.exdates.filter((d) => d !== original);
  if (to === null) r.exdates.push(original);
  else if (to !== original) r.moves[original] = to;
  return ruleText(r);
}

// shifted is a rule whose exceptions moved by n days with its series, so
// they keep naming the same occurrences.
export function shifted(rule, n) {
  const r = typeof rule === "string" ? parseRule(rule) : structuredClone(rule);
  if (!n) return ruleText(r);
  const by = (d) => dayText(dayMs(d.slice(0, 10)) + n * DAY) + d.slice(10);
  r.exdates = r.exdates.map(by);
  r.moves = Object.fromEntries(Object.entries(r.moves).map(([k, v]) => [by(k), by(v)]));
  return ruleText(r);
}
