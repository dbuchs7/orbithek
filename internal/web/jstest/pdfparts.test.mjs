// Unit tests of the PDF reader's parts (pdfparts.js, ADR-0210): which
// files are PDFs, the reader's address, the rectangles of a selection, how
// a highlight's quote is found again on a page, the list of a document's
// highlights and its export as Markdown. They run in Node.js without a
// browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The reader itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { COLORS, isPdf, readerHref, readState, normRects, locate, squash, highlightsOf, onPdf, highlightsMarkdown, zoomIn, zoomOut, clampPage, ZOOMS, attachmentLink }
  from "../static/assets/pdfparts.js";

test("a PDF is told by its type or its name", () => {
  assert.equal(isPdf({ name: "report.pdf", type: "application/octet-stream" }), true);
  assert.equal(isPdf({ name: "Contract", type: "application/pdf" }), true);
  assert.equal(isPdf({ name: "A.PDF" }), true);
  assert.equal(isPdf({ name: "scan.pdf.png", type: "image/png" }), false);
  assert.equal(isPdf(null), false);
  assert.deepEqual(COLORS, ["yellow", "green", "blue", "pink", "purple"], "the server's colours, the first the default");
});

test("the reader's address names the item, the file, a page and a highlight", () => {
  assert.equal(readerHref("PLAT-1", "f1"), "#/pdf/PLAT-1/f1");
  assert.equal(readerHref("A B", "f1", { page: 3 }), "#/pdf/A%20B/f1?page=3");
  assert.equal(readerHref("PLAT-1", "f1", { page: 1, comment: "c9" }), "#/pdf/PLAT-1/f1?c=c9");
  assert.equal(readerHref("PLAT-1", "f1", { page: 0 }), "#/pdf/PLAT-1/f1");
  assert.deepEqual(readState(new URLSearchParams("page=4&c=c2")), { page: 4, comment: "c2" });
  assert.deepEqual(readState(new URLSearchParams("page=x")), { page: 1, comment: "" });
  assert.deepEqual(readState(new URLSearchParams("page=-3")), { page: 1, comment: "" });
});

test("a link in a text to a file of an item is told apart", () => {
  const origin = "https://wiki.example.org";
  assert.deepEqual(attachmentLink(origin + "/api/items/PLAT-1/attachments/0123456789abcdef0123456789abcdef", origin),
    { item: "PLAT-1", file: "0123456789abcdef0123456789abcdef" });
  assert.equal(attachmentLink("https://other.example.org/api/items/PLAT-1/attachments/0123456789abcdef0123456789abcdef", origin), null);
  assert.equal(attachmentLink(origin + "/api/items/PLAT-1", origin), null);
  assert.equal(attachmentLink("not a url", origin), null);
});

test("the rectangles of a selection become fractions of the page, one per line", () => {
  const box = { left: 100, top: 50, width: 400, height: 800 };
  const rects = [
    { left: 140, top: 130, width: 80, height: 16 }, // line 1, word 1
    { left: 222, top: 131, width: 60, height: 15 }, // line 1, word 2, touching
    { left: 140, top: 150, width: 200, height: 16 }, // line 2
    { left: 140, top: 150, width: 0, height: 16 }, // empty
    { left: 480, top: 170, width: 100, height: 16 }, // running off the page
    { left: 10, top: 10, width: 30, height: 10 }, // outside entirely
  ];
  const got = normRects(rects, box);
  assert.equal(got.length, 3);
  assert.deepEqual(got[0], { x: 0.1, y: 0.1, w: 0.355, h: 0.02 });
  assert.deepEqual(got[1], { x: 0.1, y: 0.125, w: 0.5, h: 0.02 });
  assert.equal(got[2].x + got[2].w <= 1, true, "clipped to the page");
  assert.deepEqual(normRects([], box), []);
  assert.deepEqual(normRects(rects, { left: 0, top: 0, width: 0, height: 0 }), [], "no page, no rectangles");
});

test("a selection over many lines keeps within the budget of rectangles", () => {
  const box = { left: 0, top: 0, width: 1000, height: 1000 };
  const rects = Array.from({ length: 500 }, (_, i) => ({ left: 10, top: i * 2, width: 100 + (i % 3), height: 2 }));
  assert.equal(normRects(rects, box, 200).length <= 200, true);
});

