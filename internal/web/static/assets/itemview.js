// The views of the items of a type (ADR-0201): a table, a gallery of
// cards or a compact list of the items one may read, grouped by a field
// under headers that fold, with their counts. A view at #/view keeps its
// whole state in the address, as the roadmap and the calendar do; a view
// in a text is a block of the kind view; a label's page shows the items
// that carry its label so, ordered by hand by whoever may edit the page
// (ADR-0202). Nothing is kept for a view: it is computed from the item
// listing, so a reader sees only what they read (D10).
//
// Cells change in place where one may edit: the title, the fields the
// item page edits, each change one validated command (D3), and the state
// only by a transition of its workflow. The layout and the order are
// itemviews.js; this module draws them. helpers are what app.js shares:
// readAll reads a listing up to the boards' limit, favouritesFirst orders
// the rooms, propertyInput and readProperty are the item page's inputs of
// fields, person names a user, learnNames learns names, mayEditSpace says
// whether one may edit in a room, formDialog asks in a modal, and route
// shows the address again.
import { h } from "./dom.js";
import { fieldView, isRich } from "./fields.js";
import { t, language } from "./i18n.js";
import { get, post, put } from "./api.js";
import { BUILTINS, FIELD, LAYOUTS, address, columnsOf, coverOf, editableColumn, fieldOf, fieldText, fromBlock, groupChoices, groupItems, listURL,
  readState, shownColumns, sortItems, toBlock, transitionsFrom, valueOf } from "./itemviews.js";

// shared are app.js's helpers, given once when the UI starts, so a view
// in a text draws itself wherever a text is shown (configureViews).
let shared = null;
export function configureViews(helpers) { shared = helpers; }
export const viewsReady = () => shared !== null;

// The names of the layouts and the built-in columns, with their keys
// written out so the catalogue's tests find them (ADR-0017).
const layoutName = (l) => ({ table: t("views.table"), gallery: t("views.gallery"), list: t("views.list") })[l];
const builtinName = (c) => ({ key: t("col.key"), title: t("col.title"), state: t("col.state"), labels: t("views.labels"), space: t("col.space"),
  type: t("col.type"), updated: t("views.updated") })[c];

// columnName is a column's heading: a built-in one in the UI's language, a
// field by its name (content is not translated).
export function columnName(column) {
  return column.startsWith(FIELD) ? column.slice(FIELD.length) : builtinName(column) || column;
}

// moment writes a point in time where the reader is.
const moment = (v) => {
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleString(language() || undefined, { dateStyle: "medium", timeStyle: "short" });
};

// valueText is the text of an item's value in a column.
function valueText(it, column, type, helpers) {
  const v = valueOf(it, column);
  if (column === "labels") return (v || []).join(", ");
  if (column === "updated") return v ? moment(v) : "";
  if (BUILTINS.includes(column)) return v == null ? "" : String(v);
  return fieldText(fieldOf(type, column), v, { person: helpers.person, yes: t("views.yes"), no: t("views.no"), moment });
}

// renderField draws the value of a field by its kind, a kind it does not
// know as text, so new kinds degrade rather than fail. It is the one place
// a view draws a field, to be pointed at a shared renderer of fields.
export function renderField(field, value, helpers) {
  const kind = field?.kind || "text";
  // References, choices of several values and ratings are drawn as the
  // item page draws them (ADR-0200); a file field needs the item's files,
  // which a listing does not carry, and stays text.
  if (isRich(field) && kind !== "file") return h("span", { class: "iv-value", "data-kind": kind }, fieldView(field, value));
  const text = fieldText(field, value, { person: helpers.person, yes: t("views.yes"), no: t("views.no"), moment });
  if (kind === "url" && typeof value === "string" && /^https?:\/\//.test(value)) {
    return h("a", { class: "iv-value", "data-kind": kind, href: value, rel: "noopener noreferrer", target: "_blank", text });
  }
  return h("span", { class: "iv-value", "data-kind": kind, text });
}

