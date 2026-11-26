// Unit tests of the shop's parts (shopparts.js,
// ADR-0251): which text is shown,
// which products are picked on their own, how answers of a form and a
// basket become the engine's orders. They run in Node.js without a
// browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { textOf, offered, partsOf, answers, orders, settled, listOf, statuses, engineChoices } from "../static/assets/shopparts.js";

test("a text is shown in the language wanted, else in the first one held", () => {
  assert.equal(textOf({ de: "Laptop", fr: "Portable" }, "fr"), "Portable");
  assert.equal(textOf({ fr: "Portable", de: "Laptop" }, "en"), "Laptop");
  assert.equal(textOf({ de: " ", en: "Dock" }, "it"), "Dock");
  assert.equal(textOf(null, "de"), "");
  assert.equal(textOf({}, "de"), "");
});

test("a release offers its active products that are no part of another", () => {
  const release = { items: [{ id: "ws", state: "active" }, { id: "acct", state: "active" }, { id: "dock" }, { id: "old", state: "withdrawn" }],
    includes: { ws: ["acct"] }, options: { ws: ["dock"] } };
  assert.deepEqual(offered(release).map((it) => it.id), ["ws", "dock"]);
  assert.deepEqual(offered(null), []);
  const parts = partsOf(release, "ws");
  assert.deepEqual(parts.included.map((it) => it.id), ["acct"]);
  assert.deepEqual(parts.options.map((it) => it.id), ["dock"]);
  assert.deepEqual(partsOf(release, "dock"), { included: [], options: [] });
});

test("answers of a form become strings, empty answers none", () => {
  assert.deepEqual(answers({ size: "14", count: 2, ok: true, tags: ["a", "b"], none: null, empty: "", gone: undefined }),
    { size: "14", count: "2", ok: "true", tags: '["a","b"]' });
  assert.deepEqual(answers(undefined), {});
});

test("a basket becomes one order per release", () => {
  const got = orders([
    { releaseId: "r1", itemId: "laptop", variantId: "14", config: { colour: "grey" }, options: ["dock"] },
    { releaseId: "r2", itemId: "phone" },
    { releaseId: "r1", itemId: "laptop", variantId: "16" },
    { releaseId: "r1", itemId: "dock" },
    { itemId: "nowhere" },
  ]);
  assert.deepEqual(got, [
    { releaseId: "r1", items: ["laptop", "dock"], variants: { laptop: ["14", "16"] }, config: { laptop: { colour: "grey" } } },
    { releaseId: "r2", items: ["phone"] },
  ]);
  assert.deepEqual(orders(null), []);
});

test("settled positions and the lists the engine answers", () => {
  assert.equal(settled("done"), true);
  assert.equal(settled("running"), false);
  assert.ok(statuses.includes("returning"));
  assert.deepEqual(listOf([1]), [1]);
  assert.deepEqual(listOf({ items: [2], total: 1 }), [2]);
  assert.deepEqual(listOf(null), []);
});

test("a field naming an engine chooses among the engines set up here", () => {
  const engines = [{ id: "atlas", name: "Atlas Produktion" }, { id: "test", name: "test" }, { name: "no id" }];
  assert.deepEqual(engineChoices(engines, "atlas"), [
    { value: "", text: "—", selected: false },
    { value: "atlas", text: "Atlas Produktion (atlas)", selected: true },
    { value: "test", text: "test", selected: false },
  ]);
  assert.equal(engineChoices(engines, "").find((c) => c.selected).value, "");
  // An engine no longer set up stays chosen, marked unknown.
  const gone = engineChoices(engines, "old");
  assert.deepEqual(gone[gone.length - 1], { value: "old", text: "old", selected: true, unknown: true });
  assert.equal(gone.filter((c) => c.selected).length, 1);
  // Before the list is read, the value held stays chosen.
  assert.deepEqual(engineChoices([], "atlas").map((c) => [c.value, c.selected]), [["", false], ["atlas", true]]);
});
