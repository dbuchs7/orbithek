// Unit tests of how the album compares pictures (fingerprint.js, ADR-0195).
// They run in Node.js without a browser:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { distance, earlierTwins, fingerprintOf, fitWithin, lookAlike, sameShape } from "../static/assets/fingerprint.js";

const vector = JSON.parse(readFileSync(new URL("./fingerprint-vector.json", import.meta.url)));

test("the browser's fingerprint is the server's", () => {
  const rgba = Uint8ClampedArray.from(Buffer.from(vector.rgba, "base64"));
  assert.equal(rgba.length, vector.width * vector.height * 4);
  assert.equal(fingerprintOf(rgba, vector.width, vector.height), vector.fingerprint);
});

// gradient is a w×h picture brightening from left to right, or the other
// way with reverse.
function gradient(w, h, reverse = false) {
  const px = new Uint8ClampedArray(w * h * 4);
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const v = Math.round(((reverse ? w - 1 - x : x) / (w - 1)) * 255);
      px.set([v, v, v, 255], 4 * (y * w + x));
    }
  }
  return px;
}

test("a picture brightening to the right sets no bit, and the reverse every bit", () => {
  assert.equal(fingerprintOf(gradient(90, 80), 90, 80), "0000000000000000");
  assert.equal(fingerprintOf(gradient(90, 80, true), 90, 80), "ffffffffffffffff");
  assert.equal(fingerprintOf(gradient(45, 40, true), 45, 40), "ffffffffffffffff", "the same at another size");
});

test("flat and tiny pictures have no fingerprint", () => {
  const flat = new Uint8ClampedArray(20 * 20 * 4).fill(200);
  assert.equal(fingerprintOf(flat, 20, 20), "");
  assert.equal(fingerprintOf(gradient(8, 40), 8, 40), "");
  assert.equal(fingerprintOf(gradient(40, 7), 40, 7), "");
  const clear = gradient(30, 30);
  for (let i = 3; i < clear.length; i += 4) clear[i] = 0;
  assert.equal(fingerprintOf(clear, 30, 30), "", "a transparent picture counts as black");
});

test("fingerprints are compared bit by bit", () => {
  assert.equal(distance("0000000000000000", "0000000000000000"), 0);
  assert.equal(distance("0000000000000000", "ffffffffffffffff"), 64);
  assert.equal(distance("00000000000000ff", "0000000000000001"), 7);
  assert.equal(distance("", "0000000000000000"), -1);
  assert.equal(distance("ABCDEF0000000000", "abcdef0000000000"), -1);
  assert.equal(distance(undefined, "0000000000000000"), -1);
});

test("pictures look alike when near and of the same proportions", () => {
  const a = { fingerprint: "0000000000000000", width: 4000, height: 3000 };
  assert.ok(lookAlike(a, { fingerprint: "000000000000001f", width: 400, height: 300 }, 5));
  assert.ok(!lookAlike(a, { fingerprint: "000000000000003f", width: 400, height: 300 }, 5));
  assert.ok(!lookAlike(a, { fingerprint: "0000000000000000", width: 1600, height: 900 }, 5));
  assert.ok(!lookAlike(a, { fingerprint: "" }, 5));
  assert.ok(sameShape(0, 0, 3, 4));
  assert.ok(!sameShape(4, 3, 3, 4));
});

test("files chosen twice point at the first", () => {
  const files = [
    { sha256: "a", fingerprint: "0000000000000000", width: 40, height: 30 },
    { sha256: "b", fingerprint: "ffffffffffffffff", width: 40, height: 30 },
    { sha256: "a", fingerprint: "", width: 0, height: 0 },
    { sha256: "c", fingerprint: "0000000000000003", width: 400, height: 300 },
    { sha256: "", fingerprint: "" },
  ];
  assert.deepEqual(earlierTwins(files, 5), [-1, -1, 0, 0, -1]);
});

test("a picture is read at most 256 pixels on its longer edge", () => {
  assert.deepEqual(fitWithin(4000, 3000, 256), { width: 256, height: 192 });
  assert.deepEqual(fitWithin(3000, 4000, 256), { width: 192, height: 256 });
  assert.deepEqual(fitWithin(100, 50, 256), { width: 100, height: 50 });
  assert.deepEqual(fitWithin(10000, 1, 256), { width: 256, height: 1 });
});
