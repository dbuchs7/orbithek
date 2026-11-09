// Unit tests of the parts of a live whiteboard (whiteboard-parts.js,
// ADR-0169, ADR-0170, ADR-0171): who holds what, what changed, what a
// selection locks, the frames in reading order, the timer, a vote's
// result, the comments by element and where a badge goes. They run in
// Node.js without a browser:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The whiteboard itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import * as P from "../static/assets/whiteboard-parts.js";

const el = (id, extra = {}) => ({ id, type: "rectangle", version: 1, versionNonce: 1, x: 0, y: 0, width: 100, height: 50, ...extra });

test("an element someone else holds is shown locked, and sent with its own lock", () => {
  const held = new Map([["a", "other"], ["b", "me"]]);
  const a = el("a"), b = el("b"), c = el("c", { locked: true });
  assert.equal(P.shown(a, held, "me").locked, true);
  assert.equal(P.shown(b, held, "me"), b);
  assert.equal(P.shown(c, held, "me"), c);
  const truth = new Map([["a", { ...a, locked: false }]]);
  assert.equal(P.sendable({ ...a, locked: true, version: 2 }, truth, held, "me").locked, false);
  assert.equal(P.sendable(b, truth, held, "me"), b);
  assert.equal(P.sendable({ ...el("x"), locked: true }, new Map(), new Map([["x", "o"]]), "me").locked, false);
});

test("what changed, and what replaces what one holds", () => {
  const truth = new Map([["a", el("a", { version: 2 })], ["b", el("b")]]);
  const scene = [el("a", { version: 2 }), el("b", { version: 3 }), el("c")];
  assert.deepEqual(P.changed(scene, truth).map((e) => e.id), ["b", "c"]);
  assert.equal(P.newer(el("a", { version: 3 }), truth.get("a")), true);
  assert.equal(P.newer(el("a", { version: 2, versionNonce: 9 }), truth.get("a")), true);
  assert.equal(P.newer(el("a", { version: 2 }), truth.get("a")), false);
  assert.equal(P.newer(el("z"), undefined), true);
});

test("a selection locks the bound text and a text's container", () => {
  const els = [
    el("box", { boundElements: [{ type: "text", id: "label" }, { type: "arrow", id: "arr" }] }),
    el("label", { type: "text", containerId: "box" }),
    el("t2", { type: "text", containerId: "box2" }),
    el("gone", { isDeleted: true }),
  ];
  assert.deepEqual([...P.lockSet(["box"], els)].sort(), ["box", "label"]);
  assert.deepEqual([...P.lockSet(["t2", "gone", "nope"], els)].sort(), ["box2", "t2"]);
  assert.deepEqual(P.difference(new Set(["a", "b"]), new Set(["b", "c"])), { lock: ["c"], release: ["a"] });
});

test("elements are stacked by their index", () => {
  const out = P.byIndex([el("c", { index: "a2" }), el("a", { index: "a0" }), el("n"), el("b", { index: "a1" }), el("m")]);
  assert.deepEqual(out.map((e) => e.id), ["n", "m", "a", "b", "c"]);
});

test("the text an element shows", () => {
  const els = [
    el("box", { boundElements: [{ type: "text", id: "label" }] }),
    el("label", { type: "text", text: "wrapped\ntext", originalText: "wrapped text" }),
    el("f", { type: "frame", name: "Ideas" }),
    el("plain", { type: "text", text: "Hi" }),
    el("dead", { boundElements: [{ type: "text", id: "deadlabel" }] }),
    el("deadlabel", { type: "text", text: "x", isDeleted: true }),
  ];
  const byId = new Map(els.map((e) => [e.id, e]));
  assert.equal(P.textOf(byId.get("box"), byId), "wrapped text");
  assert.equal(P.textOf(byId.get("f"), byId), "Ideas");
  assert.equal(P.textOf(byId.get("plain"), byId), "Hi");
  assert.equal(P.textOf(byId.get("dead"), byId), "");
  assert.equal(P.textOf(undefined, byId), "");
  assert.equal(P.textOf(el("f2", { type: "frame" }), byId), "");
});

test("notes, votable elements and colours", () => {
  assert.equal(P.isNote(el("n", { customData: { wikiwikibangbang: { kind: "note" } } })), true);
  assert.equal(P.isNote(el("n")), false);
  assert.equal(P.votable(el("n")), true);
  assert.equal(P.votable(el("a", { type: "arrow" })), false);
  assert.equal(P.votable(el("d", { isDeleted: true })), false);
  assert.equal(P.colourOf(21), P.PEOPLE_COLOURS[1]);
  assert.equal(P.colourOf(undefined), P.PEOPLE_COLOURS[0]);
  assert.equal(Object.keys(P.NOTE_COLOURS).length, 6);
});

