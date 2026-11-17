// The parts of a table block that hold no DOM (ADR-0042, ADR-0214): its
// cells with their formats, split from and joined into the block's text
// and marks; rows and columns added and removed with their sizes and
// colours; colours set on a range of cells; and the sizes a resize gives.
// The model counts mark offsets in Unicode code points.

// TONES are the tones a cell is filled or bordered with, as the model
// names them (item.TableTones).
export const TONES = ["blue", "green", "yellow", "orange", "red", "purple", "teal", "gray"];

// MARKS are the formats a cell may carry; a mention and a protected
// passage are not among them.
export const MARKS = ["bold", "italic", "underline", "strike", "highlight", "code", "link"];

// The sizes a column and a row may be given, in CSS pixels.
export const COLUMN = { min: 24, max: 2000 };
export const ROW = { min: 16, max: 1000 };

const cp = (s) => Array.from(s).length;

// cellsOf splits a table's text and marks into rows of cells, each
// { text, marks } with offsets counted in the cell.
export function cellsOf(text, marks = []) {
  const rows = [];
  let at = 0;
  for (const line of (text || "").split("\n")) {
    const row = [];
    for (const cell of line.split("\t")) {
      const n = cp(cell);
      row.push({ text: cell, marks: marks.filter((m) => m.start >= at && m.end <= at + n && MARKS.includes(m.type))
        .map((m) => ({ ...m, start: m.start - at, end: m.end - at })) });
      at += n + 1;
    }
    rows.push(row);
  }
  return rows;
}

// cleanCell is a cell's text without the tabs and line breaks that would
// split it, its marks kept within it.
export function cleanCell(cell) {
  const text = cell.text.replace(/[\t\r\n]/g, " ");
  const n = cp(text);
  const marks = (cell.marks || []).map((m) => ({ ...m, start: Math.max(0, m.start), end: Math.min(n, m.end) })).filter((m) => m.end > m.start);
  return { text, marks };
}

// joinCells is the text and marks of a table of cells, as the model holds
// them; marks of one kind and target that touch are joined.
export function joinCells(rows) {
  const marks = [];
  let at = 0;
  const lines = rows.map((row) => row.map((c) => {
    const cell = cleanCell(c);
    for (const m of normalMarks(cell.marks)) marks.push({ ...m, start: m.start + at, end: m.end + at });
    at += cp(cell.text) + 1;
    return cell.text;
  }).join("\t"));
  return { text: lines.join("\n"), marks };
}

// normalMarks orders a cell's marks and joins those of one kind and target
// that touch or overlap.
export function normalMarks(marks) {
  const sorted = [...marks].sort((a, b) => a.type.localeCompare(b.type) || (a.href || "").localeCompare(b.href || "") || a.start - b.start);
  const out = [];
  for (const m of sorted) {
    const last = out[out.length - 1];
    if (last && last.type === m.type && (last.href || "") === (m.href || "") && m.start <= last.end) last.end = Math.max(last.end, m.end);
    else out.push({ ...m });
  }
  return out.sort((a, b) => a.start - b.start || a.type.localeCompare(b.type));
}

// A layout is a table's { widths, heights, cells } as the model holds
// them: widths and heights a size per column and row or null, cells the
// coloured ones { row, col, fill, border } in the order of rows and
// columns, or null.

