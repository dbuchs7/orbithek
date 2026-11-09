// Unit tests of exporting a picture of the album (picexport.js, ADR-0173):
// which formats a picture offers, what the file is called, and whether the
// stored file is handed out as it is, converted, or drawn from its scene.
// They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The conversions themselves, which need a browser, are in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { FORMATS, JPEG_QUALITY, JPEG_BACKGROUND, exportFormats, exportName, exportPlan } from "../static/assets/picexport.js";

const picture = (name, type, drawing = false) => ({ item: { id: "i1" }, attachment: { id: "a1", name, type }, drawing });

test("every picture offers PNG and JPEG; only a drawing offers SVG", () => {
  assert.deepEqual(exportFormats(picture("photo.jpg", "image/jpeg")), ["png", "jpeg"]);
  assert.deepEqual(exportFormats(picture("scan.webp", "image/webp")), ["png", "jpeg"]);
  assert.deepEqual(exportFormats(picture("anim.gif", "image/gif")), ["png", "jpeg"]);
  assert.deepEqual(exportFormats(picture("Plan.drawing.png", "image/png", true)), ["png", "jpeg", "svg"]);
  assert.deepEqual(exportFormats(picture("Plan.drawing.png", "image/png")), ["png", "jpeg", "svg"], "the file name marks a drawing too");
  assert.deepEqual(exportFormats(picture("plan.png", "image/png")), ["png", "jpeg"], "a PNG is no drawing by its type");
  assert.deepEqual(exportFormats(undefined), ["png", "jpeg"]);
});

test("each format has its media type and extension", () => {
  assert.deepEqual(FORMATS.png, { type: "image/png", ext: ".png" });
  assert.deepEqual(FORMATS.jpeg, { type: "image/jpeg", ext: ".jpg" });
  assert.deepEqual(FORMATS.svg, { type: "image/svg+xml", ext: ".svg" });
  assert.ok(JPEG_QUALITY > 0.8 && JPEG_QUALITY <= 1);
  assert.equal(JPEG_BACKGROUND, "#ffffff");
});

test("the file is named after the picture, with the format's extension", () => {
  assert.equal(exportName("Plan.drawing.png", "svg"), "Plan.svg", "a drawing's marker goes");
  assert.equal(exportName("Plan.drawing.png", "png"), "Plan.png");
  assert.equal(exportName("Plan.drawing.png", "jpeg"), "Plan.jpg");
  assert.equal(exportName("holiday.photo.jpeg", "png"), "holiday.photo.png", "only the last extension goes");
  assert.equal(exportName("Logo.webp", "jpeg"), "Logo.jpg");
  assert.equal(exportName("README", "png"), "README.png");
  assert.equal(exportName("Ablauf: Teil 1/2?.png", "png"), "Ablauf- Teil 1-2-.png", "characters file systems refuse become a dash");
  assert.equal(exportName(" .hidden. ", "png"), "hidden.png", "no leading dot or space, no trailing one");
  assert.equal(exportName(".png", "jpeg"), "picture.jpg");
  assert.equal(exportName("", "png"), "picture.png");
  assert.equal(exportName(undefined, "svg"), "picture.svg");
  assert.equal(exportName("x".repeat(300) + ".png", "png"), "x".repeat(120) + ".png", "a name is at most 120 characters before its extension");
  assert.throws(() => exportName("a.png", "bmp"), /unknown format/);
});

test("a file already in the format is handed out as stored, others are converted", () => {
  assert.equal(exportPlan("image/png", "png"), "original");
  assert.equal(exportPlan("image/jpeg", "jpeg"), "original");
  assert.equal(exportPlan("image/png", "jpeg"), "convert");
  assert.equal(exportPlan("image/jpeg", "png"), "convert");
  assert.equal(exportPlan("image/gif", "png"), "convert");
  assert.equal(exportPlan("image/webp", "jpeg"), "convert");
  assert.equal(exportPlan("image/png", "png", true), "original", "a drawing's PNG keeps its scene");
  assert.equal(exportPlan("image/png", "jpeg", true), "convert");
});

test("SVG is drawn from a drawing's scene and refused for any other picture", () => {
  assert.equal(exportPlan("image/png", "svg", true), "scene");
  assert.throws(() => exportPlan("image/png", "svg", false), /only a drawing/);
  assert.throws(() => exportPlan("image/jpeg", "svg"), /only a drawing/);
  assert.throws(() => exportPlan("image/png", "tiff", true), /unknown format/);
});