test("frames in reading order, with what lies in them", () => {
  const els = [
    el("f2", { type: "frame", x: 900, y: 0, height: 500 }),
    el("f1", { type: "frame", x: 0, y: 10, height: 500 }),
    el("f3", { type: "frame", x: 0, y: 800, height: 500 }),
    el("gone", { type: "frame", x: 0, y: 0, isDeleted: true }),
    el("inside", { frameId: "f1" }),
  ];
  assert.deepEqual(P.frames(els).map((f) => f.id), ["f1", "f2", "f3"]);
  assert.deepEqual(P.inFrame(els[1], els).map((e) => e.id), ["f1", "inside"]);
});

test("the timer counts from the server's clock", () => {
  assert.equal(P.timeLeft(null, 0), null);
  assert.equal(P.timeLeft({ running: false }, 0), null);
  assert.equal(P.timeLeft({ running: true, ends: 10_000 }, 4_000, 1_000), 5_000);
  assert.equal(P.timeLeft({ running: true, ends: 10_000 }, 20_000), 0);
  assert.equal(P.timeLeft({ running: true, paused: true, left: 7_000 }, 0), 7_000);
  assert.equal(P.clock(0), "0:00");
  assert.equal(P.clock(59_001), "1:00");
  assert.equal(P.clock(61_000), "1:01");
  assert.equal(P.clock(3_725_000), "1:02:05");
});

test("a vote's result, most votes first", () => {
  const els = [
    el("a", { boundElements: [{ type: "text", id: "ta" }] }), el("ta", { type: "text", text: "Alpha" }),
    el("b", { boundElements: [{ type: "text", id: "tb" }] }), el("tb", { type: "text", text: "Beta" }),
    el("c"),
  ];
  assert.deepEqual(P.ranking({ a: 2, b: 5, c: 2, gone: 9, zero: 0 }, els), [
    { id: "b", votes: 5, text: "Beta" }, { id: "c", votes: 2, text: "" }, { id: "a", votes: 2, text: "Alpha" },
  ]);
  assert.deepEqual(P.ranking(undefined, els), []);
});

test("open threads by element of one drawing", () => {
  const comments = [
    { id: "1", anchor: { drawing: "d", element: "x" } },
    { id: "2", anchor: { drawing: "d", element: "x" } },
    { id: "3", anchor: { drawing: "d", element: "y" }, resolved: true },
    { id: "4", anchor: { drawing: "other", element: "x" } },
    { id: "5", reply: "1" },
    { id: "6", anchor: { quote: "passage" } },
    { id: "7", anchor: { drawing: "d", element: "z" }, deleted: true },
  ];
  const by = P.threadsByElement(comments, "d");
  assert.deepEqual([...by.keys()], ["x"]);
  assert.equal(by.get("x").length, 2);
  assert.equal(P.threadsByElement(undefined, "d").size, 0);
});

test("scene points on the screen, and following a view", () => {
  const s = { scrollX: 10, scrollY: -20, zoom: { value: 2 }, width: 800, height: 600 };
  assert.deepEqual(P.onScreen(100, 100, s), { left: 220, top: 160 });
  assert.deepEqual(P.onScreen(5, 5, {}), { left: 5, top: 5 });
  const v = P.viewOf(s);
  assert.deepEqual(v, { x: 190, y: 170, zoom: 2 });
  assert.deepEqual(P.scrollFor(v, s), { scrollX: 10, scrollY: -20 });
  assert.deepEqual(P.viewOf({ scrollX: 0, scrollY: 0, width: 100, height: 100 }), { x: 50, y: 50, zoom: 1 });
});

test("throttle runs the last call once per interval", () => {
  const pending = [];
  const timers = { setTimeout: (fn) => { pending.push(fn); return pending.length; }, clearTimeout() {} };
  const calls = [];
  const f = P.throttle((x) => calls.push(x), 100, timers);
  f(1);
  f(2);
  f(3);
  assert.equal(pending.length, 1);
  pending.shift()();
  assert.deepEqual(calls, [3]);
  f(4);
  pending.shift()();
  assert.deepEqual(calls, [3, 4]);
});