// normalLayout is a layout as the model accepts it for a table of nRows
// and nCols: sizes within their bounds or 0, none when every one is 0,
// cells inside the table, each once, in order, each with a tone.
export function normalLayout({ widths, heights, cells } = {}, nRows, nCols) {
  const sizes = (v, n, { min, max }) => {
    if (!Array.isArray(v)) return null;
    const out = Array.from({ length: n }, (_, i) => {
      const x = Math.round(Number(v[i]) || 0);
      return x <= 0 ? 0 : Math.min(max, Math.max(min, x));
    });
    return out.some((x) => x) ? out : null;
  };
  const seen = new Map();
  for (const c of cells || []) {
    if (c.row < 0 || c.row >= nRows || c.col < 0 || c.col >= nCols) continue;
    const fill = TONES.includes(c.fill) ? c.fill : "";
    const border = TONES.includes(c.border) ? c.border : "";
    const key = c.row * nCols + c.col;
    if (fill || border) seen.set(key, { row: c.row, col: c.col, ...(fill ? { fill } : {}), ...(border ? { border } : {}) });
    else seen.delete(key);
  }
  const list = [...seen.entries()].sort((a, b) => a[0] - b[0]).map(([, c]) => c);
  return { widths: sizes(widths, nCols, COLUMN), heights: sizes(heights, nRows, ROW), cells: list.length ? list : null };
}

// colourOf is the colours of the cell at row r and column c.
export function colourOf(layout, r, c) {
  const cell = (layout.cells || []).find((x) => x.row === r && x.col === c);
  return { fill: cell?.fill || "", border: cell?.border || "" };
}

// rangeOf is the rectangle between two cells, { r0, c0, r1, c1 }.
export function rangeOf(a, b) {
  return { r0: Math.min(a.r, b.r), c0: Math.min(a.c, b.c), r1: Math.max(a.r, b.r), c1: Math.max(a.c, b.c) };
}

export const inRange = (range, r, c) => Boolean(range) && r >= range.r0 && r <= range.r1 && c >= range.c0 && c <= range.c1;

// paint sets what ("fill" or "border") of every cell in range to tone, or
// takes it off with an empty tone.
export function paint(layout, nRows, nCols, range, what, tone) {
  const cells = [...(layout.cells || [])];
  for (let r = range.r0; r <= range.r1; r++) {
    for (let c = range.c0; c <= range.c1; c++) {
      const i = cells.findIndex((x) => x.row === r && x.col === c);
      const next = { ...(i >= 0 ? cells[i] : { row: r, col: c }), [what]: tone };
      if (i >= 0) cells[i] = next;
      else cells.push(next);
    }
  }
  return normalLayout({ ...layout, cells }, nRows, nCols);
}

// formatCells turns a format on in every cell of range, all of each
// cell's text, or off where every cell of the range has it on already
// over all of its text.
export function formatCells(rows, range, type) {
  const covered = (cell) => cp(cell.text) === 0 || cell.marks.some((m) => m.type === type && m.start === 0 && m.end >= cp(cell.text));
  const cellsIn = [];
  rows.forEach((row, r) => row.forEach((cell, c) => { if (inRange(range, r, c)) cellsIn.push(cell); }));
  const off = cellsIn.every(covered);
  return rows.map((row, r) => row.map((cell, c) => {
    if (!inRange(range, r, c)) return cell;
    const marks = cell.marks.filter((m) => m.type !== type);
    if (!off && cp(cell.text) > 0) marks.push({ type, start: 0, end: cp(cell.text) });
    return { text: cell.text, marks };
  }));
}

const empty = () => ({ text: "", marks: [] });

// insertRow adds an empty row after row at (at the end when at is the
// last); sizes and colours below it move down with their rows.
export function insertRow(rows, layout, at) {
  const nCols = rows[0].length;
  const next = [...rows.slice(0, at + 1), Array.from({ length: nCols }, empty), ...rows.slice(at + 1)];
  const heights = layout.heights ? [...layout.heights.slice(0, at + 1), 0, ...layout.heights.slice(at + 1)] : null;
  const cells = (layout.cells || []).map((c) => (c.row > at ? { ...c, row: c.row + 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, heights, cells }, next.length, nCols) };
}

// insertColumn adds an empty column after column at; a new column of a
// table with widths takes the width of the column at.
export function insertColumn(rows, layout, at) {
  const next = rows.map((row) => [...row.slice(0, at + 1), empty(), ...row.slice(at + 1)]);
  const widths = layout.widths ? [...layout.widths.slice(0, at + 1), layout.widths[at] || 0, ...layout.widths.slice(at + 1)] : null;
  const cells = (layout.cells || []).map((c) => (c.col > at ? { ...c, col: c.col + 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, widths, cells }, next.length, next[0].length) };
}

