// Unit tests of the parts of the dialog that moves a page (moveparts.js):
// the rooms offered, the addresses it reads, whether a place lies below the
// page itself, and how the chosen place is named. They run in Node.js
// without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The dialog itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { moveRooms, treeURL, findURL, belowItself, placeTrail, crossesRoom } from "../static/assets/moveparts.js";

test("only rooms one may edit in are offered, favourites first, each group in its order", () => {
  const spaces = [
    { key: "A", role: "viewer" },
    { key: "B", role: "editor" },
    { key: "C", role: "space-admin", favourite: true },
    { key: "D", role: "editor", favourite: true },
    { key: "E", role: "" },
  ];
  assert.deepEqual(moveRooms(spaces).map((s) => s.key), ["C", "D", "B"]);
  assert.deepEqual(moveRooms([]), []);
});

test("the tree is read a level at a time, the search by words, both in one room", () => {
  assert.equal(treeURL("PLAT", "", ""), "/api/spaces/PLAT/tree");
  assert.equal(treeURL("PL AT", "id1", "id9"), "/api/spaces/PL%20AT/tree?parent=id1&after=id9");
  assert.equal(findURL("OPS", " run book "), "/api/items?space=OPS&q=run+book&limit=20");
});

test("a place below the page itself or its child pages is told", () => {
  // trail: the target first, then the items above it, nearest first.
  assert.equal(belowItself("p1", ["t", "a", "p1", "root"]), true);
  assert.equal(belowItself("p1", ["p1"]), true);
  assert.equal(belowItself("p1", ["t", "a"]), false);
  assert.equal(belowItself("p1", []), false);
});

test("the chosen place is named from the room down to the target", () => {
  assert.equal(placeTrail("Operations", []), "Operations");
  assert.equal(placeTrail("Operations", ["Runbook", "Network"]), "Operations › Network › Runbook");
});

test("a move crosses rooms only when the room differs", () => {
  assert.equal(crossesRoom({ space: "PLAT" }, "PLAT"), false);
  assert.equal(crossesRoom({ space: "PLAT" }, "OPS"), true);
});
