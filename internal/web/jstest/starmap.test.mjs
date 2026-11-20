// Unit tests of the starmap's graph (starmap-graph.js, ADR-0157) and its
// live physics (starmap-sim.js, ADR-0158): the node kinds and edge
// groups, what the filters keep with one hop of context, a node's
// neighbourhood at a depth, the drill path, the captions a zoom shows,
// the frame that fits, a seeded layout that is the same for the same map,
// a simulation that cools to the same rest every time, holds a pinned
// node and approximates the exact repulsion with its quadtree, and an
// impact walk's answer (ADR-0159) as a map, a list and an address. They run in
// Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The view itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { DEPTHS, GROUPS, itemSymbols, IMPACT_DEPTHS, IMPACT_DIRECTIONS, around, captionShown, distanceClass, drillBack, drillInto, filter, fitView, groupOf, impactAddress, impactGraph,
  impactList, impactParams, impactPath, impactSummary, kinds, kindOf, groupRadius, CLOUD_BOTTOM, CLOUD_MARGIN, CLOUD_REACH, cloudPath, cloudReach, layout, roomAddress, startRoom, matches, mergeLabelCounts, neighbourhood, size, nodeRadii, roomClusters, roomLinks, roomHull, roomStart, roomToneOf, toneOf, visible, zoomBand } from "../static/assets/starmap-graph.js";
import { DEFAULTS, chargeExact, chargeTree, quadtree, simulation } from "../static/assets/starmap-sim.js";

const graph = {
  nodes: [
    { id: "a", kind: "item", key: "PLAT-1", title: "Home", type: "page", space: "PLAT", labels: ["runbook"] },
    { id: "b", kind: "item", key: "PLAT-2", title: "Patch the kernel", type: "task", space: "PLAT", labels: ["runbook"] },
    { id: "c", kind: "item", key: "PLAT-3", title: "Größe", type: "page", space: "PLAT" },
    { id: "x", kind: "item", key: "OPS-1", title: "Ops guide", type: "page", space: "OPS", outside: true },
    { id: "picture:a/1", kind: "picture", item: "a", name: "logo.png", space: "PLAT" },
    { id: "label:runbook", kind: "label", name: "runbook" },
  ],
  edges: [
    { from: "a", to: "b", kind: "parent" },
    { from: "b", to: "c", kind: "mentions" },
    { from: "c", to: "x", kind: "relation", type: "implements" },
    { from: "picture:a/1", to: "a", kind: "picture" },
    { from: "label:runbook", to: "a", kind: "label" },
    { from: "label:runbook", to: "b", kind: "label" },
  ],
};

test("edges fall into four groups, nodes into their type, pictures and labels", () => {
  assert.deepEqual(Object.keys(GROUPS), ["tree", "relations", "pictures", "labels"]);
  assert.equal(groupOf("parent"), "tree");
  assert.equal(groupOf("embeds"), "relations");
  assert.equal(groupOf("copy"), "pictures");
  assert.equal(groupOf("label"), "labels");
  assert.equal(kindOf(graph.nodes[1]), "type:task");
  assert.equal(kindOf(graph.nodes[4]), "picture");
  assert.deepEqual(kinds(graph.nodes), [{ key: "type:page", type: "page" }, { key: "type:task", type: "task" }, { key: "picture" }, { key: "label" }, { key: "outside" }]);
  assert.deepEqual(kinds(graph.nodes.slice(0, 2)).map((k) => k.key), ["type:page", "type:task"], "only the kinds the map has");
  const tone = toneOf(graph.nodes);
  assert.equal(tone("page"), 0);
  assert.equal(tone("task"), 1);
});

test("a query matches title, key, label and name, every word, without regard to case and accents", () => {
  assert.ok(matches(graph.nodes[1], "kernel patch"));
  assert.ok(matches(graph.nodes[1], "plat-2"));
  assert.ok(matches(graph.nodes[0], "RUNBOOK"));
  assert.ok(matches(graph.nodes[2], "grosse") === false, "ß is not ss");
  assert.ok(matches(graph.nodes[2], "GRÖ"));
  assert.ok(matches({ ...graph.nodes[2], title: "Café" }, "cafe"));
  assert.ok(matches(graph.nodes[4], "logo"));
  assert.ok(!matches(graph.nodes[1], "kernel home"));
  assert.ok(!matches(graph.nodes[1], "  "));
});

test("without filters everything shows", () => {
  const f = filter(graph);
  assert.equal(f.nodes.size, graph.nodes.length);
  assert.deepEqual(f.edges, [0, 1, 2, 3, 4, 5]);
  assert.equal(f.matches.size, 0);
});

test("a query keeps the matches and one hop of context", () => {
  const f = filter(graph, { query: "kernel" });
  assert.deepEqual([...f.matches], ["b"]);
  assert.deepEqual([...f.nodes].sort(), ["a", "b", "c", "label:runbook"], "b's neighbours, not theirs");
  assert.deepEqual(f.edges, [0, 1, 4, 5], "the edges whose ends both show");
  const none = filter(graph, { query: "nothing like it" });
  assert.equal(none.nodes.size, 0);
  assert.deepEqual(none.edges, []);
});

test("a kind switched off hides its nodes and their edges; an edge group switched off hides its edges and the context they gave", () => {
  const pages = filter(graph, { hidden: new Set(["type:page"]) });
  assert.deepEqual([...pages.nodes].sort(), ["b", "label:runbook", "picture:a/1"]);
  assert.deepEqual(pages.edges, [5]);
  const outside = filter(graph, { hidden: new Set(["outside"]) });
  assert.ok(!outside.nodes.has("x"));
  assert.ok(outside.nodes.has("c"));
  const tree = filter(graph, { hidden: new Set(["tree", "labels"]) });
  assert.deepEqual(tree.edges, [1, 2, 3]);
  const context = filter(graph, { query: "kernel", hidden: new Set(["labels", "tree"]) });
  assert.deepEqual([...context.nodes].sort(), ["b", "c"], "the context comes through edges shown only");
});

