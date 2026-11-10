// Unit tests of presenting pictures of the album (present-parts.js,
// ADR-0176): the order of the slides, the geometry of zooming and of the
// laser pointer, the keys and the messages between the two windows. They
// run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The windows themselves, which need a browser, are in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  MAX_ZOOM, ZOOM_STEP, captionOf, slidesOf, deckParams, step, fit, clampView, placement, toImage, toView,
  zoomAt, panBy, keyAction, clock, channelName, newId, validMessage, apply,
} from "../static/assets/present-parts.js";

const pic = (name, id, extra = {}) => ({ item: { id: "i1", key: "PLAT-1", title: "Talk" }, attachment: { id, name, type: "image/png" }, ...extra });
const near = (a, b, msg) => assert.ok(Math.abs(a - b) < 1e-9, `${msg}: ${a} != ${b}`);

test("a slide's name is its file name without extension or drawing marker", () => {
  assert.equal(captionOf("01 Intro.drawing.png"), "01 Intro");
  assert.equal(captionOf("photo.final.jpg"), "photo.final");
  assert.equal(captionOf("plain"), "plain");
});

test("slides are ordered by name, numbers by their value", () => {
  const slides = slidesOf([
    pic("10 Outlook.png", "a3"), pic("2 Plan.drawing.png", "a2", { drawing: true, notes: "Say it", editable: true }), pic("1 intro.jpg", "a1"),
  ]);
  assert.deepEqual(slides.map((s) => s.name), ["1 intro", "2 Plan", "10 Outlook"]);
  assert.equal(slides[1].notes, "Say it");
  assert.equal(slides[1].editable, true);
  assert.equal(slides[1].drawing, true);
  assert.equal(slides[0].notes, "", "a picture without notes has empty notes");
  assert.equal(slides[0].editable, false);
});

test("slides of the same name keep one order in every window", () => {
  const a = slidesOf([pic("Plan.png", "b"), pic("Plan.png", "a")]);
  const b = slidesOf([pic("Plan.png", "a"), pic("Plan.png", "b")]);
  assert.deepEqual(a.map((s) => s.attachment.id), ["a", "b"]);
  assert.deepEqual(b.map((s) => s.attachment.id), ["a", "b"]);
  assert.deepEqual(slidesOf(undefined), []);
});

test("a deck is chosen by the album's filters only", () => {
  const p = deckParams(new URLSearchParams("folder=F1&q=&kind=drawing&s=abc&at=3&label=talk"));
  assert.equal(p.toString(), "folder=F1&kind=drawing&label=talk");
  assert.equal(deckParams(new URLSearchParams("collection=C1&space=PLAT")).toString(), "collection=C1&space=PLAT", "a collection is presented too");
});

test("stepping stays within the deck", () => {
  assert.equal(step(0, 3, -1), 0);
  assert.equal(step(1, 3, 1), 2);
  assert.equal(step(2, 3, 1), 2);
  assert.equal(step(0, 0, 1), 0);
});

test("an image fits its view whole", () => {
  assert.equal(fit({ w: 800, h: 600 }, { w: 1600, h: 600 }), 0.5);
  assert.equal(fit({ w: 800, h: 600 }, { w: 400, h: 300 }), 2);
  assert.equal(fit({ w: 800, h: 600 }, { w: 0, h: 0 }), 1, "an image not loaded yet");
});

test("unzoomed, an image is centred", () => {
  const p = placement({ w: 800, h: 600 }, { w: 400, h: 400 }, 1, { x: 0.1, y: 0.9 });
  assert.equal(p.scale, 1.5);
  assert.equal(p.left, 100);
  assert.equal(p.top, 0);
});

test("a zoom stays within its bounds, and the image fills the view", () => {
  const view = { w: 800, h: 600 }, image = { w: 800, h: 600 };
  assert.equal(clampView(view, image, 100, null).zoom, MAX_ZOOM);
  assert.equal(clampView(view, image, 0.2, null).zoom, 1);
  assert.equal(clampView(view, image, NaN, null).zoom, 1);
  const v = clampView(view, image, 2, { x: 0, y: 1 });
  assert.deepEqual(v.center, { x: 0.25, y: 0.75 }, "no gap at the edges");
  // A tall image zoomed a little still fits across: it stays centred so.
  const tall = clampView(view, { w: 300, h: 600 }, 2, { x: 0, y: 0 });
  assert.equal(tall.center.x, 0.5);
  assert.equal(tall.center.y, 0.25);
});

test("a point maps to the same spot of the slide in views of any size", () => {
  const image = { w: 1000, h: 500 };
  const big = { w: 1920, h: 1080 }, small = { w: 640, h: 480 };
  const at = toImage(big, image, 1, { x: 0.5, y: 0.5 }, { x: 960, y: 540 });
  near(at.x, 0.5, "x"); near(at.y, 0.5, "y");
  const back = toView(small, image, 1, { x: 0.5, y: 0.5 }, { x: 0.25, y: 0.75 });
  const again = toImage(small, image, 1, { x: 0.5, y: 0.5 }, back);
  near(again.x, 0.25, "x"); near(again.y, 0.75, "y");
  const outside = toImage(big, image, 1, { x: 0.5, y: 0.5 }, { x: 0, y: 0 });
  assert.ok(outside.y < 0, "above the slide");
});

