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
// read again, so it shows what the server kept. On the week's axis of
// hours, a meeting dropped at another hour goes there, keeping its length.
//
// What is shown is chosen in words: the room, the type, whom the items are
// assigned to, their state, words of their text and labels; a sentence
// says what that shows. The query language (ADR-0136) is offered only as
// an advanced field, with the query the view sends shown beside it.
import { h } from "./dom.js";
import { t, language } from "./i18n.js";
import { del, get, post, put } from "./api.js";
import { finished } from "./taskparts.js";
import { dateFields, dayString, isoWeek, normalLabel, todayDay } from "./timeline.js";
import { DAY_MINUTES, STATUS, VIEWS, WHO, address, agendaDays, appleServer, byDayAndTime, cancelChanges, dayAt, dayColumns, entries, externalEntries, listQuery, localDayOf, minutesAt,
  moves, occurrenceChanges, overdue, period, readState, retime, seriesChanges, shift, slotValues, timedSpan, typeFields, weekRows, withoutOrder } from "./calendar.js";

const DAY = 86400000;
const MONDAY = Date.UTC(2024, 0, 1) / DAY; // 1 January 2024 was a Monday.

// The names of the views, with their keys written out so the catalogue's
// tests find them (ADR-0017).
const viewName = (v) => ({ month: t("calendar.viewMonth"), week: t("calendar.viewWeek"), agenda: t("calendar.viewAgenda") })[v];
const whoName = (w) => ({ "": t("calendar.whoAny"), me: t("calendar.whoMe"), none: t("calendar.whoNone") })[w];
const statusName = (st) => ({ "": t("calendar.statusAny"), open: t("calendar.statusOpen"), done: t("calendar.statusDone") })[st];

// summary says in a sentence what the choices show (ADR-0166).
function summary(state, spaces) {
  const parts = [({ "": t("calendar.sumAll"), open: t("calendar.sumOpen"), done: t("calendar.sumDone") })[state.status]];
  if (state.type) parts.push(t("calendar.sumType", { type: state.type }));
  if (state.who === "me") parts.push(t("calendar.sumMine"));
  if (state.who === "none") parts.push(t("calendar.sumNobody"));
  const room = spaces.find((sp) => sp.key === state.space);
  parts.push(state.space ? t("calendar.sumRoom", { room: room ? room.name : state.space }) : t("calendar.sumRooms"));
  if (state.labels.length) parts.push(t("calendar.sumLabels", { labels: state.labels.join(", ") }));
  if (String(state.text || "").trim()) parts.push(t("calendar.sumText", { text: state.text.trim() }));
  if (withoutOrder(state.q)) parts.push(t("calendar.sumQuery"));
  return t("calendar.summary", { what: parts.join(", ") });
}

// fmt writes a day number in the UI language; days are counted in UTC.
const fmt = (day, opts) => new Date(day * DAY).toLocaleDateString(language() || undefined, { timeZone: "UTC", ...opts });
const dateText = (day) => fmt(day, { day: "numeric", month: "short", year: "numeric" });
const weekdays = (style) => [0, 1, 2, 3, 4, 5, 6].map((i) => fmt(MONDAY + i, { weekday: style }));
// timeText writes a point in time's time of day where the reader is.
const timeText = (ms) => new Date(ms).toLocaleTimeString(language() || undefined, { hour: "2-digit", minute: "2-digit" });
// whenText is a day, with its time when the entry stands by one there.
const whenText = (day, ms) => (ms === null || ms === undefined ? dateText(day) : `${dateText(day)} ${timeText(ms)}`);

// FILTERS_OPEN keeps, for a tab of the browser, whether the choices are
// unfolded.
const FILTERS_OPEN = "ot-calendar-filters-open";

// ask shows a modal dialog with a question and buttons, and resolves to
// the value of the button chosen, or null when it is closed otherwise.
function ask(title, text, choices) {
  return new Promise((resolve) => {
    const dialog = h("dialog", { class: "cal-dialog", id: "calendar-dialog", "aria-labelledby": "calendar-dialog-title" },
      h("h2", { id: "calendar-dialog-title", text: title }),
      h("p", { text }),
      h("p", { class: "actions" }, choices.map(([value, label, cls]) => h("button", { type: "button", class: cls || "secondary", "data-choice": value, text: label,
        onclick: () => { dialog.returnValue = value; dialog.close(); } }))));
    dialog.addEventListener("close", () => {
      dialog.remove();
      resolve(dialog.returnValue || null);
    });
    document.body.append(dialog);
    dialog.showModal();
  });
}

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
  if (e.occurrence) parts.push(e.moved ? t("calendar.movedOccurrence") : t("calendar.repeats"));
  return parts.join(" · ");
}

