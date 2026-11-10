// Unit tests of reading a download's file name (filename.js, ADR-0180),
// with which the drawing editor keeps a renamed drawing's name. They run in
// Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { filenameOf } from "../static/assets/filename.js";

test("a plain, a quoted and an encoded file name", () => {
  assert.equal(filenameOf("inline; filename=plan.drawing.png"), "plan.drawing.png");
  assert.equal(filenameOf('inline; filename="Floor plan.drawing.png"'), "Floor plan.drawing.png");
  assert.equal(filenameOf('inline; filename="say \\"hi\\".png"'), 'say "hi".png');
  assert.equal(filenameOf("inline; filename*=utf-8''Gr%C3%BCndriss%20K%C3%BCche.drawing.png"), "Gründriss Küche.drawing.png", "as the server writes a name beyond ASCII");
});

test("the encoded name wins over the plain one", () => {
  assert.equal(filenameOf("attachment; filename=\"x.png\"; filename*=UTF-8''%C3%A4.png"), "ä.png");
});

test("no name, no header, a broken encoding", () => {
  assert.equal(filenameOf("inline"), "");
  assert.equal(filenameOf(null), "");
  assert.equal(filenameOf("inline; filename*=utf-8''%E0%A4%A; filename=\"safe.png\""), "safe.png");
});
