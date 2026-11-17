// Unit tests of the depths a list of child pages offers (childpages.js,
// ADR-0239): what the page tree's API is
// asked for and that every depth has a name of its own. They run in
// Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The list in a text is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { childDepths, childDepthName, childDepthQuery } from "../static/assets/childpages.js";

test("each depth asks the page tree for its levels", () => {
  assert.deepEqual(childDepths.map(childDepthQuery), ["1", "2", "3", "4", "5", "all"]);
  assert.equal(childDepthQuery(1), "1");
  assert.equal(childDepthQuery(undefined), "1");
});

test("each depth has a name of its own", () => {
  const names = childDepths.map(childDepthName);
  assert.equal(new Set(names).size, childDepths.length);
  assert.deepEqual(names, ["1 level", "2 levels", "3 levels", "4 levels", "5 levels", "All levels"]);
  assert.equal(childDepthName(1), childDepthName(0));
});
