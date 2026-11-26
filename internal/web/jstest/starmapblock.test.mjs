// Unit tests of a starmap in a text (ADR-0252): how
// its block narrows the map when shown, which map it reads, a room's map
// narrowed to a label, the block a choice makes and where a click on a
// node leads. They run in Node.js without a browser:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The block in a page is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { BLOCK_ALL, BLOCK_KINDS, blockFilters, blockSource, filter, nodeHref, starmapBlockOf, withLabel } from "../static/assets/starmap-graph.js";

const data = {
  nodes: [
    { id: "a", kind: "item", key: "PLAT-1", title: "Home", type: "page", space: "PLAT", labels: ["runbook"] },
    { id: "b", kind: "item", key: "PLAT-2", title: "Patch", type: "task", space: "PLAT", labels: ["runbook", "ops"], done: true },
    { id: "c", kind: "item", key: "PLAT-3", title: "Notes", type: "page", space: "PLAT" },
    { id: "picture:a/1", kind: "picture", item: "a", name: "logo.png", space: "PLAT", labels: ["runbook"] },
    { id: "label:runbook", kind: "label", name: "runbook" },
    { id: "g", kind: "group", space: "OPS", type: "task", count: 4 },
  ],
  edges: [
    { from: "a", to: "b", kind: "parent" },
    { from: "a", to: "c", kind: "parent" },
    { from: "a", to: "picture:a/1", kind: "picture" },
    { from: "a", to: "label:runbook", kind: "label" },
    { from: "b", to: "label:runbook", kind: "label" },
  ],
  truncated: false,
};

test("a block's filters are the starmap's own, a stray kind or depth dropped", () => {
  const f = blockFilters({ kind: "starmap", space: BLOCK_ALL, label: "runbook", search: "patch", depth: 2, hideTypes: ["task"], hideKinds: ["done", "picture", "item"] });
  assert.deepEqual({ ...f, hidden: [...f.hidden] }, { space: "*", label: "runbook", search: "patch", depth: 2, hidden: ["type:task", "done", "picture"] });
  const none = blockFilters({ kind: "starmap", depth: 7 });
  assert.deepEqual({ ...none, hidden: [...none.hidden] }, { space: "", label: "", search: "", depth: 0, hidden: [] });
  // The filters hide what the block names, as the switches do.
  const shown = filter(data, { hidden: blockFilters({ hideKinds: ["done", "picture"] }).hidden });
  assert.deepEqual([...shown.nodes].sort(), ["a", "c", "g", "label:runbook"]);
});

test("every room's map is narrowed to a label by the server, a room's is read whole", () => {
  assert.equal(blockSource(BLOCK_ALL), "/api/starmap");
  assert.equal(blockSource(BLOCK_ALL, "on call"), "/api/starmap?label=on%20call");
  assert.equal(blockSource("PLAT", "runbook"), "/api/spaces/PLAT/starmap");
});

test("a room's map narrowed to a label keeps its carriers, its star and the item a text is about", () => {
  const got = withLabel(data, "runbook");
  assert.deepEqual(got.nodes.map((n) => n.id), ["a", "b", "picture:a/1", "label:runbook"]);
  assert.deepEqual(got.edges.map((e) => e.from + ">" + e.to), ["a>b", "a>picture:a/1", "a>label:runbook", "b>label:runbook"]);
  assert.deepEqual(withLabel(data, "ops", "c").nodes.map((n) => n.id), ["b", "c"]);
  assert.equal(withLabel(data, ""), data);
});

test("a choice makes the block the server keeps", () => {
  assert.deepEqual(starmapBlockOf({}), { kind: "starmap" });
  assert.deepEqual(starmapBlockOf({ space: "PLAT", label: " runbook ", search: "  ", depth: 3, hideTypes: ["task", "decision", "task"], hideKinds: ["picture", "done", "item"] }),
    { kind: "starmap", space: "PLAT", label: "runbook", depth: 3, hideTypes: ["decision", "task"], hideKinds: ["done", "picture"] });
  assert.deepEqual(starmapBlockOf({ depth: 4, hideTypes: [], hideKinds: [] }), { kind: "starmap" });
  assert.deepEqual(BLOCK_KINDS, [...BLOCK_KINDS].sort(), "the kinds are named in sorted order, as the server checks");
});

test("a click opens an item, a picture's item, a label's page or a bubble's room", () => {
  const byID = new Map(data.nodes.map((n) => [n.id, n]));
  assert.equal(nodeHref(byID.get("a"), byID), "#/item/PLAT-1");
  assert.equal(nodeHref(byID.get("picture:a/1"), byID), "#/item/PLAT-1");
  assert.equal(nodeHref({ kind: "picture", item: "zz" }, byID), "#/item/zz");
  assert.equal(nodeHref(byID.get("label:runbook"), byID), "#/label/runbook");
  assert.equal(nodeHref(byID.get("g"), byID), "#/starmap/OPS");
});
