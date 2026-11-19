// Unit tests of the page tree's width (treewidth.js): the bounds a width
// is kept within, how a kept width is read, and how far a key moves the
// edge. They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// Dragging the handle in a browser is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  TREE_MIN, TREE_STEP, TREE_BIG_STEP, treeMax, clampWidth, parseWidth, keyWidth,
} from "../static/assets/treewidth.js";

test("the widest tree is half the window, never narrower than the narrowest", () => {
  assert.equal(treeMax(1600), 800);
  assert.equal(treeMax(1001), 500);
  assert.equal(treeMax(300), TREE_MIN);
});

test("a width is kept between the bounds, in whole pixels", () => {
  assert.equal(clampWidth(100, 1600), TREE_MIN);
  assert.equal(clampWidth(412.6, 1600), 413);
  assert.equal(clampWidth(2000, 1600), 800);
});

test("a kept width is a positive number, anything else is none", () => {
  assert.equal(parseWidth("360"), 360);
  assert.equal(parseWidth("360.4"), 360);
  for (const v of [null, undefined, "", "0", "-20", "wide", "NaN", "Infinity"]) {
    assert.equal(parseWidth(v), 0, String(v));
  }
});

test("the arrow keys move the edge by a step, further with Shift", () => {
  assert.equal(keyWidth("ArrowRight", false, 300, 1600), 300 + TREE_STEP);
  assert.equal(keyWidth("ArrowLeft", false, 300, 1600), 300 - TREE_STEP);
  assert.equal(keyWidth("ArrowRight", true, 300, 1600), 300 + TREE_BIG_STEP);
  assert.equal(keyWidth("ArrowLeft", true, 200, 1600), TREE_MIN);
  assert.equal(keyWidth("ArrowRight", true, 790, 1600), 800);
});

test("Home and End go to the bounds; other keys do not move the edge", () => {
  assert.equal(keyWidth("Home", false, 500, 1600), TREE_MIN);
  assert.equal(keyWidth("End", false, 300, 1600), 800);
  assert.equal(keyWidth("Enter", false, 300, 1600), null);
  assert.equal(keyWidth("a", true, 300, 1600), null);
});
