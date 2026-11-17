// Unit tests of the parts of a table block (tableparts.js, ADR-0214): its
// cells with their formats split from and joined into the block's text
// and marks, rows and columns added and removed with their sizes and
// colours, colours and formats set on a range of cells, and the sizes a
// drag gives. They run in Node.js without a browser and without
// dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// Editing a table in the browser is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  TONES, cellsOf, joinCells, cleanCell, normalMarks, normalLayout, colourOf, rangeOf, inRange, paint, formatCells,
  insertRow, insertColumn, removeRow, removeColumn, resized, tableDOM, tableFromDOM, classRules, declarations,
  subTable, tableAttrs, pasteCells, clearCells, gridFromText, manyCells, specHTML, styleCells, styleOf, SIZES,
} from "../static/assets/tableparts.js";

test("the tones are the model's", () => {
  assert.deepEqual(TONES, ["blue", "green", "yellow", "orange", "red", "purple", "teal", "gray"]);
});

test("a table's marks are split into its cells and joined back, counted in code points", () => {
  const text = "Häst\tRolle\n🙂db1\tprimary";
  const marks = [{ type: "bold", start: 0, end: 4 }, { type: "link", start: 12, end: 15, href: "https://db1.example.org" }, { type: "italic", start: 16, end: 23 }];
  const rows = cellsOf(text, marks);
  assert.deepEqual(rows[0][0], { text: "Häst", marks: [{ type: "bold", start: 0, end: 4 }] });
  assert.deepEqual(rows[1][0], { text: "🙂db1", marks: [{ type: "link", start: 1, end: 4, href: "https://db1.example.org" }] });
  assert.deepEqual(rows[1][1], { text: "primary", marks: [{ type: "italic", start: 0, end: 7 }] });
  assert.deepEqual(joinCells(rows), { text, marks });
});

test("a mark a cell may not carry, or one across two cells, is dropped", () => {
  const rows = cellsOf("ab\tcd", [{ type: "mention", start: 0, end: 2, ref: "x" }, { type: "bold", start: 1, end: 4 }]);
  assert.deepEqual(rows[0].map((c) => c.marks), [[], []]);
});

test("a cell's tabs and line breaks become spaces, its marks kept within it", () => {
  assert.deepEqual(cleanCell({ text: "a\tb\nc", marks: [{ type: "bold", start: 2, end: 9 }] }), { text: "a b c", marks: [{ type: "bold", start: 2, end: 5 }] });
  assert.deepEqual(joinCells([[{ text: "x\ty", marks: [] }]]).text, "x y");
});

test("marks of one kind and target that touch are joined", () => {
  assert.deepEqual(normalMarks([{ type: "bold", start: 3, end: 5 }, { type: "bold", start: 0, end: 3 }, { type: "link", start: 0, end: 2, href: "https://a.org" },
    { type: "link", start: 2, end: 4, href: "https://b.org" }]),
  [{ type: "bold", start: 0, end: 5 }, { type: "link", start: 0, end: 2, href: "https://a.org" }, { type: "link", start: 2, end: 4, href: "https://b.org" }]);
});

test("a layout is held as the model accepts it", () => {
  assert.deepEqual(normalLayout({ widths: [0, 0], heights: [0, 0], cells: [] }, 2, 2), { widths: null, heights: null, cells: null });
  assert.deepEqual(normalLayout({ widths: [3, 5000], heights: [12.6, 0] }, 2, 2), { widths: [24, 2000], heights: [16, 0], cells: null });
  assert.deepEqual(normalLayout({ widths: [100] }, 1, 3).widths, [100, 0, 0], "a missing width is 0");
  assert.deepEqual(normalLayout({ cells: [{ row: 1, col: 0, fill: "red" }, { row: 0, col: 1, border: "blue" }, { row: 5, col: 0, fill: "red" },
    { row: 0, col: 0, fill: "pink" }, { row: 1, col: 0, fill: "teal" }] }, 2, 2).cells,
  [{ row: 0, col: 1, border: "blue" }, { row: 1, col: 0, fill: "teal" }], "in order, once each, inside the table, with a tone");
});

