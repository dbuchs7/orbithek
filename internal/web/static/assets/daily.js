// A daily note per person and day (ADR-0205): the view #/daily/<date>
// shows one's own note of a day with its text editable in place, the days
// before and after, a month marking the days one has a note on, the items
// made or changed that day that one may read, and the items the calendar
// places on it (ADR-0166). #/daily alone is today, where the browser is,
// and opens today's note, making it in one's daily room when there is
// none; another day's note is made only when one starts it. The room is
// chosen once, among the rooms one may create in, and kept with one's
// account.
//
// The note is an item like any other (D1): its page, history, comments
// and protections are the item's. The server makes it restricted to its
// author in the same step (ADR-0078). The layout's parts are plain
// functions in dailyparts.js.
import { get, post, put, ApiError } from "./api.js";
import { h } from "./dom.js";
import { t, language } from "./i18n.js";
import { liveBody } from "./live.js";
import { dayString } from "./timeline.js";
import { entries, listQuery, onDay, typeFields } from "./calendar.js";
import { activityQuery, dailyHref, dayOfAddress, month, monthTitle, noteTitle, weekdayNames } from "./dailyparts.js";
import { besidePane } from "./beside.js";


// openToday goes to today's note.
export function openToday() {
  location.hash = "#/daily";
}

// appendToDailyNote adds text, a paragraph per line, at the end of one's
// daily note of a day (today, where the browser is, when no day is given),
// making the note first: what a quick capture adds to today's note. It
// resolves with { item, created, editable }; without a room chosen for
// daily notes it rejects with an ApiError of status 409 (needsRoom), and
// at critical storage, when the note must be made, with status 507.
export function appendToDailyNote(text, day = dayOfAddress("")) {
  return post("/api/daily/" + dayString(day) + "/paragraphs", { text, title: noteTitle(day, language()) });
}