test("a node's neighbourhood is it and what a shown edge joins to it", () => {
  assert.deepEqual([...neighbourhood(graph, "b")].sort(), ["a", "b", "c", "label:runbook"]);
  assert.deepEqual([...neighbourhood(graph, "b", [1])].sort(), ["b", "c"]);
  assert.deepEqual([...neighbourhood(graph, "x")].sort(), ["c", "x"]);
});

test("the layout is the same for the same map, inside the canvas, and moves no node onto another", () => {
  const one = layout(graph), two = layout(structuredClone(graph));
  assert.deepEqual([...one], [...two]);
  const side = size(graph.nodes.length);
  assert.equal(side, 800);
  for (const { x, y } of one.values()) assert.ok(x >= 20 && x <= side - 20 && y >= 20 && y <= side - 20, `${x},${y}`);
  const points = [...one.values()];
  for (let i = 0; i < points.length; i++) {
    for (let j = i + 1; j < points.length; j++) {
      assert.ok(Math.hypot(points[i].x - points[j].x, points[i].y - points[j].y) > 20, "nodes apart");
    }
  }
  // Pinned for the fixed input: a change of the simulation shows here.
  assert.deepEqual(one.get("a"), layout(graph).get("a"));
  const pinned = Object.fromEntries([...one].map(([id, p]) => [id, [p.x, p.y]]));
  assert.deepEqual(pinned, PINNED);
  // A joined pair sits closer than the farthest pair.
  const d = (p, q) => Math.hypot(one.get(p).x - one.get(q).x, one.get(p).y - one.get(q).y);
  assert.ok(d("a", "b") < d("picture:a/1", "x"));
  assert.ok(size(10000) > 800);
  assert.equal(layout({ nodes: [], edges: [] }).size, 0);
});

const PINNED = { "a": [448.2, 414.4], "b": [358.2, 435.1], "c": [401, 372.5], "x": [422.5, 480.9], "picture:a/1": [334.5, 367.4], "label:runbook": [436.2, 329.2] };

test("the neighbourhood at a depth follows shown edges either way, as far as the depth reaches", () => {
  assert.deepEqual([...around(graph, "c", 1)].sort(), ["b", "c", "x"]);
  assert.deepEqual([...around(graph, "c", 2)].sort(), ["a", "b", "c", "label:runbook", "x"]);
  assert.deepEqual([...around(graph, "c", 3)].sort(), ["a", "b", "c", "label:runbook", "picture:a/1", "x"]);
  assert.deepEqual([...around(graph, "c", 2, [1, 2])].sort(), ["b", "c", "x"], "only through the edges given");
  assert.deepEqual([...around(graph, "nothing", 2)], ["nothing"]);
  assert.deepEqual([...neighbourhood(graph, "c")].sort(), [...around(graph, "c", 1)].sort());
  assert.deepEqual(DEPTHS, [1, 2, 3]);
});

test("the drill path grows by a station, cuts back to a station it holds, and steps back one at a time", () => {
  let trail = drillInto([], "b");
  assert.deepEqual(trail, ["b"]);
  trail = drillInto(trail, "c");
  trail = drillInto(trail, "x");
  assert.deepEqual(trail, ["b", "c", "x"]);
  assert.deepEqual(drillInto(trail, "c"), ["b", "c"], "a station on the path is not visited twice");
  assert.deepEqual(drillBack(trail), ["b", "c"]);
  assert.deepEqual(drillBack(drillBack(drillBack(trail))), []);
  assert.deepEqual(drillBack([]), []);
  assert.deepEqual(drillBack(trail, 0), ["b"]);
  assert.deepEqual(drillBack(trail, -1), [], "the whole room");
});

test("a separated node shows with its neighbourhood, and the filters still apply inside it", () => {
  const whole = visible(graph);
  assert.equal(whole.nodes.size, graph.nodes.length);
  const one = visible(graph, { trail: ["c"] });
  assert.deepEqual([...one.nodes].sort(), ["b", "c", "x"]);
  assert.deepEqual(one.edges, [1, 2]);
  const two = visible(graph, { trail: ["b", "c"], depth: 2 });
  assert.deepEqual([...two.nodes].sort(), ["a", "b", "c", "label:runbook", "x"], "the last station counts, at its depth");
  const outside = visible(graph, { trail: ["c"], hidden: new Set(["outside"]) });
  assert.deepEqual([...outside.nodes].sort(), ["b", "c"], "a switch hides inside a separated neighbourhood");
  const relations = visible(graph, { trail: ["b"], hidden: new Set(["labels", "tree"]) });
  assert.deepEqual([...relations.nodes].sort(), ["b", "c"], "the neighbourhood follows the edges the switches leave on");
  const found = visible(graph, { trail: ["c"], query: "kernel" });
  assert.deepEqual([...found.matches], ["b"]);
  assert.deepEqual([...found.nodes].sort(), ["b", "c"], "a search narrows the neighbourhood, never reaches out of it");
  assert.equal(visible(graph, { trail: ["c"], hidden: new Set(["type:page"]) }).nodes.size, 0, "a hidden station leaves nothing");
});