test("colours are painted on a range of cells and taken off", () => {
  const rg = rangeOf({ r: 1, c: 1 }, { r: 0, c: 0 });
  assert.deepEqual(rg, { r0: 0, c0: 0, r1: 1, c1: 1 });
  assert.ok(inRange(rg, 1, 0) && !inRange(rg, 2, 0) && !inRange(null, 0, 0));
  let l = paint({ widths: null, heights: null, cells: null }, 3, 2, rg, "fill", "yellow");
  assert.equal(l.cells.length, 4);
  l = paint(l, 3, 2, { r0: 1, c0: 1, r1: 1, c1: 1 }, "border", "red");
  assert.deepEqual(colourOf(l, 1, 1), { fill: "yellow", border: "red" });
  l = paint(l, 3, 2, rg, "fill", "");
  assert.deepEqual(l.cells, [{ row: 1, col: 1, border: "red" }]);
  assert.deepEqual(colourOf(l, 2, 0), { fill: "", border: "" });
});

test("a format is turned on in every chosen cell, and off where all have it", () => {
  const rows = cellsOf("a\tbc\nd\t", [{ type: "bold", start: 0, end: 1 }]);
  const on = formatCells(rows, { r0: 0, c0: 0, r1: 1, c1: 1 }, "bold");
  assert.deepEqual(on.map((r) => r.map((c) => c.marks)), [[[{ type: "bold", start: 0, end: 1 }], [{ type: "bold", start: 0, end: 2 }]], [[{ type: "bold", start: 0, end: 1 }], []]]);
  const off = formatCells(on, { r0: 0, c0: 0, r1: 1, c1: 1 }, "bold");
  assert.deepEqual(off.flat().map((c) => c.marks), [[], [], [], []]);
});

test("rows and columns are added and removed with their sizes and colours", () => {
  const rows = cellsOf("a\tb\nc\td");
  const layout = { widths: [100, 200], heights: [0, 40], cells: [{ row: 1, col: 1, fill: "red" }] };
  let x = insertRow(rows, layout, 0);
  assert.deepEqual(joinCells(x.rows).text, "a\tb\n\t\nc\td");
  assert.deepEqual(x.layout.heights, [0, 0, 40]);
  assert.deepEqual(x.layout.cells, [{ row: 2, col: 1, fill: "red" }]);
  x = insertColumn(rows, layout, 0);
  assert.deepEqual(joinCells(x.rows).text, "a\t\tb\nc\t\td");
  assert.deepEqual(x.layout.widths, [100, 100, 200]);
  assert.deepEqual(x.layout.cells, [{ row: 1, col: 2, fill: "red" }]);
  x = removeRow(rows, layout, 1);
  assert.deepEqual(joinCells(x.rows).text, "a\tb");
  assert.deepEqual(x.layout, { widths: [100, 200], heights: null, cells: null });
  x = removeColumn(rows, layout, 0);
  assert.deepEqual(joinCells(x.rows).text, "b\nd");
  assert.deepEqual(x.layout, { widths: [200], heights: [0, 40], cells: [{ row: 1, col: 0, fill: "red" }] });
  assert.equal(removeRow(cellsOf("a"), {}, 0).rows.length, 1, "the last row stays");
  assert.equal(removeColumn(cellsOf("a"), {}, 0).rows[0].length, 1, "the last column stays");
});

test("a dragged column fixes every column at its shown width; a dragged row only itself", () => {
  const l = { widths: null, heights: null, cells: null };
  assert.deepEqual(resized(l, "widths", 1, 260.4, [120.2, 180, 90]).widths, [120, 260, 90]);
  assert.deepEqual(resized({ ...l, widths: [100, 0] }, "widths", 0, 10, [100, 300]).widths, [24, 0]);
  assert.deepEqual(resized(l, "heights", 2, 55, [30, 30, 30]).heights, [0, 0, 55]);
  assert.deepEqual(resized({ ...l, heights: [40, 0] }, "heights", 1, 5000, [40, 30]).heights, [40, 1000]);
});

