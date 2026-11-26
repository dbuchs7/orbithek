// Unit tests of a slide deck in a text (slidesparts.js,
// ADR-draft-a-slide-deck-in-a-text): which album filter a deck reads its
// slides by, how the dialog names a folder or collection, which slide a
// key leads to and where a slide's pictures are served; and where a
// presentation started from a text returns to (present-parts.js). They
// run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The block in a page, which needs a browser, is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { choiceLabel, deckFilter, keyStep, slideURLs } from "../static/assets/slidesparts.js";
import { backOf, deckParams } from "../static/assets/present-parts.js";

test("a deck reads the slides of its folder, or of its collection", () => {
  assert.equal(String(deckFilter({ id: "f1", type: "folder" })), "folder=f1");
  assert.equal(String(deckFilter({ id: "c1", type: "collection" })), "collection=c1");
  // The filter is one the presentation keeps (ADR-0176).
  assert.equal(String(deckParams(deckFilter({ id: "c1", type: "collection" }))), "collection=c1");
});

test("the dialog names a folder by its room and path, a collection as one", () => {
  assert.equal(choiceLabel({ space: "ARCH", path: ["Reviews", "Q4"] }), "ARCH · Reviews / Q4");
  assert.equal(choiceLabel({ space: "ARCH", path: ["Logos"], collection: true }, "collection"), "ARCH · Logos (collection)");
});

test("the keys of a presentation step through a deck and stay within it", () => {
  assert.equal(keyStep("ArrowRight", 0, 3), 1);
  assert.equal(keyStep("PageDown", 2, 3), 2);
  assert.equal(keyStep(" ", 1, 3), 2);
  assert.equal(keyStep("ArrowLeft", 0, 3), 0);
  assert.equal(keyStep("PageUp", 2, 3), 1);
  assert.equal(keyStep("Home", 2, 3), 0);
  assert.equal(keyStep("End", 0, 3), 2);
  assert.equal(keyStep("End", 0, 0), 0);
  assert.equal(keyStep("x", 1, 3), -1);
});

test("a slide is shown scaled by the server, a video by its poster", () => {
  const s = { item: { id: "i" }, attachment: { id: "a" } };
  assert.deepEqual(slideURLs(s), { file: "/api/items/i/attachments/a", stage: "/api/items/i/attachments/a/thumbnail?edge=1600", thumb: "/api/items/i/attachments/a/thumbnail?edge=400" });
  assert.equal(slideURLs({ ...s, video: true, poster: false }).thumb, "");
  assert.equal(slideURLs({ ...s, video: true, poster: true }).thumb, "/api/items/i/attachments/a/thumbnail?edge=400");
});

test("a presentation returns to the text it was started from, nowhere else", () => {
  assert.equal(backOf(new URLSearchParams("folder=f&back=ARCH-1")), "ARCH-1");
  assert.equal(backOf(new URLSearchParams("folder=f")), "");
  for (const bad of ["../admin", "a/b", "x?y", "javascript:alert(1)", "a".repeat(65)]) {
    assert.equal(backOf(new URLSearchParams({ back: bad })), "", bad);
  }
  // The address's back is no filter of the album.
  assert.equal(String(deckParams(new URLSearchParams("folder=f&back=ARCH-1"))), "folder=f");
});