test("zooming about a point keeps it over the same spot", () => {
  const view = { w: 800, h: 600 }, image = { w: 800, h: 600 };
  const point = { x: 600, y: 200 };
  const before = toImage(view, image, 1, { x: 0.5, y: 0.5 }, point);
  const z = zoomAt(view, image, 1, { x: 0.5, y: 0.5 }, point, 2);
  assert.equal(z.zoom, 2);
  const after = toImage(view, image, z.zoom, z.center, point);
  near(after.x, before.x, "x"); near(after.y, before.y, "y");
  const out = zoomAt(view, image, z.zoom, z.center, point, 1 / 4);
  assert.equal(out.zoom, 1);
  assert.deepEqual(out.center, { x: 0.5, y: 0.5 });
});

test("a zoomed slide is moved by dragging, not beyond its edge", () => {
  const view = { w: 800, h: 600 }, image = { w: 800, h: 600 };
  const p = panBy(view, image, 2, { x: 0.5, y: 0.5 }, 100, 0);
  near(p.center.x, 0.5 - 100 / 1600, "dragged right shows more of the left");
  assert.deepEqual(panBy(view, image, 2, { x: 0.5, y: 0.5 }, 10000, -10000).center, { x: 0.25, y: 0.75 });
  assert.deepEqual(panBy(view, image, 1, { x: 0.5, y: 0.5 }, 50, 50).center, { x: 0.5, y: 0.5 }, "unzoomed it does not move");
});

test("the keys of keyboards and presentation remotes", () => {
  for (const k of ["ArrowRight", "PageDown", " ", "n"]) assert.equal(keyAction(k), "next", k);
  for (const k of ["ArrowLeft", "PageUp", "Backspace", "p"]) assert.equal(keyAction(k), "prev", k);
  assert.equal(keyAction("Home"), "first");
  assert.equal(keyAction("End"), "last");
  assert.equal(keyAction("+"), "zoomIn");
  assert.equal(keyAction("-"), "zoomOut");
  assert.equal(keyAction("0"), "zoomReset");
  assert.equal(keyAction("L"), "laser");
  assert.equal(keyAction("b"), "black");
  assert.equal(keyAction("f"), "fullscreen");
  assert.equal(keyAction("Escape"), "escape");
  assert.equal(keyAction("x"), null);
});

test("the time is written as m:ss and h:mm:ss", () => {
  assert.equal(clock(0), "0:00");
  assert.equal(clock(65_400), "1:05");
  assert.equal(clock(3_725_000), "1:02:05");
  assert.equal(clock(-5), "0:00");
});

test("a presentation's channel and id", () => {
  assert.equal(newId([0, 15, 255]), "000fff");
  assert.equal(channelName("ab12"), "ot-present-ab12");
  assert.equal(channelName("a/b<c>"), "ot-present-abc");
});

test("only a presentation's messages count", () => {
  assert.ok(validMessage({ type: "hello" }));
  assert.ok(validMessage({ type: "command", action: "next" }));
  assert.ok(validMessage({ type: "laser", at: null }));
  assert.ok(validMessage({ type: "laser", at: { x: 0.2, y: 0.4 } }));
  assert.ok(validMessage({ type: "state", state: { slides: [], index: 0 } }));
  assert.ok(!validMessage(null));
  assert.ok(!validMessage("next"));
  assert.ok(!validMessage({ type: "other" }));
  assert.ok(!validMessage({ type: "command" }));
  assert.ok(!validMessage({ type: "laser", at: { x: "1", y: 0 } }));
  assert.ok(!validMessage({ type: "state", state: { slides: {}, index: 0 } }));
});

test("commands change the state; a new slide starts unzoomed", () => {
  const view = { w: 800, h: 600 }, image = { w: 800, h: 600 };
  const s0 = { slides: [{}, {}, {}], index: 0, zoom: 1, center: { x: 0.5, y: 0.5 }, laser: false, black: false };
  const zoomed = apply(s0, "zoomIn", view, image);
  assert.equal(zoomed.zoom, ZOOM_STEP);
  const next = apply(zoomed, "next", view, image);
  assert.equal(next.index, 1);
  assert.equal(next.zoom, 1);
  assert.equal(apply(next, "last", view, image).index, 2);
  assert.equal(apply(next, "first", view, image).index, 0);
  assert.equal(apply(s0, "prev", view, image).zoom, 1);
  assert.equal(apply(s0, "laser", view, image).laser, true);
  assert.equal(apply(s0, "black", view, image).black, true);
  assert.equal(apply(zoomed, "zoomReset", view, image).zoom, 1);
  assert.equal(apply(s0, "nonsense", view, image), s0);
  assert.equal(s0.index, 0, "the state given is not changed");
});