// A small stand-in for the DOM a paste is read from: elements with
// attributes and children, and text nodes.
const el = (tag, attrs = {}, ...children) => ({
  nodeType: 1, nodeName: tag.toUpperCase(),
  childNodes: children.map((c) => (typeof c === "string" ? { nodeType: 3, nodeValue: c } : c)),
  getAttribute: (name) => (name in attrs ? attrs[name] : null),
});
// fromSpec builds that DOM from a DOM output spec, as the editor's
// serializer builds the HTML a copy carries.
const fromSpec = (spec) => {
  if (typeof spec === "string") return spec;
  const [tag, maybe, ...rest] = spec;
  const attrs = maybe && typeof maybe === "object" && !Array.isArray(maybe) ? maybe : {};
  const children = attrs === maybe ? rest : [maybe, ...rest].filter((x) => x !== undefined);
  return el(tag, attrs, ...children.map(fromSpec));
};
const safe = (href) => /^https?:/.test(href);
const formatted = {
  text: "Host\tRole\ndb1  primary\treplica of 🙂db2",
  marks: [{ type: "bold", start: 0, end: 4 }, { type: "link", start: 10, end: 13, href: "https://db1.example.org" },
    { type: "italic", start: 23, end: 38 }],
  widths: [120, 0], heights: null, cells: [{ row: 1, col: 1, fill: "yellow", border: "red" }],
};

test("a table copied as HTML is read back with its formats, sizes and colours", () => {
  const spec = tableDOM(formatted, safe);
  assert.equal(spec[0], "table");
  assert.deepEqual(tableFromDOM(fromSpec(spec), safe), formatted);
});

test("a copied table is an HTML table other programs read: a header row, formats as elements, column widths", () => {
  const spec = tableDOM(formatted, safe);
  assert.deepEqual(spec[2], ["colgroup", ["col", { width: "120" }], ["col", {}]]);
  assert.deepEqual(spec[3], ["thead", ["tr", ["th", {}, ["strong", "Host"]], ["th", {}, "Role"]]]);
  assert.deepEqual(spec[4][1][1], ["td", {}, ["a", { href: "https://db1.example.org" }, "db1"], "  primary"]);
  const { ["data-pm-table"]: _, ...rest } = spec[1];
  assert.deepEqual(rest, { class: "pm-table" });
  // Read without the block it carries, the cells give its text and formats;
  // white space collapses as a browser shows it.
  const plain = fromSpec(spec);
  const bare = { ...plain, getAttribute: (name) => (name === "data-pm-table" ? null : plain.getAttribute(name)) };
  const read = tableFromDOM(bare, safe);
  assert.equal(read.text, "Host\tRole\ndb1 primary\treplica of 🙂db2");
  assert.deepEqual(read.widths, [120, 0]);
  assert.equal(read.cells, null, "colours travel only with the block");
});

test("a link the model refuses is copied and pasted as text", () => {
  const spec = tableDOM({ text: "a\nx", marks: [{ type: "link", start: 2, end: 3, href: "javascript:alert(1)" }] }, safe);
  assert.deepEqual(spec[3], ["tbody", ["tr", ["td", {}, "x"]]]);
  const read = tableFromDOM(fromSpec(spec), safe);
  assert.deepEqual(read.marks, [], "the block it carries is checked too");
  const pasted = tableFromDOM(el("table", {}, el("tr", {}, el("td", {}, el("a", { href: "javascript:x" }, "y")))), safe);
  assert.deepEqual(pasted.marks, []);
});

test("a table from a word processor keeps its formats; paragraphs and line breaks in a cell become spaces", () => {
  const table = el("table", {}, el("tbody", {},
    el("tr", {}, el("td", {}, el("p", {}, el("b", {}, "Name"))), el("td", {}, el("p", {}, el("b", {}, "Notes")))),
    el("tr", {}, el("td", {}, el("p", {}, "Zoë")), el("td", {}, el("p", {}, "first ", el("i", {}, "line")), el("p", {}, "second", el("br"), el("u", {}, "third"))))));
  assert.deepEqual(tableFromDOM(table, safe), {
    text: "Name\tNotes\nZoë\tfirst line second third",
    marks: [{ type: "bold", start: 0, end: 4 }, { type: "bold", start: 5, end: 10 }, { type: "italic", start: 21, end: 25 }, { type: "underline", start: 33, end: 38 }],
    widths: null, heights: null, cells: null,
  });
});