// cellContent draws an item's value in a column, read-only.
function cellContent(it, column, type, helpers) {
  if (column === "key") return h("a", { href: "#/item/" + encodeURIComponent(it.key), class: "key", text: it.key });
  if (column === "title") return h("a", { href: "#/item/" + encodeURIComponent(it.key), class: "iv-title", text: it.title || it.key });
  if (column === "labels") return h("span", { class: "iv-labels" }, (it.labels || []).map((l) => h("a", { class: "label", href: "#/label/" + encodeURIComponent(l), text: l })));
  if (column === "state") return it.state ? h("span", { class: "chip", text: it.state }) : "";
  if (column.startsWith(FIELD)) return renderField(fieldOf(type, column) || { name: column.slice(FIELD.length), kind: "text" }, valueOf(it, column), helpers);
  return h("span", { text: valueText(it, column, type, helpers) });
}

// editCell turns a cell into its input for one change: the title, a
// field by the item page's input, or the state by the transitions that
// leave it. A change is saved at once; Escape and leaving without a
// change give the cell back.
function editCell(td, it, column, type, o) {
  const status = h("span", { class: "iv-status", role: "status" });
  const done = () => o.refresh();
  const back = () => td.replaceChildren(...cellBody(it, column, type, o));
  const save = async (send) => {
    status.textContent = t("common.saving");
    try {
      await send();
      done();
    } catch (err) {
      status.textContent = err.message;
    }
  };
  let input;
  if (column === "title") {
    input = h("input", { class: "iv-input", value: it.title, "aria-label": t("col.title"), maxlength: 500 });
    input.addEventListener("change", () => input.value.trim() && input.value !== it.title && save(() => put(`/api/items/${it.id}/title`, { title: input.value.trim() })));
  } else if (column === "state") {
    const ways = transitionsFrom(type.workflow, it.state);
    input = h("select", { class: "iv-input", "aria-label": t("col.state") },
      h("option", { value: "", text: it.state }), ways.map((tr) => h("option", { value: tr.name, text: t("views.transition", { name: tr.name, to: tr.to }) })));
    input.addEventListener("change", () => input.value && save(() => post(`/api/items/${it.id}/transitions`, { name: input.value })));
  } else {
    const f = fieldOf(type, column);
    input = o.helpers.propertyInput(f, valueOf(it, column));
    input.classList.add("iv-input");
    input.setAttribute("aria-label", f.name);
    input.addEventListener("change", () => save(() => put(`/api/items/${it.id}/properties/${encodeURIComponent(f.name)}`, { value: o.helpers.readProperty(f, input) })));
  }
  input.addEventListener("keydown", (ev) => { if (ev.key === "Escape") { ev.preventDefault(); back(); } });
  td.replaceChildren(input, status);
  input.focus();
}

// cellBody is what a cell holds: its value, and where it may change, a
// button that opens its input.
function cellBody(it, column, type, o) {
  const content = cellContent(it, column, type, o.helpers);
  if (!o.editable(it) || !editableColumn(column, type) || (column === "state" && !transitionsFrom(type.workflow, it.state).length)) return [content];
  return [content, h("button", { type: "button", class: "link iv-edit", title: t("views.editCell", { column: columnName(column) }), "aria-label": t("views.editCell", { column: columnName(column) }),
    text: "✎", onclick: (ev) => editCell(ev.currentTarget.closest("td, .iv-field"), it, column, type, o) })];
}

// orderTools make an item a place in the order set by hand: it is dragged
// before another, or moved up and down by its buttons and by Alt with an
// arrow key (ADR-0202).
function orderTools(el, it, o) {
  if (!o.ordering) return null;
  el.draggable = true;
  el.dataset.id = it.id;
  el.classList.add("iv-orderable");
  el.addEventListener("dragstart", (ev) => {
    ev.dataTransfer.setData("text/x-orbithek-item", it.id);
    ev.dataTransfer.effectAllowed = "move";
    el.classList.add("iv-dragging");
  });
  el.addEventListener("dragend", () => el.classList.remove("iv-dragging"));
  el.addEventListener("dragover", (ev) => {
    if (!ev.dataTransfer.types.includes("text/x-orbithek-item")) return;
    ev.preventDefault();
    el.classList.add("iv-drop");
  });
  el.addEventListener("dragleave", () => el.classList.remove("iv-drop"));
  el.addEventListener("drop", (ev) => {
    el.classList.remove("iv-drop");
    const moved = ev.dataTransfer.getData("text/x-orbithek-item");
    if (!moved || moved === it.id) return;
    ev.preventDefault();
    o.place([moved], it.id);
  });
  const ids = o.sequence;
  const at = ids.indexOf(it.id);
  const up = () => at > 0 && o.place([it.id], ids[at - 1]);
  const down = () => at >= 0 && at < ids.length - 1 && o.place([it.id], ids[at + 2] || "");
  el.addEventListener("keydown", (ev) => {
    if (!ev.altKey || (ev.key !== "ArrowUp" && ev.key !== "ArrowDown")) return;
    ev.preventDefault();
    if (ev.key === "ArrowUp") up(); else down();
  });
  return h("span", { class: "iv-move" },
    h("button", { type: "button", class: "link iv-up", title: t("views.moveUp"), "aria-label": t("views.moveUpItem", { title: it.title }), text: "↑", disabled: at <= 0, onclick: up }),
    h("button", { type: "button", class: "link iv-down", title: t("views.moveDown"), "aria-label": t("views.moveDownItem", { title: it.title }), text: "↓", disabled: at < 0 || at >= ids.length - 1, onclick: down }));
}

