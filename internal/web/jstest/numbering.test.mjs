// The numbers of numbered headings (ADR-0241), held to
// the cases the server's exports are tested with.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { headingNumbers } from "../static/assets/numbering.js";

const vector = JSON.parse(readFileSync(new URL("./heading-numbers-vector.json", import.meta.url)));

test("the vector holds cases", () => assert.ok(vector.cases.length > 0));

for (const c of vector.cases) {
  test(c.name, () => {
    const blocks = c.blocks.map(([kind, level, text]) => ({ kind, level, text }));
    assert.deepEqual(headingNumbers(blocks), c.numbers);
  });
}

test("a heading without a level reads as level 1", () => {
  assert.deepEqual(headingNumbers([{ kind: "heading", text: "A" }, { kind: "heading", level: 2, text: "B" }]), ["1", "1.1"]);
});

// In the editor the numbers count the text as a reader reads it: the
// headings of an embed of its text alone are the text's own, those of a
// whole embed are not (ADR-0237), and a table of contents shows the same
// numbers.
import { docFromBlocks, tocHeadings, wovenBlocks, wovenNumbers } from "../static/assets/editor.js";

const fakeView = (doc, loaded = {}) => ({
  state: { doc },
  nodeDOM: (offset) => {
    const node = doc.nodeAt(offset);
    const inner = node?.type.name === "embed" ? loaded[node.attrs.ref] : null;
    return { scrollIntoView() {}, querySelector: () => (inner ? { embeddedView: fakeView(inner) } : null) };
  },
});

test("the headings of an embed of its text alone are numbered as the text's own", () => {
  const doc = docFromBlocks([{ kind: "toc" }, { kind: "heading", level: 1, text: "Start" }, { kind: "embed", ref: "a", view: "text" },
    { kind: "embed", ref: "b" }, { kind: "heading", level: 1, text: "End" }]);
  const part = docFromBlocks([{ kind: "heading", level: 2, text: "Gloves" }, { kind: "heading", level: 1, text: "Safety" }]);
  const whole = docFromBlocks([{ kind: "heading", level: 1, text: "Elsewhere" }]);
  const woven = wovenBlocks(fakeView(doc, { a: part, b: whole }));
  const nums = wovenNumbers(woven, true);
  assert.deepEqual(tocHeadings(woven, 0, nums).map((x) => `${x.num} ${x.text}`), ["1 Start", "1.1 Gloves", "2 Safety", "3 End"]);
  assert.deepEqual(tocHeadings(woven, 1, nums).map((x) => x.num), ["1", "2", "3"]);
  assert.deepEqual(tocHeadings(woven, 0, wovenNumbers(woven, false)).map((x) => x.num), ["", "", "", ""]);
});