test("a spreadsheet's formats come from inline styles and the classes of its stylesheet", () => {
  const rules = classRules(`<!--table {mso-displayed-decimal-separator:"\\.";}
    .xl65 {mso-style-parent:style0; font-weight:700;}
    .xl66, td.xl67 {font-style:italic; text-decoration:underline;}
    .font5 {font-weight:400;} -->`);
  assert.deepEqual([...rules.keys()], ["xl65", "xl66", "xl67", "font5"]);
  const table = el("table", {},
    el("colgroup", {}, el("col", { width: "87", span: "2" }), el("col", { width: "40%" })),
    el("tr", {}, el("td", { class: "xl65" }, "Region"), el("td", { class: "xl65" }, "Q1 ", el("font", { class: "font5" }, "(CHF)")), el("td", { style: "font-weight: bold" }, "Q2")),
    el("tr", {}, el("td", { class: "xl66" }, "North"), el("td", { style: "text-decoration: line-through" }, "1\u00a0200"),
      el("td", { style: "font-weight:normal" }, el("b", { style: "font-weight:normal" }, "1 300"))));
  assert.deepEqual(tableFromDOM(table, safe, rules), {
    text: "Region\tQ1 (CHF)\tQ2\nNorth\t1 200\t1 300",
    marks: [{ type: "bold", start: 0, end: 6 }, { type: "bold", start: 7, end: 10 }, { type: "bold", start: 16, end: 18 },
      { type: "italic", start: 19, end: 24 }, { type: "underline", start: 19, end: 24 }, { type: "strike", start: 25, end: 30 }],
    widths: [87, 87, 0], heights: null, cells: null,
  });
});

test("merged cells keep the grid: a spanned place is an empty cell, and short rows are filled up", () => {
  const table = el("table", {},
    el("thead", {}, el("tr", {}, el("th", { colspan: "2" }, "Both"), el("th", {}, "C"))),
    el("tbody", {}, el("tr", {}, el("td", { rowspan: "9" }, "Tall"), el("td", {}, "b"), el("td", {}, "c")),
      el("tr", {}, el("td", {}, "x")), el("tr", {})));
  // The third row is held by the tall cell alone.
  assert.equal(tableFromDOM(table, safe).text, "Both\t\tC\nTall\tb\tc\n\tx\t\n\t\t");
});

test("a table inside a cell is read as its text, and one without cells is no table", () => {
  const inner = el("table", {}, el("tr", {}, el("td", {}, "in"), el("td", {}, "side")));
  assert.equal(tableFromDOM(el("table", {}, el("tr", {}, el("td", {}, "out"), el("td", {}, inner))), safe).text, "out\tin side");
  assert.equal(tableFromDOM(el("table", {}, el("caption", {}, "Empty")), safe), null);
  assert.equal(tableFromDOM(el("table", { "data-pm-table": "not json" }, el("tr", {}, el("td", {}, "kept"))), safe).text, "kept", "an unreadable block falls back to the cells");
});

test("a part of a copied table is pasted as that part, read from its cells", () => {
  const whole = fromSpec(tableDOM(formatted, safe));
  // The browser copies the selected rows of a page being read, with the
  // table's attributes and inline styles of its own.
  const part = el("table", { "data-pm-table": whole.getAttribute("data-pm-table") },
    el("tbody", {}, el("tr", {}, el("th", { style: "font-weight: 600" }, "Host"), el("th", { style: "font-weight: 600" }, "Role"))),
    el("colgroup", {}, el("col", { style: "width: 120px;" })));
  assert.deepEqual(tableFromDOM(part, safe), { text: "Host\tRole", marks: [{ type: "bold", start: 0, end: 4 }, { type: "bold", start: 5, end: 9 }],
    widths: [120, 0], heights: null, cells: null });
  // The table the editor draws, copied whole: the block it carries is taken.
  const drawn = el("table", { "data-pm-table": whole.getAttribute("data-pm-table") }, el("colgroup", {}, el("col", { style: "width: 120px;" }), el("col")),
    el("tbody", {}, el("tr", {}, el("th", {}, el("div", {}, el("strong", {}, "Host"))), el("th", {}, el("div", {}, "Role"))),
      el("tr", {}, el("td", {}, el("div", {}, "db1  primary"), el("span", { class: "pm-col-handle" })), el("td", { class: "pm-fill-yellow" }, el("div", {}, "replica of 🙂db2")))));
  assert.deepEqual(tableFromDOM(drawn, safe), formatted);
});

