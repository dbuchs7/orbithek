// The calendar of the items' dates (ADR-0166): readable items with date
// fields on the days of a month, a week or an agenda. Nothing is kept: the
// view is computed in the browser from the query listing (ADR-0136), which
// reads only the items of the period shown, and its whole state is in the
// address, so a link shows the same calendar to anyone who may read the
// same items (D10). The layout is calendar.js; this module only draws it.
// Entries are links to their items; colours are the look's tokens
// (style.css), and shape and text, not colour alone, tell a finished, an
// overdue and a misdated item.
import { h } from "./dom.js";
import { t, language } from "./i18n.js";
import { get } from "./api.js";
import { finished } from "./taskparts.js";
import { dateFields, isoWeek, normalLabel, todayDay } from "./timeline.js";
import { VIEWS, address, agendaDays, entries, listQuery, overdue, period, readState, shift, typeFields, weekRows, withoutOrder } from "./calendar.js";

const DAY = 86400000;
const MONDAY = Date.UTC(2024, 0, 1) / DAY; // 1 January 2024 was a Monday.

// The names of the views, with their keys written out so the catalogue's
// tests find them (ADR-0017).
const viewName = (v) => ({ month: t("calendar.viewMonth"), week: t("calendar.viewWeek"), agenda: t("calendar.viewAgenda") })[v];

// fmt writes a day number in the UI language; days are counted in UTC.
const fmt = (day, opts) => new Date(day * DAY).toLocaleDateString(language() || undefined, { timeZone: "UTC", ...opts });
const dateText = (day) => fmt(day, { day: "numeric", month: "short", year: "numeric" });
const weekdays = (style) => [0, 1, 2, 3, 4, 5, 6].map((i) => fmt(MONDAY + i, { weekday: style }));

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
  const when = e.from === e.to ? dateText(e.from) : t("calendar.fromTo", { from: dateText(e.from), to: dateText(e.to) });
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
  const el = h("a", { class: cls, href: "#/item/" + encodeURIComponent(e.item.key), title, "aria-label": title, "data-key": e.item.key },
    late ? h("span", { class: "cal-mark", "aria-hidden": "true", text: "!" }) : null,
    h("span", { class: "cal-key", text: e.item.key }), " ", h("span", { class: "cal-title", text: e.item.title }));
  if (seg) el.style.gridColumn = `${seg.col + 1} / span ${seg.span}`;
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
export async function calendarView(container, params, { readAll, favouritesFirst }) {
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
  const ctx = { today, done: (it) => finished(typeOf.get(it.type)?.workflow, it.state) };

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
    truncated ? h("p", { class: "notice warning", text: t("board.truncated", { n: items.length }) }) : null,
    nav,
    content,
    h("p", { class: "muted tl-legend", text: t("calendar.legend") }),
  ].filter(Boolean));
  if (mistake?.at !== null && mistake?.at !== undefined) {
    q.focus();
    q.setSelectionRange(mistake.at, mistake.at);
  }
}
