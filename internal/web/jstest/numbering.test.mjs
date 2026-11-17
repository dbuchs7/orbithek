// The numbers of numbered headings (ADR-draft-numbered-headings), held to
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
