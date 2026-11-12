// Inputs and views of the field kinds item, multi, rating and file
// (ADR-0200), for every place that shows or edits a field by its kind: the
// item page, the form that makes an item, and any table, gallery or card.
//
//   fieldInput(f, value, ctx) is an input: a fieldset named "p:<field>"
//   whose readValue() gives the value as the API takes it, which sends a
//   "change" event when the value changes and is disabled as a whole.
//   fieldView(f, value, ctx) shows a value for reading.
//
// ctx.item is the item whose field it is (for its files) and
// ctx.attachments its files, as GET /api/items/{ref}/attachments lists
// them. A referenced item the reader may not read never reaches the page:
// the server leaves such a value out (D10).
import { h } from "./dom.js";
import { get } from "./api.js";
import { t } from "./i18n.js";
import { maxRating, richKinds, stars } from "./typeparts.js";

export { richKinds };

// isRich reports whether fields.js draws a field of this kind.
export function isRich(f) {
  return richKinds.includes(f?.kind);
}

// refs remembers what an item is called, by ID, for one page.
const refs = new Map();

// itemName names an item by its key and title, read once; null when the
// reader may not read it (or it is gone).
export async function itemName(id) {
  if (!id) return null;
  if (!refs.has(id)) {
    refs.set(id, get("/api/items/" + encodeURIComponent(id))
      .then(({ item }) => ({ key: item.key, title: item.title, presence: item.presence }))
      .catch(() => null));
  }
  return refs.get(id);
}

const fileURL = (item, id) => `/api/items/${encodeURIComponent(item.id)}/attachments/${encodeURIComponent(id)}`;
const isImage = (a) => /^image\//.test(a?.type || "");

// fileOf is the file a file field names, among the item's live files.
function fileOf(ctx, id) {
  return (ctx.attachments || []).find((a) => a.id === id && !a.deleted) || null;
}

// fieldView shows a value of a field for reading; for the other kinds it
// is the value as text.
export function fieldView(f, value, ctx = {}) {
  const none = () => h("span", { class: "muted", text: "—" });
  if (value === null || value === undefined || value === "" || (Array.isArray(value) && !value.length)) return none();
  switch (f.kind) {
    case "multi":
      return h("span", { class: "field-chips" }, (Array.isArray(value) ? value : [value]).map((v) => h("span", { class: "chip", text: v })));
    case "rating":
      return h("span", { class: "field-stars", role: "img", "aria-label": t("fields.ratingOf", { n: value, max: maxRating }), text: stars(value) });
    case "item": {
      const a = h("a", { class: "field-ref", href: "#/item/" + encodeURIComponent(value), text: "…" });
      itemName(value).then((n) => {
        if (!n) {
          a.replaceWith(h("span", { class: "muted", text: t("fields.notRevealed") }));
          return;
        }
        a.href = "#/item/" + encodeURIComponent(n.key);
        a.textContent = `${n.key} ${n.title}`.trim();
      });
      return a;
    }
    case "file": {
      const file = fileOf(ctx, value);
      if (!file || !ctx.item) return none();
      return isImage(file)
        ? h("a", { href: fileURL(ctx.item, file.id), target: "_blank", rel: "noopener", class: "field-cover" },
          h("img", { src: fileURL(ctx.item, file.id), alt: file.name, loading: "lazy" }))
        : h("a", { href: fileURL(ctx.item, file.id), text: file.name });
    }
    default:
      return h("span", { text: String(value) });
  }
}

// fieldInput is the input of a field of the kinds item, multi, rating and
// file.
export function fieldInput(f, value, ctx = {}) {
  const box = h("fieldset", { class: "field-kind field-kind-" + f.kind, name: "p:" + f.name });
  // The inputs inside speak only through the box: one change event, sent
  // when the value changed, and readValue().
  box.addEventListener("change", (ev) => { if (ev.target !== box) ev.stopPropagation(); }, true);
  const changed = () => box.dispatchEvent(new Event("change"));
  switch (f.kind) {
    case "multi": return multiInput(box, f, value, changed);
    case "rating": return ratingInput(box, f, value, changed);
    case "item": return itemInput(box, f, value, changed);
    case "file": return fileInput(box, f, value, ctx, changed);
  }
  return box;
}

function multiInput(box, f, value, changed) {
  const chosen = new Set(Array.isArray(value) ? value : value ? [value] : []);
  box.append(h("legend", { class: "visually-hidden", text: f.name }),
    ...(f.values || []).map((v) => {
      const cb = h("input", { type: "checkbox", value: v, checked: chosen.has(v) });
      cb.addEventListener("input", () => { (cb.checked ? chosen.add(v) : chosen.delete(v)); changed(); });
      return h("label", { class: "check" }, cb, " ", v);
    }));
  // The values in the order the field lists them; none unsets the field.
  box.readValue = () => {
    const out = (f.values || []).filter((v) => chosen.has(v));
    return out.length ? out : null;
  };
  return box;
}

