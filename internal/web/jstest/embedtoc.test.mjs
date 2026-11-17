// Tests of an embed of its item's text alone and of the depth of a table
// of contents in the editor (editor.js): both survive the way from the
// model's blocks to the editor's document and back, and a table of
// contents lists the headings down to its level
// (ADR-draft-embedded-text-alone-and-toc-depth). They run in Node.js
// without a browser.
import { test } from "node:test";
import assert from "node:assert/strict";
import { docFromBlocks, blocksFromDoc, tocHeadings, tocDepths, embedViews, wovenBlocks } from "../static/assets/editor.js";

// woven makes the woven blocks of a document without a browser: an embed
// of its text alone whose text is given in loaded stands for that text.
const fakeView = (doc, loaded = {}) => ({
  state: { doc },
  nodeDOM: (offset) => {
    const node = doc.nodeAt(offset);
    const inner = node?.type.name === "embed" ? loaded[node.attrs.ref] : null;
    return { scrollIntoView() {}, querySelector: () => (inner ? { embeddedView: fakeView(inner) } : null) };
  },
});

test("an embed keeps its view and a table of contents its level", () => {
  const blocks = [{ kind: "toc", level: 2 }, { kind: "toc" }, { kind: "embed", ref: "a", view: "text" }, { kind: "embed", ref: "b" }];
  assert.deepEqual(blocksFromDoc(docFromBlocks(blocks)), blocks);
});

test("a table of contents lists the headings down to its level, or every one at level 0", () => {
  const doc = docFromBlocks([{ kind: "heading", level: 1, text: "Use" }, { kind: "heading", level: 2, text: "Steps" },
    { kind: "heading", level: 3, text: "Details" }, { kind: "heading", level: 1, text: "  " }, { kind: "paragraph", text: "x" }]);
  const blocks = wovenBlocks(fakeView(doc));
  assert.deepEqual(tocHeadings(blocks, 1).map((x) => x.text), ["Use"]);
  assert.deepEqual(tocHeadings(blocks, 2).map((x) => x.text), ["Use", "Steps"]);
  assert.deepEqual(tocHeadings(blocks, 0).map((x) => x.text), ["Use", "Steps", "Details"]);
});

test("the headings of an embed of its text alone count as the text's own, those of a whole embed do not", () => {
  const doc = docFromBlocks([{ kind: "heading", level: 1, text: "Start" }, { kind: "embed", ref: "a", view: "text" }, { kind: "embed", ref: "b" },
    { kind: "embed", ref: "c", view: "text" }, { kind: "heading", level: 1, text: "End" }]);
  const part = docFromBlocks([{ kind: "heading", level: 1, text: "Gloves" }, { kind: "heading", level: 2, text: "Sizes" }]);
  const whole = docFromBlocks([{ kind: "heading", level: 1, text: "Elsewhere" }]);
  // c is not loaded (not readable, or not yet): it stays an embed.
  const blocks = wovenBlocks(fakeView(doc, { a: part, b: whole }));
  assert.deepEqual(tocHeadings(blocks, 0).map((x) => x.text), ["Start", "Gloves", "Sizes", "End"]);
  assert.deepEqual(tocHeadings(blocks, 1).map((x) => x.text), ["Start", "Gloves", "End"]);
  assert.deepEqual(blocks.map((b) => b.node.type.name), ["heading", "heading", "heading", "embed", "embed", "heading"]);
});

test("the choices offer every level and both views", () => {
  assert.deepEqual(tocDepths().map(([v]) => v), [1, 2, 0]);
  assert.deepEqual(embedViews().map(([v]) => v), ["", "text"]);
});