test("whitespace is left out to compare texts, and positions map back", () => {
  const { text, map } = squash("a b\n c");
  assert.equal(text, "abc");
  assert.deepEqual(map, [0, 2, 5]);
});

test("a quote is found again on a page, told apart by what precedes and follows it", () => {
  const raw = "The revenue grew by 4 %. Costs grew by 2 %. The revenue grew again.";
  assert.deepEqual(locate(raw, "revenue grew", "", ""), { start: 4, end: 16 });
  assert.deepEqual(locate(raw, "revenue grew", "The ", " again"), { start: 48, end: 60 });
  assert.deepEqual(locate(raw, "grew by", "Costs ", ""), { start: 31, end: 38 });
  // The text layer runs lines together; the selection breaks them.
  assert.deepEqual(locate("Thequick brownfox", "quick\nbrown fox", "", ""), { start: 3, end: 17 });
  assert.equal(locate(raw, "profit", "", ""), null);
  assert.equal(locate(raw, "   ", "", ""), null);
});

test("a document's highlights are the threads on its first version, by page and place", () => {
  const comments = [
    { id: "c1", anchor: { file: "f1", page: 2, quote: "b", rects: [{ x: 0, y: 0.5, w: 0.1, h: 0.1 }] } },
    { id: "c2", anchor: { file: "f1", page: 1, quote: "a" } },
    { id: "c3", reply: "c1", text: "yes" },
    { id: "c4", anchor: { file: "f2", page: 1, quote: "other PDF" } },
    { id: "c5", anchor: { quote: "a passage of the body" } },
    { id: "c6", anchor: { file: "f1", page: 2, quote: "c", rects: [{ x: 0, y: 0.1, w: 0.1, h: 0.1 }] } },
    { id: "c7", deleted: true },
    { id: "c8", reply: "c7", text: "lost" },
  ];
  const got = highlightsOf(comments, "f1");
  assert.deepEqual(got.map((x) => x.root.id), ["c2", "c6", "c1"]);
  assert.deepEqual(got[2].replies.map((r) => r.id), ["c3"]);
  assert.equal(onPdf({ anchor: { file: "f1" } }), true);
  assert.equal(onPdf({ anchor: { quote: "x" } }), false);
  assert.equal(onPdf({}), false);
});

test("the highlights export as Markdown: quote, page and note", () => {
  const highlights = highlightsOf([
    { id: "c1", author: "u1", text: "Check *this*", anchor: { file: "f1", page: 3, quote: "revenue\ngrew #1", color: "green" } },
    { id: "c2", author: "u2", reply: "c1", text: "Done." },
    { id: "c3", author: "u1", text: "", resolved: true, anchor: { file: "f1", page: 1, quote: "outlook" } },
  ], "f1");
  const md = highlightsMarkdown({
    title: "Highlights in report.pdf", source: "PLAT-1 Annual report", highlights,
    page: (n) => `Page ${n}`, person: (id) => ({ u1: "Anna", u2: "Bert" })[id], resolved: "resolved",
  });
  assert.equal(md, [
    "# Highlights in report.pdf", "", "PLAT-1 Annual report", "",
    "## Page 1 (resolved)", "", "> outlook", "",
    "## Page 3", "", "> revenue", "> grew \\#1", "", "Check \\*this\\* (Anna)", "", "- Bert: Done.", "",
  ].join("\n"));
});

test("zoom goes by steps and the page stays in the document", () => {
  assert.equal(zoomIn(1), 1.25);
  assert.equal(zoomOut(1), 0.75);
  assert.equal(zoomIn(ZOOMS.at(-1)), ZOOMS.at(-1));
  assert.equal(zoomOut(ZOOMS[0]), ZOOMS[0]);
  assert.equal(zoomIn(1.1), 1.25, "from a fitted width to the next step");
  assert.equal(zoomOut(1.1), 1);
  assert.equal(clampPage(0, 5), 1);
  assert.equal(clampPage(9, 5), 5);
  assert.equal(clampPage(NaN, 5), 1);
  assert.equal(clampPage(3, 5), 3);
});
