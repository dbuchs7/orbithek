// Unit tests of a slide deck in a text (slidesparts.js,
// ADR-0260): which album filter a deck reads its
// slides by, how the dialog names a folder or collection, which slide a
// key leads to, where a slide's pictures are served, which pictures a
// deck keeps, when its folder's slides differ and how a copy is turned;
// and which deck a presentation shows and where it returns to
// (present-parts.js). They
// run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The block in a page, which needs a browser, is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { choiceLabel, deckChanged, deckFilter, keptSlides, keyStep, slideURLs, turnBetween } from "../static/assets/slidesparts.js";
import { backOf, deckParams, deckSourceOf } from "../static/assets/present-parts.js";

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

const pic = (id, sha, notes = "", versions = []) => ({ attachment: { id, sha256: sha }, first: versions.length ? versions.at(-1).attachment.id : id, notes, editable: true, versions });

test("a deck keeps the very versions it names, in its order, with their stack's notes", () => {
  const old = { attachment: { id: "d1", sha256: "x" } };
  const pictures = [pic("a2", "y", "Say hello", [old]), pic("b", "z", "Then the plan"), pic("other", "w")];
  const kept = keptSlides(pictures, ["b", "d1", "gone"]);
  assert.deepEqual(kept.map((k) => k.attachment.id), ["b", "d1"]);
  assert.equal(kept[1].notes, "Say hello"); // the stack's notes, on its older version
  assert.equal(kept[1].first, "d1");
  assert.equal(kept[0].editable, true);
  assert.deepEqual(keptSlides(pictures, []), []);
});

test("a deck's folder differs when its slides, their pictures or their notes differ", () => {
  const kept = [pic("c1", "x", "A"), pic("c2", "y", "B")];
  assert.equal(deckChanged(kept, [pic("s1", "x", "A"), pic("s2", "y", "B")]), false); // copies: other IDs, same content
  assert.equal(deckChanged(kept, [pic("s1", "x", "A")]), true);
  assert.equal(deckChanged(kept, [pic("s2", "y", "B"), pic("s1", "x", "A")]), true);
  assert.equal(deckChanged(kept, [pic("s1", "x", "A"), pic("s2", "v", "B")]), true);
  assert.equal(deckChanged(kept, [pic("s1", "x", "A"), pic("s2", "y", "B, louder")]), true);
});

test("a copy is turned as its source was turned by hand, unless the orientations mirror", () => {
  assert.equal(turnBetween(1, 6), 1);
  assert.equal(turnBetween(1, 3), 2);
  assert.equal(turnBetween(1, 8), -1);
  assert.equal(turnBetween(6, 1), -1);
  assert.equal(turnBetween(undefined, 1), 0);
  assert.equal(turnBetween(1, 2), 0); // mirrored
});

test("a presentation shows the n-th deck of an item's text, named by the item's ID", () => {
  const id = "0123456789abcdef0123456789abcdef";
  assert.deepEqual(deckSourceOf(new URLSearchParams({ deck: id, n: "2" })), { item: id, n: 2 });
  assert.deepEqual(deckSourceOf(new URLSearchParams({ deck: id })), { item: id, n: 0 });
  assert.equal(deckSourceOf(new URLSearchParams("folder=f")), null);
  for (const bad of [{ deck: "ARCH-1" }, { deck: id, n: "-1" }, { deck: id, n: "1.5" }, { deck: id, n: "x" }]) {
    assert.equal(deckSourceOf(new URLSearchParams(bad)), null, JSON.stringify(bad));
  }
  // The presentation keeps the deck in its address, with the album's filters.
  assert.equal(String(deckParams(new URLSearchParams({ deck: id, n: "1", back: "ARCH-1" }))), `deck=${id}&n=1`);
});