test("captions fade with the zoom: far shows only what matters now, mid the items, near everything", () => {
  assert.equal(zoomBand(0.3), "far");
  assert.equal(zoomBand(0.6), "mid");
  assert.equal(zoomBand(1), "mid");
  assert.equal(zoomBand(1.2), "near");
  assert.equal(captionShown("far", { kind: "item" }), false);
  assert.equal(captionShown("far", { kind: "label", lit: true }), true);
  assert.equal(captionShown("mid", { kind: "item" }), true);
  assert.equal(captionShown("mid", { kind: "picture" }), false);
  assert.equal(captionShown("near", { kind: "picture" }), true);
});

test("the view fits the points into the canvas, centred, within its zoom", () => {
  const v = fitView([{ x: 0, y: 0 }, { x: 200, y: 100 }], 500, 300, { pad: 50 });
  assert.equal(v.zoom, 2);
  assert.deepEqual([v.panX + v.zoom * 100, v.panY + v.zoom * 50], [250, 150], "the middle of the points is the middle of the canvas");
  assert.equal(fitView([{ x: 5, y: 5 }], 500, 300).zoom, 2.5, "a single point is not blown up beyond the zoom's top");
  assert.equal(fitView([{ x: 0, y: 0 }, { x: 1e6, y: 0 }], 500, 300).zoom, 0.1);
  assert.deepEqual(fitView([], 500, 300), { zoom: 1, panX: 250, panY: 150 });
});

// A fixed start, not the seeded layout: the seeded layout uses sine and
// cosine, whose last digits may differ between engines, while the
// simulation uses only arithmetic and square roots, which do not.
function grid(count, links) {
  const x0 = [], y0 = [];
  for (let i = 0; i < count; i++) { x0.push(300 + (i % 4) * 40 + (i % 3)); y0.push(300 + Math.floor(i / 4) * 40); }
  return { x0, y0, links, center: { x: 360, y: 340 } };
}
const REST = [[0, 286.7, 301.1], [1, 335.3, 230.1], [2, 417.3, 251.3], [3, 360.5, 313.5], [4, 287.4, 380], [5, 337.2, 450.2], [6, 418.8, 427.4], [7, 360.8, 366.4]];
const ring = grid(8, [[0, 1], [1, 2], [2, 3], [3, 4], [4, 5], [5, 6], [6, 7], [7, 0], [0, 4]]);

test("the simulation comes to the same rest from the same start, nodes apart and joined ones close", () => {
  const one = simulation(ring), two = simulation(structuredClone(ring));
  const ticks = one.run();
  two.run();
  assert.ok(one.settled);
  assert.ok(ticks > 100 && ticks <= 400, `${ticks} ticks to rest`);
  assert.deepEqual(one.positions(), two.positions());
  assert.deepEqual(one.positions(), REST, "pinned for the fixed input: a change of the forces shows here");
  const at = (i) => [one.x[i], one.y[i]];
  const d = (i, j) => Math.hypot(at(i)[0] - at(j)[0], at(i)[1] - at(j)[1]);
  for (let i = 0; i < 8; i++) for (let j = i + 1; j < 8; j++) assert.ok(d(i, j) >= 2 * DEFAULTS.radius - 1, `nodes ${i} and ${j} apart`);
  assert.ok(d(0, 1) < d(2, 6), "a joined pair sits closer than an opposite one");
  assert.equal(simulation({ x0: [], y0: [] }).run(), 300, "an empty map cools like any other, in 300 ticks as d3");
});

test("alpha cools to rest and stays warm while a target holds it; a reheat never cools", () => {
  const sim = simulation(ring);
  let last = sim.alpha;
  for (let k = 0; k < 50; k++) { sim.tick(); assert.ok(sim.alpha < last); last = sim.alpha; }
  assert.ok(!sim.settled);
  sim.run();
  assert.ok(sim.settled && sim.alpha < DEFAULTS.alphaMin);
  assert.equal(sim.tick(), false, "at rest a tick reports no motion");
  sim.reheat(0.3);
  assert.equal(sim.alpha, 0.3);
  sim.reheat(0.1);
  assert.equal(sim.alpha, 0.3, "a smaller reheat does not cool");
  sim.alphaTarget = 0.3;
  for (let k = 0; k < 1000; k++) sim.tick();
  assert.ok(!sim.settled && Math.abs(sim.alpha - 0.3) < 1e-6, "a held node keeps the map warm");
  sim.alphaTarget = 0;
  sim.run();
  assert.ok(sim.settled);
});

test("a pinned node stays where it is put while its neighbours follow it; unpinned it moves again", () => {
  const sim = simulation(ring);
  sim.run();
  const before = [sim.x[1], sim.y[1]];
  sim.pin(0, { x: 900, y: 900 });
  sim.reheat(0.5);
  for (let k = 0; k < 120; k++) {
    sim.tick();
    assert.deepEqual([sim.x[0], sim.y[0]], [900, 900]);
  }
  assert.ok(sim.pinned(0));
  assert.ok(Math.hypot(sim.x[1] - before[0], sim.y[1] - before[1]) > 100, "a neighbour is pulled along by its spring");
  sim.unpin(0);
  assert.ok(!sim.pinned(0));
  sim.reheat(0.5);
  sim.tick();
  assert.notDeepEqual([sim.x[0], sim.y[0]], [900, 900]);
});

test("only the active nodes move and push; the rest is frozen where it is", () => {
  const sim = simulation(ring);
  sim.setActive((i) => i < 4, (k) => k !== 1);
  assert.deepEqual(sim.active, [0, 1, 2, 3]);
  assert.ok(sim.isActive(2) && !sim.isActive(5));
  const frozen = [sim.x[5], sim.y[5]];
  sim.run();
  assert.deepEqual([sim.x[5], sim.y[5]], frozen);
  assert.deepEqual(sim.positions().map((p) => p[0]), [0, 1, 2, 3]);
});

