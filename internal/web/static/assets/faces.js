// Faces in the album (ADR-0194). The browser finds faces on a picture
// with the vendored face-api (TensorFlow.js, run on the graphics card or
// the processor, nothing fetched from elsewhere) and, where administrators
// turned recognition on, computes a vector of each face to recognise it.
// Whoever may edit a picture names its faces; every reader sees the names.
// The server keeps the boxes and names with the picture and the vectors
// apart, and suggests names from the faces one may read.
//
// The decisions that need no browser are in faceparts.js, tested in Node
// (jstest/faces.test.mjs).
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get, put } from "./api.js";
import { boxOf, mergeFaces, facesBody, coverCut } from "./faceparts.js";

const MODELS = "/vendor/face-models";
const fileURL = (p) => `/api/items/${p.item.id}/attachments/${p.attachment.id}`;
const thumbURL = (p, edge) => `${fileURL(p)}/thumbnail?edge=${edge}`;

// recognition says whether administrators turned recognition on, asked
// once per page.
let recognitionAsked = null;
export const recognition = () => (recognitionAsked ||= get("/api/album/imports").then((r) => !!r.faces).catch(() => false));

// The library and its networks are loaded once, when a face is first
// looked for: about 1.5 MB to find faces, 6.5 MB more to recognise them.
let library = null;
const load = () => (library ||= (async () => {
  const f = await import("/vendor/face-api.js");
  // The graphics card where the browser offers it, else the processor.
  // The WebAssembly backend's files are not shipped, so it is never tried:
  // asking which backend runs before one is set would try it.
  try { f.tf.removeBackend("wasm"); } catch { /* not registered */ }
  const gpu = await f.tf.setBackend("webgl").catch(() => false);
  if (!gpu) await f.tf.setBackend("cpu");
  await f.tf.ready();
  await f.nets.tinyFaceDetector.loadFromUri(MODELS);
  return f;
})().catch((err) => { library = null; throw err; }));
let recognising = null;
const loadRecognition = (f) => (recognising ||= Promise.all([f.nets.faceLandmark68TinyNet.loadFromUri(MODELS), f.nets.faceRecognitionNet.loadFromUri(MODELS)])
  .catch((err) => { recognising = null; throw err; }));

// findFaces finds the faces on a loaded image: boxes as fractions of it
// and, with recognise, a vector of each.
export async function findFaces(image, recognise) {
  const f = await load();
  const w = image.naturalWidth || image.width, hgt = image.naturalHeight || image.height;
  const options = new f.TinyFaceDetectorOptions({ inputSize: 512, scoreThreshold: 0.5 });
  let found;
  if (recognise) {
    await loadRecognition(f);
    found = await f.detectAllFaces(image, options).withFaceLandmarks(true).withFaceDescriptors();
  } else found = await f.detectAllFaces(image, options);
  const out = [];
  for (const d of found) {
    const box = boxOf((d.detection || d).box, w, hgt);
    if (box) out.push({ ...box, ...(d.descriptor ? { vector: Array.from(d.descriptor) } : {}) });
  }
  return out;
}

// loadImage loads a picture of this site as an image to look at.
const loadImage = (src) => new Promise((resolve, reject) => {
  const img = new Image();
  img.decoding = "async";
  img.onload = () => resolve(img);
  img.onerror = () => reject(new Error(t("faces.unreadable")));
  img.src = src;
});

// scan looks for faces on a picture, keeps the names it had, and stores
// what it found. It answers the picture's faces as stored, or null when
// keep, asked once the search is done, says the picture is no longer
// wanted: the viewer moved on, and the picture may be gone.
export async function scan(p, image, keep = () => true) {
  const recognise = await recognition();
  const found = await findFaces(image || await loadImage(thumbURL(p, 1600)), recognise);
  if (!keep()) return null;
  const faces = mergeFaces(p.attachment.faces, found);
  const r = await put(`${fileURL(p)}/faces`, facesBody(faces));
  p.attachment.faces = r.faces || [];
  p.attachment.facesSeen = true;
  return p.attachment.faces;
}