test("a copied block is checked as anything pasted: sizes and colours the model accepts, formats inside one cell", () => {
  const data = JSON.stringify({ text: "a\tb\nc", marks: [{ type: "bold", start: 0, end: 3 }, { type: "mention", start: 0, end: 1, ref: "x" }, { type: "code", start: 2, end: 3, ref: "x" }],
    widths: [5, 9000], heights: [0, 0], cells: [{ row: 0, col: 0, fill: "pink" }, { row: 1, col: 0, fill: "red" }, { row: 7, col: 0, fill: "red" }, { row: "0", col: 1, fill: "red" }] });
  const cells = el("tr", {}, el("td", {}, "a"), el("td", {}, "b"));
  assert.deepEqual(tableFromDOM(el("table", { "data-pm-table": data }, cells, el("tr", {}, el("td", {}, "c"))), safe), {
    text: "a\tb\nc\t", marks: [{ type: "code", start: 2, end: 3 }],
    widths: [24, 2000], heights: null, cells: [{ row: 1, col: 0, fill: "red" }],
  });
});

test("declarations are read case-insensitively", () => {
  assert.deepEqual([...declarations("Font-Weight: BOLD; color:red;;x")], [["font-weight", "bold"], ["color", "red"]]);
});

const cell = (text, marks = []) => ({ text, marks });

test("chosen cells are a table of their own, with their sizes and colours", () => {
  const rows = cellsOf("a\tb\tc\nd\te\tf\ng\th\ti", [{ type: "bold", start: 8, end: 9 }]);
  const layout = { widths: [10, 40, 50], heights: [0, 30, 0], cells: [{ row: 1, col: 2, fill: "red" }, { row: 0, col: 0, border: "blue" }] };
  const sub = subTable(rows, layout, { r0: 1, c0: 1, r1: 2, c1: 2 });
  assert.deepEqual(sub.rows, [[cell("e", [{ type: "bold", start: 0, end: 1 }]), cell("f")], [cell("h"), cell("i")]]);
  assert.deepEqual(sub.layout, { widths: [40, 50], heights: [30, 0], cells: [{ row: 0, col: 1, fill: "red" }] });
  assert.deepEqual(tableAttrs(sub.rows, sub.layout), { text: "e\tf\nh\ti", marks: [{ type: "bold", start: 0, end: 1 }],
    widths: [40, 50], heights: [30, 0], cells: [{ row: 0, col: 1, fill: "red" }] });
});

