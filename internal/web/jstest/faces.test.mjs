// Unit tests of faces in the album (faceparts.js, ADR-0194). They run in
// Node.js without a browser:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { boxOf, overlap, mergeFaces, facesBody, coverCut, personNames } from "../static/assets/faceparts.js";

test("a box in pixels becomes fractions within the picture", () => {
  assert.deepEqual(boxOf({ x: 100, y: 50, width: 200, height: 100 }, 1000, 500), { x: 0.1, y: 0.1, w: 0.2, h: 0.2 });
  assert.deepEqual(boxOf({ x: -50, y: 450, width: 200, height: 100 }, 1000, 500), { x: 0, y: 0.9, w: 0.15, h: 0.1 }, "cut at the edges");
  assert.equal(boxOf({ x: 10, y: 10, width: 2, height: 2 }, 1000, 1000), null, "too small");
  assert.equal(boxOf({ x: 1, y: 1, width: 5, height: 5 }, 0, 10), null);
  const b = boxOf({ x: 333.3, y: 0, width: 666.7, height: 10 }, 1000, 100);
  assert.ok(b.x + b.w <= 1.0001);
});

test("two boxes overlap by their common share", () => {
  const a = { x: 0, y: 0, w: 0.2, h: 0.2 };
  assert.equal(overlap(a, a), 1);
  assert.equal(overlap(a, { x: 0.5, y: 0.5, w: 0.1, h: 0.1 }), 0);
  assert.ok(Math.abs(overlap(a, { x: 0.1, y: 0, w: 0.2, h: 0.2 }) - 1 / 3) < 1e-9);
});

test("faces found again keep their names, and no name is dropped", () => {
  const known = [{ x: 0.1, y: 0.1, w: 0.2, h: 0.2, person: "Anna" }, { x: 0.6, y: 0.1, w: 0.2, h: 0.2 }, { x: 0.4, y: 0.6, w: 0.1, h: 0.1, person: "Ben" }];
  const found = [{ x: 0.11, y: 0.1, w: 0.2, h: 0.21, vector: [1] }, { x: 0.6, y: 0.1, w: 0.2, h: 0.2 }];
  const merged = mergeFaces(known, found);
  assert.equal(merged.length, 3);
  assert.equal(merged[0].person, "Anna");
  assert.deepEqual(merged[0].vector, [1]);
  assert.equal(merged[1].person, undefined);
  assert.equal(merged[2].person, "Ben", "a named face nothing was found at stays");
  assert.deepEqual(mergeFaces(undefined, undefined), []);
});

test("faces are sent with a vector only where there is one", () => {
  assert.deepEqual(facesBody([{ x: 0.1, y: 0.1, w: 0.2, h: 0.2, person: "Anna", vector: new Float32Array([0.5]) }, { x: 0.5, y: 0.5, w: 0.1, h: 0.1, vector: [] }]),
    { faces: [{ x: 0.1, y: 0.1, w: 0.2, h: 0.2, person: "Anna", vector: [0.5] }, { x: 0.5, y: 0.5, w: 0.1, h: 0.1 }] });
});

test("a cover is cut around the face", () => {
  const c = coverCut({ x: 0.4, y: 0.4, w: 0.2, h: 0.2 }, 1000, 1000, 100, 2);
  assert.equal(c.scale, 0.25);
  assert.equal(c.left, 50 - 500 * 0.25);
  assert.equal(c.top, 50 - 500 * 0.25);
});

test("the persons of faces are named once", () => {
  assert.deepEqual(personNames([{ person: "Anna" }, { person: "anna" }, {}, { person: "Ben" }]), ["Anna", "Ben"]);
});
