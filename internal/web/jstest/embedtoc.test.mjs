// Tests of an embed of its item's text alone and of the depth of a table
// of contents in the editor (editor.js): both survive the way from the
// model's blocks to the editor's document and back, and a table of
// contents lists the headings down to its level
// (ADR-draft-embedded-text-alone-and-toc-depth). They run in Node.js
// without a browser.
import { test } from "node:test";
import assert from "node:assert/strict";
import { docFromBlocks, blocksFromDoc, tocHeadings, tocDepths, embedViews } from "../static/assets/editor.js";

test("an embed keeps its view and a table of contents its level", () => {
  const blocks = [{ kind: "toc", level: 2 }, { kind: "toc" }, { kind: "embed", ref: "a", view: "text" }, { kind: "embed", ref: "b" }];
  assert.deepEqual(blocksFromDoc(docFromBlocks(blocks)), blocks);
});

test("a table of contents lists the headings down to its level, or every one at level 0", () => {
  const doc = docFromBlocks([{ kind: "heading", level: 1, text: "Use" }, { kind: "heading", level: 2, text: "Steps" },
    { kind: "heading", level: 3, text: "Details" }, { kind: "heading", level: 1, text: "  " }, { kind: "paragraph", text: "x" }]);
  assert.deepEqual(tocHeadings(doc, 1).map((x) => x.text), ["Use"]);
  assert.deepEqual(tocHeadings(doc, 2).map((x) => x.text), ["Use", "Steps"]);
  assert.deepEqual(tocHeadings(doc, 0).map((x) => x.text), ["Use", "Steps", "Details"]);
});

test("the choices offer every level and both views", () => {
  assert.deepEqual(tocDepths().map(([v]) => v), [1, 2, 0]);
  assert.deepEqual(embedViews().map(([v]) => v), ["", "text"]);
});
