// The album's photo management in the browser (ADR-0177): the viewer with
// a picture's photo data, favourites, the archive of chosen pictures, and
// the imports — files and ZIP archives exported from Apple Photos, an
// iPhone or any camera, photos chosen in Google Photos, and shared iCloud
// albums. Files are unpacked and uploaded here, one by one, through the
// files of the folder's item; Google and iCloud photos are fetched by the
// server, which asks Google or Apple for them.
import { facesPanel, scanFile } from "./faces.js";
import { h, when } from "./dom.js";
import { t } from "./i18n.js";
import { get, post, put, del, upload, download } from "./api.js";
import { baseName, durationText, exposureText, hidden, isVideo, placeText, posterTime, sorts, typeOf } from "./photoparts.js";
import { entries as zipEntries, read as zipRead } from "./zipread.js";
import { folderName, isDeck, pair, readDeck, slideName, widthOf } from "./pptx.js";
import { pickPlace, placeInput } from "./places.js";

export const fileURL = (p) => `/api/items/${p.item.id}/attachments/${p.attachment.id}`;
export const thumbURL = (p, edge = 400) => `${fileURL(p)}/thumbnail?edge=${edge}`;
// whoAmI is the signed-in person's ID, asked once.
let me = null;
const whoAmI = async () => {
  if (!me) me = get("/api/me").then((r) => r.user?.id || "").catch(() => "");
  return me;
};
const caption = (name) => name.replace(/(\.drawing)?\.[^.]*$/, "");

// setFavourite marks a picture as one's favourite, or unmarks it.
export async function setFavourite(p, on) {
  await put("/api/album/favourites", { item: p.item.id, file: p.attachment.id, favourite: on });
  p.favourite = on;
}

// wallClock writes a photo's time of capture as the camera told it, in
// the reader's language, without moving it into the reader's zone.
export function wallClock(taken) {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})$/.exec(taken || "");
  if (!m) return "";
  const d = new Date(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6]);
  return d.toLocaleString(document.documentElement.lang || undefined, { dateStyle: "full", timeStyle: "short" });
}

// dayText writes a day, "2026-07-14", in the reader's language.
export function dayText(day, style = "full") {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day || "");
  if (!m) return day || "";
  return new Date(+m[1], +m[2] - 1, +m[3]).toLocaleDateString(document.documentElement.lang || undefined, { dateStyle: style });
}

// monthText writes a month, "2026-07", in the reader's language.
export function monthText(month) {
  const m = /^(\d{4})-(\d{2})$/.exec(month || "");
  if (!m) return month || "";
  return new Date(+m[1], +m[2] - 1, 1).toLocaleDateString(document.documentElement.lang || undefined, { month: "long", year: "numeric" });
}

