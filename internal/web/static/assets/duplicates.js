// Duplicates in the album (ADR-0195): before files are added, the browser
// reads each one's SHA-256 and, for a picture, how it looks, asks the
// server which readable pictures are the same file or look the same, and
// asks the person which files to skip; the album's view of duplicates
// shows the groups the server finds. The fingerprint itself is computed in
// fingerprint.js, which runs without a browser.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get, post } from "./api.js";
import { earlierTwins, fingerprintOf, fitWithin } from "./fingerprint.js";

// READ_EDGE is the longer edge a picture is drawn at to read how it looks;
// the grid of 9×8 cells hardly changes below the full size.
const READ_EDGE = 256;
// HASH_BYTES is the largest file whose SHA-256 the browser computes: the
// digest needs the whole file in memory. Larger videos are not checked.
const HASH_BYTES = 256 * 1024 * 1024;
// CHECK_FILES is how many files one request asks about, within the
// server's budget DuplicateChecks.
const CHECK_FILES = 100;

const hex = (buf) => [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, "0")).join("");

// readLook draws a picture small and reads its fingerprint and its size
// as shown, upright; nothing for what this browser cannot decode.
async function readLook(blob) {
  let bitmap;
  try { bitmap = await createImageBitmap(blob); } catch { return {}; }
  try {
    const size = fitWithin(bitmap.width, bitmap.height, READ_EDGE);
    const canvas = typeof OffscreenCanvas === "function" ? new OffscreenCanvas(size.width, size.height) : h("canvas", { width: size.width, height: size.height });
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(bitmap, 0, 0, size.width, size.height);
    const data = ctx.getImageData(0, 0, size.width, size.height).data;
    return { fingerprint: fingerprintOf(data, size.width, size.height), width: bitmap.width, height: bitmap.height };
  } catch {
    return {};
  } finally {
    bitmap.close?.();
  }
}

// readFile reads what the duplicates are found by: the SHA-256 and, for a
// picture, its fingerprint and size; sha256 is "" where it cannot be read.
export async function readFile(blob, video) {
  if (!globalThis.crypto?.subtle || blob.size > HASH_BYTES) return { sha256: "" };
  const sha256 = hex(await crypto.subtle.digest("SHA-256", await blob.arrayBuffer()));
  return video ? { sha256 } : { sha256, ...(await readLook(blob)) };
}

// findDuplicates reads the files queued for upload ({name, read, video})
// and answers those that are duplicates: of readable pictures (matches)
// or of a file chosen before (twin, its index). Where the browser cannot
// compute a digest, nothing is checked.
export async function findDuplicates(queue, progress) {
  if (!globalThis.crypto?.subtle) return [];
  const read = [];
  for (const [n, q] of queue.entries()) {
    progress(t("duplicates.reading", { n: n + 1, total: queue.length, name: q.name }));
    try { read.push(await readFile(await q.read(), q.video)); } catch { read.push({ sha256: "" }); }
  }
  const ask = read.map((r, i) => ({ r, i })).filter((x) => x.r.sha256);
  const matches = new Map();
  let bits = 5;
  for (let at = 0; at < ask.length; at += CHECK_FILES) {
    const part = ask.slice(at, at + CHECK_FILES);
    const files = part.map(({ r }) => ({ sha256: r.sha256, ...(r.fingerprint ? { fingerprint: r.fingerprint, width: r.width || 0, height: r.height || 0 } : {}) }));
    // Pictures stored before fingerprints were kept are compared as the
    // server makes theirs, a share per request.
    let answer, last = Infinity;
    for (;;) {
      answer = await post("/api/album/duplicates", { files });
      if (!answer.pending || answer.pending >= last) break;
      last = answer.pending;
      progress(t("duplicates.comparing", { n: answer.pending }));
    }
    bits = answer.similarBits ?? bits;
    part.forEach(({ i }, k) => { if (answer.matches[k].length) matches.set(i, answer.matches[k]); });
  }
  const twins = earlierTwins(read, bits);
  const found = [];
  queue.forEach((q, i) => {
    if (matches.has(i) || twins[i] >= 0) found.push({ index: i, name: q.name, matches: matches.get(i) || [], twin: twins[i] >= 0 ? queue[twins[i]].name : "" });
  });
  return found;
}

