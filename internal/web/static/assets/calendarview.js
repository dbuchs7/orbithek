// The calendar of the items' dates (ADR-0166): readable items with date
// fields on the days of a month, a week or an agenda. Nothing is kept: the
// view is computed in the browser from the query listing (ADR-0136), which
// reads only the items of the period shown, and its whole state is in the
// address, so a link shows the same calendar to anyone who may read the
// same items (D10). The layout is calendar.js; this module only draws it.
// Entries are links to their items; colours are the look's tokens
// (style.css), and shape and text, not colour alone, tell a finished, an
// overdue and a misdated item.
//
// Where the reader may edit, an entry is moved to another day by dragging
// it there, or by Shift and an arrow key (a day left and right, a week up
// and down). Each move is a validated change of the item's date fields
// through the API (D3), a span's start and end alike; the calendar is then
// read again, so it shows what the server kept.
import { h } from "./dom.js";
import { t, language } from "./i18n.js";
import { get, put } from "./api.js";
import { finished } from "./taskparts.js";
import { dateFields, isoWeek, normalLabel, todayDay } from "./timeline.js";
import { VIEWS, address, agendaDays, dayAt, entries, listQuery, moves, overdue, period, readState, shift, typeFields, weekRows, withoutOrder } from "./calendar.js";

const DAY = 86400000;
const MONDAY = Date.UTC(2024, 0, 1) / DAY; // 1 January 2024 was a Monday.

// The names of the views, with their keys written out so the catalogue's
// tests find them (ADR-0017).
const viewName = (v) => ({ month: t("calendar.viewMonth"), week: t("calendar.viewWeek"), agenda: t("calendar.viewAgenda") })[v];

// fmt writes a day number in the UI language; days are counted in UTC.
const fmt = (day, opts) => new Date(day * DAY).toLocaleDateString(language() || undefined, { timeZone: "UTC", ...opts });
const dateText = (day) => fmt(day, { day: "numeric", month: "short", year: "numeric" });
const weekdays = (style) => [0, 1, 2, 3, 4, 5, 6].map((i) => fmt(MONDAY + i, { weekday: style }));
// timeText writes a point in time's time of day where the reader is.
const timeText = (ms) => new Date(ms).toLocaleTimeString(language() || undefined, { hour: "2-digit", minute: "2-digit" });
// whenText is a day, with its time when the entry stands by one there.
const whenText = (day, ms) => (ms === null || ms === undefined ? dateText(day) : `${dateText(day)} ${timeText(ms)}`);

// The keys that move an entry with Shift held, and by how many days.
const moveKeys = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 };

const field = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);

// heading names the period shown: a month with its year, or a week with
// its number and days.
function heading(p) {
  if (p.view === "week") return t("calendar.weekOf", { n: isoWeek(p.first).week, from: dateText(p.first), to: dateText(p.last) });
  return fmt(p.first, { month: "long", year: "numeric" });
}

// entryTitle tells everything about an entry in words: its item, its
// days, its state, and whether it is finished, overdue or misdated; it is
// the entry's accessible name.
function entryTitle(e, done, late) {
  const it = e.item;
  let when;
  if (e.from === e.to && e.at.from === e.at.to) when = whenText(e.from, e.at.from);
  // Within one day, the times from and to: 11 Nov 2026, 14:30–16:00.
  else if (e.from === e.to && e.at.from !== null && e.at.to !== null) when = `${dateText(e.from)}, ${timeText(e.at.from)}–${timeText(e.at.to)}`;
  else when = t("calendar.fromTo", { from: whenText(e.from, e.at.from), to: whenText(e.to, e.at.to) });
  const parts = [`${it.key} ${it.title}`, when];
  if (it.state) parts.push(it.state);
  if (done) parts.push(t("calendar.finished"));
  if (late) parts.push(t("calendar.overdue"));
  if (e.inverted) parts.push(t("calendar.inverted", { start: e.start, end: e.end }));
  return parts.join(" · ");
}

