// The views of the items of a type (ADR-0201) without a browser: the
// view's state as an address and as a block of a text, the columns a type
// offers and shows by default, what a view may be grouped by, the groups
// of items with their counts, the order of the items by a column or by
// hand (ADR-0202), the cover picture of a card, and the text of a value
// of any field kind, a kind this module does not know read as text.
//
// Plain functions over plain data, so node --test runs them
// (internal/web/jstest/itemviews.test.mjs); itemview.js draws them.

// LAYOUTS are the layouts of a view: a table, a gallery of cards and a
// compact list.
export const LAYOUTS = ["table", "gallery", "list"];

// BUILTINS are the columns every item has, as the server names them
// (item.ViewBuiltins); a field of the type is FIELD and its name.
export const BUILTINS = ["key", "title", "state", "labels", "space", "type", "updated"];
export const FIELD = "field:";

// GROUP_KINDS are the kinds of fields a view is grouped by, besides the
// state and the labels: one value of a few, a person, yes or no.
export const GROUP_KINDS = ["enum", "user", "bool"];

// NONE is the key of the group of items without a value.
export const NONE = "";

// fieldOf is the field a column, a grouping or a sort names, or null.
export function fieldOf(type, column) {
  if (!column?.startsWith(FIELD)) return null;
  const name = column.slice(FIELD.length);
  return (type?.fields || []).find((f) => f.name === name) || null;
}

// readState reads a view's state from its address's parameters. Unknown
// values fall back to the defaults; columns are named by col, each once.
export function readState(params) {
  const get = (k) => params.get(k) || "";
  const cols = [...new Set(params.getAll("col").filter((c) => BUILTINS.includes(c) || (c.startsWith(FIELD) && c.length > FIELD.length)))];
  return {
    layout: LAYOUTS.includes(get("layout")) ? get("layout") : "table",
    space: get("space"),
    type: get("type"),
    query: get("query"),
    group: get("group"),
    columns: cols,
    sort: get("sort"),
  };
}

// address writes a view's state as its address, leaving out what holds
// its default, so a shared link stays short.
export function address(state) {
  const p = new URLSearchParams();
  if (state.layout && state.layout !== "table") p.set("layout", state.layout);
  for (const k of ["space", "type", "query", "group", "sort"]) if (state[k]) p.set(k, state[k]);
  for (const c of state.columns || []) p.append("col", c);
  const q = p.toString();
  return "#/view" + (q ? "?" + q : "");
}

// toBlock is the block of a text that keeps a view's state (block kind
// view, item.ItemsView); fromBlock reads it back.
export function toBlock(state) {
  const b = { kind: "view", layout: LAYOUTS.includes(state.layout) ? state.layout : "table" };
  for (const [k, key] of [["type", "itemType"], ["space", "space"], ["query", "query"], ["group", "group"], ["sort", "sort"]]) if (state[k]) b[key] = state[k];
  if (state.columns?.length) b.columns = [...state.columns];
  return b;
}

export function fromBlock(b) {
  return {
    layout: LAYOUTS.includes(b.layout) ? b.layout : "table",
    type: b.itemType || "", space: b.space || "", query: b.query || "", group: b.group || "",
    columns: [...(b.columns || [])], sort: b.sort || "",
  };
}

// columnsOf lists the columns a type offers: the built-in ones, the state
// only with a workflow, and every field of the type. Without a type, the
// built-in ones.
export function columnsOf(type) {
  const builtins = BUILTINS.filter((c) => c !== "state" || !type || type.workflow);
  return [...builtins, ...(type?.fields || []).map((f) => FIELD + f.name)];
}

// defaultColumns are the columns shown when a view names none: the key,
// the title, the state with a workflow, the first four fields, the labels
// and when the item last changed.
export function defaultColumns(type) {
  return ["key", "title", ...(type?.workflow ? ["state"] : []), ...(type?.fields || []).slice(0, 4).map((f) => FIELD + f.name), "labels", "updated"];
}