// table draws items as rows with a column per chosen column; a heading
// sorts by its column, a second click the other way.
function table(items, cols, type, o, state) {
  const sortBy = state.sort.replace(/^-/, "");
  const down = state.sort.startsWith("-");
  const head = h("tr", {}, o.ordering ? h("th", { scope: "col", class: "iv-move-col" }, h("span", { class: "visually-hidden", text: t("views.order") })) : null,
    cols.map((c) => h("th", { scope: "col", "data-col": c, "aria-sort": c === sortBy ? (down ? "descending" : "ascending") : "none" },
      o.onSort ? h("button", { type: "button", class: "link sort", "data-sort": c, text: columnName(c), onclick: () => o.onSort(c === sortBy && !down ? "-" + c : c) }) : columnName(c))));
  const rows = items.map((it) => {
    const tr = h("tr", { "data-key": it.key, tabindex: o.ordering ? "0" : null });
    const move = orderTools(tr, it, o);
    tr.append(...[move ? h("td", { class: "iv-move-col" }, move) : null, ...cols.map((c) => h("td", { "data-col": c }, cellBody(it, c, type, o)))].filter(Boolean));
    return tr;
  });
  return h("div", { class: "table-scroll" }, h("table", { class: "list iv-table" }, h("thead", {}, head), h("tbody", {}, rows)));
}

// gallery draws items as cards: a cover picture, the title and the chosen
// fields.
function gallery(items, cols, type, o) {
  const shown = cols.filter((c) => c !== "title" && c !== "key");
  return h("ul", { class: "iv-gallery" }, items.map((it) => {
    const cover = coverOf(it, type);
    const card = h("li", { class: "iv-card", "data-key": it.key, tabindex: o.ordering ? "0" : null },
      h("a", { class: "iv-cover", href: "#/item/" + encodeURIComponent(it.key), tabindex: "-1", "aria-hidden": "true" },
        cover ? h("img", { src: `/api/items/${encodeURIComponent(cover.item)}/attachments/${encodeURIComponent(cover.attachment)}/thumbnail?edge=400`, alt: "", loading: "lazy", draggable: "false" })
          : h("span", { class: "iv-cover-none", text: (it.title || it.key).slice(0, 1).toUpperCase() })),
      h("div", { class: "iv-card-body" },
        h("p", { class: "iv-card-title" }, h("span", { class: "muted iv-card-key", text: it.key + " " }), h("span", { class: "iv-field", "data-col": "title" }, cellBody(it, "title", type, o))),
        shown.length ? h("dl", { class: "iv-fields" }, shown.flatMap((c) => [h("dt", { text: columnName(c) }), h("dd", { class: "iv-field", "data-col": c }, cellBody(it, c, type, o))])) : null));
    const move = orderTools(card, it, o);
    if (move) card.append(move);
    return card;
  }));
}

// list draws items as compact rows: the key and title, then the other
// columns' values in a line.
function list(items, cols, type, o) {
  const shown = cols.filter((c) => c !== "title" && c !== "key");
  return h("ul", { class: "iv-list" }, items.map((it) => {
    const row = h("li", { class: "iv-row", "data-key": it.key, tabindex: o.ordering ? "0" : null });
    const move = orderTools(row, it, o);
    row.append(...[move,
      h("a", { href: "#/item/" + encodeURIComponent(it.key), class: "key", text: it.key }), " ",
      h("span", { class: "iv-field", "data-col": "title" }, cellBody(it, "title", type, o)),
      h("span", { class: "iv-row-rest muted" }, shown.map((c) => h("span", { class: "iv-field", "data-col": c, title: columnName(c) }, cellBody(it, c, type, o))))].filter(Boolean));
    return row;
  }));
}