const thumb = (p) => `/api/items/${p.item.id}/attachments/${p.attachment.id}/thumbnail?edge=400`;

// matchText says how a picture found matches and where it is.
export function matchText(m) {
  return t(m.same ? "duplicates.sameIn" : "duplicates.alikeIn", { name: m.attachment.name, folder: m.item.title });
}

// askDuplicates asks which of the duplicates found to skip: every one is
// skipped unless unticked. It resolves to the indexes to skip, or to null
// when the upload is cancelled.
export function askDuplicates(found) {
  return new Promise((resolve) => {
    const boxes = found.map((f) => h("input", { type: "checkbox", checked: true, "data-index": String(f.index), "aria-label": t("duplicates.skipOne", { name: f.name }) }));
    const rows = found.map((f, k) => h("li", { class: "duplicate-row" },
      h("label", { class: "duplicate-choice" }, boxes[k], h("span", { class: "duplicate-name", text: f.name })),
      h("ul", { class: "duplicate-matches" },
        ...f.matches.slice(0, 3).map((m) => h("li", { class: "duplicate-match" },
          m.video && !m.poster ? h("span", { class: "duplicate-video", "aria-hidden": "true" }) : h("img", { src: thumb(m), alt: "", loading: "lazy" }),
          h("span", { class: m.same ? "chip duplicate-same" : "chip duplicate-alike", text: t(m.same ? "duplicates.same" : "duplicates.alike") }),
          h("span", { class: "muted", text: matchText(m) }))),
        f.twin ? h("li", { class: "duplicate-match muted", text: t("duplicates.twin", { name: f.twin }) }) : null)));
    let done = false;
    const finish = (v) => {
      if (done) return;
      done = true;
      dialog.close();
      dialog.remove();
      resolve(v);
    };
    const skipChosen = () => finish(new Set(boxes.filter((b) => b.checked).map((b) => Number(b.dataset.index))));
    const dialog = h("dialog", { class: "album-target album-duplicates", id: "album-duplicates", "aria-label": t("duplicates.title") },
      h("h2", { text: t("duplicates.title") }),
      h("p", { class: "muted", text: t("duplicates.hint", { n: found.length }) }),
      h("ul", { class: "duplicate-list" }, ...rows),
      h("div", { class: "actions-row" },
        h("button", { type: "button", class: "primary", id: "album-duplicates-skip", text: t("duplicates.skip"), onclick: skipChosen }),
        h("button", { type: "button", class: "secondary", id: "album-duplicates-all", text: t("duplicates.addAll"), onclick: () => finish(new Set()) }),
        h("button", { type: "button", class: "link", id: "album-duplicates-cancel", text: t("common.cancel"), onclick: () => finish(null) })));
    dialog.addEventListener("cancel", (e) => { e.preventDefault(); finish(null); });
    document.body.append(dialog);
    dialog.showModal();
    dialog.querySelector("#album-duplicates-skip").focus();
  });
}

// loadDuplicates asks for the groups of duplicates the reader may see,
// again while the server still makes fingerprints of older pictures and
// gets on with them; progress hears how many remain.
export async function loadDuplicates(space, progress, current = () => true) {
  const q = new URLSearchParams();
  if (space) q.set("space", space);
  let answer, last = Infinity;
  for (;;) {
    answer = await get("/api/album/duplicates?" + q);
    if (!current() || !answer.pending || answer.pending >= last) break;
    last = answer.pending;
    progress(answer);
  }
  return answer;
}

// groupTitle says what a group of duplicates holds.
export function groupTitle(g) {
  return t(g.same ? "duplicates.groupSame" : "duplicates.groupAlike", { n: g.pictures.length });
}