// shownColumns are the columns a view shows: those it names that the type
// still offers, or the default.
export function shownColumns(state, type) {
  const offered = columnsOf(type);
  const named = (state.columns || []).filter((c) => offered.includes(c) || (!type && c.startsWith(FIELD)));
  return named.length ? named : defaultColumns(type);
}

// groupChoices are what a view of a type may be grouped by: its state
// with a workflow, the labels, and its fields of the kinds in GROUP_KINDS.
export function groupChoices(type) {
  return [...(type?.workflow || !type ? ["state"] : []), "labels",
    ...(type?.fields || []).filter((f) => GROUP_KINDS.includes(f.kind)).map((f) => FIELD + f.name)];
}

// valueOf is the value an item holds in a column: a built-in one or a
// field's property.
export function valueOf(item, column) {
  if (column.startsWith(FIELD)) return item.properties?.[column.slice(FIELD.length)] ?? null;
  if (column === "labels") return item.labels || [];
  return item[column] ?? null;
}

// fieldText writes a value of a field as text, by its kind: a person by
// their name (person), yes or no (yes, no), a point in time in the
// reader's time zone (moment), a list joined by commas, and any value of
// a kind not known here as its text, so a new kind degrades to text
// rather than failing.
export function fieldText(field, value, { person = (id) => id, yes = "yes", no = "no", moment = (v) => v } = {}) {
  if (value === null || value === undefined || value === "") return "";
  const kind = field?.kind;
  if (Array.isArray(value)) return value.map((v) => fieldText(field && { ...field, kind: kind === "multi" ? "text" : kind }, v, { person, yes, no, moment })).filter(Boolean).join(", ");
  switch (kind) {
    case "bool": return value === true ? yes : value === false ? no : String(value);
    case "user": return person(String(value));
    case "date": return String(value).slice(0, 10);
    case "datetime": return moment(String(value));
    case "number": return typeof value === "number" ? String(value) : String(value);
    case "rating": return typeof value === "number" ? "★".repeat(Math.max(0, Math.min(10, Math.round(value)))) : String(value);
    default:
      if (typeof value === "object") return String(value.title ?? value.name ?? value.key ?? value.id ?? JSON.stringify(value));
      return String(value);
  }
}

// sortKey is what an item is sorted by in a column: a number for numbers
// and times, else its text in lower case.
function sortKey(item, column, type, text) {
  const v = valueOf(item, column);
  const f = fieldOf(type, column);
  if (v === null || v === undefined || v === "" || (Array.isArray(v) && !v.length)) return null;
  if (typeof v === "number") return v;
  if (column === "updated" || f?.kind === "date" || f?.kind === "datetime") {
    const ms = Date.parse(v);
    if (!Number.isNaN(ms)) return ms;
  }
  return text(item, column).toLowerCase();
}

// sortItems orders items by the column the sort names, - before it for
// the other way; items without a value come last either way. Without a
// sort, an order set by hand (ids, in their order) puts the items it names
// first, in its order; the others keep the order they came in.
export function sortItems(items, sort, type, { text = (it, c) => fieldText(fieldOf(type, c), valueOf(it, c)), order = [] } = {}) {
  const out = [...items];
  if (!sort) {
    const rank = new Map(order.map((id, i) => [id, i]));
    return out.sort((a, b) => (rank.get(a.id) ?? Infinity) - (rank.get(b.id) ?? Infinity) || 0);
  }
  const down = sort.startsWith("-");
  const column = down ? sort.slice(1) : sort;
  const keyed = out.map((it, i) => ({ it, i, k: sortKey(it, column, type, text) }));
  keyed.sort((a, b) => {
    if (a.k === null || b.k === null) return (a.k === null) - (b.k === null) || a.i - b.i;
    let c = typeof a.k === "number" && typeof b.k === "number" ? a.k - b.k : String(a.k).localeCompare(String(b.k), undefined, { numeric: true });
    if (down) c = -c;
    return c || a.i - b.i;
  });
  return keyed.map((x) => x.it);
}