test("the quadtree's repulsion comes close to the exact one, and both push apart", () => {
  let seed = 7;
  const random = () => (seed = (seed * 16807) % 2147483647) / 2147483647;
  const count = 600;
  const x = Float64Array.from({ length: count }, () => random() * 1500), y = Float64Array.from({ length: count }, () => random() * 1500);
  x[1] = x[0]; y[1] = y[0]; // two on one spot
  const active = Array.from({ length: count }, (_, i) => i);
  const exact = [new Float64Array(count), new Float64Array(count)], tree = [new Float64Array(count), new Float64Array(count)];
  chargeExact(x, y, active, ...exact);
  chargeTree(x, y, active, ...tree);
  let err = 0, total = 0;
  for (let i = 0; i < count; i++) {
    err += Math.hypot(exact[0][i] - tree[0][i], exact[1][i] - tree[1][i]);
    total += Math.hypot(exact[0][i], exact[1][i]);
  }
  assert.ok(err / total < 0.03, `relative error ${err / total}`);
  assert.ok(Math.sign(exact[0][0]) === -Math.sign(exact[0][1]) && exact[0][0] !== 0, "two on one spot part");
  assert.ok(Math.sign(tree[0][0]) === Math.sign(exact[0][0]), "the tree parts them the same way");
  const root = quadtree(x, y, active);
  assert.equal(root.weight, count, "the root weighs every point");
  assert.equal(quadtree(x, y, []), null);
  // Beyond the cut-off, nothing pushes.
  const far = [new Float64Array(2), new Float64Array(2)];
  chargeExact(Float64Array.of(0, 1000), Float64Array.of(0, 0), [0, 1], ...far, { chargeMax: 500 });
  assert.deepEqual([...far[0]], [0, 0]);
});

test("the simulation over the budget's 600 nodes uses the quadtree and comes to rest", () => {
  const count = 600;
  const links = [];
  for (let i = 1; i < count; i++) links.push([Math.floor((i - 1) / 3), i]);
  const start = grid(count, links);
  const sim = simulation(start);
  assert.ok(count >= DEFAULTS.exactBelow);
  const ticks = sim.run();
  assert.ok(sim.settled && ticks <= 400);
  for (let i = 0; i < count; i++) assert.ok(Number.isFinite(sim.x[i]) && Number.isFinite(sim.y[i]));
});

// An impact walk's answer as the server gives it (ADR-0159): the start,
// then what was reached, each with its distance and the edges that
// reached it.
const walk = {
  start: "a", space: "PLAT", direction: "affected", depth: 3, truncated: false, complete: true,
  nodes: [
    { id: "a", kind: "item", key: "PLAT-1", title: "Home", type: "page", space: "PLAT", distance: 0, via: [] },
    { id: "b", kind: "item", key: "PLAT-2", title: "Patch", type: "task", space: "PLAT", distance: 1, via: [{ from: "a", to: "b", kind: "parent" }] },
    { id: "picture:a/1", kind: "picture", item: "a", name: "logo.png", space: "PLAT", distance: 1, via: [{ from: "picture:a/1", to: "a", kind: "picture" }] },
    { id: "x", kind: "item", key: "OPS-1", title: "Ops guide", type: "page", space: "OPS", outside: true, distance: 2,
      via: [{ from: "x", to: "b", kind: "relation", type: "depends-on" }] },
  ],
  edges: [
    { from: "a", to: "b", kind: "embeds" },
    { from: "a", to: "b", kind: "parent" },
    { from: "picture:a/1", to: "a", kind: "picture" },
    { from: "x", to: "b", kind: "relation", type: "depends-on" },
    { from: "x", to: "b", kind: "relation", type: "blocks" },
  ],
};

test("an impact view is read from the address, with depth 1 to 5 or all", () => {
  const p = (q) => impactParams(new URLSearchParams(q));
  assert.equal(p(""), null);
  assert.equal(p("depth=2"), null, "a drill depth alone is no impact view");
  assert.equal(p("impact=sideways&depth=2"), null);
  assert.deepEqual(p("impact=affected&depth=3"), { direction: "affected", depth: 3 });
  assert.deepEqual(p("impact=dependson&depth=all"), { direction: "dependson", depth: "all" });
  assert.deepEqual(p("impact=affected&depth=9"), { direction: "affected", depth: 1 });
  assert.deepEqual(p("impact=affected"), { direction: "affected", depth: 1 });
  assert.deepEqual(IMPACT_DIRECTIONS, ["affected", "dependson"]);
  assert.deepEqual(IMPACT_DEPTHS, [1, 2, 3, 4, 5, "all"]);
});

test("an impact view's address keeps the room, the drill path, the start, direction and depth", () => {
  assert.equal(impactAddress("PLAT", ["PLAT-2"], { direction: "affected", depth: 3 }), "#/starmap/PLAT/PLAT-2?impact=affected&depth=3");
  assert.equal(impactAddress("PLAT", ["PLAT-5", "picture:a/1"], { direction: "dependson", depth: "all" }),
    "#/starmap/PLAT/PLAT-5/picture%3Aa%2F1?impact=dependson&depth=all");
  const back = impactParams(new URLSearchParams(impactAddress("PLAT", ["PLAT-2"], { direction: "dependson", depth: 5 }).split("?")[1]));
  assert.deepEqual(back, { direction: "dependson", depth: 5 }, "the address reads back as it was written");
});