const bytesText = (n) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`);

// openViewer shows the pictures of list one at a time, from index, large,
// with what each tells about itself. The arrow keys and the buttons go to
// the next and the previous; changed is told when a favourite changes.
export function openViewer(list, index, changed = () => {}) {
  let i = index;
  const img = h("img", { class: "photo-viewer-img", alt: "" });
  // A video plays in the viewer, its poster shown until it starts
  // (ADR-0189).
  const video = h("video", { class: "photo-viewer-img photo-viewer-video", controls: true, preload: "metadata", playsinline: true, hidden: true });
  const info = h("div", { class: "photo-viewer-info", "aria-live": "polite" });
  // The faces found on a picture lie over it (ADR-0194).
  const layer = h("div", { class: "photo-faces-layer", "aria-hidden": "true" });
  let faces = null;
  const prev = h("button", { type: "button", class: "photo-viewer-nav prev", "aria-label": t("photos.previous"), text: "‹" });
  const next = h("button", { type: "button", class: "photo-viewer-nav next", "aria-label": t("photos.next"), text: "›" });
  const dialog = h("dialog", { class: "photo-viewer", id: "photo-viewer", "aria-label": t("photos.viewer") },
    h("div", { class: "drawing-bar" },
      h("strong", { class: "photo-viewer-title" }),
      h("span", { class: "muted photo-viewer-pos" }),
      h("button", { type: "button", id: "photo-viewer-close", text: t("common.close"), onclick: () => dialog.close() })),
    h("div", { class: "photo-viewer-body" }, h("div", { class: "photo-viewer-stage" }, prev, img, layer, video, next), info));
  dialog.addEventListener("close", () => { video.pause(); video.removeAttribute("src"); video.load(); faces?.stop(); });
  const row = (label, value) => (value ? h("div", { class: "photo-fact" }, h("dt", { text: label }), h("dd", {}, value)) : null);
  const show = async () => {
    const p = list[i];
    const name = caption(p.attachment.name);
    const playing = isVideo(p);
    img.hidden = playing;
    video.hidden = !playing;
    video.pause();
    if (playing) {
      img.removeAttribute("src");
      if (p.poster) video.poster = thumbURL(p, 1600);
      else video.removeAttribute("poster");
      video.src = fileURL(p);
      video.setAttribute("aria-label", name);
    } else {
      video.removeAttribute("src");
      img.src = thumbURL(p, 1600);
      img.alt = name;
    }
    dialog.querySelector(".photo-viewer-title").textContent = name;
    dialog.querySelector(".photo-viewer-pos").textContent = t("photos.position", { n: i + 1, total: list.length });
    prev.disabled = i === 0;
    next.disabled = i === list.length - 1;
    const fav = h("button", { type: "button", class: "secondary photo-favourite", "aria-pressed": String(!!p.favourite),
      text: p.favourite ? t("photos.unfavourite") : t("photos.favourite") });
    fav.addEventListener("click", async () => {
      try {
        await setFavourite(p, !p.favourite);
        changed(p);
        show();
      } catch (err) { info.append(h("p", { class: "error", role: "alert", text: err.message })); }
    });
    let photo = p.attachment.photo;
    const facts = () => {
      const place = placeText(photo);
      const camera = [photo?.make, photo?.model].filter(Boolean).join(" ");
      return h("dl", { class: "photo-facts" },
        row(t("photos.taken"), photo?.taken ? wallClock(photo.taken) : p.taken?.day ? t("photos.addedOn", { day: dayText(p.taken.day) }) : ""),
        row(t("photos.camera"), camera),
        row(t("photos.lens"), photo?.lens),
        row(t("photos.exposure"), exposureText(photo)),
        row(t("photos.duration"), durationText(photo?.duration)),
        row(t("photos.size"), [photo?.width && `${photo.width} × ${photo.height}`, p.attachment.size && bytesText(p.attachment.size)].filter(Boolean).join(" · ")),
        row(t("photos.place"), place && h("span", {}, photo.placeName ? photo.placeName + " · " : "",
          h("a", { href: `https://www.openstreetmap.org/?mlat=${photo.lat}&mlon=${photo.lon}#map=15/${photo.lat}/${photo.lon}`,
            target: "_blank", rel: "noopener noreferrer", text: place, title: t("photos.placeOpen") }))),
        row(t("photos.labels"), (p.attachment.labels || []).join(", ")),
        row(t("photos.item"), p.item.key ? h("a", { href: "#/item/" + encodeURIComponent(p.item.key), text: `${p.item.key} ${p.item.title || ""}`.trim(), onclick: () => dialog.close() }) : ""),
        row(t("photos.added"), p.attachment.created ? when(p.attachment.created) : ""));
    };
    // The one who added a photo keeps its place to themselves, or shows it
    // to every reader again (ADR-0178).
    const placeRow = h("div", { class: "photo-place-choice" });
    const showPlaceChoice = async () => {
      const mine = (await whoAmI()) === p.attachment.author;
      if (!mine || photo?.lat == null || list[i] !== p) return placeRow.replaceChildren();
      const priv = !!photo.placePrivate;
      const b = h("button", { type: "button", class: "secondary", id: "photo-place-private", "aria-pressed": String(priv),
        text: priv ? t("photos.placeShowAll") : t("photos.placeKeepMine") });
      b.addEventListener("click", async () => {
        try {
          const r = await put(`${fileURL(p)}/place`, { private: !priv });
          photo = r.attachment.photo;
          p.attachment.photo = photo;
          info.firstChild.replaceWith(facts());
          showPlaceChoice();
        } catch (err) { info.append(h("p", { class: "error", role: "alert", text: err.message })); }
      });
      placeRow.replaceChildren(h("p", { class: "muted", text: priv ? t("photos.placeIsMine") : t("photos.placeIsEveryones") }), b);
    };
    // Who may edit the picture sets its place by hand, from suggestions or
    // on a map, or removes it (ADR-0181).
    const placeEdit = h("div", { class: "photo-place-edit" });
    const placed = (r) => {
      photo = r.attachment.photo;
      p.attachment.photo = photo;
      info.firstChild.replaceWith(facts());
      showPlaceEdit();
      showPlaceChoice();
      changed(p);
    };
    const setPlace = async (pl) => {
      try { placed(await put(`${fileURL(p)}/geotag`, { lat: pl.lat, lon: pl.lon, name: pl.name || "" })); } catch (err) { info.append(h("p", { class: "error", role: "alert", text: err.message })); }
    };
    const showPlaceEdit = () => {
      if (!p.editable || list[i] !== p) return placeEdit.replaceChildren();
      const field = placeInput({ value: photo?.placeName || "", label: t("places.label"), id: "photo-place-input", pick: setPlace });
      placeEdit.replaceChildren(h("label", { class: "field", for: "photo-place-input" }, h("span", { text: t("places.label") })), field.element,
        h("div", { class: "actions-row" },
          h("button", { type: "button", class: "secondary", id: "photo-place-map", text: t("places.onMap"), onclick: async () => {
            const pl = await pickPlace({ lat: photo?.lat, lon: photo?.lon, name: photo?.placeName });
            if (pl) setPlace(pl);
          } }),
          photo?.lat != null ? h("button", { type: "button", class: "link danger", id: "photo-place-remove", text: t("places.remove"), onclick: async () => {
            try { placed(await del(`${fileURL(p)}/geotag`)); } catch (err) { info.append(h("p", { class: "error", role: "alert", text: err.message })); }
          } }) : null));
    };
    faces?.stop();
    faces = playing ? null : facesPanel({ p, image: img, layer, still: () => list[i] === p && dialog.open, changed });
    info.replaceChildren(facts(), faces ? faces.element : "", placeRow, placeEdit, h("div", { class: "actions-row" }, fav,
      h("a", { class: "button secondary", href: fileURL(p), download: p.attachment.name, text: t("photos.original") })));
    // A picture stored before photo data was recorded is read now.
    if (!photo && p.item.id) {
      try {
        const r = await get(`${fileURL(p)}/photo`);
        if (list[i] === p && r.photo) {
          photo = r.photo;
          info.firstChild.replaceWith(facts());
        }
      } catch { /* the facts known so far */ }
    }
    showPlaceChoice();
    showPlaceEdit();
  };
  prev.addEventListener("click", () => { if (i > 0) { i--; show(); } });
  next.addEventListener("click", () => { if (i < list.length - 1) { i++; show(); } });
  dialog.addEventListener("keydown", (e) => {
    if (e.target instanceof HTMLInputElement) return;
    if (e.key === "ArrowLeft" && i > 0) { e.preventDefault(); i--; show(); }
    if (e.key === "ArrowRight" && i < list.length - 1) { e.preventDefault(); i++; show(); }
  });
  dialog.addEventListener("close", () => dialog.remove());
  document.body.append(dialog);
  dialog.showModal();
  show();
}

