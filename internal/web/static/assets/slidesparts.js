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

// keptSlides picks, from the album's listing of the pictures of the item a
// deck is on (stacks of versions, ADR-0103), the files the deck keeps, in
// its order: the very version each names, with its stack's notes and
// whether one may edit them. A file the listing lacks — deleted, or not
// the reader's to see — is left out.
export function keptSlides(pictures, files) {
  const byId = new Map();
  for (const top of pictures || []) {
    for (const p of [top, ...(top.versions || [])]) {
      byId.set(p.attachment.id, { ...p, notes: top.notes || "", editable: !!top.editable, first: top.first || top.attachment.id, versions: undefined });
    }
  }
  return (files || []).map((id) => byId.get(id)).filter(Boolean);
}

// deckChanged tells whether the slides of a deck's source differ from
// those the deck keeps: another number of slides, another picture at a
// place, or other notes. A kept slide is its source's picture when it is
// a copy of that very file (source), or holds the same content: a picture
// turned or timed by hand, or with its place kept back, is given out
// rewritten, so its copy never has its source's content. Both are album
// pictures in the order of the slides.
export function deckChanged(kept, source) {
  if ((kept || []).length !== (source || []).length) return true;
  const same = (k, s) => (!!k.source && k.source === s.attachment.id) || k.attachment.sha256 === s.attachment.sha256;
  return kept.some((k, i) => !same(k, source[i]) || (k.notes || "") !== (source[i].notes || ""));
}

// QUARTERS are the quarter turns clockwise of the orientations of a
// picture that only turn it (EXIF 1, 6, 3 and 8).
const QUARTERS = { 1: 0, 6: 1, 3: 2, 8: 3 };

// turnBetween is the turn a copy needs to show as its source does — 1 a
// quarter clockwise, -1 counter-clockwise, 2 upside down — or 0 when it
// needs none or the orientations also mirror.
export function turnBetween(copy, source) {
  const from = QUARTERS[copy || 1], to = QUARTERS[source || 1];
  if (from === undefined || to === undefined) return 0;
  return [0, 1, 2, -1][(to - from + 4) % 4];
}