function ratingInput(box, f, value, changed) {
  let current = Number(value) || 0;
  const buttons = [];
  const paint = () => buttons.forEach((b, i) => {
    b.textContent = i < current ? "★" : "☆";
    b.setAttribute("aria-pressed", String(i + 1 === current));
  });
  for (let i = 1; i <= maxRating; i++) {
    const b = h("button", { type: "button", class: "star", "aria-label": t("fields.ratingOf", { n: i, max: maxRating }) });
    b.addEventListener("click", () => { current = i; paint(); changed(); });
    buttons.push(b);
  }
  const clear = h("button", { type: "button", class: "link", text: t("fields.clear") });
  clear.addEventListener("click", () => { current = 0; paint(); changed(); });
  box.append(h("legend", { class: "visually-hidden", text: f.name }), h("span", { class: "stars" }, buttons), " ", clear);
  paint();
  box.readValue = () => (current ? current : null);
  return box;
}

function itemInput(box, f, value, changed) {
  let current = value || null;
  const targets = f.targets || [];
  const input = h("input", { type: "search", class: "item-search", autocomplete: "off", placeholder: t("fields.searchItem"),
    "aria-label": f.name, role: "combobox", "aria-expanded": "false", "aria-autocomplete": "list" });
  const list = h("ul", { class: "item-suggestions", role: "listbox", hidden: true });
  const shown = h("a", { class: "field-ref", hidden: true });
  const clear = h("button", { type: "button", class: "link", text: t("fields.clear"), hidden: true });
  const show = async () => {
    const n = current ? await itemName(current) : null;
    shown.hidden = clear.hidden = !n;
    if (n) {
      shown.href = "#/item/" + encodeURIComponent(n.key);
      shown.textContent = `${n.key} ${n.title}`.trim();
    }
  };
  const choose = (it) => {
    current = it ? it.id : null;
    if (it) refs.set(it.id, Promise.resolve({ key: it.key, title: it.title, presence: it.presence }));
    input.value = "";
    list.hidden = true;
    input.setAttribute("aria-expanded", "false");
    show();
    changed();
  };
  clear.addEventListener("click", () => choose(null));
  let timer = 0;
  let found = [];
  let at = -1;
  const mark = () => [...list.children].forEach((li, i) => li.setAttribute("aria-selected", String(i === at)));
  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value.trim();
    if (!q) {
      list.hidden = true;
      return;
    }
    timer = setTimeout(async () => {
      const p = new URLSearchParams({ q, limit: "20" });
      if (targets.length === 1) p.set("type", targets[0]);
      const { items } = await get("/api/items?" + p).catch(() => ({ items: [] }));
      found = items.filter((it) => !targets.length || targets.includes(it.type)).slice(0, 10);
      at = -1;
      list.replaceChildren(...found.map((it) => h("li", { role: "option", "aria-selected": "false" },
        h("button", { type: "button", class: "link", onclick: () => choose(it) },
          h("strong", { text: it.key }), " " + it.title + " ", h("span", { class: "muted", text: it.type })))));
      if (!found.length) list.replaceChildren(h("li", { class: "muted", text: t("fields.noMatch") }));
      list.hidden = false;
      input.setAttribute("aria-expanded", "true");
    }, 150);
  });
  input.addEventListener("keydown", (ev) => {
    if (list.hidden || !found.length) return;
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      at = (at + (ev.key === "ArrowDown" ? 1 : found.length - 1)) % found.length;
      mark();
    } else if (ev.key === "Enter" && at >= 0) {
      ev.preventDefault();
      choose(found[at]);
    } else if (ev.key === "Escape") {
      list.hidden = true;
    }
  });
  box.append(h("legend", { class: "visually-hidden", text: f.name }), h("span", { class: "item-chosen" }, shown, " ", clear),
    h("span", { class: "item-picker" }, input, list));
  show();
  box.readValue = () => current;
  return box;
}

function fileInput(box, f, value, ctx, changed) {
  const files = (ctx.attachments || []).filter((a) => !a.deleted);
  if (!ctx.item) {
    box.append(h("span", { class: "muted", text: t("fields.fileLater") }));
    box.readValue = () => null;
    return box;
  }
  if (!files.length) {
    box.append(h("span", { class: "muted", text: t("fields.noFiles") }));
    box.readValue = () => value || null;
    return box;
  }
  // Pictures first: a file field is mostly a cover picture.
  const sorted = [...files.filter(isImage), ...files.filter((a) => !isImage(a))];
  const select = h("select", { "aria-label": f.name },
    h("option", { value: "", text: "—" }),
    sorted.map((a) => h("option", { value: a.id, text: a.name, selected: a.id === value })));
  const preview = h("span", { class: "field-cover-preview" });
  const paint = () => preview.replaceChildren(select.value ? fieldView(f, select.value, ctx) : "");
  select.addEventListener("input", () => { paint(); changed(); });
  box.append(h("legend", { class: "visually-hidden", text: f.name }), select, preview);
  paint();
  box.readValue = () => select.value || null;
  return box;
}