test("the walk is asked of the item, a picture by its item and attachment", () => {
  assert.equal(impactPath("PLAT-2", { direction: "affected", depth: 2 }), "/api/items/PLAT-2/impact?direction=affected&depth=2");
  assert.equal(impactPath("picture:id1/att 9", { direction: "dependson", depth: "all" }), "/api/items/id1/impact?direction=dependson&depth=all&picture=att+9");
});

test("an impact walk becomes a map whose walked edges are marked", () => {
  const g = impactGraph(walk);
  assert.equal(g.nodes, walk.nodes);
  assert.deepEqual(g.edges.map((e) => e.walk), [false, true, true, true, false],
    "the embed against the walk and the second relation between the same items are not walked");
  assert.equal(walk.edges[0].walk, undefined, "the answer itself is left as it is");
  assert.deepEqual(impactGraph({ nodes: [{ id: "a", distance: 0 }], edges: [] }).edges, []);
});

test("a node's distance gives its class", () => {
  assert.deepEqual([0, 1, 2, 3, 4, 5, 6, 40, -1].map(distanceClass), ["dist-0", "dist-1", "dist-2", "dist-3", "dist-4", "dist-5", "dist-far", "dist-far", "dist-0"]);
});

test("the list holds what was reached, nearest first, the server's order kept within a distance", () => {
  const shuffled = { ...walk, nodes: [walk.nodes[3], walk.nodes[1], walk.nodes[0], walk.nodes[2]] };
  assert.deepEqual(impactList(shuffled).map((n) => n.id), ["b", "picture:a/1", "x"]);
  assert.deepEqual(impactList(walk).map((n) => n.distance), [1, 1, 2]);
});

test("the summary counts items and pictures apart and says when the walk ended before its depth", () => {
  assert.deepEqual(impactSummary(walk), { items: 2, pictures: 1, farthest: 2, endedEarly: true });
  assert.equal(impactSummary({ ...walk, depth: 2 }).endedEarly, false, "the walk reached its depth");
  assert.equal(impactSummary({ ...walk, complete: false }).endedEarly, false, "more is reachable");
  assert.equal(impactSummary({ ...walk, depth: "all" }).endedEarly, false, "all has no depth to end before");
  assert.deepEqual(impactSummary({ ...walk, nodes: [walk.nodes[0]] }), { items: 0, pictures: 0, farthest: 0, endedEarly: true });
});

test("a group of the starmap of all rooms is filtered by its type and drawn as big as its items (ADR-0191)", () => {
  const groups = [{ id: "group:PLAT/page", kind: "group", space: "PLAT", type: "page", count: 3 }, { id: "group:OPS/task", kind: "group", space: "OPS", type: "task", count: 400 }];
  assert.equal(kindOf(groups[0]), "type:page");
  assert.deepEqual(kinds(groups).map((k) => k.key), ["type:page", "type:task"]);
  assert.equal(groupRadius(0), 11);
  assert.ok(groupRadius(3) > groupRadius(1) && groupRadius(100) > groupRadius(10));
  assert.equal(groupRadius(1e6), 64, "a bound keeps a big group on the canvas");
  assert.equal(groupRadius("x"), 11);
});

test("the labels of items and of pictures are suggested as one list, the most carried first", () => {
  assert.deepEqual(mergeLabelCounts([{ label: "q4", count: 2 }, { label: "ops", count: 1 }], [{ label: "q4", count: 3 }, { label: "beach", count: 1 }]),
    [{ label: "q4", count: 5 }, { label: "beach", count: 1 }, { label: "ops", count: 1 }]);
  assert.deepEqual(mergeLabelCounts([], []), []);
});

test("the starmap opens on every room unless the address names a readable one", () => {
  const rooms = [{ key: "PLAT" }, { key: "OPS" }];
  assert.equal(startRoom("", rooms), "*");
  assert.equal(startRoom(undefined, rooms), "*");
  assert.equal(startRoom("*", rooms), "*");
  assert.equal(startRoom("OPS", rooms), "OPS");
  assert.equal(startRoom("GONE", rooms), "*", "a room one does not read falls back to every room");
});

test("a double-click on a bubble opens its room, on any other node it separates", () => {
  assert.equal(roomAddress({ id: "group:API/page", kind: "group", space: "API", count: 1548 }), "#/starmap/API");
  assert.equal(roomAddress({ id: "group:a b/page", kind: "group", space: "a b" }), "#/starmap/a%20b");
  assert.equal(roomAddress({ id: "a", kind: "item", key: "PLAT-1", space: "PLAT" }), null);
  assert.equal(roomAddress({ id: "label:runbook", kind: "label", name: "runbook" }), null);
  assert.equal(roomAddress({ id: "group:x", kind: "group" }), null, "a bubble without a room opens nothing");
});

test("a group of the starmap of all rooms is drawn as a cloud within its radius", () => {
  for (const r of [groupRadius(1), groupRadius(1548), 64]) {
    const d = cloudPath(r);
    assert.match(d, /^M[-\d. ,]+(A[-\d. ,]+)+Z$/, "a closed path of arcs: " + d);
    const nums = d.replace(/[MAZ]/g, " ").trim().split(/[ ,]+/).map(Number);
    assert.ok(nums.every(Number.isFinite), "only numbers: " + d);
    // The end points of the arcs: M x y, then A rx ry rot large sweep x y.
    const points = [[nums[0], nums[1]]];
    for (let i = 2; i + 6 < nums.length + 1; i += 7) points.push([nums[i + 5], nums[i + 6]]);
    assert.equal(points.length, 6, "five lobes: " + d);
    for (const [x, y] of points) assert.ok(Math.hypot(x, y) <= r + 0.1, `(${x}, ${y}) lies within ${r}`);
    assert.ok(points.some(([, y]) => y < -0.5 * r), "the lobes rise above the middle");
    assert.equal(points[0][1], Math.round(CLOUD_BOTTOM * r * 10) / 10, "the cloud stands on a flat bottom");
    assert.equal(points[5][1], points[0][1], "the bottom is flat");
  }
  assert.equal(cloudPath(20), cloudPath(20), "the same size draws the same cloud");
  assert.notEqual(cloudPath(20), cloudPath(40), "a larger group draws a larger cloud");
});