// dailyView draws the daily note of the day the address names into show.
// tools are the item page's helpers for a text (see beside.js), with
// favouritesFirst(spaces), onLeave(fn) and reload(), which draws the view
// again.
export async function dailyView(show, arg, params, tools) {
  const day = dayOfAddress(arg);
  if (day === null) {
    show(h("p", { class: "error", role: "alert", text: t("daily.badDate") }));
    return;
  }
  const today = dayOfAddress("");
  const date = dayString(day);
  const status = h("div", { class: "status", role: "status" });
  const lang = language();
  const p = month(day, [], today);
  const [info, { spaces }, { types }] = await Promise.all([
    get(`/api/daily?from=${dayString(p.from)}&to=${dayString(p.to)}`),
    get("/api/spaces"),
    get("/api/types"),
  ]);
  const rooms = tools.favouritesFirst(spaces.filter((s) => s.role === "editor" || s.role === "space-admin"));

  // The note: today's is opened at once, made where it is missing; any
  // other day's is read, and made when one starts it.
  let note = null;
  let editable = false;
  let needsRoom = false;
  const open = async () => {
    try {
      const res = await post("/api/daily/" + date, { title: noteTitle(day, lang) });
      note = res.item;
      editable = res.editable;
      if (!info.days.includes(date)) info.days.push(date);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) needsRoom = true;
      else throw err;
    }
  };
  // The month's days tell whether the day has a note, so a day without
  // one is not asked for.
  if (!arg && info.space && info.mayCreate) {
    await open();
  } else if (info.days.includes(date)) {
    const res = await get("/api/daily/" + date);
    note = res.item;
    editable = res.editable;
  }
  needsRoom = needsRoom || (!note && (!info.space || !info.mayCreate));

  // The room the notes go to, chosen once and changed here.
  const roomSelect = h("select", { id: "daily-room", name: "room", "aria-label": t("daily.room") },
    h("option", { value: "", text: t("daily.roomNone") }),
    rooms.map((s) => h("option", { value: s.key, text: `${s.name} (${s.key})`, selected: s.key === info.space })));
  roomSelect.addEventListener("change", async () => {
    try {
      await put("/api/daily/room", { space: roomSelect.value });
      status.textContent = roomSelect.value ? t("daily.roomSaved", { space: roomSelect.value }) : "";
      if (roomSelect.value && !note) tools.reload();
    } catch (err) {
      status.textContent = err.message;
    }
  });
  const roomCard = h("section", { class: "side-card", id: "daily-room-card" },
    h("h2", { text: t("daily.room") }),
    h("p", { class: "muted", text: t("daily.roomHint") }),
    rooms.length ? roomSelect : h("p", { class: "muted", text: t("daily.noRooms") }),
    info.space && !info.mayCreate ? h("p", { class: "notice", text: t("daily.roomLost", { space: info.space }) }) : null);

  // The text, in place: editing joins the note's live session (ADR-0010).
  let main;
  if (note) {
    const path = "/api/items/" + note.id;
    if (editable && note.readsAllSecrets !== false) {
      const [{ embeds }, { attachments }] = await Promise.all([get(path + "/embeds"), get(path + "/attachments")]);
      const resolve = async (ref) => (await get("/api/items/" + encodeURIComponent(ref))).item;
      const session = liveBody({ ref: note.id, embeds, resolve, childPages: tools.childPages(note), files: tools.files(note, attachments, true),
        embedContent: tools.embeds([note.id]), tasks: tools.tasks(note, true), markdown: tools.markdown(note), create: tools.create(note) });
      tools.onLeave(() => session.finish().catch(() => {}).finally(session.dispose));
      main = h("section", { id: "daily-note", class: "editing daily-note" }, session.element);
    } else {
      main = h("section", { id: "daily-note", class: "reading daily-note" },
        h("p", {}, h("a", { href: "#/item/" + encodeURIComponent(note.key), text: t("daily.openNote", { key: note.key }) })));
    }
  } else if (needsRoom) {
    main = h("section", { id: "daily-note", class: "daily-empty" },
      h("p", { text: rooms.length ? t("daily.chooseRoom") : t("daily.noRooms") }));
  } else {
    const start = h("button", { type: "button", class: "primary", id: "daily-start", text: t("daily.start"), onclick: async () => {
      start.disabled = true;
      try {
        await open();
        tools.reload();
      } catch (err) {
        status.textContent = err.message;
        start.disabled = false;
      }
    } });
    main = h("section", { id: "daily-note", class: "daily-empty" }, h("p", { class: "muted", text: t("daily.empty") }), start);
  }

  // The month beside, with the days one has a note on.
  const m = month(day, info.days, today);
  const names = weekdayNames(lang);
  const cal = h("section", { class: "side-card daily-month", id: "daily-month" },
    h("div", { class: "daily-month-head" },
      h("a", { href: dailyHref(m.first - 1), class: "button icon-btn", title: t("daily.prevMonth"), "aria-label": t("daily.prevMonth"), text: "‹" }),
      h("h2", { text: monthTitle(day, lang) }),
      h("a", { href: dailyHref(m.last + 1), class: "button icon-btn", title: t("daily.nextMonth"), "aria-label": t("daily.nextMonth"), text: "›" })),
    h("table", { class: "daily-grid", "aria-label": monthTitle(day, lang) },
      h("thead", {}, h("tr", {}, names.map((n) => h("th", { scope: "col", text: n })))),
      h("tbody", {}, m.weeks.map((w) => h("tr", {}, w.map((d) => h("td", {},
        h("a", { href: dailyHref(d.day), "data-day": dayString(d.day),
          class: ["daily-day", d.inMonth ? "" : "other", d.noted ? "noted" : "", d.today ? "today" : "", d.shown ? "shown" : ""].filter(Boolean).join(" "),
          "aria-current": d.shown ? "date" : null,
          "aria-label": noteTitle(d.day, lang) + (d.noted ? " · " + t("daily.hasNote") : "") }, String(d.date)))))))),
    h("p", { class: "muted daily-legend" }, h("span", { class: "daily-dot", "aria-hidden": "true" }), " " + t("daily.legend")));

  // What was made or changed that day, and what the calendar puts on it.
  const activity = h("section", { class: "side-card", id: "daily-activity" }, h("h2", { text: t("daily.activity") }), h("p", { class: "muted", text: t("daily.loading") }));
  const agenda = h("section", { class: "side-card", id: "daily-calendar" }, h("h2", { text: t("daily.calendar") }), h("p", { class: "muted", text: t("daily.loading") }));
  const entry = (it, extra) => h("li", { "data-key": it.key }, h("a", { href: "#/item/" + encodeURIComponent(it.key) }, h("span", { class: "context-key", text: it.key }), " ", it.title || it.key),
    extra ? h("span", { class: "muted", text: " · " + extra }) : null);
  get("/api/items?limit=50&query=" + encodeURIComponent(activityQuery(day))).then(({ items }) => {
    const shown = items.filter((it) => !note || it.id !== note.id);
    activity.replaceChildren(h("h2", { text: t("daily.activity") }),
      shown.length ? h("ul", { class: "list" }, shown.map((it) => entry(it, it.type))) : h("p", { class: "muted", text: t("daily.noActivity") }));
  }).catch((err) => activity.append(h("p", { class: "error", text: err.message })));
  const fields = typeFields(types, { type: "", start: "", end: "" });
  const q = listQuery({ status: "", text: "", labels: [], q: "", who: "" }, fields, { from: day, to: day });
  (q ? get("/api/items?limit=200&query=" + encodeURIComponent(q)) : Promise.resolve({ items: [] })).then(({ items }) => {
    // Daily notes stand on their day by their date; they are not dates
    // of the day's work.
    const list = onDay(entries(items.filter((it) => !(it.labels || []).includes("daily-note")), fields, { from: day, to: day }), day);
    const seen = new Set();
    const unique = list.filter((e) => !seen.has(e.item.key) && seen.add(e.item.key));
    agenda.replaceChildren(h("h2", { text: t("daily.calendar") }),
      unique.length ? h("ul", { class: "list" }, unique.map((e) => entry(e.item, e.at?.from ? new Date(e.at.from).toLocaleTimeString(lang, { hour: "2-digit", minute: "2-digit" }) : ""))) : h("p", { class: "muted", text: t("daily.noCalendar") }),
      h("p", {}, h("a", { href: "#/calendar?view=week&at=" + date, text: t("daily.toCalendar") })));
  }).catch((err) => agenda.append(h("p", { class: "error", text: err.message })));

  show(h("div", { class: "daily-view" },
    h("div", { class: "page-head daily-head" },
      h("h1", { id: "daily-title", text: noteTitle(day, lang) }),
      h("p", { class: "actions daily-nav" },
        h("a", { href: dailyHref(day - 1), class: "button", id: "daily-prev", rel: "prev", text: "‹ " + t("daily.prev") }),
        day !== today ? h("a", { href: "#/daily", class: "button", id: "daily-today", text: t("daily.today") }) : null,
        h("a", { href: dailyHref(day + 1), class: "button", id: "daily-next", rel: "next", text: t("daily.next") + " ›" }),
        note ? h("a", { href: "#/item/" + encodeURIComponent(note.key), class: "button", id: "daily-item", text: t("daily.asItem") }) : null)),
    h("p", { class: "muted", text: note ? t("daily.private") : t("daily.intro") }),
    status,
    h("div", { class: "item-layout daily-layout" },
      h("div", { class: "item-main" }, main),
      h("aside", { class: "item-side", "aria-label": t("daily.side") }, cal, roomCard, agenda, activity)),
    besidePane(params, tools)));
}

// installDailyKeys opens today's note with Alt+Shift+D, anywhere.
export function installDailyKeys() {
  document.addEventListener("keydown", (ev) => {
    if (ev.altKey && ev.shiftKey && !ev.ctrlKey && !ev.metaKey && ev.code === "KeyD") {
      ev.preventDefault();
      openToday();
    }
  });
}