// saveBlob hands a file to the browser's download.
function saveBlob(blob, name) {
  // A blob address is set directly: h() admits only addresses of pages.
  const a = h("a", { download: name });
  a.href = URL.createObjectURL(blob);
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

// zipPictures downloads the chosen pictures as one ZIP archive.
export async function zipPictures(pictures) {
  const { blob, name } = await download("/api/album/zip", { pictures: pictures.map((p) => ({ item: p.item.id, file: p.attachment.id })) });
  saveBlob(blob, name);
}

// allFolders lists every folder one may read, with its path, for moving
// pictures; it walks the folders below the top ones, a few levels deep.
export async function allFolders(depth = 4) {
  const out = [];
  const walk = async (parent, path, level) => {
    const q = parent ? "?parent=" + encodeURIComponent(parent) : "";
    const { folders } = await get("/api/album/folders" + q);
    for (const f of folders) {
      const name = path ? `${path} › ${f.name}` : `${f.space} › ${f.name}`;
      out.push({ ...f, path: name });
      if (level < depth) await walk(f.id, name, level + 1);
    }
  };
  await walk("", "", 1);
  return out;
}

// copyTo copies a picture into a folder with its source, as the album's
// insert does (ADR-0054, ADR-0055); the original stays where it is.
export async function copyTo(p, folder) {
  const res = await fetch(fileURL(p), { credentials: "same-origin" });
  if (!res.ok) throw new Error(res.statusText);
  const file = new File([await res.blob()], p.attachment.name, { type: p.attachment.type });
  await upload(`/api/items/${folder}`, file, { item: p.item.id, attachment: p.attachment.id });
}

// moveTo moves a picture with all its versions into a folder, keeping its
// identity (ADR-0188): its versions, labels, notes, place and favourites
// go with it. A picture one may not change, or that a text shows, stays
// where it is; the error says why.
export async function moveTo(p, folder) {
  if (!p.editable) throw new Error(t("photos.moveNotEditable"));
  if (p.used) throw new Error(t("photos.moveUsed"));
  await post(`${fileURL(p)}/move`, { to: folder });
}

// PLACES are the choices of what an import does with where a photo was
// taken: keep it for every reader, keep it to oneself (ADR-0178), or
// remove it from the file (ADR-0177).
export const PLACES = ["", "private", "strip"];

// importPlace is the choice this browser remembers for imports; with a
// value it is set. An earlier "remove the place" is taken over.
export function importPlace(value) {
  try {
    if (value !== undefined) localStorage.setItem("ot.importPlace", value);
    const kept = localStorage.getItem("ot.importPlace");
    if (kept !== null) return PLACES.includes(kept) ? kept : "";
    return localStorage.getItem("ot.stripPlace") === "on" ? "strip" : "";
  } catch {
    return PLACES.includes(value) ? value : "";
  }
}

// labelUpload gives a picture just uploaded the labels asked, so that a
// collection gathers it (ADR-0187).
const labelUpload = (folder, att, labels) =>
  labels.length ? put(`/api/items/${folder}/attachments/${att.id}/labels`, { labels: [...new Set([...(att.labels || []), ...labels])] }) : null;

// importFiles uploads chosen or dropped files into a folder: pictures as
// they are, ZIP archives unpacked here first. HEIC photos and other files
// are skipped and counted. progress is told each step; place is what the
// server does with where each photo was taken (PLACES); labels are given
// to every picture uploaded.
export async function importFiles(files, folder, progress, place = "", labels = []) {
  const sum = { done: 0, heic: 0, skipped: 0, failed: [] };
  const queue = [];
  for (const f of files) {
    if (/\.zip$/i.test(f.name) || f.type === "application/zip" || f.type === "application/x-zip-compressed") {
      try {
        for (const e of await zipEntries(f)) {
          if (hidden(e.name)) continue;
          const name = baseName(e.name);
          const kind = sorts(name, typeOf(name));
          if (kind === "picture" || kind === "video") queue.push({ name, read: () => zipRead(f, e, typeOf(name)), video: kind === "video" });
          else if (kind === "heic") sum.heic++;
          else sum.skipped++;
        }
      } catch (err) { sum.failed.push(`${f.name}: ${err.message}`); }
      continue;
    }
    const kind = sorts(f.name, f.type);
    if (kind === "picture" || kind === "video") queue.push({ name: f.name, read: async () => f, video: kind === "video" });
    else if (kind === "heic") sum.heic++;
    else sum.skipped++;
  }
  for (const [n, q] of queue.entries()) {
    progress(t("photos.uploading", { n: n + 1, total: queue.length, name: q.name }));
    try {
      const blob = await q.read();
      const extra = { ...(place ? { place } : {}), ...(q.video ? { video: "1" } : {}) };
      const att = await upload(`/api/items/${folder}`, blob instanceof File ? blob : new File([blob], q.name, { type: blob.type }), null, extra);
      // The browser takes a picture of a video as its poster (ADR-0189);
      // a video it cannot play keeps its symbol.
      if (q.video) await setPoster(folder, att);
      await labelUpload(folder, att, labels);
      // Faces are looked for on the file uploaded, here and now; a picture
      // this browser cannot search stays unsearched (ADR-0194).
      if (!q.video && /^image\//.test(att.type || "")) {
        progress(t("faces.scanningUpload", { n: n + 1, total: queue.length, name: q.name }));
        await scanFile({ item: { id: folder }, attachment: att, editable: true }, blob).catch(() => {});
      }
      sum.done++;
    } catch (err) { sum.failed.push(`${q.name}: ${err.message}`); }
  }
  return sum;
}

// importDeck brings a presentation into the album (ADR-0186): when the
// files chosen or dropped hold a .pptx or .ppsx — on its own or in a ZIP
// archive — a folder named after the presentation is made below folder,
// each shown slide becomes a picture in it named by its place and title,
// and the slide's speaker's notes become the picture's notes. The browser
// draws the slides itself (assets/pptxdraw.js); chosen together with the
// pictures its slides were exported as, those are taken instead. It gives null when
// the files hold no presentation, so they are imported as pictures.
export async function importDeck(files, folder, progress, place = "", labels = []) {
  const decks = [];
  const pictures = [];
  let skipped = 0;
  for (const f of files) {
    if (isDeck(f.name)) { decks.push({ name: f.name, read: async () => f }); continue; }
    if (/\.zip$/i.test(f.name) || f.type === "application/zip" || f.type === "application/x-zip-compressed") {
      let list;
      try { list = await zipEntries(f); } catch { skipped++; continue; }
      for (const e of list) {
        if (hidden(e.name)) continue;
        const name = baseName(e.name);
        if (isDeck(name)) decks.push({ name, read: () => zipRead(f, e) });
        else if (sorts(name, typeOf(name)) === "picture") pictures.push({ name, read: () => zipRead(f, e, typeOf(name)) });
        else skipped++;
      }
      continue;
    }
    if (sorts(f.name, f.type) === "picture") pictures.push({ name: f.name, read: async () => f });
    else skipped++;
  }
  if (!decks.length) return null;
  if (decks.length > 1) throw new Error(t("slides.oneAtATime"));
  const file = decks[0];
  progress(t("slides.reading", { name: file.name }));
  let deck;
  try {
    deck = await readDeck(await file.read());
  } catch {
    throw new Error(t("slides.unreadable", { name: file.name }));
  }
  // With the pictures of an exported presentation, they are the slides;
  // without, the browser draws each slide itself.
  let plan;
  if (pictures.length) {
    try {
      plan = pair(deck.slides, pictures);
    } catch (err) {
      if (err.code === "mismatch") throw new Error(t("slides.mismatch", { slides: err.slides, pictures: err.pictures }));
      throw err;
    }
  } else {
    const shown = deck.slides.filter((s) => !s.hidden);
    plan = { pairs: shown.map((slide) => ({ slide })), hidden: deck.slides.length - shown.length };
  }
  const { item } = await post("/api/album/folders", { parent: folder, name: folderName(file.name, t("slides.folder")) });
  const sum = { folder: item, done: 0, notes: 0, hidden: plan.hidden, skipped, failed: [], drawn: !pictures.length, approximated: 0 };
  const width = widthOf(plan.pairs.length);
  const draw = pictures.length ? null : (await import("./pptxdraw.js")).renderSlide;
  const cache = new Map();
  for (const [i, { slide, picture }] of plan.pairs.entries()) {
    let name = slideName(i + 1, width, slide.title, picture ? picture.name : "slide.png");
    try {
      let blob;
      if (picture) blob = await picture.read();
      else {
        progress(t("slides.drawing", { n: i + 1, total: plan.pairs.length }));
        const drawn = await draw(deck, slide, cache);
        blob = drawn.blob;
        if (drawn.missing.length) sum.approximated++;
        if (blob.type === "image/jpeg") name = slideName(i + 1, width, slide.title, "slide.jpg");
      }
      progress(t("photos.uploading", { n: i + 1, total: plan.pairs.length, name }));
      const att = await upload(`/api/items/${item.id}`, new File([blob], name, { type: blob.type || typeOf(picture?.name || name) }), null, place ? { place } : {});
      await labelUpload(item.id, att, labels);
      sum.done++;
      if (slide.notes) {
        await put(`/api/items/${item.id}/attachments/${att.id}/notes`, { notes: slide.notes });
        sum.notes++;
      }
    } catch (err) { sum.failed.push(`${name}: ${err.message}`); }
  }
  for (const img of cache.values()) img.then((b) => b?.close?.()).catch(() => {});
  return sum;
}

// deckText says what a presentation's import did.
export function deckText(sum) {
  return [t("slides.imported", { n: sum.done, folder: sum.folder.title }), sum.notes ? t("slides.withNotes", { n: sum.notes }) : "",
    sum.hidden ? t("slides.hiddenLeft", { n: sum.hidden }) : "", sum.approximated ? t("slides.approximated", { n: sum.approximated }) : "", sum.skipped ? t("photos.otherSkipped", { n: sum.skipped }) : "",
    sum.failed.length ? t("photos.failed", { n: sum.failed.length }) : ""].filter(Boolean).join(" ");
}

// posterOf takes a picture of a video at an address of this site, at
// most 1600 pixels on its longer edge, as a JPEG; null when this browser
// cannot play the video. The video is read from the server, not from the
// file chosen: the content security policy plays media of this site only.
export function posterOf(url, timeout = 15000) {
  return new Promise((resolve) => {
    const v = document.createElement("video");
    let done = false;
    const end = (out) => {
      if (done) return;
      done = true;
      clearTimeout(timer);
      v.removeAttribute("src");
      v.load();
      resolve(out);
    };
    const timer = setTimeout(() => end(null), timeout);
    v.muted = true;
    v.preload = "auto";
    v.playsInline = true;
    v.addEventListener("error", () => end(null));
    const take = () => {
      const scale = Math.min(1, 1600 / Math.max(v.videoWidth, v.videoHeight));
      if (!v.videoWidth) return end(null);
      const canvas = document.createElement("canvas");
      canvas.width = Math.round(v.videoWidth * scale);
      canvas.height = Math.round(v.videoHeight * scale);
      canvas.getContext("2d").drawImage(v, 0, 0, canvas.width, canvas.height);
      canvas.toBlob((b) => end(b), "image/jpeg", 0.85);
    };
    // A video that tells no length, as a recording made in a browser, is
    // taken at its first frame.
    v.addEventListener("loadeddata", () => {
      const at = posterTime(v.duration);
      if (at > 0) v.currentTime = at;
      else take();
    });
    v.addEventListener("seeked", take, { once: true });
    v.src = url;
  });
}

// setPoster stores a video's poster, taken from it in this browser.
export async function setPoster(folder, att) {
  const poster = await posterOf(`/api/items/${folder}/attachments/${att.id}`);
  if (!poster) return false;
  const res = await fetch(`/api/items/${folder}/attachments/${att.id}/poster`, {
    method: "PUT", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream" }, body: poster,
  });
  return res.ok;
}

// summaryText says what an import did.
export function summaryText(sum) {
  return [t("photos.imported", { n: sum.done }), sum.duplicates ? t("photos.duplicates", { n: sum.duplicates }) : "",
    sum.heic ? t("photos.heicSkipped", { n: sum.heic }) : "",
    sum.skipped ? t("photos.otherSkipped", { n: sum.skipped }) : "", sum.failed.length ? t("photos.failed", { n: sum.failed.length }) : ""].filter(Boolean).join(" ");
}

const pause = (ms) => new Promise((r) => setTimeout(r, ms));

// googleImport lets one choose photos in Google Photos and brings them
// into a folder. The picker opens in a window opened at once, while the
// click still counts, and given its address once Google answers; the
// album then waits until the photos are chosen.
export async function googleImport(folder, progress, cancelled = () => false, place = "") {
  const win = window.open("", "_blank");
  let session;
  try {
    session = await post("/api/album/google/sessions");
  } catch (err) {
    win?.close();
    throw err;
  }
  const picker = session.pickerUri.replace(/\/$/, "") + "/autoclose";
  if (win) win.location.href = picker;
  else progress(t("photos.googleOpen"), picker);
  const sum = { done: 0, duplicates: 0, heic: 0, skipped: 0, failed: [] };
  try {
    const until = Date.now() + (session.timeout || 1800000);
    let state = { picked: false, pollInterval: session.pollInterval };
    while (!state.picked) {
      if (cancelled() || Date.now() > until) return sum;
      progress(t("photos.googleWaiting"), win ? null : picker);
      await pause(Math.max(2000, state.pollInterval || 5000));
      state = await get(`/api/album/google/sessions/${encodeURIComponent(session.id)}`);
    }
    const items = state.items || [];
    for (const [n, m] of items.entries()) {
      if (cancelled()) break;
      progress(t("photos.uploading", { n: n + 1, total: items.length, name: m.name }));
      try {
        const r = await post(`/api/album/google/sessions/${encodeURIComponent(session.id)}/import`, { media: m.id, folder, place });
        if (r.skipped) sum.duplicates++;
        else sum.done++;
        // A video Google keeps no picture of gets one taken here (ADR-0192).
        if (m.video && !r.skipped && !r.poster) await setPoster(folder, r.attachment);
      } catch (err) {
        if (err.status === 415) sum.skipped++;
        else sum.failed.push(`${m.name}: ${err.message}`);
      }
    }
    return sum;
  } finally {
    del(`/api/album/google/sessions/${encodeURIComponent(session.id)}`).catch(() => {});
  }
}

// icloudImport reads a shared iCloud album by its address and brings its
// photos into a folder; giving the address again brings only new ones.
export async function icloudImport(address, folder, progress, cancelled = () => false, place = "") {
  progress(t("photos.icloudReading"));
  const album = await post("/api/album/icloud/albums", { url: address });
  const sum = { done: 0, duplicates: 0, heic: 0, skipped: 0, failed: [], name: album.name };
  for (const [n, p] of album.photos.entries()) {
    if (cancelled()) break;
    progress(t("photos.uploading", { n: n + 1, total: album.photos.length, name: p.caption || album.name }));
    try {
      const r = await post("/api/album/icloud/import", { url: address, photo: p.guid, folder, place });
      if (r.skipped) sum.duplicates++;
      else sum.done++;
      // A video Apple keeps no poster frame of gets one taken here (ADR-0192).
      if (p.video && !r.skipped && !r.poster) await setPoster(folder, r.attachment);
    } catch (err) {
      if (err.status === 415) sum.skipped++;
      else sum.failed.push(`${p.caption || p.guid}: ${err.message}`);
    }
  }
  return sum;
}

// rememberedAlbum is the address of the shared iCloud album last brought
// into a folder in this browser, a convenience only.
export function rememberedAlbum(folder, address) {
  const key = "ot.icloudAlbum." + folder;
  try {
    if (address !== undefined) localStorage.setItem(key, address);
    return localStorage.getItem(key) || "";
  } catch {
    return address || "";
  }
}