// entryEl draws an entry as a link to its item. A segment that goes on
// before or after its week says so by its open ends.
function entryEl(e, ctx, seg) {
  if (e.external) return externalEl(e, seg);
  const done = ctx.done(e.item);
  // Only work can be late: an item of a type with a workflow, not finished.
  const late = ctx.hasWorkflow(e.item) && overdue(e, ctx.today, done);
  const cls = ["cal-entry", e.from !== e.to ? "cal-span" : "", done ? "cal-done" : "", late ? "cal-overdue" : "", e.inverted ? "cal-inverted" : "",
    seg?.before ? "cal-before" : "", seg?.after ? "cal-after" : ""].filter(Boolean).join(" ");
  const title = entryTitle(e, done, late);
  const movable = ctx.canMove(e.item);
  // The time stands where the entry begins: not on a week it goes on in.
  const time = e.at.from !== null && !seg?.before ? h("span", { class: "cal-time", text: timeText(e.at.from) }) : null;
  const el = h("a", { class: cls + (movable ? " cal-movable" : ""), href: "#/item/" + encodeURIComponent(e.item.key), title, "aria-label": title, "data-key": e.item.key,
    draggable: movable ? "true" : null },
  late ? h("span", { class: "cal-mark", "aria-hidden": "true", text: "!" }) : null,
  e.occurrence ? h("span", { class: "cal-repeat", "aria-hidden": "true", text: e.moved ? "↷" : "↻" }) : null,
  time, time ? " " : null,
  h("span", { class: "cal-key", text: e.item.key }), " ", h("span", { class: "cal-title", text: e.item.title }));
  if (seg) el.style.gridColumn = `${seg.col + 1} / span ${seg.span}`;
  if (movable) {
    el.addEventListener("dragstart", (ev) => {
      // The day it is grabbed at: a span moves by as many days as the
      // pointer does, wherever it is taken.
      const week = el.closest(".cal-week");
      const rect = el.getBoundingClientRect();
      ctx.drag = { entry: e, from: week?.dayAt ? week.dayAt(ev.clientX) : e.from, offsetY: ev.clientY - rect.top };
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
      // Delete cancels one occurrence of a series, after asking.
      if (e.occurrence && (ev.key === "Delete" || ev.key === "Backspace") && !ev.altKey && !ev.ctrlKey && !ev.metaKey) {
        ev.preventDefault();
        ctx.cancel(e);
        return;
      }
      const n = moveKeys[ev.key];
      if (!ev.shiftKey || ev.altKey || ev.ctrlKey || ev.metaKey || !n) return;
      ev.preventDefault();
      ctx.move(e, n);
    });
  }
  return el;
}

// whenOf says when an entry is, in words.
function whenOf(e) {
  if (e.from === e.to && e.at.from === e.at.to) return whenText(e.from, e.at.from);
  if (e.from === e.to && e.at.from !== null && e.at.to !== null) return `${dateText(e.from)}, ${timeText(e.at.from)}–${timeText(e.at.to)}`;
  return t("calendar.fromTo", { from: whenText(e.from, e.at.from), to: whenText(e.to, e.at.to) });
}

// swatch is a calendar's colour as a small mark beside its name.
function swatch(colour) {
  const el = h("span", { class: "cal-swatch", "aria-hidden": "true" });
  el.style.setProperty("--cal-colour", colour || "var(--accent)");
  return el;
}

// externalEl draws an entry of a calendar of another service (ADR-0174):
// in its calendar's colour, with its time and title, read only; a click
// tells what is known of it, as it is no item to open.
function externalEl(e, seg) {
  const x = e.external;
  const name = e.item.title || t("extcal.untitled");
  const title = [name, whenOf(e), x.location, t("extcal.of", { name: x.name })].filter(Boolean).join(" · ");
  const cls = ["cal-entry", "cal-external", e.from !== e.to ? "cal-span" : "", seg?.before ? "cal-before" : "", seg?.after ? "cal-after" : ""].filter(Boolean).join(" ");
  const time = e.at.from !== null && !seg?.before ? h("span", { class: "cal-time", text: timeText(e.at.from) }) : null;
  const el = h("button", { type: "button", class: cls, title, "aria-label": title, "data-source": x.source },
    swatch(x.colour), time, time ? " " : null, h("span", { class: "cal-title", text: name }));
  el.style.setProperty("--cal-colour", x.colour || "var(--accent)");
  el.addEventListener("click", () => ask(name, [whenOf(e), x.location, t("extcal.of", { name: x.name })].filter(Boolean).join(" · "),
    [["", t("common.close"), "primary"]]));
  return el;
}

// weekEl draws one week as a grid of seven columns: the days behind, the
// entries in rows over them, a span across its days. In the week view (big)
// a column at the left names the row of entries without a time, and the
// entries with a time stand on the axis of hours below instead.
function weekEl(monday, p, list, ctx, big) {
  const rows = weekRows(big ? list.filter((e) => !timedSpan(e)) : list, monday);
  const off = big ? 1 : 0;
  const week = h("div", { class: "cal-week" + (big ? " cal-week-big" : ""), "data-week": String(monday) });
  week.style.gridTemplateRows = `auto repeat(${rows.length}, auto) minmax(${big ? "2rem" : "0.75rem"}, 1fr)`;
  if (big) {
    const gutter = h("div", { class: "cal-gutter", "aria-hidden": "true" }, h("span", { text: t("calendar.allDay") }));
    week.append(gutter);
  }
  for (let i = 0; i < 7; i++) {
    const day = monday + i;
    const out = day < p.first || day > p.last;
    const cls = ["cal-day", out ? "cal-out" : "", day === ctx.today ? "cal-today" : "", i >= 5 ? "cal-weekend" : ""].filter(Boolean).join(" ");
    // The day's ground spans every row; its date stands in the first row,
    // which the entries leave free.
    const ground = h("div", { class: cls, "aria-hidden": "true" });
    ground.style.gridColumn = String(i + 1 + off);
    const date = h("div", { class: cls.replace("cal-day", "cal-head"), "aria-current": day === ctx.today ? "date" : null },
      h("span", { class: "cal-date", "aria-hidden": "true", text: big ? fmt(day, { weekday: "short", day: "numeric", month: "short" }) : fmt(day, { day: "numeric" }) }),
      h("span", { class: "visually-hidden", text: fmt(day, { weekday: "long", day: "numeric", month: "long", year: "numeric" }) }));
    date.style.gridColumn = String(i + 1 + off);
    week.append(ground, date);
  }
  rows.forEach((row, r) => row.forEach((seg) => {
    const el = entryEl(seg.entry, ctx, seg);
    el.style.gridColumn = `${seg.col + 1 + off} / span ${seg.span}`;
    el.style.gridRow = String(r + 2);
    week.append(el);
  }));
  // A dragged entry drops on the day under the pointer; that day is lit
  // while it hovers there. The days are measured from their own grounds,
  // so the column at the left of the week view counts for none.
  const grounds = [...week.querySelectorAll(".cal-day")];
  week.dayAt = (clientX) => {
    const first = grounds[0].getBoundingClientRect(), last = grounds[6].getBoundingClientRect();
    return dayAt(monday, clientX - first.left, last.right - first.left);
  };
  const dayUnder = (ev) => week.dayAt(ev.clientX);
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
  // A double-click on a day, not on an entry, makes a new item there.
  if (ctx.canCreate) {
    week.addEventListener("dblclick", (ev) => {
      if (ev.target.closest(".cal-entry")) return;
      window.getSelection()?.removeAllRanges();
      ctx.create(dayUnder(ev));
    });
  }
  return week;
}

