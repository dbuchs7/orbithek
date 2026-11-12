// Unit tests of the parts of a table block (tableparts.js, ADR-0200): its
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
  insertRow, insertColumn, removeRow, removeColumn, resized,
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
  assert.deepEqual(normalLayout({ columns: [0, 0], rows: [0, 0], cells: [] }, 2, 2), { columns: null, rows: null, cells: null });
  assert.deepEqual(normalLayout({ columns: [3, 5000], rows: [12.6, 0] }, 2, 2), { columns: [24, 2000], rows: [16, 0], cells: null });
  assert.deepEqual(normalLayout({ columns: [100] }, 1, 3).columns, [100, 0, 0], "a missing width is 0");
  assert.deepEqual(normalLayout({ cells: [{ row: 1, col: 0, fill: "red" }, { row: 0, col: 1, border: "blue" }, { row: 5, col: 0, fill: "red" },
    { row: 0, col: 0, fill: "pink" }, { row: 1, col: 0, fill: "teal" }] }, 2, 2).cells,
  [{ row: 0, col: 1, border: "blue" }, { row: 1, col: 0, fill: "teal" }], "in order, once each, inside the table, with a tone");
});

test("colours are painted on a range of cells and taken off", () => {
  const rg = rangeOf({ r: 1, c: 1 }, { r: 0, c: 0 });
  assert.deepEqual(rg, { r0: 0, c0: 0, r1: 1, c1: 1 });
  assert.ok(inRange(rg, 1, 0) && !inRange(rg, 2, 0) && !inRange(null, 0, 0));
  let l = paint({ columns: null, rows: null, cells: null }, 3, 2, rg, "fill", "yellow");
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
  const layout = { columns: [100, 200], rows: [0, 40], cells: [{ row: 1, col: 1, fill: "red" }] };
  let x = insertRow(rows, layout, 0);
  assert.deepEqual(joinCells(x.rows).text, "a\tb\n\t\nc\td");
  assert.deepEqual(x.layout.rows, [0, 0, 40]);
  assert.deepEqual(x.layout.cells, [{ row: 2, col: 1, fill: "red" }]);
  x = insertColumn(rows, layout, 0);
  assert.deepEqual(joinCells(x.rows).text, "a\t\tb\nc\t\td");
  assert.deepEqual(x.layout.columns, [100, 100, 200]);
  assert.deepEqual(x.layout.cells, [{ row: 1, col: 2, fill: "red" }]);
  x = removeRow(rows, layout, 1);
  assert.deepEqual(joinCells(x.rows).text, "a\tb");
  assert.deepEqual(x.layout, { columns: [100, 200], rows: null, cells: null });
  x = removeColumn(rows, layout, 0);
  assert.deepEqual(joinCells(x.rows).text, "b\nd");
  assert.deepEqual(x.layout, { columns: [200], rows: [0, 40], cells: [{ row: 1, col: 0, fill: "red" }] });
  assert.equal(removeRow(cellsOf("a"), {}, 0).rows.length, 1, "the last row stays");
  assert.equal(removeColumn(cellsOf("a"), {}, 0).rows[0].length, 1, "the last column stays");
});

test("a dragged column fixes every column at its shown width; a dragged row only itself", () => {
  const l = { columns: null, rows: null, cells: null };
  assert.deepEqual(resized(l, "columns", 1, 260.4, [120.2, 180, 90]).columns, [120, 260, 90]);
  assert.deepEqual(resized({ ...l, columns: [100, 0] }, "columns", 0, 10, [100, 300]).columns, [24, 0]);
  assert.deepEqual(resized(l, "rows", 2, 55, [30, 30, 30]).rows, [0, 0, 55]);
  assert.deepEqual(resized({ ...l, rows: [40, 0] }, "rows", 1, 5000, [40, 30]).rows, [40, 1000]);
});
