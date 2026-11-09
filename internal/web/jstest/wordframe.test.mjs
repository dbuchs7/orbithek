// The parts of the Word template frame that need no browser (ADR-0167).
import { test } from "node:test";
import assert from "node:assert/strict";
import { chromeOf, fieldText, knownClass, pictureSource, points, runStyle, tabSegments } from "../static/assets/wordframe.js";

test("the first sheet carries its own header and footer when the template has a title page", () => {
  const frame = { titlePage: true, header: ["h"], footer: ["f"], firstHeader: ["fh"], firstFooter: [] };
  assert.deepEqual(chromeOf(frame, 0), { header: ["fh"], footer: [] });
  assert.deepEqual(chromeOf(frame, 1), { header: ["h"], footer: ["f"] });
  assert.deepEqual(chromeOf({ header: ["h"], footer: ["f"] }, 0), { header: ["h"], footer: ["f"] });
  assert.deepEqual(chromeOf({}, 3), { header: [], footer: [] });
});

test("a run's formatting becomes checked style values", () => {
  assert.deepEqual(runStyle({ b: true, i: false, u: true, s: true, caps: true, size: 11, color: "#DC0018", fill: "#ffff00", font: "Frutiger LT 55" }), {
    fontWeight: "700", fontStyle: "normal", textDecoration: "underline line-through", textTransform: "uppercase", fontSize: "11pt",
    color: "#DC0018", backgroundColor: "#ffff00", fontFamily: "\"Frutiger LT 55\", system-ui, sans-serif",
  });
  assert.deepEqual(runStyle({ b: false, i: true, u: false, caps: false }), { fontWeight: "400", fontStyle: "italic", textDecoration: "none", textTransform: "none" });
  // What is not a value of its kind is dropped.
  assert.deepEqual(runStyle({ color: "red;background:url(x)", fill: "#12345", font: "x\";}body{", size: 9000 }), {});
  assert.deepEqual(runStyle({}), {});
});

test("tabs split a line, and page fields show the sheet's number", () => {
  const runs = [{ text: "Titel" }, { tab: true }, { text: "Seite " }, { field: "page" }];
  assert.deepEqual(tabSegments(runs), [[{ text: "Titel" }], [{ text: "Seite " }, { field: "page" }]]);
  assert.deepEqual(tabSegments(undefined), [[]]);
  assert.equal(fieldText({ field: "page" }, 2, 5), "2");
  assert.equal(fieldText({ field: "pages" }, 2, 5), "5");
  assert.equal(fieldText({ field: "other" }, 2, 5), "");
});

test("classes, picture addresses and lengths are admitted only in their form", () => {
  assert.equal(knownClass("s12"), "s12");
  assert.equal(knownClass("s1 evil"), "");
  assert.equal(knownClass(undefined), "");
  assert.equal(pictureSource("/api/word-templates/bericht/versions/2/media/logo.png"), "/api/word-templates/bericht/versions/2/media/logo.png");
  assert.equal(pictureSource("https://evil.example/x.png"), "");
  assert.equal(pictureSource("/api/word-templates/bericht/versions/2/media/../../x"), "");
  assert.equal(points(100), "100pt");
  assert.equal(points(0), "");
  assert.equal(points(5000), "");
  assert.equal(points("10"), "");
});