// scanFile looks for faces on a picture just uploaded, read from the file
// in this browser rather than from the server, and stores what it found
// (ADR-0194). A file the browser cannot draw is left unsearched.
export async function scanFile(p, blob) {
  const bitmap = await createImageBitmap(blob);
  const scale = Math.min(1, 1600 / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, Math.round(bitmap.width * scale));
  canvas.height = Math.max(1, Math.round(bitmap.height * scale));
  canvas.getContext("2d").drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close?.();
  return scan(p, canvas);
}

// scanAll looks for faces on every picture of list one may edit that was
// not searched yet, one after another; progress is told each step and
// cancelled is asked before each. It answers how many were searched and
// how many faces were found.
export async function scanAll(list, progress = () => {}, cancelled = () => false) {
  const todo = list.filter((p) => p.editable && !p.attachment.facesSeen && !p.video && /^image\//.test(p.attachment.type || ""));
  let done = 0, faces = 0;
  for (const p of todo) {
    if (cancelled()) break;
    progress(t("faces.scanning", { n: done + 1, total: todo.length }));
    try { faces += (await scan(p) || []).length; done++; } catch { /* the next picture */ }
  }
  return { done, faces, total: todo.length };
}

// knownPeople are the names of the persons one may read, for the
// suggestions of the name field, asked once per page and kept current.
let people = null;
export const knownPeople = () => (people ||= get("/api/album/people").then((r) => r.people.map((x) => x.name)).catch(() => []));
const learn = (name) => { if (people && name) people.then((list) => { if (!list.includes(name)) list.push(name); }); };

// facesPanel shows the faces of a picture in the viewer: their boxes over
// the image in layer, and below the facts a row per face with its name.
// Whoever may edit the picture looks for faces, names them, takes names
// from suggestions and removes faces. A picture never searched is
// searched when it is shown to one who may edit it. still says whether
// the picture is still the one shown.
export function facesPanel({ p, image, layer, still, changed = () => {} }) {
  const box = h("div", { class: "photo-faces", id: "photo-faces" });
  let faces = p.attachment.faces || [];
  let suggestions = new Map();
  let busy = false;
  const say = (text, kind = "muted") => box.querySelector(".photo-faces-status")?.replaceChildren(h("span", { class: kind, text }));

  const place = () => {
    layer.replaceChildren();
    if (!image.naturalWidth || image.hidden) return;
    // The layer lies over the image as it is drawn, contained in its box.
    const r = image.getBoundingClientRect(), s = layer.parentElement.getBoundingClientRect();
    const scale = Math.min(r.width / image.naturalWidth, r.height / image.naturalHeight);
    const w = image.naturalWidth * scale, hgt = image.naturalHeight * scale;
    Object.assign(layer.style, { left: `${r.left - s.left + (r.width - w) / 2}px`, top: `${r.top - s.top + (r.height - hgt) / 2}px`, width: `${w}px`, height: `${hgt}px` });
    faces.forEach((f, i) => {
      const mark = h("div", { class: "photo-face" + (f.person ? " named" : ""), "data-index": String(i), title: f.person || t("faces.unnamed") },
        f.person ? h("span", { class: "photo-face-name", text: f.person }) : null);
      Object.assign(mark.style, { left: `${f.x * 100}%`, top: `${f.y * 100}%`, width: `${f.w * 100}%`, height: `${f.h * 100}%` });
      layer.append(mark);
    });
  };

  const save = async (next) => {
    const r = await put(`${fileURL(p)}/faces`, facesBody(next));
    faces = p.attachment.faces = r.faces || [];
    p.attachment.facesSeen = true;
    changed(p);
  };

  const suggest = async () => {
    suggestions = new Map();
    if (!p.editable || !faces.some((f) => !f.person) || !(await recognition())) return;
    try {
      const r = await get(`${fileURL(p)}/faces/suggestions`);
      for (const s of r.suggestions || []) suggestions.set(s.index, s.person);
    } catch { /* none */ }
  };

  const render = () => {
    if (!still()) return;
    place();
    const rows = faces.map((f, i) => {
      if (!p.editable) return f.person ? h("li", { class: "photo-face-row", text: f.person }) : null;
      const input = h("input", { class: "photo-face-input", id: `photo-face-${i}`, list: "photo-face-people", value: f.person || "",
        placeholder: t("faces.namePlaceholder"), "aria-label": t("faces.nameOf", { n: i + 1 }) });
      const commit = async (name) => {
        name = name.trim();
        if (name === (f.person || "")) return;
        try {
          await save(faces.map((g, j) => (j === i ? { ...g, person: name } : g)));
          learn(name);
          await suggest();
          render();
        } catch (err) { say(err.message, "error"); }
      };
      input.addEventListener("change", () => commit(input.value));
      input.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); input.blur(); } });
      input.addEventListener("focus", () => layer.querySelector(`[data-index="${i}"]`)?.classList.add("focus"));
      input.addEventListener("blur", () => layer.querySelector(`[data-index="${i}"]`)?.classList.remove("focus"));
      const guess = !f.person && suggestions.get(i);
      return h("li", { class: "photo-face-row" }, input,
        guess ? h("button", { type: "button", class: "secondary photo-face-suggest", id: `photo-face-suggest-${i}`, text: t("faces.suggestion", { name: guess }),
          title: t("faces.suggestionTitle"), onclick: () => commit(guess) }) : null,
        h("button", { type: "button", class: "link danger photo-face-remove", text: t("faces.remove"), "aria-label": t("faces.removeOf", { n: i + 1 }), onclick: async () => {
          try { await save(faces.filter((_, j) => j !== i)); await suggest(); render(); } catch (err) { say(err.message, "error"); }
        } }));
    }).filter(Boolean);
    box.replaceChildren(
      h("h3", { class: "photo-faces-title", text: t("faces.title") }),
      rows.length ? h("ul", { class: "photo-face-list" }, ...rows) : h("p", { class: "muted", text: p.attachment.facesSeen ? t("faces.none") : t("faces.notSearched") }),
      p.editable ? h("div", { class: "actions-row" },
        h("button", { type: "button", class: "secondary", id: "photo-faces-find", disabled: busy, text: t("faces.find"), onclick: () => find() })) : null,
      h("p", { class: "photo-faces-status", role: "status" }),
      p.editable ? h("datalist", { id: "photo-face-people" }) : null);
    if (p.editable) knownPeople().then((list) => box.querySelector("#photo-face-people")?.replaceChildren(...list.map((n) => h("option", { value: n }))));
  };

  const find = async () => {
    if (busy) return;
    busy = true;
    render();
    say(t("faces.looking"));
    try {
      if (!image.complete || !image.naturalWidth) await new Promise((r) => image.addEventListener("load", r, { once: true }));
      // A search finished after the viewer moved on stores nothing.
      if (!(await scan(p, image, still))) { busy = false; return; }
      faces = p.attachment.faces;
      changed(p);
      await suggest();
      busy = false;
      render();
      say(faces.length ? t("faces.found", { n: faces.length }) : t("faces.none"));
    } catch (err) {
      busy = false;
      render();
      say(t("faces.failed", { reason: err.message }), "error");
    }
  };

  image.addEventListener("load", place);
  const resize = "ResizeObserver" in window ? new ResizeObserver(place) : null;
  resize?.observe(layer.parentElement);
  render();
  suggest().then(render);
  if (p.editable && !p.attachment.facesSeen && !p.video) find();
  return { element: box, stop: () => { resize?.disconnect(); image.removeEventListener("load", place); layer.replaceChildren(); } };
}

// personCover is a square picture of a person's face, cut from the
// thumbnail of their cover picture.
export function personCover(person, size = 96) {
  const frame = h("span", { class: "album-person-face", "aria-hidden": "true" });
  const c = person.cover;
  if (c.video && !c.poster) return frame;
  const img = h("img", { alt: "", loading: "lazy", draggable: "false" });
  img.addEventListener("load", () => {
    const cut = coverCut(person.face, img.naturalWidth, img.naturalHeight, size);
    Object.assign(img.style, { width: `${img.naturalWidth * cut.scale}px`, height: `${img.naturalHeight * cut.scale}px`, left: `${cut.left}px`, top: `${cut.top}px` });
    frame.classList.add("ready");
  });
  img.src = thumbURL(c, 400);
  frame.append(img);
  return frame;
}
