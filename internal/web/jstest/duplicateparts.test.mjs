// Unit tests of deleting the album's duplicates at once (duplicateparts.js,
// ADR-0196). They run in Node.js without a browser:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { cleanupPlan, keeper } from "../static/assets/duplicateparts.js";

// pic is a picture as the view of duplicates lists it.
const pic = (name, { sha = "a", w = 400, h = 300, size = 1000, created = "2026-01-01T00:00:00Z", editable = true, used = false } = {}) =>
  ({ attachment: { name, sha256: sha, size, created, photo: { width: w, height: h } }, editable, used });
const names = (list) => list.map((p) => p.attachment.name).sort();

test("the picture that stays is one a text shows or one may not delete, else the largest, else the oldest", () => {
  const old = pic("old.jpg", { created: "2025-01-01T00:00:00Z" });
  const young = pic("young.jpg", { created: "2026-06-01T00:00:00Z" });
  assert.equal(keeper([young, old]).attachment.name, "old.jpg");
  const big = pic("big.jpg", { w: 4000, h: 3000, created: "2026-06-01T00:00:00Z" });
  assert.equal(keeper([old, big]).attachment.name, "big.jpg", "pixels count before age");
  const heavy = pic("heavy.jpg", { size: 9000, created: "2026-06-01T00:00:00Z" });
  assert.equal(keeper([old, heavy]).attachment.name, "heavy.jpg", "bytes count when the pixels are equal");
  const shown = pic("shown.jpg", { w: 10, h: 10, used: true });
  assert.equal(keeper([big, shown]).attachment.name, "shown.jpg", "a picture a text shows stays");
  const foreign = pic("foreign.jpg", { w: 10, h: 10, editable: false });
  assert.equal(keeper([big, foreign]).attachment.name, "foreign.jpg", "a picture one may not delete stays");
});

test("without looks, only copies of the same file go, one of each staying", () => {
  const groups = [{ pictures: [
    pic("a1.jpg", { sha: "a", created: "2025-01-01T00:00:00Z" }), pic("a2.jpg", { sha: "a" }), pic("a3.jpg", { sha: "a" }),
    pic("b1.jpg", { sha: "b", w: 200, h: 150, created: "2024-01-01T00:00:00Z" }), pic("b2.jpg", { sha: "b", w: 200, h: 150 }),
    pic("c.jpg", { sha: "c", w: 100, h: 75 }),
  ] }];
  const plan = cleanupPlan(groups);
  assert.deepEqual(names(plan.remove), ["a2.jpg", "a3.jpg", "b2.jpg"]);
  assert.equal(plan.kept, 0);
  assert.equal(plan.alike, 2, "b1 and c would go as well with looks");
});

test("with looks, one picture stays of every group", () => {
  const groups = [
    { pictures: [pic("big.jpg", { sha: "a", w: 4000, h: 3000 }), pic("small.jpg", { sha: "b", w: 400, h: 300 }), pic("small2.jpg", { sha: "b", w: 400, h: 300 })] },
    { pictures: [pic("x.jpg", { sha: "x" }), pic("y.jpg", { sha: "x" })] },
  ];
  const plan = cleanupPlan(groups, true);
  assert.deepEqual(names(plan.remove), ["small.jpg", "small2.jpg", "y.jpg"]);
  assert.equal(plan.alike, 1);
});

test("pictures a text shows or one may not delete stay and are counted", () => {
  const groups = [{ pictures: [pic("a.jpg", { created: "2024-01-01T00:00:00Z" }), pic("b.jpg", { used: true }), pic("c.jpg", { editable: false }), pic("d.jpg")] }];
  const plan = cleanupPlan(groups);
  // b stays as the picture a text shows; a and d go; c may not be deleted.
  assert.deepEqual(names(plan.remove), ["a.jpg", "d.jpg"]);
  assert.equal(plan.kept, 1);
});

test("nothing goes from groups of one or without pictures", () => {
  assert.deepEqual(cleanupPlan([{ pictures: [pic("a.jpg")] }, {}], true), { remove: [], kept: 0, alike: 0 });
  assert.deepEqual(cleanupPlan([]), { remove: [], kept: 0, alike: 0 });
});