// removeRow takes row at away with its size and colours; the last row
// stays.
export function removeRow(rows, layout, at) {
  if (rows.length < 2) return { rows, layout };
  const next = rows.filter((_, i) => i !== at);
  const heights = layout.heights ? layout.heights.filter((_, i) => i !== at) : null;
  const cells = (layout.cells || []).filter((c) => c.row !== at).map((c) => (c.row > at ? { ...c, row: c.row - 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, heights, cells }, next.length, next[0].length) };
}

// removeColumn takes column at away with its width and colours; the last
// column stays.
export function removeColumn(rows, layout, at) {
  if (rows[0].length < 2) return { rows, layout };
  const next = rows.map((row) => row.filter((_, i) => i !== at));
  const widths = layout.widths ? layout.widths.filter((_, i) => i !== at) : null;
  const cells = (layout.cells || []).filter((c) => c.col !== at).map((c) => (c.col > at ? { ...c, col: c.col - 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, widths, cells }, next.length, next[0].length) };
}

// resized is the layout after column (what "widths") or row (what
// "heights") i was dragged to size px. A column dragged in a table without widths
// fixes every column at the width it is shown with, measured, so that the
// others keep theirs; a row keeps the others as they are.
export function resized(layout, what, i, px, measured) {
  const bounds = what === "widths" ? COLUMN : ROW;
  const base = what === "widths" && !layout.widths ? measured.map((w) => Math.round(w)) : [...(layout[what] || measured.map(() => 0))];
  base[i] = Math.min(bounds.max, Math.max(bounds.min, Math.round(px)));
  return { ...layout, [what]: base };
}

// filledClass and borderClass are the classes that colour a cell; the
// stylesheet mixes a fill's tone into the theme's surface, so the theme's
// text stays readable on it, light or dark.
export const filledClass = (fill) => (fill ? "pm-cell-fill pm-fill-" + fill : "");
export const borderClass = (border) => (border ? "pm-cell-border pm-border-" + border : "");

// --- copied and pasted (ADR-0236) ---

// A table is copied as an HTML table, the header row as th: other
// programs take it as a table with its formats and column widths, and the
// editor reads it back whole from data-pm-table, which holds the block's
// text, marks and layout. A table from elsewhere (a spreadsheet, a word
// processor, a web page) is read from its cells: their text, the formats
// its tags, its inline styles and the classes of its stylesheet give, and
// its columns' widths; colours and row heights are left to the editor.

const ORDER = ["link", "bold", "italic", "underline", "strike", "highlight", "code"];
const TAGS = { bold: "strong", italic: "em", underline: "u", strike: "s", highlight: "mark", code: "code" };

// segments cuts a cell into runs of text with the marks over all of each.
function segments(cell) {
  const chars = Array.from(cell.text);
  const cuts = [...new Set([0, chars.length, ...cell.marks.flatMap((m) => [m.start, m.end])])].filter((x) => x >= 0 && x <= chars.length).sort((x, y) => x - y);
  const out = [];
  for (let i = 0; i + 1 < cuts.length; i++) {
    if (cuts[i] >= cuts[i + 1]) continue;
    out.push({ text: chars.slice(cuts[i], cuts[i + 1]).join(""), marks: cell.marks.filter((m) => m.start <= cuts[i] && m.end >= cuts[i + 1]) });
  }
  return out;
}

// tableDOM is the DOM output spec a table block is copied as; safe says
// whether a link's target may be written (ADR-0014).
// tableData is what data-pm-table holds of a table block: its text, marks
// and layout, as JSON. The table the editor draws carries it too, so that
// a copy the browser makes of a page being read holds it as well.
export function tableData(attrs) {
  const rows = cellsOf(attrs.text, attrs.marks || []);
  const layout = normalLayout({ widths: attrs.widths, heights: attrs.heights, cells: attrs.cells }, rows.length, Math.max(...rows.map((r) => r.length)));
  return JSON.stringify({ text: attrs.text, marks: attrs.marks || [], ...(layout.widths ? { widths: layout.widths } : {}),
    ...(layout.heights ? { heights: layout.heights } : {}), ...(layout.cells ? { cells: layout.cells } : {}) });
}

