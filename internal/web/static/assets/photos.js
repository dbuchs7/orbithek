// The album's photo management in the browser (ADR-0176): the viewer with
// a picture's photo data, favourites, the archive of chosen pictures, and
// the imports — files and ZIP archives exported from Apple Photos, an
// iPhone or any camera, photos chosen in Google Photos, and shared iCloud
// albums. Files are unpacked and uploaded here, one by one, through the
// files of the folder's item; Google and iCloud photos are fetched by the
// server, which asks Google or Apple for them.
import { h, when } from "./dom.js";
import { t } from "./i18n.js";
import { get, post, put, del, upload, download } from "./api.js";
import { baseName, exposureText, hidden, placeText, sorts, typeOf } from "./photoparts.js";
import { entries as zipEntries, read as zipRead } from "./zipread.js";

export const fileURL = (p) => `/api/items/${p.item.id}/attachments/${p.attachment.id}`;
export const thumbURL = (p, edge = 400) => `${fileURL(p)}/thumbnail?edge=${edge}`;
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
  const info = h("div", { class: "photo-viewer-info", "aria-live": "polite" });
  const prev = h("button", { type: "button", class: "photo-viewer-nav prev", "aria-label": t("photos.previous"), text: "‹" });
  const next = h("button", { type: "button", class: "photo-viewer-nav next", "aria-label": t("photos.next"), text: "›" });
  const dialog = h("dialog", { class: "photo-viewer", id: "photo-viewer", "aria-label": t("photos.viewer") },
    h("div", { class: "drawing-bar" },
      h("strong", { class: "photo-viewer-title" }),
      h("span", { class: "muted photo-viewer-pos" }),
      h("button", { type: "button", id: "photo-viewer-close", text: t("common.close"), onclick: () => dialog.close() })),
    h("div", { class: "photo-viewer-body" }, h("div", { class: "photo-viewer-stage" }, prev, img, next), info));
  const row = (label, value) => (value ? h("div", { class: "photo-fact" }, h("dt", { text: label }), h("dd", {}, value)) : null);
  const show = async () => {
    const p = list[i];
    const name = caption(p.attachment.name);
    img.src = thumbURL(p, 1600);
    img.alt = name;
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
        row(t("photos.size"), [photo?.width && `${photo.width} × ${photo.height}`, p.attachment.size && bytesText(p.attachment.size)].filter(Boolean).join(" · ")),
        row(t("photos.place"), place && h("a", { href: `https://www.openstreetmap.org/?mlat=${photo.lat}&mlon=${photo.lon}#map=15/${photo.lat}/${photo.lon}`,
          target: "_blank", rel: "noopener noreferrer", text: place, title: t("photos.placeOpen") })),
        row(t("photos.labels"), (p.attachment.labels || []).join(", ")),
        row(t("photos.item"), p.item.key ? h("a", { href: "#/item/" + encodeURIComponent(p.item.key), text: `${p.item.key} ${p.item.title || ""}`.trim(), onclick: () => dialog.close() }) : ""),
        row(t("photos.added"), p.attachment.created ? when(p.attachment.created) : ""));
    };
    info.replaceChildren(facts(), h("div", { class: "actions-row" }, fav,
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
// insert does (ADR-0054, ADR-0055), and deletes the original when move is
// asked, one may and no text shows it. It answers whether the original
// went.
export async function copyTo(p, folder, move) {
  const res = await fetch(fileURL(p), { credentials: "same-origin" });
  if (!res.ok) throw new Error(res.statusText);
  const file = new File([await res.blob()], p.attachment.name, { type: p.attachment.type });
  await upload(`/api/items/${folder}`, file, { item: p.item.id, attachment: p.attachment.id });
  if (move && p.editable && !p.used) {
    await del(fileURL(p));
    return true;
  }
  return false;
}

// importFiles uploads chosen or dropped files into a folder: pictures as
// they are, ZIP archives unpacked here first. HEIC photos and other files
// are skipped and counted. progress is told each step.
export async function importFiles(files, folder, progress) {
  const sum = { done: 0, heic: 0, skipped: 0, failed: [] };
  const queue = [];
  for (const f of files) {
    if (/\.zip$/i.test(f.name) || f.type === "application/zip" || f.type === "application/x-zip-compressed") {
      try {
        for (const e of await zipEntries(f)) {
          if (hidden(e.name)) continue;
          const name = baseName(e.name);
          const kind = sorts(name, typeOf(name));
          if (kind === "picture") queue.push({ name, read: () => zipRead(f, e, typeOf(name)) });
          else if (kind === "heic") sum.heic++;
          else sum.skipped++;
        }
      } catch (err) { sum.failed.push(`${f.name}: ${err.message}`); }
      continue;
    }
    const kind = sorts(f.name, f.type);
    if (kind === "picture") queue.push({ name: f.name, read: async () => f });
    else if (kind === "heic") sum.heic++;
    else sum.skipped++;
  }
  for (const [n, q] of queue.entries()) {
    progress(t("photos.uploading", { n: n + 1, total: queue.length, name: q.name }));
    try {
      const blob = await q.read();
      await upload(`/api/items/${folder}`, blob instanceof File ? blob : new File([blob], q.name, { type: blob.type }));
      sum.done++;
    } catch (err) { sum.failed.push(`${q.name}: ${err.message}`); }
  }
  return sum;
}

// summaryText says what an import did.
export function summaryText(sum) {
  return [t("photos.imported", { n: sum.done }), sum.duplicates ? t("photos.duplicates", { n: sum.duplicates }) : "",
    sum.heic ? t("photos.heicSkipped", { n: sum.heic }) : "", sum.videos ? t("photos.videosSkipped", { n: sum.videos }) : "",
    sum.skipped ? t("photos.otherSkipped", { n: sum.skipped }) : "", sum.failed.length ? t("photos.failed", { n: sum.failed.length }) : ""].filter(Boolean).join(" ");
}

const pause = (ms) => new Promise((r) => setTimeout(r, ms));

// googleImport lets one choose photos in Google Photos and brings them
// into a folder. The picker opens in a window opened at once, while the
// click still counts, and given its address once Google answers; the
// album then waits until the photos are chosen.
export async function googleImport(folder, progress, cancelled = () => false) {
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
  const sum = { done: 0, duplicates: 0, videos: 0, heic: 0, skipped: 0, failed: [] };
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
      if (m.video) { sum.videos++; continue; }
      progress(t("photos.uploading", { n: n + 1, total: items.length, name: m.name }));
      try {
        const r = await post(`/api/album/google/sessions/${encodeURIComponent(session.id)}/import`, { media: m.id, folder });
        if (r.skipped) sum.duplicates++;
        else sum.done++;
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
export async function icloudImport(address, folder, progress, cancelled = () => false) {
  progress(t("photos.icloudReading"));
  const album = await post("/api/album/icloud/albums", { url: address });
  const sum = { done: 0, duplicates: 0, videos: 0, heic: 0, skipped: 0, failed: [], name: album.name };
  for (const [n, p] of album.photos.entries()) {
    if (cancelled()) break;
    if (p.video) { sum.videos++; continue; }
    progress(t("photos.uploading", { n: n + 1, total: album.photos.length, name: p.caption || album.name }));
    try {
      const r = await post("/api/album/icloud/import", { url: address, photo: p.guid, folder });
      if (r.skipped) sum.duplicates++;
      else sum.done++;
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