// hoursEl draws the week's axis of hours: a column of hours at the left,
// then a column per day in which the entries of that day with a time stand
// from their start to their end, side by side where they overlap. A line
// marks the time now on today. An entry dropped there goes to that day and
// time, rounded to a quarter of an hour.
function hoursEl(monday, list, ctx) {
  const labels = h("div", { class: "cal-hour-labels", "aria-hidden": "true" },
    Array.from({ length: 24 }, (_, hour) => h("span", { text: timeText(new Date(2024, 0, 1, hour).getTime()) })));
  const days = [];
  for (let i = 0; i < 7; i++) {
    const day = monday + i;
    const col = h("div", { class: "cal-hour-day" + (day === ctx.today ? " cal-today" : "") + (i >= 5 ? " cal-weekend" : ""), "data-day": String(day) });
    for (const x of dayColumns(list.filter((e) => e.from === day))) {
      const el = entryEl(x.entry, ctx);
      el.classList.add("cal-timed");
      el.style.top = (x.span.from / DAY_MINUTES) * 100 + "%";
      el.style.height = ((x.span.to - x.span.from) / DAY_MINUTES) * 100 + "%";
      el.style.left = (x.col / x.cols) * 100 + "%";
      el.style.width = 100 / x.cols + "%";
      col.append(el);
    }
    if (day === ctx.today) {
      const now = new Date();
      const line = h("span", { class: "cal-now", title: t("calendar.now") });
      line.style.top = ((now.getHours() * 60 + now.getMinutes()) / DAY_MINUTES) * 100 + "%";
      col.append(line);
    }
    const target = (ev) => {
      const rect = col.getBoundingClientRect();
      return minutesAt(ev.clientY - rect.top - (ctx.drag?.offsetY || 0), rect.height);
    };
    const over = (ev) => {
      if (!ctx.drag) return;
      ev.preventDefault();
      ev.dataTransfer.dropEffect = "move";
      ctx.clearDrop();
      col.classList.add("cal-drop");
    };
    col.addEventListener("dragenter", over);
    col.addEventListener("dragover", over);
    col.addEventListener("drop", (ev) => {
      if (!ctx.drag) return;
      ev.preventDefault();
      const { entry, from } = ctx.drag;
      ctx.drag = null;
      ctx.clearDrop();
      // A meeting goes to the day and time it is dropped at; an entry
      // without a time of its own moves by days only.
      if (timedSpan(entry) || entry.at.from !== null) ctx.place(entry, day - entry.from, target(ev));
      else ctx.move(entry, day - from);
    });
    // A double-click on an hour makes a new item at that day and time,
    // rounded to a quarter of an hour.
    if (ctx.canCreate) {
      col.addEventListener("dblclick", (ev) => {
        if (ev.target.closest(".cal-entry")) return;
        window.getSelection()?.removeAllRanges();
        const rect = col.getBoundingClientRect();
        ctx.create(day, minutesAt(ev.clientY - rect.top, rect.height));
      });
    }
    days.push(col);
  }
  const grid = h("div", { class: "cal-hours" }, labels, days);
  return h("div", { class: "cal-hours-scroll", id: "calendar-hours", role: "region", "aria-label": t("calendar.hours"), tabindex: "0" }, grid);
}