// groupName is the header of a group: a person by name, yes or no, a
// value as it is.
function groupName(group, key, type, helpers) {
  const f = fieldOf(type, group);
  if (f) return fieldText(f, key, { person: helpers.person, yes: t("views.yes"), no: t("views.no"), moment });
  return String(key);
}

// itemsView draws items as a view's state asks. o: helpers; editable(it)
// whether one may change it; refresh() shows the view again after a
// change; onSort(sort) chooses a sort, or none for headings that do not
// sort; order, the IDs set by hand, and place(moved, before) to change
// them, when one may (ADR-0202).
export function itemsView(state, items, type, o) {
  const cols = shownColumns(state, type);
  const text = (it, c) => valueText(it, c, type, o.helpers);
  const sorted = sortItems(items, state.sort, type, { text, order: o.order || [] });
  const groups = groupItems(sorted, state.group, type, { text: (k) => groupName(state.group, k, type, o.helpers) });
  const opts = { ...o, ordering: Boolean(o.place) && !state.sort, sequence: sorted.map((it) => it.id) };
  const draw = (g) => state.layout === "gallery" ? gallery(g.items, cols, type, opts) : state.layout === "list" ? list(g.items, cols, type, opts) : table(g.items, cols, type, opts, state);
  if (!items.length) return h("p", { class: "empty iv-empty", text: t("views.none") });
  const content = state.group ? groups.map((g) => h("details", { class: "iv-group", open: true, "data-group": g.key === "" ? "" : String(g.key) },
    h("summary", {}, h("span", { class: "iv-group-name", text: g.name || t("views.noValue") }), " ", h("span", { class: "muted iv-count", text: t("views.count", { n: g.items.length }) })),
    draw(g))) : [draw(groups[0])];
  return h("div", { class: "iv iv-" + state.layout }, opts.ordering ? h("p", { class: "muted iv-order-hint", text: t("views.orderHint") }) : null, content);
}

// readItems reads what a view shows, as far as the boards' limit; a query
// that does not parse throws, saying where.
async function readItems(state, helpers, label = "") {
  const found = await helpers.readAll(listURL(state, label));
  return found;
}

// typeOfState is the type a view shows, or null for every type.
const typeOfState = (types, state) => types.find((ty) => ty.name === state.type) || null;

// controls are the choices of a view: its layout, room, type, query,
// grouping, sort and columns; change(patch) applies them.
function controls(state, types, spaces, type, helpers, change, idPrefix) {
  const field = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);
  const select = (id, label, options, value, key) => field(label, h("select", { id: idPrefix + id, onchange: (ev) => change({ [key]: ev.target.value }) },
    options.map(([v, text]) => h("option", { value: v, text, selected: v === value }))));
  const query = h("textarea", { id: idPrefix + "query", rows: 2, placeholder: t("views.queryPlaceholder"), "aria-label": t("views.query") });
  query.value = state.query;
  const cols = shownColumns(state, type);
  const chooser = h("details", { class: "iv-columns", id: idPrefix + "columns" }, h("summary", { text: t("views.columns") }),
    h("p", { class: "iv-column-choice" }, columnsOf(type).map((c) => h("label", { class: "check" },
      h("input", { type: "checkbox", value: c, checked: cols.includes(c), onchange: (ev) => {
        const next = ev.target.checked ? [...cols, c] : cols.filter((x) => x !== c);
        change({ columns: next });
      } }), " ", columnName(c)))));
  return h("div", { class: "card iv-controls", id: idPrefix + "controls" },
    h("nav", { class: "tabs", "aria-label": t("views.layout") }, LAYOUTS.map((l) =>
      h("button", { type: "button", class: l === state.layout ? "tab active" : "tab", id: idPrefix + "layout-" + l, "aria-pressed": String(l === state.layout), text: layoutName(l), onclick: () => change({ layout: l }) }))),
    h("div", { class: "form-row" },
      select("space", t("col.space"), [["", t("views.allSpaces")], ...helpers.favouritesFirst(spaces).map((sp) => [sp.key, `${sp.key} ${sp.name}`])], state.space, "space"),
      select("type", t("col.type"), [["", t("views.allTypes")], ...types.map((ty) => [ty.name, ty.name])], state.type, "type"),
      select("group", t("views.group"), [["", t("views.noGroup")], ...groupChoices(type).map((g) => [g, columnName(g)])], state.group, "group"),
      select("sort", t("views.sort"), [["", t("views.sortNone")], ...columnsOf(type).flatMap((c) => [[c, t("views.sortUp", { column: columnName(c) })], ["-" + c, t("views.sortDown", { column: columnName(c) })]])], state.sort, "sort")),
    h("div", { class: "iv-query-row" }, field(t("views.query"), query),
      h("button", { type: "button", class: "secondary", id: idPrefix + "query-apply", text: t("views.queryApply"), onclick: () => change({ query: query.value.trim() }) })),
    chooser);
}

