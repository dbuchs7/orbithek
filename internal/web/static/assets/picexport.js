// Exporting a picture of the album (ADR-0173): as PNG or JPEG, and a
// drawing also as SVG, drawn from its scene. Whoever may read a picture may
// export it: its bytes come through the same download as the picture shown
// in the album, with the reader's rights, and are converted in this
// browser; nothing new is asked of the server.
//
// The decisions — which formats a picture offers, what a file is called,
// whether the stored file is handed out as it is — are plain functions
// tested in Node (jstest/picexport.test.mjs); the conversions need a
// browser and are in e2e/smoke.cjs.

// FORMATS are the formats a picture is exported in, with their media type
// and file extension. SVG is offered for drawings only: a photograph wrapped
// in SVG would be no drawing in lines and shapes, only a bigger file.
export const FORMATS = {
  png: { type: "image/png", ext: ".png" },
  jpeg: { type: "image/jpeg", ext: ".jpg" },
  svg: { type: "image/svg+xml", ext: ".svg" },
};

// JPEG_QUALITY is the quality a picture is written as JPEG with: high
// enough that lines and text stay clean.
export const JPEG_QUALITY = 0.92;

// JPEG_BACKGROUND fills what is transparent, as JPEG has no transparency;
// pictures show on white in pages too.
export const JPEG_BACKGROUND = "#ffffff";

const DRAWING = ".drawing.png";

// isDrawingName tells a drawing by its file name (ADR-0053).
const isDrawingName = (name) => typeof name === "string" && name.endsWith(DRAWING);

// exportFormats are the formats a picture of the album offers, in the
// order of the menu.
export function exportFormats(p) {
  const drawing = Boolean(p?.drawing) || isDrawingName(p?.attachment?.name);
  return drawing ? ["png", "jpeg", "svg"] : ["png", "jpeg"];
}

// exportName is the name of the exported file: the picture's name without
// its extension (and without a drawing's marker), with that of the format.
// Characters file systems refuse become a dash; an empty name becomes
// "picture".
export function exportName(name, format) {
  const f = FORMATS[format];
  if (!f) throw new Error(`unknown format ${format}`);
  let base = String(name ?? "");
  base = isDrawingName(base) ? base.slice(0, -DRAWING.length) : base.replace(/\.[^./\\]*$/, "");
  base = base.replace(/[\\/:*?"<>|\u0000-\u001f]+/g, "-").replace(/^[\s.]+|[\s.]+$/g, "").slice(0, 120);
  return (base || "picture") + f.ext;
}

// exportPlan says how a picture becomes a file of format: "original" hands
// out the stored file as it is, since it is already in that format;
// "convert" draws it on a canvas and writes it anew; "scene" draws a
// drawing's scene as SVG, in the drawing page.
export function exportPlan(type, format, drawing = false) {
  if (!FORMATS[format]) throw new Error(`unknown format ${format}`);
  if (format === "svg") {
    if (!drawing) throw new Error("only a drawing exports as SVG");
    return "scene";
  }
  return FORMATS[format].type === type ? "original" : "convert";
}

// The rest needs a browser.

const fileURL = (p) => `/api/items/${encodeURIComponent(p.item.id)}/attachments/${encodeURIComponent(p.attachment.id)}`;

// readPicture reads a picture with the reader's rights, as the album shows it.
async function readPicture(p) {
  const res = await fetch(fileURL(p), { credentials: "same-origin" });
  if (!res.ok) throw new Error(res.statusText || String(res.status));
  return res.blob();
}

// convert draws a picture on a canvas and writes it as type; transparency
// becomes white for JPEG. An animated GIF gives its first frame.
async function convert(blob, type) {
  const bitmap = await createImageBitmap(blob);
  const canvas = document.createElement("canvas");
  canvas.width = bitmap.width;
  canvas.height = bitmap.height;
  const ctx = canvas.getContext("2d");
  if (type === FORMATS.jpeg.type) {
    ctx.fillStyle = JPEG_BACKGROUND;
    ctx.fillRect(0, 0, canvas.width, canvas.height);
  }
  ctx.drawImage(bitmap, 0, 0);
  bitmap.close?.();
  return new Promise((resolve, reject) => canvas.toBlob(
    (b) => (b ? resolve(b) : reject(new Error("the picture could not be written"))),
    type, type === FORMATS.jpeg.type ? JPEG_QUALITY : undefined));
}

// sceneAsSVG has the drawing page draw a drawing's scene as SVG. The page
// runs Excalidraw under its own policy (ADR-0053); a hidden frame loads it,
// is handed the drawing, answers with the SVG and is removed again.
export function sceneAsSVG(scene, h) {
  return new Promise((resolve, reject) => {
    const origin = window.location.origin;
    const frame = h("iframe", { src: "/draw", title: "SVG", class: "export-frame", "aria-hidden": "true", tabindex: "-1" });
    let timer = 0;
    const finish = (fn, value) => {
      clearTimeout(timer);
      window.removeEventListener("message", onMessage);
      frame.remove();
      fn(value);
    };
    const onMessage = (e) => {
      if (e.origin !== origin || e.source !== frame.contentWindow || !e.data) return;
      if (e.data.type === "ready") frame.contentWindow.postMessage({ type: "svg", scene }, origin);
      else if (e.data.type === "svg") finish(resolve, e.data.blob);
      else if (e.data.type === "error") finish(reject, new Error(e.data.message));
    };
    window.addEventListener("message", onMessage);
    // The drawing page loads Excalidraw once; a page that never answers
    // does not leave the export waiting for good.
    timer = setTimeout(() => finish(reject, new Error("the drawing page did not answer")), 60000);
    document.body.append(frame);
  });
}

// save hands a file to the browser's download.
function save(blob, name, h) {
  // A blob address is set directly: h() admits only addresses of pages.
  const a = h("a", { download: name });
  a.href = URL.createObjectURL(blob);
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

// exportPicture exports a picture of the album in format and answers the
// name of the file saved. h is dom.js's, passed in so this module loads in
// Node for its tests.
export async function exportPicture(p, format, h) {
  const drawing = exportFormats(p).includes("svg");
  const plan = exportPlan(p.attachment.type, format, drawing);
  const stored = await readPicture(p);
  let blob = stored;
  if (plan === "convert") blob = await convert(stored, FORMATS[format].type);
  else if (plan === "scene") blob = await sceneAsSVG(stored, h);
  const name = exportName(p.attachment.name, format);
  save(blob, name, h);
  return name;
}