// entryEl draws an entry as a link to its item. A segment that goes on
// before or after its week says so by its open ends.
function entryEl(e, ctx, seg) {
  const done = ctx.done(e.item);
  const late = overdue(e, ctx.today, done);
  const cls = ["cal-entry", e.from !== e.to ? "cal-span" : "", done ? "cal-done" : "", late ? "cal-overdue" : "", e.inverted ? "cal-inverted" : "",
    seg?.before ? "cal-before" : "", seg?.after ? "cal-after" : ""].filter(Boolean).join(" ");
  const title = entryTitle(e, done, late);
  const movable = ctx.canMove(e.item);
  // The time stands where the entry begins: not on a week it goes on in.
  const time = e.at.from !== null && !seg?.before ? h("span", { class: "cal-time", text: timeText(e.at.from) }) : null;
  const el = h("a", { class: cls + (movable ? " cal-movable" : ""), href: "#/item/" + encodeURIComponent(e.item.key), title, "aria-label": title, "data-key": e.item.key,
    draggable: movable ? "true" : null },
  late ? h("span", { class: "cal-mark", "aria-hidden": "true", text: "!" }) : null,
  time, time ? " " : null,
  h("span", { class: "cal-key", text: e.item.key }), " ", h("span", { class: "cal-title", text: e.item.title }));
  if (seg) el.style.gridColumn = `${seg.col + 1} / span ${seg.span}`;
  if (movable) {
    el.addEventListener("dragstart", (ev) => {
      // The day it is grabbed at: a span moves by as many days as the
      // pointer does, wherever it is taken.
      const week = el.closest(".cal-week");
      const rect = week?.getBoundingClientRect();
      ctx.drag = { entry: e, from: week ? dayAt(Number(week.dataset.week), ev.clientX - rect.left, rect.width) : e.from };
      ev.dataTransfer.effectAllowed = "move";
      ev.dataTransfer.setData("text/plain", e.item.key);
      el.classList.add("cal-dragging");
    });
    el.addEventListener("dragend", () => {
      ctx.drag = null;
      el.classList.remove("cal-dragging");
      ctx.clearDrop();
    });
    el.addEventListener("keydown", (ev) => {
      const n = moveKeys[ev.key];
      if (!ev.shiftKey || ev.altKey || ev.ctrlKey || ev.metaKey || !n) return;
      ev.preventDefault();
      ctx.move(e, n);
    });
  }
  return el;
}

// weekEl draws one week as a grid of seven columns: the days behind, the
// entries in rows over them, a span across its days.
function weekEl(monday, p, list, ctx, big) {
  const rows = weekRows(list, monday);
  const week = h("div", { class: "cal-week" + (big ? " cal-week-big" : ""), "data-week": String(monday) });
  week.style.gridTemplateRows = `auto repeat(${rows.length}, auto) minmax(0.75rem, 1fr)`;
  for (let i = 0; i < 7; i++) {
    const day = monday + i;
    const out = day < p.first || day > p.last;
    const cls = ["cal-day", out ? "cal-out" : "", day === ctx.today ? "cal-today" : "", i >= 5 ? "cal-weekend" : ""].filter(Boolean).join(" ");
    // The day's ground spans every row; its date stands in the first row,
    // which the entries leave free.
    const ground = h("div", { class: cls, "aria-hidden": "true" });
    ground.style.gridColumn = String(i + 1);
    const date = h("div", { class: cls.replace("cal-day", "cal-head"), "aria-current": day === ctx.today ? "date" : null },
      h("span", { class: "cal-date", "aria-hidden": "true", text: big ? fmt(day, { weekday: "short", day: "numeric", month: "short" }) : fmt(day, { day: "numeric" }) }),
      h("span", { class: "visually-hidden", text: fmt(day, { weekday: "long", day: "numeric", month: "long", year: "numeric" }) }));
    date.style.gridColumn = String(i + 1);
    week.append(ground, date);
  }
  rows.forEach((row, r) => row.forEach((seg) => {
    const el = entryEl(seg.entry, ctx, seg);
    el.style.gridRow = String(r + 2);
    week.append(el);
  }));
  // A dragged entry drops on the day under the pointer; that day is lit
  // while it hovers there.
  const grounds = [...week.querySelectorAll(".cal-day")];
  const dayUnder = (ev) => {
    const rect = week.getBoundingClientRect();
    return dayAt(monday, ev.clientX - rect.left, rect.width);
  };
  const over = (ev) => {
    if (!ctx.drag) return;
    ev.preventDefault();
    ev.dataTransfer.dropEffect = "move";
    ctx.clearDrop();
    grounds[dayUnder(ev) - monday]?.classList.add("cal-drop");
  };
  // Both are cancelled, so every browser takes the week as a place to
  // drop on.
  week.addEventListener("dragenter", over);
  week.addEventListener("dragover", over);
  week.addEventListener("drop", (ev) => {
    if (!ctx.drag) return;
    ev.preventDefault();
    const { entry, from } = ctx.drag;
    ctx.drag = null;
    ctx.clearDrop();
    ctx.move(entry, dayUnder(ev) - from);
  });
  return week;
}