// viewScreen shows the view at #/view: its controls, then the items.
export async function viewScreen(container, params, helpers = shared) {
  const state = readState(params);
  const [{ types }, { spaces }] = await Promise.all([get("/api/types"), get("/api/spaces"), helpers.loadRoles()]);
  const type = typeOfState(types, state);
  const go = (patch) => {
    const next = { ...state, ...patch };
    // Another type has other fields: what named the old ones is dropped.
    if (patch.type !== undefined && patch.type !== state.type) Object.assign(next, { columns: [], group: next.group.startsWith(FIELD) ? "" : next.group, sort: next.sort.replace(/^-/, "").startsWith(FIELD) ? "" : next.sort });
    location.hash = address(next);
  };
  const result = h("div", { id: "view-items" });
  const status = h("div", { class: "status" });
  container.replaceChildren(controls(state, types, spaces, type, helpers, go, "view-"), status, result);
  let found;
  try {
    found = await readItems(state, helpers);
  } catch (err) {
    status.replaceChildren(h("p", { class: "error", id: "view-error", text: err.at !== undefined ? t("views.queryError", { error: err.message, at: err.at + 1 }) : err.message }));
    return;
  }
  const items = type ? found.items.filter((it) => it.type === type.name) : found.items;
  await helpers.learnNames(personIDs(items, type));
  result.replaceChildren(
    found.truncated ? h("p", { class: "notice warning", text: t("board.truncated", { n: items.length }) }) : "",
    itemsView(state, items, type, { helpers, editable: (it) => helpers.mayEditSpace(it.space), refresh: helpers.route, onSort: (sort) => go({ sort }) }),
    h("p", { class: "muted iv-count-all", id: "view-count", text: t("views.total", { n: items.length }) }));
}

// personIDs are the people the user fields of items name.
function personIDs(items, type) {
  const users = (type?.fields || []).filter((f) => f.kind === "user").map((f) => f.name);
  return [...new Set(items.flatMap((it) => (type ? users : Object.keys(it.properties || {})).map((n) => it.properties?.[n]).filter((v) => typeof v === "string" && v)))];
}

// viewBlock draws a view placed in a text: the items it shows, as far as
// the reader reads them, and a link to the same view at #/view.
export async function viewBlock(block, helpers = shared) {
  const state = fromBlock(block);
  const [{ types }] = await Promise.all([get("/api/types"), helpers.loadRoles()]);
  const type = typeOfState(types, state);
  const box = h("div", { class: "iv-block-items" });
  const draw = async () => {
    try {
      const found = await readItems(state, helpers);
      const items = type ? found.items.filter((it) => it.type === type.name) : found.items;
      await helpers.learnNames(personIDs(items, type));
      box.replaceChildren(itemsView(state, items, type, { helpers, editable: (it) => helpers.mayEditSpace(it.space), refresh: draw }),
        found.truncated ? h("p", { class: "muted", text: t("board.truncated", { n: items.length }) }) : "");
    } catch (err) {
      box.replaceChildren(h("p", { class: "error", text: err.message }));
    }
  };
  await draw();
  return h("div", { class: "iv-block" },
    h("p", { class: "iv-block-head" }, h("span", { class: "muted", text: describe(state) }), " ", h("a", { href: address(state), class: "iv-open", text: t("views.open") })),
    box);
}