export function tableDOM(attrs, safe) {
  const rows = cellsOf(attrs.text, attrs.marks || []);
  const nCols = Math.max(...rows.map((r) => r.length));
  const layout = normalLayout({ widths: attrs.widths, heights: attrs.heights, cells: attrs.cells }, rows.length, nCols);
  const inline = (cell) => segments(cell).map((s) => {
    let el = s.text;
    for (const type of [...ORDER].reverse()) {
      const m = s.marks.find((x) => x.type === type);
      if (!m) continue;
      if (type === "link") { if (safe(m.href)) el = ["a", { href: m.href }, el]; } else el = [TAGS[type], el];
    }
    return el;
  });
  const row = (cells, tag) => ["tr", ...cells.map((c) => [tag, {}, ...inline(c)])];
  return ["table", { class: "pm-table", "data-pm-table": tableData(attrs) },
    ...(layout.widths ? [["colgroup", ...layout.widths.map((w) => ["col", w ? { width: String(w) } : {}])]] : []),
    ["thead", row(rows[0], "th")],
    ...(rows.length > 1 ? [["tbody", ...rows.slice(1).map((r) => row(r, "td"))]] : [])];
}

// declarations reads the declarations of a style attribute or rule into a
// map of lower-case property names to values.
export function declarations(text) {
  const out = new Map();
  for (const part of (text || "").split(";")) {
    const i = part.indexOf(":");
    if (i > 0) out.set(part.slice(0, i).trim().toLowerCase(), part.slice(i + 1).trim().toLowerCase());
  }
  return out;
}