// gridEl draws a month in whole weeks under the names of the days, or a
// week with the names at its days and its axis of hours.
function gridEl(p, list, ctx) {
  const big = p.view === "week";
  return h("div", { class: "cal-grid cal-view-" + p.view, id: "calendar-grid", role: "region", "aria-label": heading(p) },
    big ? null : h("div", { class: "cal-weekdays", "aria-hidden": "true" }, weekdays("short").map((n) => h("span", { text: n }))),
    p.weeks.map((monday) => weekEl(monday, p, list, ctx, big)),
    big ? hoursEl(p.weeks[0], list, ctx) : null);
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

// savedParams are the parameters of the view as it is saved: everything
// chosen but the day shown, so a saved view always opens on today.
const savedParams = (state, today) => address({ ...state, at: today }, today).replace(/^#\/calendar\??/, "");

// savedBar offers the views one saved (ADR-0168): open one, save the view
// as it is under a name, remove the one shown, and subscribe to it from a
// calendar program with a feed address of its own.
function savedBar(container, params, helpers, state, today, views, feeds, after) {
  const current = savedParams(state, today);
  const active = views.find((v) => v.params === current);
  const redraw = (next) => calendarView(container, params, helpers, next);
  const run = async (fn, okText) => {
    try {
      const next = await fn();
      await redraw({ notice: okText ? { ok: true, text: okText } : null, ...next });
    } catch (err) {
      await redraw({ notice: { ok: false, text: err.message } });
    }
  };
  const choose = h("select", { id: "calendar-saved-select", onchange: (ev) => {
    const v = views.find((x) => x.id === ev.target.value);
    location.hash = "#/calendar" + (v?.params ? "?" + v.params : "");
  } },
  h("option", { value: "", text: views.length ? t("calendar.savedNone") : t("calendar.savedEmpty"), selected: !active }),
  views.map((v) => h("option", { value: v.id, text: v.name, selected: v === active })));
  const name = h("input", { id: "calendar-saved-name", type: "text", maxlength: "80", placeholder: t("calendar.saveName"), value: active?.name || "" });
  const saveForm = h("form", { class: "cal-save-form", id: "calendar-save-form", hidden: true, onsubmit: (ev) => {
    ev.preventDefault();
    const n = name.value.trim();
    if (!n) return;
    // A name already used saves over that view; a new one makes a view.
    const same = views.find((v) => v.name === n);
    run(async () => {
      if (same) await put("/api/me/calendar-views/" + same.id, { name: n, params: current });
      else await post("/api/me/calendar-views", { name: n, params: current });
      return {};
    }, t("calendar.viewSaved", { name: n }));
  } },
  name,
  h("button", { type: "submit", class: "primary", id: "calendar-saved-save", text: t("calendar.save") }),
  h("button", { type: "button", class: "link", text: t("calendar.cancel"), onclick: () => { saveForm.hidden = true; saveButton.hidden = false; } }));
  const saveButton = h("button", { type: "button", class: "secondary", id: "calendar-saved-new", text: t("calendar.saveAs"), onclick: () => {
    saveForm.hidden = false;
    saveButton.hidden = true;
    name.focus();
    name.select();
  } });
  const remove = active ? h("button", { type: "button", class: "link danger", id: "calendar-saved-remove", text: t("calendar.deleteView"), onclick: async () => {
    const ok = await ask(t("calendar.deleteView"), t("calendar.deleteViewConfirm", { name: active.name }), [["yes", t("calendar.deleteView"), "danger"], ["", t("calendar.cancel"), "link"]]);
    if (ok === "yes") run(async () => { await del("/api/me/calendar-views/" + active.id); return {}; }, t("calendar.viewDeleted", { name: active.name }));
  } }) : null;
  // A server with feeds turned off offers none; a feed made before can
  // still be ended.
  const subscribe = !active ? null : !feeds
    ? h("span", { class: "cal-feed-state" }, h("span", { class: "muted", id: "calendar-feeds-off", text: t("calendar.feedsOff") + " " }),
      active.feed ? h("button", { type: "button", class: "link danger", id: "calendar-feed-revoke", text: t("calendar.feedRevoke"),
        onclick: () => run(async () => { await del(`/api/me/calendar-views/${active.id}/feed`); return {}; }, t("calendar.feedRevoked")) }) : null)
    : !active.feed
    ? h("button", { type: "button", class: "secondary", id: "calendar-feed-new", text: t("calendar.subscribe"),
      onclick: () => run(async () => ({ feed: await post(`/api/me/calendar-views/${active.id}/feed`, {}) })) })
    : h("span", { class: "cal-feed-state" },
      h("span", { class: "muted", text: t("calendar.feedActive", { date: new Date(active.feedCreated).toLocaleDateString(language() || undefined) }) + " " }),
      h("button", { type: "button", class: "link", id: "calendar-feed-renew", text: t("calendar.feedRenew"),
        onclick: () => run(async () => ({ feed: await post(`/api/me/calendar-views/${active.id}/feed`, {}) })) }), " · ",
      h("button", { type: "button", class: "link danger", id: "calendar-feed-revoke", text: t("calendar.feedRevoke"),
        onclick: () => run(async () => { await del(`/api/me/calendar-views/${active.id}/feed`); return {}; }, t("calendar.feedRevoked")) }));
  return h("div", { class: "cal-saved", id: "calendar-saved" },
    field(t("calendar.savedView"), choose), saveButton, saveForm, remove, subscribe);
}

// COLOURS are offered in turn to a calendar of another service.
const COLOURS = ["#3a7bd5", "#2e9d5b", "#d9822b", "#8e5bd9", "#d64553", "#1f9aa8", "#7a7f2a", "#c2569e"];

// othersBar shows one's calendars of other services (ADR-0174), which
// stand beside the wiki's: each in its colour, shown or hidden, removed;
// the accounts they come from, whose calendars one chooses and which one
// disconnects; and the ways to add one: an iCalendar address, a Google
// account, or an Apple iCloud or other CalDAV account. While the server
// has them turned off, it says so and only offers to remove them.
function othersBar(container, params, helpers, cals, after) {
  const { enabled, google } = cals;
  const sources = cals.sources || [], accounts = cals.accounts || [];
  if (!enabled && !sources.length && !accounts.length) return null;
  const redraw = (next) => calendarView(container, params, helpers, { others: true, ...next });
  const run = async (fn, okText) => {
    try {
      const next = await fn();
      await redraw({ notice: okText ? { ok: true, text: okText } : null, ...next });
    } catch (err) {
      await redraw({ notice: { ok: false, text: err.message } });
    }
  };
  const next = COLOURS[sources.length % COLOURS.length];
  const accountOf = (id) => accounts.find((a) => a.id === id);
  const confirmThen = async (title, text, label, fn, okText) => {
    if (await ask(title, text, [["yes", label, "danger"], ["", t("calendar.cancel"), "link"]]) === "yes") run(fn, okText);
  };

  const list = sources.length ? h("ul", { class: "cal-others-list", id: "calendar-others-list" }, sources.map((s) =>
    h("li", { "data-source": s.id },
      h("label", { class: "check" },
        h("input", { type: "checkbox", checked: !s.hidden, disabled: !enabled, "data-show-source": s.id, "aria-label": t("extcal.show", { name: s.name }),
          onchange: (ev) => run(async () => { await put("/api/me/calendars/" + s.id, { name: s.name, colour: s.colour, hidden: !ev.target.checked }); return {}; }) }),
        swatch(s.colour), h("span", { text: s.name })),
      h("span", { class: "muted", text: s.host || accountOf(s.account)?.name || "" }),
      h("button", { type: "button", class: "link danger", "data-remove-source": s.id, text: t("extcal.remove"),
        onclick: () => confirmThen(t("extcal.remove"), t("extcal.removeConfirm", { name: s.name }), t("extcal.remove"),
          async () => { await del("/api/me/calendars/" + s.id); return {}; }, t("extcal.removed", { name: s.name })) }))))
    : h("p", { class: "muted", id: "calendar-others-none", text: t("extcal.none") });

  // The calendars of an account, to choose which are shown.
  const pick = h("div", { class: "cal-others-pick", id: "calendar-others-pick" });
  const showPicker = (account, remote) => {
    const shown = new Set(sources.filter((s) => s.account === account.id).map((s) => s.calendar));
    pick.replaceChildren(
      h("h3", { text: t("extcal.pickTitle", { name: account.name }) }),
      remote.length ? h("ul", { class: "cal-others-list" }, remote.map((c, i) => h("li", {},
        swatch(c.colour || COLOURS[(sources.length + i) % COLOURS.length]), h("span", { text: c.name }),
        shown.has(c.id)
          ? h("span", { class: "muted", text: t("extcal.shown") })
          : h("button", { type: "button", class: "secondary", "data-add-calendar": c.id, text: t("extcal.showThis"),
            onclick: () => run(async () => {
              await post("/api/me/calendars", { kind: account.kind, account: account.id, calendar: c.id, name: c.name, colour: c.colour || COLOURS[(sources.length + i) % COLOURS.length] });
              // The list stays open, to show more of the account's calendars.
              return { picked: { account, calendars: remote } };
            }, t("extcal.added", { name: c.name })) }))))
        : h("p", { class: "muted", text: t("extcal.pickNone") }));
  };
  const choose = (account) => async () => {
    pick.replaceChildren(h("p", { class: "muted", text: t("extcal.reading") }));
    try {
      const { calendars } = await get(`/api/me/calendar-accounts/${account.id}/calendars`);
      showPicker(account, calendars);
    } catch (err) {
      pick.replaceChildren(h("p", { class: "notice warning", role: "alert", text: err.message }));
    }
  };
  const accountList = accounts.length ? h("ul", { class: "cal-others-accounts", id: "calendar-others-accounts" }, accounts.map((a) =>
    h("li", { "data-account": a.id },
      h("span", { text: (a.kind === "google" ? "Google" : t("extcal.caldav")) + ": " + a.name }),
      enabled ? h("button", { type: "button", class: "link", "data-choose-account": a.id, text: t("extcal.choose"), onclick: choose(a) }) : null,
      h("button", { type: "button", class: "link danger", "data-disconnect": a.id, text: t("extcal.disconnect"),
        onclick: () => confirmThen(t("extcal.disconnect"), t("extcal.disconnectConfirm", { name: a.name }), t("extcal.disconnect"),
          async () => { await del("/api/me/calendar-accounts/" + a.id); return {}; }, t("extcal.disconnected", { name: a.name })) })))) : null;

  // Adding: an address, Google, or a CalDAV account such as iCloud.
  const forms = h("div", { class: "cal-others-forms", id: "calendar-others-forms" });
  const colour = () => h("input", { type: "color", name: "colour", value: next, "aria-label": t("extcal.colour") });
  const icsForm = () => {
    const f = h("form", { class: "cal-others-form", id: "calendar-ics-form", onsubmit: (ev) => {
      ev.preventDefault();
      const d = new FormData(f);
      run(async () => {
        const { source } = await post("/api/me/calendars", { kind: "ics", url: String(d.get("url")).trim(), name: String(d.get("name")).trim(), colour: d.get("colour") });
        return { notice: { ok: true, text: t("extcal.added", { name: source.name }) } };
      });
    } },
    h("p", { class: "muted", text: t("extcal.icsHelp") }),
    field(t("extcal.url"), h("input", { type: "url", name: "url", id: "calendar-ics-url", required: true, placeholder: "https://calendar.google.com/calendar/ical/…/basic.ics" })),
    h("div", { class: "form-row" },
      field(t("extcal.name"), h("input", { type: "text", name: "name", id: "calendar-ics-name", maxlength: "80", placeholder: t("extcal.nameHint") })),
      field(t("extcal.colour"), colour())),
    h("button", { type: "submit", class: "primary", id: "calendar-ics-add", text: t("extcal.add") }));
    return f;
  };
  const davForm = () => {
    const server = h("input", { type: "url", name: "server", id: "calendar-dav-server", required: true, value: "https://caldav.icloud.com" });
    const f = h("form", { class: "cal-others-form", id: "calendar-dav-form", onsubmit: (ev) => {
      ev.preventDefault();
      const d = new FormData(f);
      const server = String(d.get("server")).trim();
      run(async () => {
        try {
          const { account, calendars } = await post("/api/me/calendar-accounts", { kind: "caldav", server, username: String(d.get("username")).trim(), password: d.get("password") });
          return { picked: { account, calendars }, notice: { ok: true, text: t("extcal.connected", { name: account.name }) } };
        } catch (err) {
          // A refused password is the most common failure; for iCloud it is
          // nearly always the Apple ID's own password instead of an
          // app-specific one.
          if (err.code === "credentials") throw new Error(t(appleServer(server) ? "extcal.error.appleCredentials" : "extcal.error.credentials"));
          throw err;
        }
      });
    } },
    h("p", { class: "muted", text: t("extcal.davHelp") }),
    field(t("extcal.server"), server),
    h("div", { class: "form-row" },
      field(t("extcal.username"), h("input", { type: "text", name: "username", id: "calendar-dav-username", required: true, autocomplete: "username" })),
      field(t("extcal.password"), h("input", { type: "password", name: "password", id: "calendar-dav-password", required: true, autocomplete: "off" }))),
    h("button", { type: "submit", class: "primary", id: "calendar-dav-connect", text: t("extcal.connect") }));
    return f;
  };
  const way = (id, label, make) => h("button", { type: "button", class: "secondary", id, text: label, disabled: !enabled, onclick: () => forms.replaceChildren(make()) });
  const ways = enabled ? h("p", { class: "cal-others-ways" },
    way("calendar-add-ics", t("extcal.addIcs"), icsForm),
    google ? h("a", { class: "button secondary", id: "calendar-add-google", href: "/api/me/calendar-accounts/google/connect", text: t("extcal.addGoogle") })
      : h("span", { class: "muted", id: "calendar-google-off", text: t("extcal.googleOff") }),
    way("calendar-add-dav", t("extcal.addDav"), davForm)) : h("p", { class: "muted", id: "calendar-others-off", text: t("extcal.off") });

  if (after.picked) showPicker(after.picked.account, after.picked.calendars);
  else if (after.pick && accountOf(after.pick)) queueMicrotask(choose(accountOf(after.pick)));
  const shownCount = sources.filter((s) => !s.hidden).length;
  return h("details", { class: "cal-others", id: "calendar-others", open: Boolean(after.others || after.pick) },
    h("summary", {}, t("extcal.title"), shownCount ? h("span", { class: "muted", text: " " + t("extcal.count", { n: shownCount }) }) : null),
    list, accountList, pick, ways, forms);
}

// feedBox shows a feed's address once, as it was made, to copy into a
// calendar program, with what holding it means.
function feedBox(feed) {
  const url = h("input", { id: "calendar-feed-url", type: "text", readonly: true, value: feed.url, "aria-label": t("calendar.feedUrl") });
  const copied = h("span", { class: "muted", role: "status" });
  return h("div", { class: "notice cal-feed", id: "calendar-feed" },
    h("p", { text: t("calendar.feedMade") }),
    h("p", { class: "cal-feed-row" }, url, h("button", { type: "button", class: "secondary", id: "calendar-feed-copy", text: t("calendar.feedCopy"), onclick: async () => {
      url.select();
      try {
        await navigator.clipboard.writeText(feed.url);
        copied.textContent = t("calendar.feedCopied");
      } catch {
        copied.textContent = t("calendar.feedCopyHint");
      }
    } }), copied),
    h("p", { class: "muted", text: t("calendar.feedWarning") }));
}

// NEW_TYPE keeps, for this tab of the browser, the type the last new
// entry was made of.
const NEW_TYPE = "ot-calendar-new-type";

// createDialog asks for what a new item needs when a day, or an hour of
// the week, is double-clicked, or "New entry" is chosen: its type among
// those the calendar places by date fields, its room among those one may
// edit, its title, its date fields filled in from the place, and the other
// fields its type requires. It resolves to the item made and whether to
// open it, or to null when it is closed without making one. The server
// validates the item as every other new item (D3).
function createDialog({ day, minutes, fields, typeOf, rooms, state, helpers }) {
  return new Promise((resolve) => {
    let kept = "";
    try { kept = sessionStorage.getItem(NEW_TYPE) || ""; } catch { /* not kept */ }
    const names = fields.map((f) => f.type);
    // The type the view shows, else the last one used; an hour of the week
    // prefers a type with a time.
    const timed = minutes === undefined ? "" : fields.find((f) => f.startTimed || f.endTimed)?.type;
    const first = [state.type, kept, timed, names[0]].find((n) => n && names.includes(n));
    const typeSelect = h("select", { id: "calendar-new-type" }, names.map((n) => h("option", { value: n, text: n, selected: n === first })));
    const ordered = helpers.favouritesFirst(rooms);
    const room = ordered.some((sp) => sp.key === state.space) ? state.space : ordered[0].key;
    const roomSelect = h("select", { id: "calendar-new-space" }, ordered.map((sp) => h("option", { value: sp.key, text: `${sp.key} ${sp.name}`, selected: sp.key === room })));
    const title = h("input", { id: "calendar-new-title", name: "title", required: true, autocomplete: "off" });
    const box = h("div", { id: "calendar-new-fields" });
    const error = h("p", { class: "error", id: "calendar-new-error", role: "alert", hidden: true });
    let shown = [];
    const draw = () => {
      const f = fields.find((x) => x.type === typeSelect.value);
      const values = slotValues(f, day, minutes);
      shown = (typeOf.get(f.type)?.fields || []).filter((fd) => fd.name in values || fd.required);
      box.replaceChildren(...shown.map((fd) => field(fd.name + (fd.required ? " *" : ""), helpers.propertyInput(fd, values[fd.name]))));
    };
    typeSelect.addEventListener("change", draw);
    draw();
    const u = new Date(day * DAY);
    const when = minutes === undefined ? dateText(day) : `${dateText(day)} ${timeText(new Date(u.getUTCFullYear(), u.getUTCMonth(), u.getUTCDate(), 0, minutes).getTime())}`;
    let made = null;
    const form = h("form", { class: "cal-new-form", onsubmit: async (ev) => {
      ev.preventDefault();
      const open = ev.submitter?.value === "open";
      const properties = {};
      for (const fd of shown) {
        const v = helpers.readProperty(fd, form.querySelector(`[name="p:${CSS.escape(fd.name)}"]`));
        if (v !== null) properties[fd.name] = v;
      }
      error.hidden = true;
      form.querySelectorAll("button").forEach((b) => { b.disabled = true; });
      try {
        const { item } = await post("/api/items", { space: roomSelect.value, type: typeSelect.value, title: title.value.trim(), properties });
        try { sessionStorage.setItem(NEW_TYPE, typeSelect.value); } catch { /* not kept */ }
        made = { item, open };
        dialog.close();
      } catch (err) {
        error.textContent = err.message;
        error.hidden = false;
        form.querySelectorAll("button").forEach((b) => { b.disabled = false; });
      }
    } },
    h("div", { class: "form-row" }, field(t("col.type"), typeSelect), field(t("col.space"), roomSelect)),
    field(t("col.title"), title),
    box,
    error,
    h("p", { class: "actions" },
      h("button", { type: "submit", class: "primary", id: "calendar-new-create", value: "stay", text: t("calendar.create") }),
      h("button", { type: "submit", class: "secondary", id: "calendar-new-open", value: "open", text: t("calendar.createOpen") }),
      h("button", { type: "button", class: "link", text: t("calendar.cancel"), onclick: () => dialog.close() })));
    const dialog = h("dialog", { class: "cal-dialog cal-new", id: "calendar-new", "aria-labelledby": "calendar-new-heading" },
      h("h2", { id: "calendar-new-heading", text: t("calendar.newTitle", { when }) }), form);
    dialog.addEventListener("close", () => {
      dialog.remove();
      resolve(made);
    });
    document.body.append(dialog);
    dialog.showModal();
    title.focus();
  });
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
  // Google sends one back to /?calendar_account=… (or calendar_error=…);
  // the calendar then offers that account's calendars, and the address
  // keeps only its route.
  const back = new URLSearchParams(location.search);
  if (back.has("calendar_account") || back.has("calendar_error")) {
    const error = back.get("calendar_error");
    const told = { off: t("extcal.error.off"), config: t("extcal.error.config"), state: t("extcal.error.state"), denied: t("extcal.error.denied"),
      unavailable: t("extcal.error.unavailable") };
    if (error) after = { ...after, others: true, notice: { ok: false, text: told[error] || told.unavailable } };
    else after = { ...after, pick: back.get("calendar_account") };
    history.replaceState(null, "", location.pathname + location.hash);
  }
  const [{ types }, { spaces }, { views, feeds }, cals] = await Promise.all([get("/api/types"), get("/api/spaces"),
    get("/api/me/calendar-views").catch(() => ({ views: [], feeds: false })),
    get("/api/me/calendars").catch(() => ({ enabled: false, sources: [], accounts: [] }))]);
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
  // One's calendars of other services stand beside the items (ADR-0174).
  let others = [], unreadable = [];
  if (cals.enabled && (cals.sources || []).some((src) => !src.hidden)) {
    let zone = "";
    try { zone = Intl.DateTimeFormat().resolvedOptions().timeZone || ""; } catch { /* the server takes UTC */ }
    try {
      const got = await get(`/api/me/calendars/events?from=${dayString(p.from - 1)}&to=${dayString(p.to + 2)}&zone=${encodeURIComponent(zone)}`);
      others = externalEntries(got.events, cals.sources).filter((e) => e.to >= p.from && e.from <= p.to);
      const named = new Map(cals.sources.map((src) => [src.id, src.name]));
      unreadable = (got.problems || []).map((x) => t("extcal.unreadable", { name: named.get(x.source) || x.source, message: x.message }));
    } catch (err) {
      unreadable = [err.message];
    }
  }
  const list = [...entries(items, fields, { from: p.from, to: p.to }), ...others].sort(byDayAndTime);
  const editable = new Set(spaces.filter((sp) => sp.role === "editor" || sp.role === "space-admin").map((sp) => sp.key));
  const rooms = spaces.filter((sp) => editable.has(sp.key));
  const ctx = {
    // A new item is made where one may edit a room and the calendar
    // places a type (ADR-0166): by a double-click on a day or an hour.
    canCreate: Boolean(rooms.length && fields.length && helpers.propertyInput && helpers.readProperty),
    create: async (day, minutes) => {
      const made = await createDialog({ day, minutes, fields, typeOf, rooms, state, helpers });
      if (!made) return;
      if (made.open) {
        location.hash = "#/item/" + encodeURIComponent(made.item.key) + (typeOf.get(made.item.type)?.body ? "?edit" : "");
        return;
      }
      await calendarView(container, params, helpers, { notice: { ok: true, text: t("calendar.created", { key: made.item.key, title: made.item.title }) }, focus: made.item.key });
    },
    today,
    done: (it) => finished(typeOf.get(it.type)?.workflow, it.state),
    hasWorkflow: (it) => Boolean(typeOf.get(it.type)?.workflow?.states?.length),
    canMove: (it) => editable.has(it.space),
    drag: null,
    clearDrop: () => container.querySelectorAll(".cal-drop").forEach((el) => el.classList.remove("cal-drop")),
    // move changes the entry's dates by n days, place to a time of day as
    // well, then the calendar is drawn again from what the server kept,
    // telling what happened. An occurrence of a series asks whether only
    // it moves or the whole series (ADR-0168).
    move: (e, n) => ctx.place(e, n),
    place: async (e, n, minutes) => {
      if (!e.occurrence) return ctx.change(e, n, minutes === undefined ? moves(e, n) : retime(e, n, minutes));
      const scope = await ask(t("calendar.scopeTitle"), t("calendar.scopeText", { key: e.item.key, date: dateText(e.from) }),
        [["one", t("calendar.scopeOne"), "primary"], ["all", t("calendar.scopeAll")], ["", t("calendar.cancel"), "link"]]);
      if (scope === "one") return ctx.change(e, n, occurrenceChanges(e, n, minutes));
      if (scope === "all") return ctx.change(e, n, seriesChanges(e, n, minutes));
    },
    cancel: async (e) => {
      const ok = await ask(t("calendar.cancelTitle"), t("calendar.cancelText", { key: e.item.key, date: dateText(e.from) }),
        [["yes", t("calendar.cancelConfirm"), "danger"], ["", t("calendar.cancel"), "link"]]);
      if (ok === "yes") return ctx.change(e, 0, cancelChanges(e), t("calendar.cancelled", { key: e.item.key, date: dateText(e.from) }));
    },
    change: async (e, n, changes, told) => {
      if (!changes.length) return;
      let notice;
      try {
        for (const c of changes) await put(`/api/items/${e.item.id}/properties/${encodeURIComponent(c.field)}`, { value: c.value });
        // A point in time is told with its new time.
        const timed = changes.find((c) => (c.field === e.start && e.startTimed) || (c.field === e.end && e.endTimed));
        const when = timed ? whenText(localDayOf(timed.value), Date.parse(timed.value)) : dateText(e.from + n);
        notice = { ok: true, text: told || t("calendar.moved", { key: e.item.key, date: when }) };
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
  const text = h("input", { id: "calendar-text", type: "search", value: state.text, placeholder: t("calendar.textHint") });
  const q = h("input", { id: "calendar-q", type: "search", value: state.q, placeholder: t("calendar.queryHint"), spellcheck: "false" });
  const apply = (ev) => {
    ev.preventDefault();
    go({ labels: [...new Set(labels.value.split(",").map(normalLabel).filter(Boolean))], text: text.value.trim(), q: q.value.trim() });
  };
  // The advanced choices: the fields an item stands by, and a query of the
  // query language with the query the view sends, to read and learn from.
  const advanced = h("details", { class: "cal-advanced", id: "calendar-advanced", open: Boolean(state.q || state.start || state.end) },
    h("summary", { text: t("calendar.advanced") }),
    h("p", { class: "muted", text: t("calendar.advancedHelp") }),
    h("div", { class: "form-row" },
      select("calendar-start", t("calendar.startField"), [["", t("calendar.guessed")], ...choices.map((f) => [f, f])], state.start, "start"),
      select("calendar-end", t("calendar.endField"), [["", t("calendar.guessed")], ...choices.map((f) => [f, f])], state.end, "end")),
    field(t("calendar.query"), q),
    h("p", { class: "muted" }, t("calendar.queryHelp") + " ", h("a", { href: "#/query", text: t("calendar.queryHelpLink") })),
    query ? h("p", { class: "muted cal-sent" }, t("calendar.sent") + " ", h("code", { id: "calendar-sent", text: query })) : null);
  const form = h("form", { class: "cal-filter", id: "calendar-filter", onsubmit: apply },
    h("div", { class: "form-row" },
      select("calendar-space", t("col.space"), [["", t("calendar.allSpaces")], ...favouritesFirst(spaces).map((sp) => [sp.key, `${sp.key} ${sp.name}`])], state.space, "space"),
      select("calendar-type", t("col.type"), [["", t("calendar.allTypes")], ...dated.map((ty) => [ty.name, ty.name])], state.type, "type"),
      select("calendar-who", t("calendar.who"), WHO.map((w) => [w, whoName(w)]), state.who, "who"),
      select("calendar-status", t("calendar.status"), STATUS.map((st) => [st, statusName(st)]), state.status, "status")),
    h("div", { class: "form-row" },
      field(t("calendar.text"), text),
      field(t("calendar.labels"), labels),
      h("button", { type: "submit", class: "secondary", id: "calendar-apply", text: t("calendar.apply") })),
    advanced);

  // The choices are folded away until asked for; the sentence under the
  // period says what they show. Once opened, they stay open while one
  // chooses, for this tab of the browser.
  const active = ["space", "type", "who", "status", "text", "q", "start", "end"].filter((k) => state[k]).length + (state.labels.length ? 1 : 0);
  let open = false;
  try { open = sessionStorage.getItem(FILTERS_OPEN) === "1"; } catch { /* not kept */ }
  const filters = h("details", { class: "cal-filters", id: "calendar-filters", open },
    h("summary", {}, t("calendar.filters"), active ? h("span", { class: "muted", text: " " + t("calendar.filtersActive", { n: active }) }) : null),
    form);
  filters.addEventListener("toggle", () => {
    try { sessionStorage.setItem(FILTERS_OPEN, filters.open ? "1" : "0"); } catch { /* not kept */ }
  });
  const controls = h("div", { class: "card tl-controls cal-controls", id: "calendar-controls" },
    h("nav", { class: "tabs", "aria-label": t("calendar.view") }, VIEWS.map((v) =>
      h("a", { href: address({ ...state, view: v }, today), class: v === state.view ? "tab active" : "tab", id: "calendar-view-" + v, text: viewName(v) }))),
    savedBar(container, params, helpers, state, today, views, feeds, after),
    othersBar(container, params, helpers, cals, after),
    filters);

  const nav = h("div", { class: "cal-nav" },
    h("a", { class: "button secondary", id: "calendar-prev", href: address({ ...state, at: shift(state.view, state.at, -1) }, today), text: t("calendar.previous") }),
    h("a", { class: "button secondary", id: "calendar-today", href: address({ ...state, at: today }, today), text: t("calendar.today") }),
    h("a", { class: "button secondary", id: "calendar-next", href: address({ ...state, at: shift(state.view, state.at, 1) }, today), text: t("calendar.next") }),
    // The same as a double-click, by keyboard or touch: today, when the
    // period shows it, else its first day.
    ctx.canCreate ? h("button", { type: "button", class: "secondary", id: "calendar-new-entry", text: t("calendar.newEntry"),
      onclick: () => ctx.create(today >= p.first && today <= p.last ? today : p.first) }) : null,
    h("h2", { id: "calendar-period", "aria-live": "polite", text: heading(p) }),
    h("p", { class: "muted cal-summary", id: "calendar-summary", text: summary(state, spaces) }));

  // A mistake in the query typed puts the cursor on its place there.
  const mistake = failed ? queryError(failed, query, state) : null;
  if (mistake) q.setAttribute("aria-invalid", "true");

  let content;
  if (!dated.length && !others.length) content = h("p", { class: "empty", id: "calendar-grid", text: t("calendar.noDateFields") });
  else if (state.view === "agenda") content = agendaEl(p, list, ctx);
  else content = gridEl(p, list, ctx);

  container.replaceChildren(...[
    controls,
    mistake ? h("p", { class: "notice warning", id: "calendar-error", role: "alert", text: mistake.text }) : null,
    after.notice ? h("p", { class: after.notice.ok ? "notice" : "notice warning", id: "calendar-moved", role: after.notice.ok ? "status" : "alert", text: after.notice.text }) : null,
    after.feed ? feedBox(after.feed) : null,
    unreadable.length ? h("div", { class: "notice warning", id: "calendar-unreadable", role: "status" }, unreadable.map((text) => h("p", { text }))) : null,
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
  // The axis of hours opens at the first time of the week, or at seven.
  const hours = container.querySelector("#calendar-hours");
  if (hours) {
    const first = Math.min(7 * 60, ...list.map(timedSpan).filter(Boolean).map((x) => x.from));
    hours.scrollTop = Math.max(0, (first - 30) / DAY_MINUTES * hours.scrollHeight);
  }
}
