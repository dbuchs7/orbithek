// Unit tests of a search field in a text
// (ADR-0264): the block a choice makes, what
// the field searches when the reader chooses a scope, and the address of
// the search it asks. They run in Node.js without a browser:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { SCOPES, searchable, searchBlockOf, searchTarget, searchUrl } from "../static/assets/searchparts.js";

test("the scopes are the room and the server's scopes of a page, in their order", () => {
  assert.deepEqual(SCOPES, ["", "page", "tree", "below"]);
});

test("a choice makes a block that says it one way", () => {
  assert.deepEqual(searchBlockOf({}), { kind: "search" });
  assert.deepEqual(searchBlockOf({ space: "PLAT", ref: "x" }), { kind: "search", space: "PLAT" });
  assert.deepEqual(searchBlockOf({ scope: "tree", space: "PLAT", ref: "x" }), { kind: "search", scope: "tree", ref: "x" });
  // The item itself is named as none.
  assert.deepEqual(searchBlockOf({ scope: "below", ref: "self" }, "self"), { kind: "search", scope: "below" });
  assert.deepEqual(searchBlockOf({ scope: "children" }), { kind: "search" });
});

test("the reader's scope searches the block's page or room, else the item's", () => {
  const here = { id: "self", space: "PLAT" };
  assert.deepEqual(searchTarget({ kind: "search" }, "", here), { scope: "", space: "PLAT" });
  assert.deepEqual(searchTarget({ kind: "search", space: "OPS" }, "", here), { scope: "", space: "OPS" });
  assert.deepEqual(searchTarget({ kind: "search", space: "OPS" }, "tree", here), { scope: "tree", within: "self" });
  assert.deepEqual(searchTarget({ kind: "search", scope: "below", ref: "hb" }, "page", here), { scope: "page", within: "hb" });
  assert.deepEqual(searchTarget({ kind: "search", scope: "below" }, "below", here), { scope: "below", within: "self" });
  // A block of a page searches the item's own room when the room is chosen.
  assert.deepEqual(searchTarget({ kind: "search", scope: "tree", ref: "hb" }, "", here), { scope: "", space: "PLAT" });
});

test("the search address narrows to a page or a room", () => {
  assert.equal(searchUrl(" rocket fuel ", { scope: "tree", within: "hb" }), "/api/search?q=rocket+fuel&limit=10&within=hb&scope=tree");
  assert.equal(searchUrl("rocket", { scope: "", space: "PLAT" }, { limit: 5, after: "c1" }), "/api/search?q=rocket&limit=5&space=PLAT&after=c1");
  assert.equal(searchUrl("rocket", { scope: "", space: "" }), "/api/search?q=rocket&limit=10");
});

test("only a letter or a digit is searched for", () => {
  assert.equal(searchable("--"), false);
  assert.equal(searchable(""), false);
  assert.equal(searchable("Öl"), true);
  assert.equal(searchable("42"), true);
});
