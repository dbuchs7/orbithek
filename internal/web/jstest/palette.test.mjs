// Unit tests of the parts of the command palette (paletteparts.js,
// ADR-0205): how what is typed matches names, their order, the recently
// opened items and the keys that open it. They run in Node.js without a
// browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The palette itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { RECENT, opens, rank, remember, score } from "../static/assets/paletteparts.js";

test("the letters typed match in their order", () => {
  assert.ok(score("tdn", "Today's daily note") > 0);
  assert.equal(score("ndt", "Today's daily note"), -1);
  assert.equal(score("x", "Calendar"), -1);
  assert.equal(score("", "anything"), 0);
  // Accents and case do not matter.
  assert.ok(score("ubersicht", "Übersicht") > 0);
  // Word starts and a prefix count more than letters inside words.
  assert.ok(score("cal", "Calendar") > score("cal", "Local alarm"));
  assert.ok(score("rm", "Reading mode") > score("rm", "Platform"));
});

test("matches come best first, ties in their order", () => {
  const entries = ["Light or dark look", "Calendar", "Query", "Go to Calendar room"];
  assert.deepEqual(rank("cal", entries, (e) => e), ["Calendar", "Go to Calendar room"]);
  assert.deepEqual(rank("", entries, (e) => e), entries);
  assert.deepEqual(rank("zzz", entries, (e) => e), []);
});

test("recently opened items are remembered once, newest first, bounded", () => {
  let list = [];
  list = remember(list, { key: "PLAT-1", title: "One" });
  list = remember(list, { key: "PLAT-2", title: "Two" });
  list = remember(list, { key: "PLAT-1", title: "One, renamed" });
  assert.deepEqual(list, [{ key: "PLAT-1", title: "One, renamed" }, { key: "PLAT-2", title: "Two" }]);
  for (let i = 0; i < 30; i++) list = remember(list, { key: "K-" + i, title: "" });
  assert.equal(list.length, RECENT);
  assert.equal(list[0].key, "K-29");
  // What a browser kept that is not a list, or broken entries, is dropped.
  assert.deepEqual(remember("junk", { key: "A", title: 1 }), [{ key: "A", title: "1" }]);
  assert.deepEqual(remember([null, { title: "x" }], { key: "A" }), [{ key: "A", title: "" }]);
});

test("Ctrl+Shift+P opens the palette anywhere, Ctrl+K outside a text being edited", () => {
  assert.equal(opens({ ctrlKey: true, shiftKey: true, key: "P" }, true), true);
  assert.equal(opens({ metaKey: true, shiftKey: true, key: "p" }, false), true);
  assert.equal(opens({ ctrlKey: true, key: "k" }, false), true);
  assert.equal(opens({ ctrlKey: true, key: "k" }, true), false);
  assert.equal(opens({ ctrlKey: true, shiftKey: true, key: "k" }, false), false);
  assert.equal(opens({ ctrlKey: true, altKey: true, key: "k" }, false), false);
  assert.equal(opens({ key: "k" }, false), false);
  assert.equal(opens({ ctrlKey: true, key: "p" }, false), false);
});