test("the clouds of one room share a tone, rooms in the order of their keys", () => {
  const nodes = [
    { id: "group:OPS/page", kind: "group", space: "OPS", type: "page", count: 3 },
    { id: "group:API/page", kind: "group", space: "API", type: "page", count: 9 },
    { id: "group:OPS/task", kind: "group", space: "OPS", type: "task", count: 2 },
    { id: "group:HR/page", kind: "group", space: "HR", type: "page", count: 1 },
    { id: "label:x", kind: "label", name: "x" },
  ];
  const tone = roomToneOf(nodes);
  assert.equal(tone("API"), 0);
  assert.equal(tone("HR"), 1);
  assert.equal(tone("OPS"), 2);
  assert.equal(tone(nodes[0].space), tone(nodes[2].space), "both clouds of OPS have one tone");
  assert.equal(tone("GONE"), 0, "a room without clouds takes the first tone");
  const many = Array.from({ length: 8 }, (_, i) => ({ id: "group:R" + i + "/page", kind: "group", space: "R" + i, type: "page", count: 1 }));
  const manyTone = roomToneOf(many);
  assert.deepEqual(many.map((n) => manyTone(n.space)), [0, 1, 2, 3, 4, 5, 0, 1], "beyond six rooms the tones repeat");
});

test("the clouds of one room are tied together, the first of the room to each other", () => {
  const nodes = [
    { id: "group:OPS/page", kind: "group", space: "OPS" },
    { id: "group:API/page", kind: "group", space: "API" },
    { id: "group:OPS/task", kind: "group", space: "OPS" },
    { id: "label:x", kind: "label", name: "x" },
    { id: "group:OPS/note", kind: "group", space: "OPS" },
    { id: "group:HR/page", kind: "group", space: "HR" },
  ];
  assert.deepEqual(roomLinks(nodes), [
    { from: "group:OPS/page", to: "group:OPS/task", kind: "room", rest: 43 },
    { from: "group:OPS/page", to: "group:OPS/note", kind: "room", rest: 43 },
  ]);
  assert.deepEqual(roomLinks(nodes.filter((n) => n.space !== "OPS")), [], "a room with one cloud needs no tie");
});

test("a room's outline encloses its clouds with a margin", () => {
  assert.equal(roomHull([]), "", "no clouds, no outline");
  const circles = [{ x: 100, y: 100, r: 20 }, { x: 220, y: 140, r: 35 }, { x: 150, y: 230, r: 15 }];
  const d = roomHull(circles, 12);
  assert.match(d, /^M[-\d.]+ [-\d.]+(L[-\d.]+ [-\d.]+)+Z$/, d);
  const pts = d.slice(1, -1).split("L").map((p) => p.split(" ").map(Number));
  assert.ok(pts.length >= 3);
  // Convex and counter-clockwise or clockwise throughout: every point of
  // every cloud's margin lies on the inner side of every edge.
  const cross = (o, a, b) => (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0]);
  const sign = Math.sign(cross(pts[0], pts[1], pts[2]));
  for (const c of circles) for (let k = 0; k < 8; k++) {
    const q = [c.x + (c.r + 11) * Math.cos(k * Math.PI / 4), c.y + (c.r + 11) * Math.sin(k * Math.PI / 4)];
    for (let i = 0; i < pts.length; i++) {
      const v = cross(pts[i], pts[(i + 1) % pts.length], q);
      assert.ok(v === 0 || Math.sign(v) === sign, `(${q}) lies inside the outline`);
    }
  }
  assert.equal(roomHull(circles, 12), d, "the same clouds draw the same outline");
  assert.match(roomHull([{ x: 50, y: 60, r: 10 }], 5), /^M/, "one cloud gets a ring");
});

test("the clouds of one room start together, each room around a centre of its own", () => {
  const nodes = [];
  for (const space of ["OPS", "HR", "API", "FIN"]) for (const type of ["page", "task"]) nodes.push({ id: `group:${space}/${type}`, kind: "group", space, type, count: 5 });
  nodes.push({ id: "label:x", kind: "label", name: "x" });
  const start = roomStart(nodes, 400);
  assert.equal(start.size, 8, "every cloud has a start, nothing else");
  const d = (a, b) => Math.hypot(start.get(a).x - start.get(b).x, start.get(a).y - start.get(b).y);
  for (const space of ["OPS", "HR", "API", "FIN"]) for (const other of ["OPS", "HR", "API", "FIN"]) {
    if (space === other) continue;
    assert.ok(d(`group:${space}/page`, `group:${space}/task`) < d(`group:${space}/page`, `group:${other}/page`),
      `${space}'s clouds start nearer each other than to ${other}'s`);
  }
  assert.deepEqual(roomStart(nodes, 400), start, "the same map starts the same");
  // The seeded layout keeps a room's clouds together.
  const graph = { nodes, edges: roomLinks(nodes) };
  const at = layout(graph, { start });
  const dl = (a, b) => Math.hypot(at.get(a).x - at.get(b).x, at.get(a).y - at.get(b).y);
  for (const space of ["OPS", "HR", "API", "FIN"]) for (const other of ["OPS", "HR", "API", "FIN"]) {
    if (space === other) continue;
    assert.ok(dl(`group:${space}/page`, `group:${space}/task`) < dl(`group:${space}/page`, `group:${other}/task`),
      `after the layout ${space}'s clouds lie nearer each other than to ${other}'s`);
  }
});

