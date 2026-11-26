// The parts of a slide deck in a text (ADR-0260)
// that need no browser: which album filter a deck reads its slides by,
// how a folder or collection is named in the dialog, and which slide a
// key leads to. They are tested in Node (jstest/slides.test.mjs);
// slidesblock.js draws with them.
import { step } from "./present-parts.js";

// MAX_CHOICES bounds the folders and collections the dialog offers, and
// CHOICE_DEPTH how deep below the top it looks for folders; deeper ones
// are reached through the album.
export const MAX_CHOICES = 300;
export const CHOICE_DEPTH = 4;

// deckFilter is the album's filter of the slides of a deck: its folder,
// or its collection when the item it names is one (ADR-0175).
export function deckFilter(target) {
  const q = new URLSearchParams();
  q.set(target?.type === "collection" ? "collection" : "folder", target?.id || "");
  return q;
}

// choiceLabel names a folder or collection in the dialog: its room, then
// the path of folders down to it, a collection marked as one.
export function choiceLabel({ space, path, collection }, collectionWord) {
  const name = `${space} · ${path.join(" / ")}`;
  return collection ? `${name} (${collectionWord})` : name;
}

// keyStep is the slide a key leads to from index among count slides, or
// -1 when the key does not step: arrows, page up and down (the keys of
// presentation remotes), space, Home and End, as in a presentation
// (ADR-0176).
export function keyStep(key, index, count) {
  switch (key) {
    case "ArrowRight": case "ArrowDown": case "PageDown": case " ":
      return step(index, count, 1);
    case "ArrowLeft": case "ArrowUp": case "PageUp":
      return step(index, count, -1);
    case "Home":
      return 0;
    case "End":
      return Math.max(0, count - 1);
  }
  return -1;
}

// slideURLs are the addresses of a slide's picture for the stage and for
// the strip: a picture scaled and turned upright by the server, a video's
// poster; a video without a poster has none for the strip.
export function slideURLs(s) {
  const file = `/api/items/${s.item.id}/attachments/${s.attachment.id}`;
  return { file, stage: `${file}/thumbnail?edge=1600`, thumb: s.video && !s.poster ? "" : `${file}/thumbnail?edge=400` };
}
