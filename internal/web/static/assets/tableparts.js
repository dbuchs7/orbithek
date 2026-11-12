// The parts of a table block that hold no DOM (ADR-0042, ADR-0200): its
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

// A layout is a table's { columns, rows, cells } as the model holds them:
// columns and rows a size per column and row or null, cells the coloured
// ones { row, col, fill, border } in the order of rows and columns, or
// null.

// normalLayout is a layout as the model accepts it for a table of nRows
// and nCols: sizes within their bounds or 0, none when every one is 0,
// cells inside the table, each once, in order, each with a tone.
export function normalLayout({ columns, rows, cells } = {}, nRows, nCols) {
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
  return { columns: sizes(columns, nCols, COLUMN), rows: sizes(rows, nRows, ROW), cells: list.length ? list : null };
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
  const sizes = layout.rows ? [...layout.rows.slice(0, at + 1), 0, ...layout.rows.slice(at + 1)] : null;
  const cells = (layout.cells || []).map((c) => (c.row > at ? { ...c, row: c.row + 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, rows: sizes, cells }, next.length, nCols) };
}

// insertColumn adds an empty column after column at; a new column of a
// table with widths takes the width of the column at.
export function insertColumn(rows, layout, at) {
  const next = rows.map((row) => [...row.slice(0, at + 1), empty(), ...row.slice(at + 1)]);
  const columns = layout.columns ? [...layout.columns.slice(0, at + 1), layout.columns[at] || 0, ...layout.columns.slice(at + 1)] : null;
  const cells = (layout.cells || []).map((c) => (c.col > at ? { ...c, col: c.col + 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, columns, cells }, next.length, next[0].length) };
}

// removeRow takes row at away with its size and colours; the last row
// stays.
export function removeRow(rows, layout, at) {
  if (rows.length < 2) return { rows, layout };
  const next = rows.filter((_, i) => i !== at);
  const sizes = layout.rows ? layout.rows.filter((_, i) => i !== at) : null;
  const cells = (layout.cells || []).filter((c) => c.row !== at).map((c) => (c.row > at ? { ...c, row: c.row - 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, rows: sizes, cells }, next.length, next[0].length) };
}

// removeColumn takes column at away with its width and colours; the last
// column stays.
export function removeColumn(rows, layout, at) {
  if (rows[0].length < 2) return { rows, layout };
  const next = rows.map((row) => row.filter((_, i) => i !== at));
  const columns = layout.columns ? layout.columns.filter((_, i) => i !== at) : null;
  const cells = (layout.cells || []).filter((c) => c.col !== at).map((c) => (c.col > at ? { ...c, col: c.col - 1 } : c));
  return { rows: next, layout: normalLayout({ ...layout, columns, cells }, next.length, next[0].length) };
}

// resized is the layout after column (what "columns") or row (what "rows")
// i was dragged to size px. A column dragged in a table without widths
// fixes every column at the width it is shown with, measured, so that the
// others keep theirs; a row keeps the others as they are.
export function resized(layout, what, i, px, measured) {
  const bounds = what === "columns" ? COLUMN : ROW;
  const base = what === "columns" && !layout.columns ? measured.map((w) => Math.round(w)) : [...(layout[what] || measured.map(() => 0))];
  base[i] = Math.min(bounds.max, Math.max(bounds.min, Math.round(px)));
  return { ...layout, [what]: base };
}

// filledClass and borderClass are the classes that colour a cell; the
// stylesheet mixes a fill's tone into the theme's surface, so the theme's
// text stays readable on it, light or dark.
export const filledClass = (fill) => (fill ? "pm-cell-fill pm-fill-" + fill : "");
export const borderClass = (border) => (border ? "pm-cell-border pm-border-" + border : "");
