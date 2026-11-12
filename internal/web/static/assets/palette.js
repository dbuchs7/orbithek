// The command palette (ADR-0207): one field that finds items, the items
// opened recently in this browser, and the commands of the interface, and
// goes there with the keyboard alone. It opens with Ctrl+K where no text
// is being edited (the editor keeps Ctrl+K for links, ADR-0096) and with
// Ctrl+Shift+P everywhere; Cmd on a Mac. Items are found by the search of
// the server (ADR-0033), which shows only what the reader may read (D10);
// the commands are the shell's, handed over by app.js. Nothing is kept on
// the server: the recent items are a convenience of this browser.
//
// The field is a combobox and the results a listbox (WAI-ARIA): arrows
// move the active option, Enter takes it, Escape closes the palette and
// gives the focus back where it was.
import { get } from "./api.js";
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { opens, rank, remember } from "./paletteparts.js";

const STORE = "ot.recent";

// recentItems are the items opened recently in this browser.
export function recentItems() {
  try {
    const v = JSON.parse(localStorage.getItem(STORE) || "[]");
    return Array.isArray(v) ? v.filter((e) => e && typeof e.key === "string") : [];
  } catch {
    return [];
  }
}

// rememberItem notes that an item was opened.
export function rememberItem(item) {
  try {
    localStorage.setItem(STORE, JSON.stringify(remember(recentItems(), { key: item.key, title: item.title })));
  } catch { /* kept for this page only */ }
}

// editingText tells whether the focus is in a text being edited: the
// body's editor, a field or a text area.
function editingText(target) {
  return target instanceof Element && Boolean(target.closest("[contenteditable=true], .ProseMirror[contenteditable]"));
}

let open = null;

// installPalette listens for the keys that open the palette. commands()
// returns, or resolves with, the commands offered now: { id, name, hint,
// run }. Another dialog that is open keeps the keys.
export function installPalette(commands) {
  let asking = false;
  document.addEventListener("keydown", async (ev) => {
    if (!opens(ev, editingText(ev.target))) return;
    if (open) {
      ev.preventDefault();
      open.focus();
      return;
    }
    if (document.querySelector("dialog[open]")) return;
    ev.preventDefault();
    if (asking) return;
    asking = true;
    try {
      const list = await commands();
      if (list.length) openPalette(list);
    } finally {
      asking = false;
    }
  });
}

// openPalette shows the palette with the commands given.
export function openPalette(commands) {
  if (open) return;
  const before = document.activeElement;
  const listId = "palette-list";
  const input = h("input", { type: "text", id: "palette-input", class: "palette-input", role: "combobox", autocomplete: "off", spellcheck: "false",
    "aria-expanded": "true", "aria-controls": listId, "aria-autocomplete": "list", "aria-label": t("palette.label"), placeholder: t("palette.placeholder") });
  const list = h("ul", { id: listId, class: "palette-list", role: "listbox", "aria-label": t("palette.results") });
  const hint = h("p", { class: "palette-hint muted", id: "palette-hint", text: t("palette.hint") });
  const live = h("p", { class: "visually-hidden", role: "status", "aria-live": "polite" });
  const dialog = h("dialog", { class: "palette", id: "palette", "aria-label": t("palette.title") }, input, list, hint, live);
  let options = [];
  let active = 0;
  let asked = 0;
  let timer = 0;
  let found = [];

  const close = () => {
    clearTimeout(timer);
    dialog.close();
    dialog.remove();
    open = null;
    if (before instanceof HTMLElement && before.isConnected) before.focus();
  };
  const take = (o) => {
    close();
    o.run();
  };
  const mark = (i) => {
    active = options.length ? (i + options.length) % options.length : 0;
    options.forEach((o, j) => o.el.setAttribute("aria-selected", String(j === active)));
    const cur = options[active];
    if (cur) {
      input.setAttribute("aria-activedescendant", cur.el.id);
      cur.el.scrollIntoView({ block: "nearest" });
    } else {
      input.removeAttribute("aria-activedescendant");
    }
  };
  const draw = () => {
    const typed = input.value.trim();
    const recent = rank(typed, recentItems(), (e) => `${e.title} ${e.key}`).slice(0, typed ? 4 : 6);
    const cmds = rank(typed, commands, (c) => `${c.name} ${c.hint || ""}`).slice(0, typed ? 8 : 12);
    const seen = new Set(recent.map((e) => e.key));
    const items = found.filter((it) => !seen.has(it.key));
    options = [];
    let n = 0;
    const option = (run, kind, ...children) => {
      const o = { run };
      o.el = h("li", { role: "option", id: "palette-option-" + n++, class: "palette-option palette-" + kind, "aria-selected": "false",
        onmousedown: (ev) => { ev.preventDefault(); take(o); } }, ...children);
      options.push(o);
      return o.el;
    };
    const group = (id, text, entries) => entries.length
      ? [h("li", { role: "presentation", class: "palette-group", id: "palette-group-" + id, text }), ...entries] : [];
    const go = (key) => () => { location.hash = "#/item/" + encodeURIComponent(key); };
    list.replaceChildren(
      ...group("items", t("palette.items"), items.map((it) => option(go(it.key), "item",
        h("span", { class: "palette-name", text: it.title || it.key }), h("span", { class: "palette-meta muted", text: `${it.key} · ${it.type}` })))),
      ...group("recent", t("palette.recent"), recent.map((e) => option(go(e.key), "recent",
        h("span", { class: "palette-name", text: e.title || e.key }), h("span", { class: "palette-meta muted", text: e.key })))),
      ...group("commands", t("palette.commands"), cmds.map((c) => option(c.run, "command",
        h("span", { class: "palette-name", text: c.name }), c.hint ? h("span", { class: "palette-meta muted", text: c.hint }) : null))));
    if (!options.length) list.append(h("li", { class: "palette-none muted", role: "presentation", text: t("palette.none") }));
    live.textContent = t("palette.count", { n: options.length });
    mark(0);
  };
  const search = () => {
    clearTimeout(timer);
    const typed = input.value.trim();
    if (!/[\p{L}\p{N}]/u.test(typed)) {
      found = [];
      return draw();
    }
    draw();
    timer = setTimeout(async () => {
      const mine = ++asked;
      const res = await get("/api/search?limit=8&q=" + encodeURIComponent(typed)).catch(() => ({ hits: [] }));
      if (mine !== asked || !open) return;
      found = (res.hits || []).map((x) => x.item);
      draw();
    }, 150);
  };
  input.addEventListener("input", search);
  input.addEventListener("keydown", (ev) => {
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      mark(active + (ev.key === "ArrowDown" ? 1 : -1));
    } else if (ev.key === "Home" && ev.ctrlKey) {
      ev.preventDefault();
      mark(0);
    } else if (ev.key === "End" && ev.ctrlKey) {
      ev.preventDefault();
      mark(options.length - 1);
    } else if (ev.key === "Enter") {
      ev.preventDefault();
      if (options[active]) take(options[active]);
    }
  });
  dialog.addEventListener("cancel", (ev) => { ev.preventDefault(); close(); });
  dialog.addEventListener("mousedown", (ev) => { if (ev.target === dialog) close(); });
  document.body.append(dialog);
  dialog.showModal();
  open = { focus: () => input.focus() };
  draw();
  input.focus();
}
