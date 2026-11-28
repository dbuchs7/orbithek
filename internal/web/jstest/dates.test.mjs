// A point in time and a calendar day are written the same in every view
// (dom.js: when, day).
import { test } from "node:test";
import assert from "node:assert/strict";

globalThis.document = { documentElement: { lang: "en-GB" } };
const { day, when } = await import("../static/assets/dom.js");

test("a point in time is written to the minute, without seconds", () => {
  const text = when("2026-11-28T09:05:42Z");
  assert.match(text, /2026/);
  assert.doesNotMatch(text, /:42/);
  assert.equal(when(""), "");
  assert.equal(when("soon"), "soon");
});

test("a calendar day is the same day wherever it is read", () => {
  assert.equal(day("2026-11-30"), "30 Nov 2026");
  assert.equal(day("2026-01-01"), "1 Jan 2026");
  assert.equal(day(""), "");
  assert.equal(day("next week"), "next week");
  assert.doesNotMatch(day("2026-11-28T09:05:42Z"), /:42/);
});

test("the day follows the language of the page", () => {
  document.documentElement.lang = "de";
  assert.equal(day("2026-11-30"), "30.11.2026");
  document.documentElement.lang = "en-GB";
});