test("a link of the simulation may rest at a length of its own", () => {
  const sim = simulation({ x0: [0, 300, 600, 900], y0: [0, 0, 0, 0], links: [[0, 1], [2, 3, 50]], center: { x: 450, y: 0 }, charge: 0, gravity: 0, alpha: 1 });
  for (let i = 0; i < 600 && sim.tick(); i++);
  const d = (a, b) => Math.hypot(sim.x[a] - sim.x[b], sim.y[a] - sim.y[b]);
  assert.ok(Math.abs(d(0, 1) - DEFAULTS.linkDistance) < 3, "the default rest: " + d(0, 1));
  assert.ok(Math.abs(d(2, 3) - 50) < 3, "its own rest: " + d(2, 3));
});

test("a cloud keeps others off by its own size, every other node by the default", () => {
  assert.equal(nodeRadii(graph.nodes, 18), null, "a map without clouds keeps one radius for all");
  const nodes = [{ id: "group:A/page", kind: "group", space: "A", count: 1548 }, { id: "group:A/task", kind: "group", space: "A", count: 1 }, { id: "label:x", kind: "label", name: "x" }];
  assert.deepEqual(nodeRadii(nodes, 18), [cloudReach(1548) + CLOUD_MARGIN, cloudReach(1) + CLOUD_MARGIN, 18]);
  // A room's tie rests outside the clouds' collision, so the two do not fight.
  const [tie] = roomLinks(nodes);
  const r = nodeRadii(nodes, 18);
  assert.ok(tie.rest >= r[0] + r[1], `rest ${tie.rest} keeps the clouds apart (${r[0] + r[1]})`);
});

test("the simulation keeps nodes apart by their own radii when it has them", () => {
  const sim = simulation({ x0: [0, 5, 400, 405], y0: [0, 0, 0, 0], radii: [40, 30, 18, 18], center: { x: 200, y: 0 }, charge: 0, gravity: 0.2, alpha: 1 });
  for (let i = 0; i < 800 && sim.tick(); i++);
  const d = (a, b) => Math.hypot(sim.x[a] - sim.x[b], sim.y[a] - sim.y[b]);
  assert.ok(d(0, 1) >= 70 - 2, "large nodes keep the sum of their radii apart: " + d(0, 1));
  assert.ok(d(1, 2) >= 48 - 2 && d(0, 3) >= 58 - 2, "a large and a small node too");
  assert.ok(d(2, 3) >= 36 - 2, "small nodes as before: " + d(2, 3));
});

// arcExtent samples an SVG elliptical arc (circular here) from its end
// points as SVG defines it (endpoint to centre parameterisation, F.6.5)
// and reports the points on it.
function arcPoints([x1, y1], r, large, sweep, [x2, y2]) {
  const mx = (x1 - x2) / 2, my = (y1 - y2) / 2;
  r = Math.max(r, Math.hypot(mx, my));
  const f = (large === sweep ? -1 : 1) * Math.sqrt(Math.max(0, (r * r - mx * mx - my * my) / (mx * mx + my * my)));
  const cx = f * my + (x1 + x2) / 2, cy = -f * mx + (y1 + y2) / 2;
  let a1 = Math.atan2(y1 - cy, x1 - cx), a2 = Math.atan2(y2 - cy, x2 - cx);
  let da = a2 - a1;
  if (sweep && da < 0) da += 2 * Math.PI;
  if (!sweep && da > 0) da -= 2 * Math.PI;
  return Array.from({ length: 65 }, (_, k) => [cx + r * Math.cos(a1 + (da * k) / 64), cy + r * Math.sin(a1 + (da * k) / 64)]);
}

test("a cloud reaches CLOUD_REACH times its radius to its sides, no further", () => {
  for (const count of [1, 40, 1548]) {
    const r = groupRadius(count);
    const nums = cloudPath(r).replace(/[MAZ]/g, " ").trim().split(/[ ,]+/).map(Number);
    let at = [nums[0], nums[1]];
    const pts = [at];
    for (let i = 2; i + 6 < nums.length + 1; i += 7) {
      const to = [nums[i + 5], nums[i + 6]];
      pts.push(...arcPoints(at, nums[i], nums[i + 3], nums[i + 4], to));
      at = to;
    }
    const reach = Math.max(...pts.map(([x]) => Math.abs(x)));
    assert.ok(reach <= cloudReach(count) + 0.5, `the cloud of ${count} reaches ${reach.toFixed(1)}, within ${cloudReach(count).toFixed(1)}`);
    assert.ok(reach >= cloudReach(count) - 0.03 * r, `CLOUD_REACH (${CLOUD_REACH}) is no wider than the cloud: ${reach.toFixed(1)}`);
    assert.ok(Math.max(...pts.map(([, y]) => Math.abs(y))) <= r, "upwards and downwards it stays within its radius");
  }
});

test("the clouds of each room form a cluster, rooms in the order of their keys", () => {
  const nodes = [
    { id: "group:OPS/page", kind: "group", space: "OPS" },
    { id: "label:x", kind: "label", name: "x" },
    { id: "group:API/page", kind: "group", space: "API" },
    { id: "group:OPS/task", kind: "group", space: "OPS" },
  ];
  assert.deepEqual(roomClusters(nodes), [[2], [0, 3]]);
  assert.equal(roomClusters(graph.nodes), null, "a map without clouds has no clusters");
});