// describe says what a view shows, in words.
export function describe(state) {
  return t("views.describe", { layout: layoutName(state.layout), type: state.type || t("views.allTypes"), space: state.space || t("views.allSpaces") })
    + (state.query ? " · " + state.query : "");
}

// pickView asks for what a view in a text shows, with the choices of the
// view at #/view; initial is the state to start from. It answers the
// block, or null when cancelled.
export async function pickView(initial, helpers = shared) {
  const [{ types }, { spaces }] = await Promise.all([get("/api/types"), get("/api/spaces")]);
  let state = { layout: "table", space: "", type: "", query: "", group: "", columns: [], sort: "", ...initial };
  const holder = h("div", { class: "iv-pick" });
  const redraw = () => holder.replaceChildren(controls(state, types, spaces, typeOfState(types, state), helpers, (patch) => {
    if (patch.type !== undefined && patch.type !== state.type) patch = { ...patch, columns: [], group: "", sort: "" };
    state = { ...state, ...patch };
    redraw();
  }, "view-pick-"));
  redraw();
  return helpers.formDialog(t("views.insert"), "view-dialog", [holder, h("p", { class: "muted", text: t("views.insertHint") })], async () => {
    const q = holder.querySelector("textarea").value.trim();
    state.query = q;
    if (q) await get(listURL({ ...state }).replace("limit=100", "limit=1")); // says where a mistake is
    return toBlock(state);
  }, t("views.insertOk"));
}

// labelItems draws the items that carry a label as a view whose layout
// and grouping are chosen on the spot and kept in the address of the
// label's page (topic), which is not one of them; order is the order set
// by hand on the page, which orderable lets the reader change (ADR-0202).
export async function labelItems(label, { state, order = [], topic, orderable = false, helpers = shared, change }) {
  const [found, { types }] = await Promise.all([helpers.readAll(listURL({}, label)), get("/api/types"), helpers.loadRoles()]);
  const items = found.items.filter((it) => it.id !== topic.id);
  // One type among the items gives its fields; several give the built-in
  // columns only.
  const kinds = [...new Set(items.map((it) => it.type))];
  const type = kinds.length === 1 ? types.find((ty) => ty.name === kinds[0]) || null : null;
  await helpers.learnNames(personIDs(items, type));
  const status = h("div", { class: "status" });
  const place = orderable ? async (moved, before) => {
    try {
      await put(`/api/items/${topic.id}/order`, { items: moved, before });
      helpers.route();
    } catch (err) {
      status.replaceChildren(h("p", { class: "error", text: err.message }));
    }
  } : null;
  const layouts = h("nav", { class: "tabs", "aria-label": t("views.layout") }, LAYOUTS.map((l) =>
    h("button", { type: "button", class: l === state.layout ? "tab active" : "tab", id: "label-layout-" + l, "aria-pressed": String(l === state.layout), text: layoutName(l), onclick: () => change({ layout: l }) })));
  const groupSelect = h("select", { id: "label-group", "aria-label": t("views.group"), onchange: (ev) => change({ group: ev.target.value }) },
    [["", t("views.noGroup")], ...groupChoices(type).map((g) => [g, columnName(g)])].map(([v, text]) => h("option", { value: v, text, selected: v === state.group })));
  const sortSelect = h("select", { id: "label-sort", "aria-label": t("views.sort"), onchange: (ev) => change({ sort: ev.target.value }) },
    [["", t("views.sortHand")], ...columnsOf(type).flatMap((c) => [[c, t("views.sortUp", { column: columnName(c) })], ["-" + c, t("views.sortDown", { column: columnName(c) })]])]
      .map(([v, text]) => h("option", { value: v, text, selected: v === state.sort })));
  return h("section", { class: "iv-label", id: "label-view" },
    h("div", { class: "iv-label-bar" }, layouts, h("label", { class: "field" }, h("span", { text: t("views.group") }), groupSelect),
      h("label", { class: "field" }, h("span", { text: t("views.sort") }), sortSelect)),
    status,
    itemsView(state, items, type, { helpers, editable: (it) => helpers.mayEditSpace(it.space), refresh: helpers.route, order, place,
      onSort: (sort) => change({ sort }) }),
    found.truncated ? h("p", { class: "notice warning", text: t("board.truncated", { n: items.length }) }) : null);
}