// classRules reads the rules of a stylesheet whose selectors are single
// classes, as spreadsheets copy their cells' formats, into a map of class
// names to declarations; other rules are left out.
export function classRules(css) {
  const rules = new Map();
  const text = (css || "").replace(/\/\*[\s\S]*?\*\//g, "").replace(/<!--|-->/g, "");
  for (const [, selectors, body] of text.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    for (const sel of selectors.split(",")) {
      const m = sel.trim().match(/^(?:[a-z0-9]*)\.([\w-]+)$/i);
      if (m) rules.set(m[1], new Map([...(rules.get(m[1]) || []), ...declarations(body)]));
    }
  }
  return rules;
}

// styled turns on or off the formats a set of declarations states.
function styled(marks, decl) {
  const weight = decl.get("font-weight");
  if (weight) {
    if (/^(bold|bolder|[6-9]00)/.test(weight)) marks.bold = true;
    else if (/^(normal|lighter|[1-5]00)/.test(weight)) delete marks.bold;
  }
  const style = decl.get("font-style");
  if (style) {
    if (/^(italic|oblique)/.test(style)) marks.italic = true;
    else if (/^normal/.test(style)) delete marks.italic;
  }
  const deco = decl.get("text-decoration-line") || decl.get("text-decoration");
  if (deco) {
    if (/\bnone\b/.test(deco)) { delete marks.underline; delete marks.strike; }
    if (/\bunderline\b/.test(deco)) marks.underline = true;
    if (/\bline-through\b/.test(deco)) marks.strike = true;
  }
}

const tagOf = (el) => (el.nodeName || el.tagName || "").toLowerCase();
const kids = (el) => Array.from(el.childNodes || []);
const SKIP = new Set(["style", "script", "template", "head", "title", "meta", "link", "colgroup", "caption"]);
const BREAKS = new Set(["br", "p", "div", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "table", "tr", "td", "th"]);

// marksAt is the formats of an element's text: those it inherits, those
// its tag gives and those its classes and inline style state, the inline
// style last.
function marksAt(el, inherited, rules, safe) {
  const marks = { ...inherited };
  const tag = tagOf(el);
  if (tag === "b" || tag === "strong") marks.bold = true;
  if (tag === "i" || tag === "em" || tag === "cite") marks.italic = true;
  if (tag === "u" || tag === "ins") marks.underline = true;
  if (tag === "s" || tag === "strike" || tag === "del") marks.strike = true;
  if (tag === "mark") marks.highlight = true;
  if (["code", "kbd", "samp", "tt"].includes(tag)) marks.code = true;
  if (tag === "a") {
    const href = el.getAttribute?.("href");
    if (href && safe(href)) marks.link = href;
  }
  for (const cls of (el.getAttribute?.("class") || "").split(/\s+/)) if (rules.has(cls)) styled(marks, rules.get(cls));
  styled(marks, declarations(el.getAttribute?.("style")));
  return marks;
}

// cellFrom reads a cell's text with its formats; white space is collapsed
// as a browser shows it, and line breaks and paragraphs become spaces.
function cellFrom(td, rules, safe) {
  const chars = []; // { ch, marks }
  const walk = (el, marks) => {
    for (const n of kids(el)) {
      if (n.nodeType === 3) {
        for (const ch of (n.nodeValue ?? n.textContent ?? "").replace(/[\s ]+/g, " ")) chars.push({ ch, marks });
      } else if (n.nodeType === 1) {
        const tag = tagOf(n);
        if (SKIP.has(tag)) continue;
        const brk = BREAKS.has(tag);
        if (brk) chars.push({ ch: " ", marks: {} });
        walk(n, marksAt(n, marks, rules, safe));
        if (brk) chars.push({ ch: " ", marks: {} });
      }
    }
  };
  walk(td, marksAt(td, {}, rules, safe));
  const kept = [];
  for (const c of chars) {
    if (c.ch === " " && (kept.length === 0 || kept[kept.length - 1].ch === " ")) continue;
    kept.push(c);
  }
  while (kept.length && kept[kept.length - 1].ch === " ") kept.pop();
  const marks = [];
  const open = new Map(); // key -> mark
  kept.forEach((c, i) => {
    for (const [type, v] of Object.entries(c.marks)) {
      const key = type === "link" ? "link " + v : type;
      const m = open.get(key);
      if (m && m.end === i) m.end = i + 1;
      else { const next = { type, start: i, end: i + 1, ...(type === "link" ? { href: v } : {}) }; open.set(key, next); marks.push(next); }
    }
  });
  return { text: kept.map((c) => c.ch).join(""), marks: normalMarks(marks.filter((m) => MARKS.includes(m.type))) };
}

// rowsOf lists a table's rows: those of its head, bodies and foot and
// those directly in it, never those of a table inside a cell.
function rowsOf(table) {
  const out = [];
  for (const n of kids(table)) {
    const tag = tagOf(n);
    if (tag === "tr") out.push(n);
    else if (tag === "thead" || tag === "tbody" || tag === "tfoot") out.push(...kids(n).filter((r) => tagOf(r) === "tr"));
  }
  return out;
}

// span is a cell's colspan or rowspan, at least 1 and at most max.
const span = (el, name, max) => Math.max(1, Math.min(max, parseInt(el.getAttribute?.(name) || "1", 10) || 1));

// MAX_COLUMNS bounds the columns a pasted table may make, so that a
// wrong span cannot make a table no one can use.
const MAX_COLUMNS = 100;

// fromData reads the block a table copied from the editor carries; it is
// checked as anything pasted is: formats a cell may carry, inside one
// cell, links the model allows, a layout the model accepts.
function fromData(json, safe) {
  let d;
  try { d = JSON.parse(json); } catch { return null; }
  if (!d || typeof d.text !== "string") return null;
  const marks = (Array.isArray(d.marks) ? d.marks : []).filter((m) => m && MARKS.includes(m.type) && Number.isInteger(m.start) && Number.isInteger(m.end)
    && (m.type !== "link" || (typeof m.href === "string" && safe(m.href))))
    .map((m) => ({ type: m.type, start: m.start, end: m.end, ...(m.type === "link" ? { href: m.href } : {}) }));
  const rows = cellsOf(d.text, marks);
  const cells = (Array.isArray(d.cells) ? d.cells : []).filter((c) => c && Number.isInteger(c.row) && Number.isInteger(c.col));
  return { rows, layout: { widths: d.widths, heights: d.heights, cells } };
}

// colWidth is a column's width in CSS pixels as its width attribute or
// its inline style states it, or 0.
function colWidth(col) {
  const w = (col.getAttribute?.("width") || declarations(col.getAttribute?.("style")).get("width") || "").trim();
  return /^\d+(\.\d+)?(px)?$/.test(w) ? Number(w.replace("px", "")) : 0;
}

// fromCells reads a table's cells into a grid, a spanned place an empty
// cell, and its columns' widths from its column groups.
function fromCells(table, safe, rules) {
  const trs = rowsOf(table);
  const grid = trs.map(() => []);
  trs.forEach((tr, r) => {
    let c = 0;
    for (const td of kids(tr).filter((n) => ["td", "th"].includes(tagOf(n)))) {
      while (grid[r][c]) c++;
      if (c >= MAX_COLUMNS) break;
      const cs = span(td, "colspan", MAX_COLUMNS - c), rs = span(td, "rowspan", trs.length - r);
      const cell = cellFrom(td, rules, safe);
      for (let i = 0; i < rs; i++) for (let j = 0; j < cs; j++) grid[r + i][c + j] = i === 0 && j === 0 ? cell : { text: "", marks: [] };
      c += cs;
    }
  });
  const widths = [];
  for (const group of kids(table).filter((n) => tagOf(n) === "colgroup")) {
    const cols = kids(group).filter((n) => tagOf(n) === "col");
    for (const col of cols.length ? cols : [group]) for (let i = 0; i < span(col, "span", MAX_COLUMNS); i++) widths.push(colWidth(col));
  }
  return { rows: grid.filter((row) => row.length > 0), layout: { widths: widths.length ? widths : null } };
}

// padded is a grid with every row as long as the longest, at most
// MAX_COLUMNS, its cells without tabs and line breaks.
function padded(rows) {
  const nCols = Math.min(MAX_COLUMNS, Math.max(0, ...rows.map((r) => r.length)));
  return rows.map((row) => Array.from({ length: nCols }, (_, c) => (row[c] ? cleanCell(row[c]) : { text: "", marks: [] })));
}

// shown is a grid's text as a browser shows it, white space collapsed.
const shown = (rows) => JSON.stringify(rows.map((row) => row.map((c) => c.text.replace(/[\s\u00a0]+/g, " ").trim())));

// tableFromDOM reads a pasted HTML table into the attributes of a table
// block { text, marks, widths, heights, cells }, or null when it has no
// cells. A table copied from the editor brings its block along, which is
// taken when its cells show the same text: a copy of a part of a table,
// or one another program changed, is read from its cells. rules are the
// class rules of the pasted stylesheet (classRules), safe says whether a
// link's target may be kept (ADR-0014).
export function tableFromDOM(table, safe, rules = new Map()) {
  const read = fromCells(table, safe, rules);
  let rows = padded(read.rows), layout = read.layout;
  const data = table.getAttribute?.("data-pm-table");
  const copied = data ? fromData(data, safe) : null;
  if (copied) {
    const whole = padded(copied.rows);
    if (shown(whole) === shown(rows)) ({ rows, layout } = { rows: whole, layout: copied.layout });
  }
  if (rows.length === 0 || rows[0].length === 0) return null;
  const { text, marks } = joinCells(rows);
  const l = normalLayout(layout, rows.length, rows[0].length);
  return { text, marks, widths: l.widths, heights: l.heights, cells: l.cells };
}