// groupItems puts items into the groups of what a view is grouped by, in
// a fixed order: a workflow's states and an enum's values in the type's
// order, then other values by their text, yes before no, and the group of
// items without a value (NONE) last. Grouped by labels, an item stands in
// the group of each of its labels. Each group keeps the items' order.
// Without a grouping, all items are one group.
export function groupItems(items, group, type, { text = (v) => String(v) } = {}) {
  if (!group) return [{ key: null, name: "", items: [...items] }];
  const field = fieldOf(type, group);
  const groups = new Map();
  const add = (key, it) => {
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(it);
  };
  for (const it of items) {
    if (group === "labels") {
      if (!(it.labels || []).length) add(NONE, it);
      for (const l of it.labels || []) add(l, it);
      continue;
    }
    const v = group === "state" ? it.state : it.properties?.[group.slice(FIELD.length)];
    add(v === null || v === undefined || v === "" ? NONE : typeof v === "boolean" ? v : String(v), it);
  }
  const fixed = group === "state" ? type?.workflow?.states || [] : field?.kind === "enum" ? field.values || [] : field?.kind === "bool" ? [true, false] : [];
  const keys = [...groups.keys()];
  const rank = (k) => (k === NONE ? Infinity : fixed.includes(k) ? fixed.indexOf(k) : fixed.length);
  keys.sort((a, b) => rank(a) - rank(b) || (rank(a) === fixed.length ? text(a).localeCompare(text(b), undefined, { numeric: true }) : 0));
  return keys.map((k) => ({ key: k, name: k === NONE ? "" : text(k), items: groups.get(k) }));
}

// coverOf names the cover picture of an item's card: the first picture of
// its text, or else the file of its first field of the kind file, as
// {item, attachment}; null without one. A file field's value is an
// attachment's ID, or an object that names it.
export function coverOf(item, type) {
  const img = (item.body || []).find((b) => b.kind === "image" && b.ref);
  if (img) return { item: item.id, attachment: img.ref };
  for (const f of type?.fields || []) {
    if (f.kind !== "file") continue;
    let v = item.properties?.[f.name];
    if (Array.isArray(v)) v = v[0];
    const id = typeof v === "string" ? v : v?.attachment ?? v?.id;
    if (id) return { item: v?.item || item.id, attachment: id };
  }
  return null;
}

// moveWithin is the order of ids after moving moved (in their order) right
// before the id before, or to the end without it; the client shows it
// while the server keeps it (PUT /api/items/{ref}/order).
export function moveWithin(ids, moved, before) {
  const rest = ids.filter((id) => !moved.includes(id));
  const at = before && rest.includes(before) ? rest.indexOf(before) : rest.length;
  return [...rest.slice(0, at), ...moved, ...rest.slice(at)];
}

// editableColumn says whether a cell of a column is changed in place: the
// title, a field of a kind the item page edits, and the state, by its
// workflow's transitions (D3).
export const EDIT_KINDS = ["text", "number", "date", "datetime", "enum", "user", "bool", "url", "item", "multi", "rating"];
export function editableColumn(column, type) {
  if (column === "title") return true;
  if (column === "state") return Boolean(type?.workflow);
  const f = fieldOf(type, column);
  return Boolean(f && EDIT_KINDS.includes(f.kind));
}

// transitionsFrom are the transitions of a workflow that leave a state.
export function transitionsFrom(workflow, state) {
  return (workflow?.transitions || []).filter((tr) => (tr.from || []).includes(state));
}

// listURL is the listing a view reads: its type and room, narrowed by its
// query, at most a hundred at a time; label narrows it to the items that
// carry a label (a label's page).
export function listURL(state, label = "") {
  const p = new URLSearchParams({ limit: "100" });
  if (state.space) p.set("space", state.space);
  if (state.type) p.set("type", state.type);
  if (label) p.set("label", label);
  if (state.query) p.set("query", state.query);
  return "/api/items?" + p.toString();
}