// gridEl draws a month in whole weeks under the names of the days, or a
// week with the names at its days.
function gridEl(p, list, ctx) {
  const big = p.view === "week";
  return h("div", { class: "cal-grid cal-view-" + p.view, id: "calendar-grid", role: "region", "aria-label": heading(p) },
    big ? null : h("div", { class: "cal-weekdays", "aria-hidden": "true" }, weekdays("short").map((n) => h("span", { text: n }))),
    p.weeks.map((monday) => weekEl(monday, p, list, ctx, big)));
}

// agendaEl lists the days of the period that hold entries, each with them.
function agendaEl(p, list, ctx) {
  const days = agendaDays(list, p.first, p.last);
  if (!days.length) return h("p", { class: "empty", id: "calendar-grid", text: t("calendar.none") });
  return h("ol", { class: "cal-agenda", id: "calendar-grid" }, days.map(({ day, entries: held }) =>
    h("li", { class: day === ctx.today ? "cal-today" : "", "aria-current": day === ctx.today ? "date" : null, "data-day": String(day) },
      h("h3", { text: fmt(day, { weekday: "long", day: "numeric", month: "long" }) }),
      h("ul", {}, held.map((e) => h("li", {}, entryEl(e, ctx)))))));
}

// queryError tells what is wrong with the query typed: its text, and the
// place of the mistake in the query typed when it lies there. The server
// counts the place in the whole query the view sent.
function queryError(err, sent, state) {
  const message = String(err.message).replace(/^at character \d+:\s*/, "");
  const own = withoutOrder(state.q);
  const start = own ? sent.indexOf(`(${own})`) + 1 : -1;
  if (start > 0 && Number.isInteger(err.at) && err.at >= start && err.at <= start + own.length) {
    const at = err.at - start;
    return { text: t("calendar.queryErrorAt", { n: at + 1, error: message }), at };
  }
  return { text: t("calendar.queryError", { error: message }), at: null };
}

