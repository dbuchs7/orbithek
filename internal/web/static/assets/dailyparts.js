// The parts of a person's daily notes (ADR-0205) that need no browser:
// which day an address names, the days around it, the month drawn beside
// the note, and the query that reads what was made or changed on a day.
// daily.js draws the view; these are plain functions, tested by
// internal/web/jstest/daily.test.mjs.
import { dayString, parseDay, todayDay } from "./timeline.js";
import { period } from "./calendar.js";

const DAY = 86400000;

// dayOfAddress is the day an address of the view names: #/daily/YYYY-MM-DD,
// or today, where the browser is, for #/daily alone. A day that is none
// (2026-02-30) is null.
export function dayOfAddress(arg, now = new Date()) {
  if (!arg) return todayDay(now);
  return parseDay(arg) !== null && /^\d{4}-\d{2}-\d{2}$/.test(arg) ? parseDay(arg) : null;
}

// dailyHref is the address of the daily note of a day number.
export function dailyHref(day) {
  return "#/daily/" + dayString(day);
}

// month is the month drawn beside a note: whole weeks, Monday first, as
// the calendar draws them (ADR-0166), with each day's number, whether it
// belongs to the month, has a note, is today or is the day shown.
export function month(day, noted, today) {
  const p = period("month", day);
  const has = new Set(noted);
  return {
    first: p.first, last: p.last, from: p.from, to: p.to,
    weeks: p.weeks.map((w) => Array.from({ length: 7 }, (_, i) => {
      const d = w + i;
      return { day: d, date: new Date(d * DAY).getUTCDate(), inMonth: d >= p.first && d <= p.last, noted: has.has(dayString(d)), today: d === today, shown: d === day };
    })),
  };
}

// dayBounds is the beginning of a day and of the next, where the browser
// is, as points in time (RFC 3339 in UTC): a day of 23 or 25 hours, when
// the clocks change, is still one day.
export function dayBounds(day) {
  const d = new Date(day * DAY);
  const at = (n) => new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + n).toISOString();
  return { from: at(0), to: at(1) };
}

// activityQuery selects, in the query language (ADR-0136), the items made
// on a day where the browser is, or last changed on it. An item changed on
// the day and again later counts by its later change: the query knows an
// item's last change only.
export function activityQuery(day) {
  const { from, to } = dayBounds(day);
  const within = (f) => `(${f} >= "${from}" AND ${f} < "${to}")`;
  return `${within("created")} OR ${within("updated")} ORDER BY updated DESC`;
}

// noteTitle is the title of a new note: its day written out in the
// language of the interface, as people name a day.
export function noteTitle(day, lang) {
  try {
    return new Intl.DateTimeFormat(lang, { weekday: "long", day: "numeric", month: "long", year: "numeric", timeZone: "UTC" }).format(new Date(day * DAY));
  } catch {
    return dayString(day);
  }
}

// monthTitle names a month in the language of the interface.
export function monthTitle(day, lang) {
  try {
    return new Intl.DateTimeFormat(lang, { month: "long", year: "numeric", timeZone: "UTC" }).format(new Date(day * DAY));
  } catch {
    return dayString(day).slice(0, 7);
  }
}

// weekdayNames are the short names of the days, Monday first.
export function weekdayNames(lang) {
  const monday = Date.UTC(2024, 0, 1); // 1 January 2024 was a Monday.
  return Array.from({ length: 7 }, (_, i) => {
    try {
      return new Intl.DateTimeFormat(lang, { weekday: "short", timeZone: "UTC" }).format(new Date(monday + i * DAY));
    } catch {
      return ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"][i];
    }
  });
}