test("clusters keep apart as wholes, each keeping its own shape", () => {
  // Two rooms of two nodes each, laid into each other.
  const sim = simulation({ x0: [0, 40, 20, 60], y0: [0, 0, 10, 10], radii: [15, 15, 15, 15], clusters: [[0, 1], [2, 3]], clusterGap: 20,
    links: [[0, 1, 40], [2, 3, 40]], center: { x: 30, y: 5 }, charge: 0, gravity: 0.05, alpha: 1 });
  for (let i = 0; i < 1500 && sim.tick(); i++);
  const c = (m) => [(sim.x[m[0]] + sim.x[m[1]]) / 2, (sim.y[m[0]] + sim.y[m[1]]) / 2];
  const extent = (m) => { const [cx, cy] = c(m); return Math.max(...m.map((i) => Math.hypot(sim.x[i] - cx, sim.y[i] - cy) + 15)); };
  const [ax, ay] = c([0, 1]), [bx, by] = c([2, 3]);
  const d = Math.hypot(ax - bx, ay - by);
  assert.ok(d >= extent([0, 1]) + extent([2, 3]) + 20 - 3, `the rooms keep apart: ${d.toFixed(1)} against ${(extent([0, 1]) + extent([2, 3]) + 20).toFixed(1)}`);
  assert.ok(Math.abs(Math.hypot(sim.x[0] - sim.x[1], sim.y[0] - sim.y[1]) - 40) < 4, "a room keeps its shape");
  // Without clusters nothing keeps the rooms apart beyond their nodes.
  const free = simulation({ x0: [0, 40, 20, 60], y0: [0, 0, 10, 10], radii: [15, 15, 15, 15], links: [[0, 1, 40], [2, 3, 40]], center: { x: 30, y: 5 }, charge: 0, gravity: 0.05, alpha: 1 });
  for (let i = 0; i < 1500 && free.tick(); i++);
  const fd = Math.hypot((free.x[0] + free.x[1] - free.x[2] - free.x[3]) / 2, (free.y[0] + free.y[1] - free.y[2] - free.y[3]) / 2);
  assert.ok(fd < d, "the clusters are what keeps them apart");
});

test("the frame takes in a point's reach when it has one", () => {
  const w = 800, h = 560, pad = 56;
  const points = [{ x: 0, y: 0, r: 100 }, { x: 400, y: 300, r: 20 }];
  const { zoom, panX, panY } = fitView(points, w, h, { pad, max: 1.6 });
  for (const p of points) {
    for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1]]) {
      const sx = panX + zoom * (p.x + dx * p.r), sy = panY + zoom * (p.y + dy * p.r);
      assert.ok(sx >= pad - 0.5 && sx <= w - pad + 0.5 && sy >= pad - 0.5 && sy <= h - pad + 0.5, `(${sx.toFixed(1)}, ${sy.toFixed(1)}) lies in the frame`);
    }
  }
  assert.deepEqual(fitView([{ x: 0, y: 0 }, { x: 400, y: 300 }], w, h, { pad }), fitView([{ x: 0, y: 0, r: 0 }, { x: 400, y: 300, r: 0 }], w, h, { pad }), "points without a reach frame as before");
});

test("a cloud keeps its caption at a middle zoom, as an item does", () => {
  assert.equal(captionShown("mid", { kind: "group" }), true, "room, type and count stay readable");
  assert.equal(captionShown("far", { kind: "group" }), false);
  assert.equal(captionShown("mid", { kind: "label" }), false, "labels as before");
  assert.equal(captionShown("mid", { kind: "item" }), true, "items as before");
});

test("an item shows the page tree's symbol: a task's box for a task type, a stack above an item on the map, green when the task is finished", () => {
  const map = {
    nodes: [
      { id: "p", kind: "item", type: "page" },
      { id: "t1", kind: "item", type: "task", state: "open" },
      { id: "t2", kind: "item", type: "task", state: "done" },
      { id: "d", kind: "item", type: "decision", state: "accepted" },
      { id: "x", kind: "item", type: "task", state: "done", outside: true },
      { id: "picture:p/a", kind: "picture" },
      { id: "label:x", kind: "label" },
      { id: "g", kind: "group", type: "task", count: 3 },
    ],
    edges: [
      { from: "p", to: "t1", kind: "parent" },
      { from: "t1", to: "t2", kind: "parent" },
      { from: "d", to: "p", kind: "relation" },
      { from: "p", to: "picture:p/a", kind: "picture" },
    ],
  };
  const taskTypes = new Set(["task"]);
  const finished = (type, state) => state === (type === "decision" ? "accepted" : "done");
  const sym = itemSymbols(map, { taskTypes, finished });
  assert.deepEqual(sym.get("p"), { kind: "page", stacked: true, done: false });
  assert.deepEqual(sym.get("t1"), { kind: "task", stacked: true, done: false });
  assert.deepEqual(sym.get("t2"), { kind: "task", stacked: false, done: true });
  assert.deepEqual(sym.get("d"), { kind: "page", stacked: false, done: false }, "a finished item that is no task is not marked done");
  assert.deepEqual(sym.get("x"), { kind: "task", stacked: false, done: true }, "an item of another room shows its symbol too");
  for (const id of ["picture:p/a", "label:x", "g"]) assert.equal(sym.has(id), false, "pictures, labels and clouds keep their shapes");
  assert.deepEqual(itemSymbols(map).get("t2"), { kind: "page", stacked: false, done: false }, "without the task types every item is a page");
});