// calendarView mounts the view into container. helpers are what app.js
// shares: readAll reads a listing up to the boards' limit, and
// favouritesFirst orders the spaces as every choice of a space does.
// after, when the calendar is drawn again after a move, says what to tell
// and which entry to put the focus on.
export async function calendarView(container, params, helpers, after = {}) {
  const { readAll, favouritesFirst } = helpers;
  const today = todayDay();
  const state = readState(params, today);
  const [{ types }, { spaces }] = await Promise.all([get("/api/types"), get("/api/spaces")]);
  const dated = types.filter((ty) => dateFields([ty]).length);
  const typeOf = new Map(types.map((ty) => [ty.name, ty]));
  const go = (change) => { location.hash = address({ ...state, ...change }, today); };
  const p = period(state.view, state.at);
  const fields = typeFields(types, state);
  const query = listQuery(state, fields, p);

  let items = [], truncated = false, failed = null;
  if (query) {
    const url = "/api/items?limit=100" + (state.space ? "&space=" + encodeURIComponent(state.space) : "") + "&query=" + encodeURIComponent(query);
    try {
      ({ items, truncated } = await readAll(url));
    } catch (err) {
      // A mistake in the query typed is told, and the view stays usable.
      if (err.status !== 400) throw err;
      failed = err;
    }
  }
  const list = entries(items, fields);
  const editable = new Set(spaces.filter((sp) => sp.role === "editor" || sp.role === "space-admin").map((sp) => sp.key));
  const ctx = {
    today,
    done: (it) => finished(typeOf.get(it.type)?.workflow, it.state),
    canMove: (it) => editable.has(it.space),
    drag: null,
    clearDrop: () => container.querySelectorAll(".cal-drop").forEach((el) => el.classList.remove("cal-drop")),
    // move changes the entry's dates by n days, then draws the calendar
    // again from what the server kept, telling what happened.
    move: async (e, n) => {
      const changes = moves(e, n);
      if (!changes.length) return;
      let notice;
      try {
        for (const c of changes) await put(`/api/items/${e.item.id}/properties/${encodeURIComponent(c.field)}`, { value: c.value });
        notice = { ok: true, text: t("calendar.moved", { key: e.item.key, date: dateText(e.from + n) }) };
      } catch (err) {
        notice = { ok: false, text: t("calendar.moveFailed", { key: e.item.key, error: err.message }) };
      }
      await calendarView(container, params, helpers, { notice, focus: e.item.key });
    },
  };

  const choices = dateFields(state.type ? dated.filter((ty) => ty.name === state.type) : dated);
  const select = (id, label, options, value, key) => field(label, h("select", { id, onchange: (ev) => go({ [key]: ev.target.value }) },
    options.map(([v, text]) => h("option", { value: v, text, selected: v === value }))));
  const labels = h("input", { id: "calendar-labels", type: "search", value: state.labels.join(", "), placeholder: t("calendar.labelsHint") });
  const q = h("input", { id: "calendar-q", type: "search", value: state.q, placeholder: t("calendar.queryHint"), spellcheck: "false" });
  const form = h("form", { class: "form-row cal-filter", id: "calendar-filter", onsubmit: (ev) => {
    ev.preventDefault();
    go({ labels: [...new Set(labels.value.split(",").map(normalLabel).filter(Boolean))], q: q.value.trim() });
  } },
  field(t("calendar.labels"), labels),
  field(t("calendar.query"), q),
  h("button", { type: "submit", class: "secondary", id: "calendar-apply", text: t("calendar.apply") }));

  const controls = h("div", { class: "card tl-controls cal-controls", id: "calendar-controls" },
    h("nav", { class: "tabs", "aria-label": t("calendar.view") }, VIEWS.map((v) =>
      h("a", { href: address({ ...state, view: v }, today), class: v === state.view ? "tab active" : "tab", id: "calendar-view-" + v, text: viewName(v) }))),
    h("div", { class: "form-row" },
      select("calendar-space", t("col.space"), [["", t("calendar.allSpaces")], ...favouritesFirst(spaces).map((sp) => [sp.key, `${sp.key} ${sp.name}`])], state.space, "space"),
      select("calendar-type", t("col.type"), [["", t("calendar.allTypes")], ...dated.map((ty) => [ty.name, ty.name])], state.type, "type"),
      select("calendar-start", t("calendar.startField"), [["", t("calendar.guessed")], ...choices.map((f) => [f, f])], state.start, "start"),
      select("calendar-end", t("calendar.endField"), [["", t("calendar.guessed")], ...choices.map((f) => [f, f])], state.end, "end")),
    form);

  const nav = h("div", { class: "cal-nav" },
    h("a", { class: "button secondary", id: "calendar-prev", href: address({ ...state, at: shift(state.view, state.at, -1) }, today), text: t("calendar.previous") }),
    h("a", { class: "button secondary", id: "calendar-today", href: address({ ...state, at: today }, today), text: t("calendar.today") }),
    h("a", { class: "button secondary", id: "calendar-next", href: address({ ...state, at: shift(state.view, state.at, 1) }, today), text: t("calendar.next") }),
    h("h2", { id: "calendar-period", "aria-live": "polite", text: heading(p) }));

  // A mistake in the query typed puts the cursor on its place there.
  const mistake = failed ? queryError(failed, query, state) : null;
  if (mistake) q.setAttribute("aria-invalid", "true");

  let content;
  if (!dated.length) content = h("p", { class: "empty", id: "calendar-grid", text: t("calendar.noDateFields") });
  else if (state.view === "agenda") content = agendaEl(p, list, ctx);
  else content = gridEl(p, list, ctx);

  container.replaceChildren(...[
    controls,
    mistake ? h("p", { class: "notice warning", id: "calendar-error", role: "alert", text: mistake.text }) : null,
    after.notice ? h("p", { class: after.notice.ok ? "notice" : "notice warning", id: "calendar-moved", role: after.notice.ok ? "status" : "alert", text: after.notice.text }) : null,
    truncated ? h("p", { class: "notice warning", text: t("board.truncated", { n: items.length }) }) : null,
    nav,
    content,
    h("p", { class: "muted tl-legend", text: t("calendar.legend") }),
  ].filter(Boolean));
  if (mistake?.at !== null && mistake?.at !== undefined) {
    q.focus();
    q.setSelectionRange(mistake.at, mistake.at);
  } else if (after.focus) {
    container.querySelector(`.cal-entry[data-key="${CSS.escape(after.focus)}"]`)?.focus();
  }
}