test("copied cells written as HTML are read back whole", () => {
  const attrs = { text: "x & y\t<b>", marks: [{ type: "italic", start: 0, end: 1 }], widths: null, heights: null, cells: [{ row: 0, col: 1, fill: "teal" }] };
  const html = specHTML(tableDOM(attrs, safe));
  assert.match(html, /^<table class="pm-table" data-pm-table="\{&quot;text&quot;/);
  assert.match(html, /<th><em>x<\/em> &amp; y<\/th><th>&lt;b&gt;<\/th>/);
  assert.equal(specHTML(["colgroup", ["col", { width: "5" }], ["col", {}]]), '<colgroup><col width="5"><col></colgroup>');
});

test("pasted cells overwrite from a cell on, and the table grows as they need", () => {
  const rows = cellsOf("h1\th2\na\tb");
  const layout = { widths: [100, 120], heights: [0, 20], cells: [{ row: 1, col: 1, fill: "yellow" }] };
  const grid = [[cell("X", [{ type: "bold", start: 0, end: 1 }]), cell("Y")], [cell("Z"), cell("")]];
  const out = pasteCells(rows, layout, { r: 1, c: 1 }, grid, { widths: [60, 70], cells: [{ row: 1, col: 0, border: "red" }] });
  assert.deepEqual(tableAttrs(out.rows, out.layout), {
    text: "h1\th2\t\na\tX\tY\n\tZ\t", marks: [{ type: "bold", start: 9, end: 10 }],
    widths: [100, 120, 70], heights: [0, 20, 0],
    cells: [{ row: 1, col: 1, fill: "yellow" }, { row: 2, col: 1, border: "red" }],
  }, "uncoloured pasted cells keep the table's colours; a new column takes the grid's width");
  const plain = pasteCells(cellsOf("a"), {}, { r: 0, c: 0 }, [[cell("1"), cell("2")]]);
  assert.deepEqual(tableAttrs(plain.rows, plain.layout), { text: "1\t2", marks: [], widths: null, heights: null, cells: null });
  const wide = pasteCells(cellsOf("a"), {}, { r: 0, c: 98 }, [[cell("1"), cell("2"), cell("3"), cell("4")]]);
  assert.equal(wide.rows[0].length, 100, "at most 100 columns");
});

test("chosen cells are emptied, their colours kept apart", () => {
  const rows = cellsOf("a\tb\nc\td", [{ type: "bold", start: 0, end: 1 }]);
  assert.deepEqual(clearCells(rows, { r0: 0, c0: 0, r1: 1, c1: 0 }).map((r) => r.map((c) => c.text)), [["", "b"], ["", "d"]]);
});

test("text from a spreadsheet is read into cells, quoted cells holding tabs, line breaks and quotes", () => {
  assert.deepEqual(gridFromText("a\tb\r\nc\td\r\n").map((r) => r.map((c) => c.text)), [["a", "b"], ["c", "d"]]);
  assert.deepEqual(gridFromText('"x\ty"\t"line\nbreak"\t"say ""hi"""\nplain "quote"').map((r) => r.map((c) => c.text)),
    [["x y", "line break", 'say "hi"'], ['plain "quote"']]);
  assert.ok(manyCells(gridFromText("a\tb")) && manyCells(gridFromText("a\nb")) && !manyCells(gridFromText("just text")));
});

test("a cell's text takes a tone and a size, kept apart from other values", () => {
  assert.deepEqual(SIZES, ["small", "large", "larger"]);
  const rows = cellsOf("ab\tc\nde\t", [{ type: "color", start: 0, end: 1, value: "red" }]);
  const styled = styleCells(rows, { r0: 0, c0: 0, r1: 1, c1: 1 }, "color", "teal");
  assert.deepEqual(styled[0][0].marks, [{ type: "color", start: 0, end: 2, value: "teal" }], "the colour before is replaced");
  assert.deepEqual(styled[1][1].marks, [], "an empty cell takes none");
  assert.equal(styleOf(styled[1][0], "color"), "teal");
  assert.deepEqual(styleCells(styled, { r0: 0, c0: 0, r1: 0, c1: 0 }, "color", "")[0][0].marks, [], "an empty value takes it off");
  assert.deepEqual(normalMarks([{ type: "size", start: 0, end: 2, value: "large" }, { type: "size", start: 2, end: 4, value: "small" }, { type: "size", start: 4, end: 5, value: "small" }]),
    [{ type: "size", start: 0, end: 2, value: "large" }, { type: "size", start: 2, end: 5, value: "small" }], "only marks of one value are joined");
});

test("a colour and a size are copied and pasted, and checked as everything pasted", () => {
  const attrs = { text: "Host\tRole", marks: [{ type: "color", start: 0, end: 4, value: "purple" }, { type: "size", start: 5, end: 9, value: "larger" }],
    widths: null, heights: null, cells: null };
  const spec = tableDOM(attrs, safe);
  assert.deepEqual(spec[2], ["thead", ["tr", ["th", {}, ["span", { class: "pm-text-purple" }, "Host"]], ["th", {}, ["span", { class: "pm-size-larger" }, "Role"]]]]);
  assert.deepEqual(tableFromDOM(fromSpec(spec), safe), attrs);
  const plain = fromSpec(spec);
  const bare = { ...plain, getAttribute: (name) => (name === "data-pm-table" ? null : plain.getAttribute(name)) };
  assert.deepEqual(tableFromDOM(bare, safe).marks, attrs.marks, "read from the cells, by the editor's classes");
  const data = JSON.stringify({ text: "a\tb", marks: [{ type: "color", start: 0, end: 1, value: "#f00" }, { type: "size", start: 2, end: 3, value: "huge" },
    { type: "color", start: 2, end: 3, value: "red" }, { type: "bold", start: 0, end: 1, value: "red" }] });
  assert.deepEqual(tableFromDOM(el("table", { "data-pm-table": data }, el("tr", {}, el("td", {}, "a"), el("td", {}, "b"))), safe).marks,
    [{ type: "color", start: 2, end: 3, value: "red" }], "values the model refuses are dropped");
  assert.deepEqual(tableFromDOM(el("table", {}, el("tr", {}, el("td", {}, el("span", { class: "pm-text-pink pm-size-huge" }, "x")))), safe).marks, []);
});
