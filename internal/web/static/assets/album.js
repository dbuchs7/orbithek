// The album (ADR-0054): the images and drawings of every item one may
// read, in one place. Inserting a picture into a body copies it into that
// body's item, where it takes on the item's key and access; the original
// stays where it is, unchanged. Folders (ADR-0090) are items of their own
// that hold the pictures uploaded into them; pictures carry labels of
// their own, which only narrow the album. Every picture one sees here is
// exported as PNG or JPEG, a drawing also as SVG (ADR-0173). Collections
// (ADR-0175) sit beside folders but hold nothing: their rule names labels,
// and they gather the matching pictures of every room one may read.
// The pictures shown are presented as slides, by name, with speaker's
// notes kept on each picture (ADR-0176).
// As a photo library (ADR-0177) it shows pictures by the time they were
// taken, on a map, as one's favourites and as memories of the same day in
// earlier years; pictures are chosen many at once, and photos come in from
// files, ZIP archives, Google Photos and shared iCloud albums. A picture,
// or every chosen one, is moved by dragging it onto a folder (ADR-0184).
// Shown in the order of slides, the pictures of a folder or collection
// are dragged among each other to set that order by hand (ADR-0195).
// Pictures are added where one is (ADR-0187): files dropped, pasted or
// chosen go into the open folder, or, in a collection or at the top, into
// a folder chosen each time, where a collection's labels are given to
// them; pictures already in the album are copied into a folder or given a
// collection's labels.
import { h, shareHref, when } from "./dom.js";
import { t } from "./i18n.js";
import { get, post, put, del, upload } from "./api.js";
import { DRAWING } from "./drawing.js";
import { exportFormats, exportPicture } from "./picexport.js";
import { allFolders, copyTo, moveTo, dayText, deckText, googleImport, icloudImport, importDeck, importFiles, monthText, openViewer, rememberedAlbum, setFavourite, importPlace, PLACES, summaryText, thumbURL, zipPictures } from "./photos.js";
import { dragged, durationText, folderDrop, folderTargets, groups, isVideo, onShare, orderDrop, range, shareLink, shiftTaken, yearsAgo } from "./photoparts.js";
import { askTime } from "./phototime.js";
import { photoMap } from "./photomap.js";
import { pickPlace, placeInput } from "./places.js";
import { startPresentation } from "./present.js";
import { personCover, personSuggestions, scanAll } from "./faces.js";
import { renameInPlace } from "./rename.js";
import { askCleanup, groupTitle, loadDuplicates } from "./duplicates.js";
import { cleanupPlan } from "./duplicateparts.js";

const fileURL = (p) => `/api/items/${p.item.id}/attachments/${p.attachment.id}`;

// VIEWS are the album's ways of showing pictures (ADR-0177).
const VIEWS = ["folders", "timeline", "favourites", "memories", "map", "people", "duplicates"];

// SORT keeps, in this browser, how a folder or collection is shown:
// newest first ("") or in the order of its slides ("slides", ADR-0195).
const SORT = "ot.albumSort";
const sortKept = () => {
  try { return localStorage.getItem(SORT) === "slides" ? "slides" : ""; } catch { return ""; }
};
const keepSort = (v) => {
  try { localStorage.setItem(SORT, v); } catch { /* for this visit only */ }
};

// The album's clipboard (ADR-0108): a picture copied in the album is pasted
// into a text as a copy of its original file, with its source, as "Insert…
// → From the album" does. The system clipboard carries a marker that names
// the picture, beside the picture itself as PNG for other programs; this
// browser keeps the last picture copied, for Insert… → Copied picture where
// the clipboard cannot be written.
const CLIP = "ot.albumClip";
export const clipMark = /data-ot-album="([A-Za-z0-9_-]{1,64})\/([0-9a-f]{32})"/;

// copiedPicture is the picture last copied in this browser, or null.
export function copiedPicture() {
  try {
    const p = JSON.parse(localStorage.getItem(CLIP));
    return p?.item?.id && p?.attachment?.id ? p : null;
  } catch {
    return null;
  }
}

// asPNG gives a picture's bytes as PNG, the image format clipboards take.
async function asPNG(blob) {
  if (blob.type === "image/png") return blob;
  const bitmap = await createImageBitmap(blob);
  const canvas = document.createElement("canvas");
  canvas.width = bitmap.width;
  canvas.height = bitmap.height;
  canvas.getContext("2d").drawImage(bitmap, 0, 0);
  return new Promise((resolve, reject) => canvas.toBlob((b) => (b ? resolve(b) : reject(new Error("png"))), "image/png"));
}

// copyPicture copies a picture of the album. It answers true when the
// system clipboard holds it, false when only this browser does.
export async function copyPicture(p) {
  const clip = { item: { id: p.item.id, key: p.item.key, title: p.item.title }, attachment: { id: p.attachment.id, name: p.attachment.name, type: p.attachment.type } };
  try { localStorage.setItem(CLIP, JSON.stringify(clip)); } catch { /* the system clipboard alone */ }
  if (!navigator.clipboard?.write || typeof ClipboardItem === "undefined") return false;
  const mark = `<img data-ot-album="${p.item.id}/${p.attachment.id}">`;
  try {
    // The picture's bytes are fetched inside the promise, so the click
    // that copies still counts as the user's gesture.
    await navigator.clipboard.write([new ClipboardItem({
      "text/html": new Blob([mark], { type: "text/html" }),
      "text/plain": new Blob([p.attachment.name], { type: "text/plain" }),
      "image/png": fetch(fileURL(p), { credentials: "same-origin" }).then((r) => r.blob()).then(asPNG),
    })]);
    return true;
  } catch {
    return false;
  }
}

// captionOf makes a caption from a file name.
export const captionOf = (name) => (name.endsWith(DRAWING) ? name.slice(0, -DRAWING.length) : name.replace(/\.[^.]*$/, ""));

// albumBrowser shows the album with its filters and folders. With pick,
// each picture has a button that hands it to pick; without, a picture
// leads to its item, and one who may edit it labels or deletes it there.
export function albumBrowser({ pick, pickText } = {}) {
  const search = h("input", { type: "search", placeholder: t("album.search"), "aria-label": t("album.search") });
  const kind = h("select", { "aria-label": t("album.kind") },
    [["", t("album.kindAll")], ["image", t("album.kindImages")], ["drawing", t("album.kindDrawings")], ["video", t("album.kindVideos")]].map(([v, label]) => h("option", { value: v, text: label })));
  const space = h("select", { "aria-label": t("album.space") }, h("option", { value: "", text: t("album.spaceAll") }));
  // The rooms one may see, for the filter and the room of a new folder.
  let rooms = [];
  const roomOptions = () => rooms.map((s) => h("option", { value: s.key, text: `${s.key} ${s.name}` }));
  const label = h("select", { "aria-label": t("album.label"), id: "album-label" }, h("option", { value: "", text: t("album.labelAll") }));
  const options = h("datalist", { id: "album-label-options" });
  const trail = h("nav", { class: "album-trail", "aria-label": t("album.folders") });
  // manage stands beside the trail and changes the open folder or
  // collection itself; tools below it add pictures to it.
  const manage = h("div", { class: "album-manage" });
  const headRow = h("div", { class: "album-location" }, trail, manage);
  const tools = h("div", { class: "album-tools" });
  const folders = h("ul", { class: "album-folders" });
  const grid = h("ul", { class: "album-grid" });
  const count = h("p", { class: "muted", role: "status" });
  const more = h("button", { type: "button", text: t("album.more"), hidden: true });
  // present shows the pictures the album shows now as slides, in the
  // order of the folder or collection set by hand and else by name, in a
  // presenter's window and a presentation window (ADR-0176, ADR-0195).
  const present = h("button", { type: "button", class: "secondary", id: "album-present", text: t("album.present", { n: 0 }),
    title: t("album.presentTitle"), hidden: true, onclick: () => startPresentation(filters()) });
  const status = h("div", { class: "status" });
  // files takes the pictures chosen to add where one is (ADR-0187).
  const files = h("input", { type: "file", accept: "image/png,image/jpeg,image/gif,image/webp,.heic,.heif,video/mp4,video/webm,video/quicktime,.mp4,.m4v,.webm,.mov,.zip,application/zip,.pptx,.ppsx", multiple: true, hidden: true, id: "album-files" });
  files.addEventListener("change", async () => {
    const list = [...files.files];
    files.value = "";
    await takeFiles(list);
  });
  // dropZone stands in an empty folder or collection.
  const dropZone = h("div", { class: "album-drop-zone", id: "album-drop-zone", hidden: true });
  let next = 0;
  let generation = 0;
  // path is the open folder and those above it; empty at the top. Its last
  // step may be a collection, which has no folders below it.
  let path = [];
  const here = () => path[path.length - 1];
  const inCollection = () => Boolean(here()?.rule);
  const inFolder = () => Boolean(here()) && !inCollection();
  const fail = (err) => status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
  const say = (text, cls = "muted") => status.replaceChildren(h("p", { class: cls, role: "status", text }));
  const params = new URLSearchParams(location.hash.split("?")[1] || "");
  // view is how the album shows its pictures; choosing picks many at once
  // (ADR-0177). shown are the pictures on screen, in their order.
  let view = pick ? "folders" : VIEWS.includes(params.get("view")) ? params.get("view") : "folders";
  let choosing = false;
  const chosen = new Map();
  let anchor = null;
  let shown = [];
  // dragging holds the pictures being dragged onto a folder (ADR-0184),
  // or among the others to order them as slides (ADR-0195).
  let dragging = null;
  // draggingFolder is the folder being dragged onto another folder or the
  // trail, to move it there with what it holds (ADR-0200).
  let draggingFolder = null;
  // sort is how a folder or collection is shown: newest first, or as its
  // slides; order says whether it has an order set by hand, whether one
  // may change it, and whether every picture is shown (ADR-0195).
  let sort = pick ? "" : sortKept();
  let order = { ordered: false, orderable: false, complete: false };
  const sortSelect = h("select", { id: "album-sort", "aria-label": t("album.sort"), hidden: true },
    h("option", { value: "", text: t("album.sortNewest") }), h("option", { value: "slides", text: t("album.sortSlides") }));
  sortSelect.value = sort;
  sortSelect.addEventListener("change", () => { sort = sortSelect.value; keepSort(sort); load(false); });
  const orderHint = h("span", { class: "muted album-order-hint", id: "album-order-hint", hidden: true, text: t("album.orderHint") });
  const orderReset = h("button", { type: "button", class: "link", id: "album-order-reset", hidden: true, text: t("album.orderReset"), title: t("album.orderResetTitle"),
    onclick: async () => {
      if (!confirm(t("album.orderResetConfirm"))) return;
      try {
        await del(`/api/album/order/${encodeURIComponent(here().id)}`);
        say(t("album.orderCleared"), "notice");
        refresh();
      } catch (err) { fail(err); }
    } });
  // bySlides says whether the album shows a folder or collection in the
  // order of its slides; reordering whether one may drag pictures there.
  const bySlides = () => !pick && view === "folders" && Boolean(here()) && sort === "slides";
  const reordering = () => bySlides() && order.orderable;
  // placePictures moves pictures, by the first versions of their stacks,
  // right before another, or to the end with "", and shows the album
  // again as far as it was shown.
  const placePictures = async (moved, before) => {
    try {
      await put(`/api/album/order/${encodeURIComponent(here().id)}`, { pictures: moved, before });
      say(t("album.ordered", { n: moved.length }), "notice");
      refresh();
    } catch (err) { fail(err); }
  };
  // stepPicture moves a picture one place earlier (-1) or later (+1), the
  // way without a pointer.
  const stepPicture = (p, delta) => {
    const firsts = shown.map((q) => q.first);
    const i = firsts.indexOf(p.first);
    if (i < 0) return;
    const before = delta < 0 ? (i > 0 ? firsts[i - 1] : null) : orderDrop(firsts, [p.first], i + 1, true, order.complete);
    if (before === null) return say(t("album.orderUnknown"));
    placePictures([p.first], before);
  };
  let imports = { google: false, icloud: false, connected: false };
  const timeline = h("div", { class: "album-timeline" });
  const mapBox = h("div", { class: "album-map" });
  // The persons named on pictures, and the album of the one chosen
  // (ADR-0194).
  const peopleBox = h("div", { class: "album-people", id: "album-people" });
  let person = "";

  // Labels are suggested from those of pictures and of items alike.
  const loadLabels = async () => {
    const [pics, items] = await Promise.all([get("/api/album/labels"), get("/api/labels").catch(() => ({ labels: [] }))]);
    const keep = label.value;
    label.replaceChildren(h("option", { value: "", text: t("album.labelAll") }),
      ...pics.labels.map((l) => h("option", { value: l.label, text: `${l.label} (${l.count})` })));
    label.value = pics.labels.some((l) => l.label === keep) ? keep : "";
    const all = [...new Set([...pics.labels, ...items.labels].map((l) => l.label))];
    options.replaceChildren(...all.map((l) => h("option", { value: l })));
  };

  // labelsNow reads a picture's labels as the server keeps them, written
  // as labels are (ADR-0044).
  const labelsNow = async (p) => {
    const { attachments } = await get(`/api/items/${p.item.id}/attachments`);
    return attachments.find((a) => a.id === p.attachment.id)?.labels || [];
  };

  // labelsOf shows a picture's labels; each narrows the album to it. One
  // who may edit the picture opens a field for them with one button, so
  // the field does not stand open on every card.
  const labelsOf = (p) => {
    const name = captionOf(p.attachment.name);
    const labels = () => p.attachment.labels || [];
    const chips = () => labels().map((l) => h("button", { type: "button", class: "chip album-label", text: l,
      onclick: () => { label.value = l; load(false); } }));
    if (pick || !p.editable) return labels().length ? h("p", { class: "album-labels" }, ...chips()) : null;
    const row = h("div", { class: "album-labels" });
    const show = () => row.replaceChildren(...chips(), h("button", { type: "button", class: "link album-labels-edit",
      text: labels().length ? t("album.labelsEdit") : t("album.labelsAdd"), "aria-label": t("album.labelsOf", { name }),
      onclick: edit }));
    const edit = () => {
      const input = h("input", { class: "album-labels-input", list: "album-label-options", value: labels().join(", "),
        placeholder: t("album.labelsPlaceholder"), "aria-label": t("album.labelsOf", { name }) });
      let saving = false;
      input.addEventListener("change", async () => {
        saving = true;
        try {
          await put(`/api/items/${p.item.id}/attachments/${p.attachment.id}/labels`, { labels: input.value.split(",").map((l) => l.trim()).filter(Boolean) });
          // The card changes in place: reloading the album would show only
          // its first page again, after "Show more".
          p.attachment.labels = await labelsNow(p);
          status.replaceChildren();
          show();
          await loadLabels();
        } catch (err) { saving = false; fail(err); }
      });
      // Leaving the field unchanged, or Escape, closes it again.
      input.addEventListener("keydown", (e) => { if (e.key === "Escape") { e.preventDefault(); show(); } });
      input.addEventListener("blur", () => setTimeout(() => { if (!saving && row.contains(input)) show(); }, 150));
      row.replaceChildren(input);
      input.focus();
    };
    show();
    return row;
  };

  // notesOf shows whether a picture has speaker's notes, for presenting it
  // as a slide (ADR-0176); one who may edit the picture writes them here.
  // The notes themselves show in the presenter's window.
  const notesOf = (p) => {
    if (p.versionOf) return null;
    if (pick || !p.editable) return p.notes && !pick ? h("p", { class: "album-notes muted", text: t("album.hasNotes") }) : null;
    const name = captionOf(p.attachment.name);
    const row = h("div", { class: "album-notes" });
    const show = () => row.replaceChildren(h("button", { type: "button", class: "link album-notes-edit",
      text: p.notes ? t("album.notesEdit") : t("album.notesAdd"), "aria-label": t("album.notesOf", { name }), onclick: edit }));
    const edit = () => {
      const area = h("textarea", { class: "album-notes-input", rows: "4", value: p.notes || "",
        placeholder: t("album.notesPlaceholder"), "aria-label": t("album.notesOf", { name }) });
      const save = h("button", { type: "button", class: "primary album-notes-save", text: t("album.notesSave"), onclick: async () => {
        try {
          await put(`/api/items/${p.item.id}/attachments/${p.attachment.id}/notes`, { notes: area.value });
          p.notes = area.value.replace(/\r\n?/g, "\n").trim();
          status.replaceChildren(h("p", { class: "notice", role: "status", text: t("album.notesSaved", { name }) }));
          show();
        } catch (err) { fail(err); }
      } });
      area.addEventListener("keydown", (e) => { if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); show(); } });
      row.replaceChildren(area, h("div", { class: "album-notes-actions" }, save,
        h("button", { type: "button", class: "link", text: t("common.cancel"), onclick: show })));
      area.focus();
    };
    show();
    return row;
  };

  // A card's menus: the export menu and the menu of further actions
  // (ADR-0173). One open menu at a time; a click elsewhere or Escape
  // closes it.
  const menu = (cls, summary, items) => {
    const d = h("details", { class: `export-menu album-menu ${cls}` }, summary, h("div", { class: "export-list" }, items));
    const outside = (e) => { if (!d.contains(e.target)) d.open = false; };
    d.addEventListener("toggle", () => {
      if (!d.open) return document.removeEventListener("click", outside, true);
      for (const other of root.querySelectorAll("details.album-menu[open]")) if (other !== d) other.open = false;
      document.addEventListener("click", outside, true);
    });
    return d;
  };

  // exportMenu offers a picture as a file in each of its formats
  // (ADR-0173). Whoever sees it in the album may read it, and so may
  // export it; the menu closes when a format is chosen.
  const formatText = { png: t("album.exportPng"), jpeg: t("album.exportJpeg"), svg: t("album.exportSvg") };
  const exportMenu = (p) => {
    const m = menu("album-export", h("summary", { class: "button secondary", text: t("album.export"), title: t("album.exportTitle") }),
      exportFormats(p).map((f) => h("button", { type: "button", class: "link", "data-format": f, text: formatText[f],
        onclick: async () => {
          m.open = false;
          const name = captionOf(p.attachment.name);
          status.replaceChildren(h("p", { class: "muted", role: "status", text: t("album.exporting", { name }) }));
          try {
            const file = await exportPicture(p, f, h);
            status.replaceChildren(h("p", { class: "notice", role: "status", text: t("album.exported", { name: file }) }));
          } catch (err) {
            fail(new Error(t("album.exportFailed", { name, reason: err.message === "empty" ? t("draw.empty") : err.message })));
          }
        } })));
    return m;
  };

  // moreMenu holds what one who may edit a picture does less often: clean
  // up its older versions (ADR-0104) and delete it (ADR-0090). A picture
  // its item's text shows is deleted there, not here; the menu says so.
  const moreMenu = (p, stacked) => {
    const name = captionOf(p.attachment.name);
    // An entry of a file share is renamed and deleted on the share
    // (ADR-0197); its order among the slides is still set here.
    const fixed = onShare(p);
    if (fixed && !reordering()) return null;
    const m = menu("album-more", h("summary", { class: "album-more-button", "aria-label": t("album.moreOf", { name }), title: t("album.moreTitle") }), [
      fixed ? null : h("button", { type: "button", class: "link album-rename", text: t("album.rename"),
        onclick: () => { m.open = false; rename(p, m.closest(".album-card")?.querySelector(".album-name")); } }),
      stacked ? h("button", { type: "button", class: "link album-retention", text: t("retention.open"),
        onclick: () => { m.open = false; cleanUp(p); } }) : null,
      reordering() && !p.versionOf ? h("button", { type: "button", class: "link album-earlier", text: t("album.moveEarlier"),
        onclick: () => { m.open = false; stepPicture(p, -1); } }) : null,
      reordering() && !p.versionOf ? h("button", { type: "button", class: "link album-later", text: t("album.moveLater"),
        onclick: () => { m.open = false; stepPicture(p, 1); } }) : null,
      fixed ? null : h("button", { type: "button", class: "link danger album-delete", text: t("album.delete"), disabled: p.used,
        onclick: async () => {
          m.open = false;
          if (!confirm(t("album.deleteConfirm", { name: p.attachment.name }))) return;
          try {
            await del(`/api/items/${p.item.id}/attachments/${p.attachment.id}`);
            await loadLabels();
            refresh();
          } catch (err) { fail(err); }
        } }),
      p.used && !fixed ? h("p", { class: "muted album-used", text: t("album.used") }) : null,
    ]);
    return m;
  };

  // rename lets one who may edit a picture give it another name, in place
  // of its name on the card (ADR-0180). Every version of the file takes
  // it; the extension, and a drawing's marker, stay as they are, so only
  // the name itself is typed. Enter or leaving the field saves, Escape
  // keeps the name.
  const rename = (p, label) => {
    const name = captionOf(p.attachment.name);
    renameInPlace(label, { value: name, ariaLabel: t("album.renameOf", { name }), className: "album-rename-input", save: async (next) => {
      try {
        await put(`/api/items/${p.item.id}/attachments/${p.attachment.id}/name`, { name: next });
      } catch (err) { fail(err); throw err; }
      status.replaceChildren(h("p", { class: "notice", role: "status", text: t("album.renamed", { name: next }) }));
      refresh();
    } });
  };

  // actions is the row at a card's foot: Insert while choosing a picture,
  // otherwise Copy and Export, of one width.
  const actions = (p) => {
    // pick may answer what it did, which the button then says; the album
    // stays open for the next picture (ADR-0187).
    if (pick) return h("div", { class: "album-actions album-actions-pick" },
      h("button", { type: "button", class: "primary album-pick", text: pickText || t("album.insert"), onclick: async (e) => {
        const b = e.currentTarget;
        b.disabled = true;
        try {
          const done = await pick(p);
          if (typeof done === "string") {
            b.textContent = done;
            b.classList.replace("primary", "secondary");
          } else b.disabled = false;
        } catch (err) { b.disabled = false; fail(err); }
      } }));
    // An entry of a file share opens its original where it lies, with the
    // reader's own access to the share (ADR-0197).
    // h() allows http(s) links only; an smb: address is set with shareHref,
    // which allows https: and smb: alone.
    const original = shareLink(p);
    const openOriginal = original ? h("a", { class: "button secondary album-original", target: "_blank", rel: "noopener noreferrer", text: t("album.openOriginal"), title: t("album.openOriginalTitle") }) : null;
    openOriginal?.setAttribute("href", shareHref(original));
    // Without a preview (HEIC, RAW, a video of a share) there is nothing
    // here to copy, export or download; the original opens on the share.
    if (onShare(p) && !p.attachment.share.preview) return h("div", { class: "album-actions" }, openOriginal);
    return h("div", { class: "album-actions" },
      openOriginal,
      h("button", { type: "button", class: "secondary album-copy", text: t("album.copy"), title: t("album.copyTitle"), onclick: async () => {
        const system = await copyPicture(p);
        status.replaceChildren(h("p", { class: "notice", role: "status", text: t(system ? "album.copied" : "album.copiedHere", { name: captionOf(p.attachment.name) }) }));
      } }),
      // A video is downloaded as it is; it has no picture formats to
      // export (ADR-0189).
      isVideo(p) ? h("a", { class: "button secondary album-download", href: fileURL(p), download: p.attachment.name, text: t("album.download") }) : exportMenu(p));
  };

  // videoThumb shows a video by its poster, or by its symbol when it has
  // none, with how long it plays (ADR-0189).
  const videoThumb = (p, name) => {
    const symbol = () => h("span", { class: "album-video-symbol", "aria-hidden": "true" });
    let img = symbol();
    if (p.poster) {
      img = h("img", { src: thumbURL(p), alt: name, loading: "lazy", draggable: "false" });
      img.addEventListener("error", () => img.replaceWith(symbol()), { once: true });
    }
    return h("div", { class: "album-video" }, img,
      h("span", { class: "album-video-badge", "aria-label": t("album.videoOf", { duration: durationText(p.attachment.photo?.duration) || "–" }) },
        h("span", { class: "album-play", "aria-hidden": "true", text: "▶" }), " ", durationText(p.attachment.photo?.duration)));
  };

  // card shows one picture, with the same parts in the same places on every
  // card: the picture in a box of one height, the name in at most two lines
  // with the menu of further actions beside it for one who may edit, one
  // line of facts (kind, version, own rule, date), the item it belongs to,
  // its labels, and its actions at the card's foot, so the buttons of a row
  // of cards line up. version numbers it within its stack; stacked says it
  // is the top of a stack with older versions.
  const card = (p, version, picture, stacked = false) => {
    const name = captionOf(p.attachment.name);
    // An entry of a file share without a preview (a format the server
    // does not scale) shows a symbol (ADR-0197).
    const noPreview = onShare(p) && !isVideo(p) && !p.attachment.share.preview;
    const thumb = h("div", { class: "album-thumb" }, picture || (isVideo(p) ? videoThumb(p, name)
      : noPreview ? h("span", { class: "album-share-symbol", role: "img", "aria-label": t("album.noPreview") })
        : h("img", { src: thumbURL(p), alt: name, loading: "lazy", draggable: "false" })));
    const li = h("li", { class: "album-card", "data-attachment": p.attachment.id },
      h("figure", {},
        thumb,
        h("figcaption", {},
          h("div", { class: "album-head" },
            h("span", { class: "album-name", text: name, title: name }),
            pick ? null : favouriteButton(p),
            !pick && p.editable ? moreMenu(p, stacked) : null),
          h("span", { class: "album-chips" },
            p.drawing ? h("span", { class: "chip", text: t("album.drawing") }) : null,
            isVideo(p) ? h("span", { class: "chip album-video-chip", text: t("album.video") }) : null,
            onShare(p) ? h("span", { class: "chip album-share-chip", text: t("album.onShare"), title: p.attachment.share.path }) : null,
            version ? h("span", { class: "chip album-version", text: t("album.version", { n: version }) }) : null,
            stacked && p.retention ? h("span", { class: "chip album-own-rule", text: t("retention.chip") }) : null,
            // In the view of duplicates, how it matches the others (ADR-0196).
            view === "duplicates" ? h("span", { class: p.same ? "chip duplicate-same" : "chip duplicate-alike", text: t(p.same ? "duplicates.same" : "duplicates.alike") }) : null,
            h("span", { class: "muted album-when", text: when(p.attachment.created) })),
          h("a", { href: "#/item/" + encodeURIComponent(p.item.key), class: "muted album-item", text: `${p.item.key} ${p.item.title}`,
            title: `${p.item.key} ${p.item.title}`, target: pick ? "_blank" : null, rel: pick ? "noopener" : null }))),
      labelsOf(p),
      notesOf(p),
      actions(p));
    // Outside a stack's top, a click on the picture opens the viewer, or
    // chooses it while choosing; Shift chooses the range from the last.
    if (!pick && !picture) {
      thumb.classList.add("album-open");
      thumb.setAttribute("role", "button");
      thumb.setAttribute("tabindex", "0");
      thumb.setAttribute("aria-label", t("photos.openOf", { name }));
      const act = (e) => (choosing ? choose(p, e.shiftKey) : openViewer(shown, Math.max(0, shown.indexOf(p)), () => refreshFavourite(p)));
      thumb.addEventListener("click", act);
      thumb.addEventListener("keydown", (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); act(e); } });
    }
    if (!pick) {
      const box = h("input", { type: "checkbox", class: "album-choose", "aria-label": t("photos.chooseOf", { name }), checked: chosen.has(key(p)) });
      box.addEventListener("click", (e) => { e.preventDefault(); choose(p, e.shiftKey); });
      li.prepend(box);
      li.classList.toggle("chosen", chosen.has(key(p)));
    }
    // In the folders, a picture is dragged onto a folder to move it, and
    // every chosen picture with it when it is one of them (ADR-0184). An
    // older version of a stack moves only with its stack.
    if (!pick && !p.versionOf && view === "folders") {
      li.draggable = true;
      li.addEventListener("dragstart", (e) => {
        dragging = dragged(p, chosen, key);
        e.dataTransfer.effectAllowed = "copyMove";
        e.dataTransfer.setData("text/plain", dragging.map((q) => q.attachment.name).join("\n"));
        root.classList.add("dragging-pictures");
      });
      li.addEventListener("dragend", () => {
        dragging = null;
        root.classList.remove("dragging-pictures");
        for (const el of root.querySelectorAll(".drop-target, .drop-before, .drop-after")) el.classList.remove("drop-target", "drop-before", "drop-after");
      });
      // Shown as slides, a picture dropped onto another goes before it, or
      // after it when dropped onto its latter half (ADR-0195).
      const after = (e) => {
        const r = li.getBoundingClientRect();
        return e.clientX > r.left + r.width / 2;
      };
      const over = (e) => {
        if (!dragging || !reordering()) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        const a = after(e);
        li.classList.toggle("drop-after", a);
        li.classList.toggle("drop-before", !a);
      };
      li.addEventListener("dragenter", over);
      li.addEventListener("dragover", over);
      li.addEventListener("dragleave", (e) => { if (!li.contains(e.relatedTarget)) li.classList.remove("drop-before", "drop-after"); });
      li.addEventListener("drop", (e) => {
        li.classList.remove("drop-before", "drop-after");
        if (!dragging || !reordering()) return;
        e.preventDefault();
        e.stopPropagation();
        const firsts = shown.map((q) => q.first);
        const taken = new Set(dragging.map((q) => q.first));
        const moved = firsts.filter((f) => taken.has(f));
        dragging = null;
        root.classList.remove("dragging-pictures");
        const before = orderDrop(firsts, moved, firsts.indexOf(p.first), after(e), order.complete);
        if (!moved.length) return;
        if (before === null) return say(t("album.orderUnknown"));
        placePictures(moved, before);
      });
    }
    return li;
  };

  // key names a picture on screen.
  const key = (p) => `${p.item.id}/${p.attachment.id}`;

  // relabel shows a picture's labels anew on its card, in place.
  const relabel = (p) => {
    for (const li of root.querySelectorAll(`li.album-card[data-attachment="${p.attachment.id}"]`)) {
      const row = li.querySelector(":scope > .album-labels");
      const next = labelsOf(p);
      if (row && next) row.replaceWith(next);
      else if (row) row.remove();
      else if (next) li.querySelector(":scope > figure").after(next);
    }
  };

  // favouriteButton marks a picture as one's own favourite (ADR-0177).
  const favouriteButton = (p) => {
    const b = h("button", { type: "button", class: "album-favourite", "aria-pressed": String(!!p.favourite),
      "aria-label": t(p.favourite ? "photos.unfavouriteOf" : "photos.favouriteOf", { name: captionOf(p.attachment.name) }),
      title: t(p.favourite ? "photos.unfavourite" : "photos.favourite"), text: p.favourite ? "★" : "☆" });
    b.addEventListener("click", async () => {
      try {
        await setFavourite(p, !p.favourite);
        refreshFavourite(p);
        if (view === "favourites" && !p.favourite) refresh();
      } catch (err) { fail(err); }
    });
    return b;
  };
  const refreshFavourite = (p) => {
    for (const b of root.querySelectorAll(`li[data-attachment="${p.attachment.id}"] .album-favourite`)) {
      b.setAttribute("aria-pressed", String(!!p.favourite));
      b.textContent = p.favourite ? "★" : "☆";
      b.title = t(p.favourite ? "photos.unfavourite" : "photos.favourite");
      b.setAttribute("aria-label", t(p.favourite ? "photos.unfavouriteOf" : "photos.favouriteOf", { name: captionOf(p.attachment.name) }));
    }
  };

  // choose adds a picture to the chosen ones or takes it out; with range,
  // every picture between the last chosen and this one is chosen.
  const choose = (p, withRange = false) => {
    if (!choosing) setChoosing(true);
    const ids = shown.map(key);
    const targets = withRange && anchor ? range(ids, anchor, key(p)) : [key(p)];
    const on = withRange ? true : !chosen.has(key(p));
    for (const k of targets) {
      const q = shown.find((x) => key(x) === k);
      if (on && q) chosen.set(k, q);
      else chosen.delete(k);
    }
    anchor = key(p);
    markChosen();
  };
  const markChosen = () => {
    for (const li of root.querySelectorAll("li.album-card")) {
      const box = li.querySelector(":scope > .album-choose");
      const p = shown.find((x) => x.attachment.id === li.dataset.attachment);
      const on = p ? chosen.has(key(p)) : false;
      li.classList.toggle("chosen", on);
      if (box) box.checked = on;
    }
    showBulk();
  };

  // cleanUp lets whoever may edit a picture remove its older versions and
  // give them a rule of their own (ADR-0104). The values start from the
  // picture's own rule or the album's default; a preview shows what stays
  // and what goes before anything is removed.
  const cleanUp = async (p) => {
    const base = `/api/items/${p.item.id}/attachments/${p.attachment.id}/retention`;
    const name = captionOf(p.attachment.name);
    let plan;
    try { plan = await get(base); } catch (err) { return fail(err); }
    const number = (id, value) => h("input", { type: "number", id, min: 0, max: id === "retention-keep" ? 1000 : 36500, step: 1, value: String(value) });
    const keep = number("retention-keep", plan.rule.keep);
    const days = number("retention-days", plan.rule.days);
    const source = h("p", { class: "muted", id: "retention-source" });
    const list = h("ul", { class: "retention-list", id: "retention-list" });
    const dstatus = h("div", { class: "status", role: "status" });
    const clean = h("button", { type: "button", class: "danger", id: "retention-clean" });
    const rule = () => ({ keep: Number(keep.value) || 0, days: Number(days.value) || 0 });
    const stateText = { keep: t("retention.keepState"), used: t("retention.used"), remove: t("retention.remove") };
    const show = () => {
      source.textContent = plan.own ? t("retention.fromOwn") : t("retention.fromDefault");
      const n = plan.versions.length;
      list.replaceChildren(...plan.versions.map((v, i) => h("li", { class: "retention-version " + v.state, "data-state": v.state, "data-attachment": v.id },
        h("img", { src: `/api/items/${p.item.id}/attachments/${v.id}`, alt: "", loading: "lazy" }),
        h("span", { class: "retention-what" },
          h("strong", { text: t("album.version", { n: n - i }) }), " ",
          h("span", { class: "muted", text: when(v.created) })),
        h("span", { class: "chip retention-state", text: i === 0 ? t("retention.current") : stateText[v.state] }))));
      const goes = plan.remove.length;
      clean.textContent = goes ? t("retention.clean", { n: goes }) : t("retention.nothing");
      clean.disabled = !goes;
      reset.hidden = !plan.own;
    };
    let timer = 0;
    let asked = 0;
    const preview = () => {
      clearTimeout(timer);
      timer = setTimeout(async () => {
        const r = rule();
        const mine = ++asked;
        try {
          const next = await get(`${base}?keep=${r.keep}&days=${r.days}`);
          if (mine !== asked) return;
          plan = { ...next, own: plan.own };
          dstatus.replaceChildren();
          show();
        } catch (err) { dstatus.replaceChildren(h("p", { class: "error", role: "alert", text: err.message })); }
      }, 200);
    };
    keep.addEventListener("input", preview);
    days.addEventListener("input", preview);
    const act = (fn) => async () => {
      try {
        await fn();
        refresh();
      } catch (err) { dstatus.replaceChildren(h("p", { class: "error", role: "alert", text: err.message })); }
    };
    const save = h("button", { type: "button", class: "secondary", id: "retention-save", text: t("retention.save"), onclick: act(async () => {
      plan = await put(base, rule());
      show();
      dstatus.replaceChildren(h("p", { class: "notice", text: t("retention.ruleSaved") }));
    }) });
    const reset = h("button", { type: "button", class: "link", id: "retention-default", text: t("retention.useDefault"), onclick: act(async () => {
      plan = await put(base, { default: true });
      keep.value = String(plan.rule.keep);
      days.value = String(plan.rule.days);
      show();
    }) });
    clean.addEventListener("click", act(async () => {
      if (!confirm(t("retention.cleanConfirm", { n: plan.remove.length, name }))) return;
      plan = { ...(await post(base + "/clean", rule())), own: plan.own };
      show();
      dstatus.replaceChildren(h("p", { class: "notice", text: t("retention.cleaned") }));
    }));
    const dialog = h("dialog", { class: "retention-dialog", id: "retention-dialog", "aria-label": t("retention.dialogTitle", { name }) },
      h("div", { class: "drawing-bar" }, h("strong", { text: t("retention.dialogTitle", { name }) }),
        h("button", { type: "button", id: "retention-close", text: t("common.close"), onclick: () => dialog.close() })),
      h("div", { class: "retention-body" },
        source,
        h("div", { class: "retention-fields" },
          h("label", { class: "field" }, h("span", { text: t("retention.keep") }), keep),
          h("label", { class: "field" }, h("span", { text: t("retention.days") }), days)),
        h("p", { class: "muted", text: t("retention.always") }),
        list,
        dstatus,
        h("div", { class: "actions-row" }, clean, save, reset)));
    dialog.addEventListener("close", () => dialog.remove());
    document.body.append(dialog);
    show();
    dialog.showModal();
  };

  // stack shows a file with older versions (ADR-0103): the newest on top,
  // the older ones half hidden behind it. The top fans them out below the
  // stack, newest first, and folds them again.
  const stack = (p) => {
    const versions = p.versions || [];
    if (!versions.length) return card(p);
    const n = versions.length + 1;
    let fan = null;
    const top = h("button", { type: "button", class: "album-stack-top", "aria-expanded": "false",
      title: t("album.versionsShow", { n }), "aria-label": `${captionOf(p.attachment.name)}: ${t("album.versionsShow", { n })}` },
      // The nearer an older version, the less it is hidden; the farthest
      // is drawn first, beneath.
      versions.slice(0, 2).map((v, i) => h("img", { class: `album-behind album-behind-${i + 1}`, src: thumbURL(v), alt: "", loading: "lazy", draggable: "false", "aria-hidden": "true" })).reverse(),
      h("img", { class: "album-front", src: thumbURL(p), alt: captionOf(p.attachment.name), loading: "lazy", draggable: "false" }),
      h("span", { class: "album-stack-count", "aria-hidden": "true", text: String(n) }));
    const li = card(p, n, top, true);
    li.classList.add("album-stack");
    top.addEventListener("click", () => {
      const open = !fan;
      top.setAttribute("aria-expanded", String(open));
      top.title = t(open ? "album.versionsHide" : "album.versionsShow", { n });
      top.setAttribute("aria-label", `${captionOf(p.attachment.name)}: ${top.title}`);
      li.classList.toggle("fanned", open);
      if (open) {
        fan = h("li", { class: "album-fan", "aria-label": t("album.versionsOf", { name: captionOf(p.attachment.name) }) },
          h("ul", { class: "album-grid" }, versions.map((v, i) => card({ ...v, notes: undefined, versionOf: true }, n - 1 - i))));
        li.after(fan);
      } else {
        fan.remove();
        fan = null;
      }
    });
    return li;
  };

  // ruleText says in a sentence which pictures a collection gathers.
  const ruleText = (rule) => [
    t(rule.match === "all" ? "album.ruleAll" : "album.ruleAny", { labels: rule.labels.join(", ") }),
    rule.items ? t("album.ruleItems") : "",
  ].filter(Boolean).join(" ");

  // ruleForm is the form of a collection's rule: its name and room when it
  // is made, its labels, whether a picture carries any or all of them, and
  // whether the labels of a picture's item count too. done receives the
  // rule; the form says what went wrong.
  const ruleForm = ({ id, rule, name, where, submit, done }) => {
    const labels = h("input", { id: id + "-labels", list: "album-label-options", required: true, value: rule ? rule.labels.join(", ") : "",
      placeholder: t("album.labelsPlaceholder"), "aria-label": t("album.ruleLabels") });
    const match = h("select", { id: id + "-match", "aria-label": t("album.ruleMatch") },
      h("option", { value: "any", text: t("album.ruleMatchAny") }), h("option", { value: "all", text: t("album.ruleMatchAll") }));
    match.value = rule?.match === "all" ? "all" : "any";
    const items = h("input", { type: "checkbox", id: id + "-items", checked: Boolean(rule?.items) });
    const form = h("form", { class: "album-new-folder album-rule", id, hidden: true }, ...[where, name, labels, match,
      h("label", { class: "check" }, items, t("album.ruleItemsCheck")),
      h("button", { type: "submit", class: "primary", text: submit })].filter(Boolean));
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (where && !where.value) return fail(new Error(t("album.chooseSpaceCollection")));
      try {
        await done({ labels: labels.value.split(",").map((l) => l.trim()).filter(Boolean), match: match.value, items: items.checked });
        status.replaceChildren();
      } catch (err) { fail(err); }
    });
    return { form, first: () => (where && !where.value ? where : name || labels) };
  };

  // The trail leads back up; a folder above the open one takes pictures
  // dropped onto it (ADR-0184). The top is no folder and takes no
  // pictures; a folder dropped onto it goes to the top of its room
  // (ADR-0200).
  const showTrail = () => {
    const step = (name, to) => h("button", { type: "button", class: "link", text: name, onclick: () => { path = path.slice(0, to); open(); } });
    trail.replaceChildren(dropOnto(step(t("album.top"), 0), null), ...path.flatMap((f, i) => [" › ", i === path.length - 1 ? h("strong", { text: f.name }) : dropOnto(step(f.name, i + 1), f)]));
  };

  // dropChoice asks, where pictures were dropped onto a folder, whether to
  // move or copy them there (ADR-0225). Move has the focus; Escape, a
  // click elsewhere or Cancel answers null.
  const dropChoice = (x, y, folder, n) => new Promise((resolve) => {
    let done = false;
    const finish = (how) => {
      if (done) return;
      done = true;
      document.removeEventListener("pointerdown", outside, true);
      document.removeEventListener("keydown", keys, true);
      box.remove();
      resolve(how);
    };
    const item = (id, text, how) => h("button", { type: "button", role: "menuitem", class: how ? "link" : "link muted", id, text, onclick: () => finish(how) });
    const move = item("album-drop-move", t("album.dropMove"), "move");
    const box = h("div", { class: "album-drop-menu", id: "album-drop-menu", role: "menu", "aria-label": t("album.dropMenu", { n, name: folder.name }) },
      move, item("album-drop-copy", t("album.dropCopy"), "copy"), item("album-drop-cancel", t("common.cancel"), null));
    const outside = (e) => { if (!box.contains(e.target)) finish(null); };
    const keys = (e) => {
      if (e.key === "Escape") { e.preventDefault(); finish(null); }
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const items = [...box.querySelectorAll("button")];
        const i = items.indexOf(document.activeElement);
        items[(i + (e.key === "ArrowDown" ? 1 : items.length - 1)) % items.length].focus();
      }
    };
    document.body.append(box);
    // The menu stands where the pictures were dropped, inside the window.
    const r = box.getBoundingClientRect();
    box.style.left = `${Math.max(8, Math.min(x, innerWidth - r.width - 8))}px`;
    box.style.top = `${Math.max(8, Math.min(y, innerHeight - r.height - 8))}px`;
    document.addEventListener("pointerdown", outside, true);
    document.addEventListener("keydown", keys, true);
    move.focus();
  });

  // dropOnto makes a folder's button take pictures dragged onto it, and
  // moves them there as "Move" does for chosen pictures, each with all its
  // versions, keeping what it is (ADR-0184, ADR-0188), or copies them, as
  // the menu shown where they are dropped asks; Ctrl or Alt copies and
  // Shift moves without it (ADR-0225). A picture already in the folder
  // stays. It also takes a folder dragged onto it, which goes into it with
  // what it holds; the top (folder null) takes only a folder, which goes to
  // the top of its room (ADR-0200).
  const dropOnto = (el, folder) => {
    if (pick) return el;
    if (folder) el.title = t("photos.dropOnFolder", { name: folder.name });
    const takes = () => (draggingFolder ? folderDrop(draggingFolder, folder) : folder && dragging ? folder : null);
    const over = (e) => {
      if (!takes()) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = dragging && (e.ctrlKey || e.altKey) ? "copy" : "move";
      el.classList.add("drop-target");
    };
    el.addEventListener("dragenter", over);
    el.addEventListener("dragover", over);
    el.addEventListener("dragleave", (e) => { if (!el.contains(e.relatedTarget)) el.classList.remove("drop-target"); });
    el.addEventListener("drop", async (e) => {
      el.classList.remove("drop-target");
      if (draggingFolder) {
        const f = draggingFolder, to = folderDrop(f, folder);
        draggingFolder = null;
        root.classList.remove("dragging-folder");
        if (!to) return;
        e.preventDefault();
        e.stopPropagation();
        if (await moveFolder(f, to)) loadFolders();
        return;
      }
      if (!dragging || !folder) return;
      e.preventDefault();
      e.stopPropagation();
      const list = dragging.filter((p) => p.item.id !== folder.id);
      dragging = null;
      root.classList.remove("dragging-pictures");
      if (!list.length) return;
      const how = e.ctrlKey || e.altKey ? "copy" : e.shiftKey ? "move" : await dropChoice(e.clientX, e.clientY, folder, list.length);
      if (!how) return;
      if (how === "copy") await together("photos.copiedTo", async (p) => { await copyTo(p, folder.id); return true; }, list);
      else await together("photos.moved", async (p) => { await moveTo(p, folder.id); return true; }, list, "photos.notMoved");
      for (const p of list) chosen.delete(key(p));
      markChosen();
      refresh();
    });
    return el;
  };

  // showCollection is the head of an open collection: which pictures it
  // gathers, and for one who may edit it a form to change that.
  const showCollection = () => {
    const c = here();
    const about = h("p", { class: "muted album-rule-text", id: "album-rule-text", text: ruleText(c.rule) });
    manage.replaceChildren();
    if (pick) return tools.replaceChildren(about);
    // Pictures are added to a collection by giving them its labels: new
    // files go into a folder chosen each time (ADR-0187).
    const adding = [
      h("button", { type: "button", class: "primary", id: "album-upload", text: t("album.upload"), title: t("album.uploadCollectionTitle", { labels: c.rule.labels.join(", ") }), onclick: () => files.click() }),
      h("button", { type: "button", class: "secondary", id: "album-add-existing", text: t("album.addExisting"), title: t("album.addExistingCollectionTitle", { labels: c.rule.labels.join(", ") }), onclick: addFromAlbum }),
      placeChoice(),
      dropHint()];
    if (!c.editable) return tools.replaceChildren(about, ...adding);
    const { form, first } = ruleForm({ id: "album-rule-form", rule: c.rule, submit: t("album.ruleSave"), done: async (rule) => {
      c.rule = (await put(`/api/album/collections/${c.id}`, rule)).rule;
      showCollection();
      load(false);
    } });
    manage.replaceChildren(
      h("button", { type: "button", class: "secondary", id: "album-rule-edit", text: t("album.ruleEdit"), onclick: () => {
        form.hidden = !form.hidden;
        if (!form.hidden) first().focus();
      } }),
      h("button", { type: "button", class: "danger", id: "album-collection-delete", text: t("album.collectionDelete"), onclick: deleteCollection }));
    tools.replaceChildren(about, form, ...adding);
  };

  // deleteCollection moves the open collection to the trash, from where
  // its page restores it (ADR-0193). It holds no pictures: they stay where
  // they are, with their labels.
  const deleteCollection = async () => {
    const c = here();
    if (!confirm(t("album.collectionDeleteConfirm", { name: c.name }))) return;
    try {
      await post(`/api/items/${encodeURIComponent(c.id)}/trash`, {});
    } catch (err) { return fail(err); }
    path = path.slice(0, -1);
    open();
    status.replaceChildren(h("p", { class: "notice", role: "status" }, t("album.folderTrashed", { name: c.name }), " ",
      c.key ? h("a", { href: "#/item/" + encodeURIComponent(c.key), id: "album-collection-restore", text: t("album.folderRestore") }) : null));
  };

  const showTools = () => {
    if (inCollection()) return showCollection();
    manage.replaceChildren();
    if (pick) return tools.replaceChildren();
    const name = h("input", { id: "album-folder-name", placeholder: t("album.folderName"), "aria-label": t("album.folderName"), required: true });
    // A folder at the top names its room here; below a folder it is made in
    // the parent's room. The room filter only preselects it.
    const where = here() ? null : h("select", { id: "album-folder-space", "aria-label": t("album.folderSpace"), required: true });
    const fillWhere = () => {
      where.replaceChildren(h("option", { value: "", text: t("album.folderSpacePick") }), ...roomOptions());
      where.value = space.value;
    };
    const make = h("form", { class: "album-new-folder", hidden: true }, ...[where, name,
      h("button", { type: "submit", class: "primary", text: t("album.create") })].filter(Boolean));
    make.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (where && !where.value) return fail(new Error(t("album.chooseSpace")));
      try {
        await post("/api/album/folders", here() ? { parent: here().id, name: name.value } : { space: where.value, name: name.value });
        status.replaceChildren();
        make.hidden = true;
        loadFolders();
      } catch (err) { fail(err); }
    });
    // A collection is made where a folder is: at the top in a room chosen
    // here, below a folder in its room.
    const cwhere = here() ? null : h("select", { id: "album-collection-space", "aria-label": t("album.collectionSpace"), required: true });
    const cname = h("input", { id: "album-collection-name", placeholder: t("album.collectionName"), "aria-label": t("album.collectionName"), required: true });
    const collect = ruleForm({ id: "album-new-collection-form", name: cname, where: cwhere, submit: t("album.create"), done: async (rule) => {
      await post("/api/album/collections", { ...(here() ? { parent: here().id } : { space: cwhere.value }), name: cname.value, ...rule });
      collect.form.hidden = true;
      loadFolders();
    } });
    // The open folder is moved with what it holds (ADR-0200).
    const moving = inFolder() && here().editable !== false ? moveForm() : null;
    // The open folder is changed beside the trail: a folder or collection
    // made in it, the folder moved or deleted. replaceChildren, unlike h,
    // writes null as text.
    manage.replaceChildren(...[
      h("button", { type: "button", class: "secondary", id: "album-new-folder", text: t("album.newFolder"), onclick: () => {
        make.hidden = !make.hidden;
        collect.form.hidden = true;
        if (moving) moving.form.hidden = true;
        if (make.hidden) return;
        if (where) fillWhere();
        (where && !where.value ? where : name).focus();
      } }),
      h("button", { type: "button", class: "secondary", id: "album-new-collection", text: t("album.newCollection"), title: t("album.newCollectionTitle"), onclick: () => {
        collect.form.hidden = !collect.form.hidden;
        make.hidden = true;
        if (moving) moving.form.hidden = true;
        if (collect.form.hidden) return;
        if (cwhere) {
          cwhere.replaceChildren(h("option", { value: "", text: t("album.collectionSpacePick") }), ...roomOptions());
          cwhere.value = space.value;
        }
        collect.first().focus();
      } }),
      moving ? h("button", { type: "button", class: "secondary", id: "album-folder-move", text: t("album.folderMove"), title: t("album.folderMoveTitle"), onclick: async () => {
        moving.form.hidden = !moving.form.hidden;
        make.hidden = collect.form.hidden = true;
        if (moving.form.hidden) return;
        await moving.fill();
        moving.first().focus();
      } }) : null,
      inFolder() && here().editable !== false ? h("button", { type: "button", class: "danger", id: "album-folder-delete", text: t("album.folderDelete"), onclick: deleteFolder }) : null].filter(Boolean));
    // Below it, pictures are added; the forms open beneath the buttons.
    tools.replaceChildren(...[make, collect.form, moving?.form,
      // At the top, the folder is asked for once the pictures are chosen.
      h("button", { type: "button", class: "primary", id: "album-upload", text: t("album.upload"), title: here() ? t("photos.uploadTitle") : t("album.uploadTopTitle"), onclick: () => files.click() }),
      here() ? h("button", { type: "button", class: "secondary", id: "album-add-existing", text: t("album.addExisting"), title: t("album.addExistingFolderTitle"), onclick: addFromAlbum }) : null,
      here() && imports.google ? h("button", { type: "button", class: "secondary", id: "album-google", text: t("photos.fromGoogle"), onclick: fromGoogle }) : null,
      here() && imports.icloud ? h("button", { type: "button", class: "secondary", id: "album-icloud", text: t("photos.fromICloud"), onclick: fromICloud }) : null,
      placeChoice(),
      dropHint()].filter(Boolean));
  };

  // deleteFolder moves the open folder with its folders and pictures to
  // the trash, from where its page restores it (ADR-0190). It says first
  // how much goes with it; the server refuses while anything outside the
  // folder uses what is in it (D8).
  const deleteFolder = async () => {
    const f = here();
    let folders = 0, pictures = 0;
    try {
      const walk = async (id, depth) => {
        pictures += (await get("/api/album?limit=1&folder=" + encodeURIComponent(id))).total || 0;
        if (depth > 20) return;
        for (const sub of (await get("/api/album/folders?parent=" + encodeURIComponent(id))).folders || []) {
          folders++;
          await walk(sub.id, depth + 1);
        }
      };
      await walk(f.id, 0);
    } catch (err) { return fail(err); }
    if (!confirm(t("album.folderDeleteConfirm", { name: f.name, folders, pictures }))) return;
    try {
      await post(`/api/items/${encodeURIComponent(f.id)}/trash`, {});
    } catch (err) { return fail(err); }
    path = path.slice(0, -1);
    open();
    status.replaceChildren(h("p", { class: "notice", role: "status" }, t("album.folderTrashed", { name: f.name }), " ",
      f.key ? h("a", { href: "#/item/" + encodeURIComponent(f.key), id: "album-folder-restore", text: t("album.folderRestore") }) : null));
  };

  // draggableFolder lets a folder one may edit be dragged onto another
  // folder or a step of the trail, to move it there (ADR-0200).
  const draggableFolder = (el, f) => {
    if (pick || f.editable === false) return el;
    el.draggable = true;
    el.addEventListener("dragstart", (e) => {
      draggingFolder = { ...f, parent: f.parent || "" };
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", f.name);
      root.classList.add("dragging-folder");
    });
    el.addEventListener("dragend", () => {
      draggingFolder = null;
      root.classList.remove("dragging-folder");
      for (const d of root.querySelectorAll(".drop-target")) d.classList.remove("drop-target");
    });
    return el;
  };

  // moveFolder moves a folder with its folders and pictures below another
  // folder or to the top of a room (ADR-0200), through the move every item
  // has; the server decides as for any item. Into another room it asks
  // first, since that room's access holds afterwards.
  const moveFolder = async (f, to) => {
    if (to.space !== f.space && !confirm(t("album.folderMoveRoom", { name: f.name, room: to.space }))) return false;
    try {
      await post(`/api/items/${encodeURIComponent(f.id)}/move`, { space: to.space, parent: to.parent });
    } catch (err) {
      fail(err);
      return false;
    }
    say(t("album.folderMoved", { name: f.name }), "notice");
    return true;
  };

  // openParent is the folder the open folder is in, "" at the top of its
  // room, undefined when the album does not know.
  const openParent = () => (path.length > 1 ? path[path.length - 2].id : here().parent);

  // moveForm offers where the open folder may go and moves it there; the
  // trail then leads to it where it went (ADR-0200).
  const moveForm = () => {
    const target = h("select", { id: "album-folder-move-target", "aria-label": t("album.folderMoveTo"), required: true },
      h("option", { value: "", text: t("album.folderMoveTo") }));
    const form = h("form", { class: "album-move-folder", id: "album-folder-move-form", hidden: true }, target,
      h("button", { type: "submit", class: "primary", text: t("photos.move") }));
    let all = [], targets = [];
    const fill = async () => {
      const f = here();
      try { all = await allFolders(); } catch (err) { return fail(err); }
      targets = folderTargets({ id: f.id, space: f.space, parent: openParent() }, all, rooms, t("album.folderMoveTop"));
      target.replaceChildren(h("option", { value: "", text: t("album.folderMoveTo") }), ...targets.map((o, i) => h("option", { value: String(i), text: o.text })));
    };
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const to = targets[+target.value];
      if (!target.value || !to) return fail(new Error(t("album.folderMoveChoose")));
      const f = here();
      if (!(await moveFolder(f, to))) return;
      // The trail leads from the top of its room through the folders above
      // it, as far as the album lists them.
      const byID = new Map(all.map((x) => [x.id, x]));
      const above = [];
      for (let at = to.parent, n = 0; at && byID.has(at) && n < 64; at = byID.get(at).parent, n++) above.unshift(byID.get(at));
      const notice = status.firstChild;
      path = [...above, { ...f, space: to.space, parent: to.parent }];
      open();
      if (notice) status.replaceChildren(notice);
    });
    return { form, fill, first: () => target };
  };

  // loadFolders shows the folders and collections right below the open
  // folder; a collection has none.
  const loadFolders = async () => {
    if (inCollection()) return folders.replaceChildren();
    const q = new URLSearchParams();
    if (here()) q.set("parent", here().id);
    else if (space.value) q.set("space", space.value);
    try {
      const { folders: list, collections = [] } = await get("/api/album/folders?" + q);
      folders.replaceChildren(...list.map((f) => h("li", {},
        draggableFolder(dropOnto(h("button", { type: "button", class: "album-folder", "data-folder": f.id, onclick: () => { path = [...path, f]; open(); } },
          h("span", { class: "album-folder-icon", "aria-hidden": "true" }), h("span", { text: f.name })), f), f))),
      ...collections.map((c) => h("li", {},
        h("button", { type: "button", class: "album-folder album-collection", "data-collection": c.id, title: ruleText(c.rule),
          onclick: () => { path = [...path, c]; open(); } },
          h("span", { class: "album-collection-icon", "aria-hidden": "true" }),
          h("span", { class: "album-collection-what" }, h("span", { text: c.name }),
            h("span", { class: "muted album-collection-labels", text: c.rule.labels.join(", ") }))))));
    } catch (err) { fail(err); }
  };

  // filters are the album's filters as they are now.
  const filters = () => {
    const q = new URLSearchParams();
    if (search.value.trim()) q.set("q", search.value.trim());
    if (kind.value) q.set("kind", kind.value);
    // A collection gathers from every room; the room filter narrows it.
    if (view === "folders" && inFolder()) q.set("folder", here().id);
    else if (space.value) q.set("space", space.value);
    if (view === "folders" && inCollection()) q.set("collection", here().id);
    if (label.value) q.set("label", label.value);
    if (view !== "folders") q.set("order", "taken");
    else if (bySlides()) q.set("order", "slides");
    if (view === "favourites") q.set("favourites", "on");
    if (view === "people" && person) q.set("person", person);
    if (view === "memories") {
      const today = new Date();
      const pad = (n) => String(n).padStart(2, "0");
      q.set("day", `${pad(today.getMonth() + 1)}-${pad(today.getDate())}`);
      q.set("to", `${today.getFullYear() - 1}-12-31`);
    }
    return q;
  };

  // showTimeline adds pictures under their months and days, or, for
  // memories, under how many years ago they were taken; a page that goes
  // on with the day shown last goes on in its list.
  const showTimeline = (pictures, append) => {
    if (!append) timeline.replaceChildren();
    const today = new Date().toISOString().slice(0, 10);
    for (const m of groups(pictures)) {
      for (const d of m.days) {
        let list = timeline.querySelector(`ul[data-day="${d.day}"]`);
        if (!list) {
          if (view === "memories") {
            const n = yearsAgo(d.day, today);
            timeline.append(h("h2", { class: "album-month", text: t("photos.yearsAgo", { n }) }));
          } else if (!timeline.querySelector(`h2[data-month="${m.month}"]`)) {
            timeline.append(h("h2", { class: "album-month", "data-month": m.month, text: monthText(m.month) }));
          }
          timeline.append(h("h3", { class: "album-day", text: dayText(d.day) }), (list = h("ul", { class: "album-grid", "data-day": d.day })));
        }
        list.append(...d.pictures.map((p) => stack(p)));
      }
    }
  };

  // load shows the pictures of the filters, or with append the next page;
  // keep asks for at least that many, so a change made after "Show more"
  // does not fold the album back to its first page.
  const load = async (append, keep = 0) => {
    const mine = ++generation;
    sortSelect.hidden = pick || view !== "folders" || !here();
    if (!append) orderHint.hidden = orderReset.hidden = true;
    if (view === "people") showPeople(mine);
    if (view === "people" && !person) {
      count.textContent = "";
      more.hidden = true;
      present.hidden = true;
      reset.hidden = !filteredNow();
      return;
    }
    if (view === "duplicates") return showDuplicates(mine);
    if (view === "map") {
      mapBox.replaceChildren(h("p", { class: "muted", text: t("photos.mapLoading") }));
      try {
        // The viewer steps through every picture of the point chosen.
        const asPicture = (pl) => ({ item: { id: pl.item, key: pl.key }, attachment: { id: pl.file, name: pl.name, type: pl.type, photo: null }, taken: pl.taken,
          video: !!pl.video, poster: !!pl.poster });
        const map = await photoMap(filters(), (list, i) => openViewer(list.map(asPicture), i));
        if (mine === generation) mapBox.replaceChildren(map);
      } catch (err) { fail(err); }
      count.textContent = "";
      more.hidden = true;
      present.hidden = true;
      reset.hidden = !filteredNow();
      return;
    }
    const q = filters();
    q.set("offset", String(append ? next : 0));
    try {
      let page = await get("/api/album?" + q);
      if (mine !== generation) return;
      const pictures = [...page.pictures];
      while (!append && keep > pictures.length && page.next) {
        q.set("offset", String(page.next));
        page = await get("/api/album?" + q);
        if (mine !== generation) return;
        pictures.push(...page.pictures);
      }
      // The cards are made knowing whether one may order them (ADR-0195).
      order = { ordered: !!page.ordered, orderable: !!page.orderable, complete: !page.next };
      if (!append) {
        grid.replaceChildren();
        shown = [];
      }
      shown.push(...pictures);
      if (view === "folders") grid.append(...pictures.map((p) => stack(p)));
      else showTimeline(pictures, append);
      next = page.next;
      more.hidden = !page.next;
      // The album's filters work as every list's do (ADR-0098).
      const on = filteredNow();
      const none = { favourites: t("photos.noFavourites"), memories: t("photos.noMemories"), people: t("faces.noPictures") }[view] || t("album.none");
      // An empty folder or collection invites pictures (ADR-0187).
      const invite = !pick && view === "folders" && Boolean(here()) && !page.total && !on;
      dropZone.hidden = !invite;
      if (invite) showDropZone();
      count.textContent = invite ? "" : page.total ? t("album.count", { n: page.total }) : on ? t("filter.none") : none;
      reset.hidden = !on;
      present.hidden = !!pick || !page.total;
      present.textContent = t("album.present", { n: page.total });
      orderHint.hidden = !reordering() || page.total < 2;
      orderReset.hidden = !reordering() || !order.ordered;
    } catch (err) { fail(err); }
  };

  // showDuplicates shows the groups of pictures that are the same file or
  // look the same, as far as the room filter narrows them (ADR-0196); while
  // the server still compares older pictures, it says how many remain.
  const showDuplicates = async (mine) => {
    more.hidden = true;
    present.hidden = true;
    reset.hidden = !filteredNow();
    count.textContent = "";
    timeline.replaceChildren(h("p", { class: "muted", id: "album-duplicates-loading", text: t("duplicates.loading") }));
    try {
      const answer = await loadDuplicates(space.value, (a) => {
        if (mine === generation) timeline.replaceChildren(h("p", { class: "muted", id: "album-duplicates-loading", text: t("duplicates.comparing", { n: a.pending }) }));
      }, () => mine === generation);
      if (mine !== generation) return;
      timeline.replaceChildren();
      shown = [];
      // Every duplicate is deleted at once after a warning, one picture of
      // each staying (ADR-0196).
      if (!pick && cleanupPlan(answer.groups, true).remove.length) {
        timeline.append(h("div", { class: "actions-row album-cleanup-row" },
          h("button", { type: "button", class: "danger", id: "album-cleanup-start", text: t("duplicates.cleanup"), title: t("duplicates.cleanupHint"), onclick: async () => {
            const list = await askCleanup(answer.groups);
            if (!list || !list.length) return;
            await together("duplicates.cleaned", async (p) => { await del(fileURL(p)); return true; }, list);
            await loadLabels().catch(() => {});
            load(false);
          } })));
      }
      for (const g of answer.groups) {
        shown.push(...g.pictures);
        timeline.append(h("h2", { class: "album-month", text: groupTitle(g) }),
          h("ul", { class: "album-grid album-duplicate-group" }, ...g.pictures.map((p) => stack(p))));
      }
      count.textContent = [answer.total ? t("duplicates.count", { n: answer.total }) : t("duplicates.none"),
        answer.pending ? t("duplicates.pending", { n: answer.pending }) : ""].filter(Boolean).join(" ");
    } catch (err) { fail(err); }
  };

  // showPeople lists the persons named on the pictures one may read, each
  // with their face, or the head of the chosen person's album; anyone who
  // may edit pictures looks for faces on those never searched.
  let scanning = false;
  const showPeople = async (mine) => {
    const find = h("button", { type: "button", class: "secondary", id: "album-faces-scan", disabled: scanning, text: t("faces.scanAll"), title: t("faces.scanAllTitle"),
      onclick: () => scanShown() });
    if (person) {
      peopleBox.replaceChildren(h("div", { class: "album-person-head" },
        h("button", { type: "button", class: "link", id: "album-people-back", text: t("faces.allPeople"), onclick: () => { person = ""; open(); } }),
        h("h2", { class: "album-person-name", id: "album-person-name", text: person })),
        personSuggestions({ person, space: space.value, tell: say, changed: () => refresh() }));
      return;
    }
    peopleBox.replaceChildren(h("p", { class: "muted", text: t("faces.loading") }));
    try {
      const q = new URLSearchParams();
      if (space.value) q.set("space", space.value);
      const { people } = await get("/api/album/people?" + q);
      if (mine !== generation) return;
      const wanted = search.value.trim().toLowerCase();
      const list = people.filter((x) => !wanted || x.name.toLowerCase().includes(wanted));
      peopleBox.replaceChildren(h("div", { class: "actions-row" }, find, h("span", { class: "muted", text: t("faces.privacy") })),
        list.length ? h("ul", { class: "album-person-grid" }, ...list.map((x) => h("li", {},
          h("button", { type: "button", class: "album-person", "data-person": x.name, onclick: () => { person = x.name; open(); } },
            personCover(x), h("span", { class: "album-person-label", text: x.name }), h("span", { class: "muted", text: t("faces.pictures", { n: x.pictures }) })))))
          : h("p", { class: "muted", id: "album-people-none", text: t("faces.noPeople") }));
    } catch (err) { fail(err); }
  };
  // scanShown looks for faces on every picture of the album, as far as
  // the filters show it, that one may edit and was never searched.
  const scanShown = async () => {
    if (scanning) return;
    scanning = true;
    let stop = false;
    const cancel = h("button", { type: "button", class: "link", id: "album-faces-stop", text: t("common.cancel"), onclick: () => { stop = true; } });
    const progress = (text) => status.replaceChildren(h("p", { class: "notice", role: "status" }, text, " ", cancel));
    try {
      const q = filters();
      q.delete("person");
      const all = [];
      for (let offset = 0; ; ) {
        q.set("offset", String(offset));
        q.set("limit", "200");
        const page = await get("/api/album?" + q);
        all.push(...page.pictures);
        if (!page.next || stop) break;
        offset = page.next;
      }
      const r = await scanAll(all, progress, () => stop);
      say(t("faces.scanned", { n: r.done, faces: r.faces }));
    } catch (err) { fail(err); }
    scanning = false;
    if (view === "people") load(false);
  };

  // refresh shows the album again after a change, as far as it was shown.
  const refresh = () => load(false, shown.length);

  // open shows the folder path ends in, or the top.
  const open = () => {
    status.replaceChildren();
    const folders_ = view === "folders";
    space.disabled = folders_ && inFolder();
    headRow.hidden = folders.hidden = tools.hidden = !folders_;
    grid.hidden = !folders_;
    timeline.hidden = folders_ || view === "map" || (view === "people" && !person);
    mapBox.hidden = view !== "map";
    peopleBox.hidden = view !== "people";
    if (peopleBox.hidden) peopleBox.replaceChildren();
    // A view not shown keeps no cards, so nothing counts or answers twice.
    if (timeline.hidden) timeline.replaceChildren();
    if (mapBox.hidden) mapBox.replaceChildren();
    if (grid.hidden) grid.replaceChildren();
    for (const b of viewBar.querySelectorAll("button")) b.setAttribute("aria-pressed", String(b.dataset.view === view));
    if (folders_) {
      showTrail();
      showTools();
      loadFolders();
    }
    load(false);
  };

  // The views (ADR-0177): folders, timeline, favourites, memories, map.
  const viewText = { folders: t("photos.viewFolders"), timeline: t("photos.viewTimeline"), favourites: t("photos.viewFavourites"),
    memories: t("photos.viewMemories"), map: t("photos.viewMap"), people: t("faces.viewPeople"), duplicates: t("duplicates.view") };
  const viewBar = h("div", { class: "album-views segmented", role: "group", "aria-label": t("photos.views") },
    VIEWS.map((v) => h("button", { type: "button", class: "segment", "data-view": v, id: "album-view-" + v, text: viewText[v], onclick: () => {
      view = v;
      person = "";
      chosen.clear();
      setChoosing(false);
      open();
    } })));

  // The chosen pictures and what is done with them together (ADR-0177).
  const bulk = h("div", { class: "album-bulk", id: "album-bulk", hidden: true, role: "region", "aria-label": t("photos.chosenTitle") });
  const chooseButton = h("button", { type: "button", class: "secondary", id: "album-choose", text: t("photos.choose"), onclick: () => setChoosing(!choosing) });
  const setChoosing = (on) => {
    choosing = on;
    if (!on) { chosen.clear(); anchor = null; }
    root.classList.toggle("choosing", on);
    chooseButton.textContent = on ? t("photos.chooseEnd") : t("photos.choose");
    chooseButton.setAttribute("aria-pressed", String(on));
    markChosen();
  };
  const together = async (what, fn, list = [...chosen.values()], failedKey = "photos.failed") => {
    let done = 0;
    const failed = [];
    for (const p of list) {
      say(t("photos.working", { n: done + 1, total: list.length }));
      try { if (await fn(p)) done++; } catch (err) { failed.push(`${captionOf(p.attachment.name)}: ${err.message}`); }
    }
    say(t(what, { n: done }) + (failed.length ? " " + t(failedKey, { n: failed.length }) + " " + failed.slice(0, 3).join("; ") : ""), failed.length ? "error" : "notice");
    return done;
  };
  // placeAll sets one place on every chosen picture one may edit.
  const placeAll = async (pl) => {
    await together("places.placedMany", async (p) => {
      if (!p.editable) return false;
      const r = await put(`${fileURL(p)}/geotag`, { lat: pl.lat, lon: pl.lon, name: pl.name || "" });
      p.attachment.photo = r.attachment.photo;
      return true;
    });
    if (view === "map") load(false);
  };
  // timeAll sets when every chosen picture one may edit was taken, to one
  // time or each shifted by the same span; a picture that tells no time
  // is not shifted (ADR-0198).
  const timeAll = async () => {
    const how = await askTime(chosen.size);
    if (!how) return;
    const done = await together("photos.timeSetMany", async (p) => {
      if (!p.editable) return false;
      let body = how;
      if (how.minutes !== undefined) {
        const taken = shiftTaken(p.attachment.photo?.taken, how.minutes);
        if (!taken) return false;
        body = { taken, offset: p.attachment.photo.offset || "" };
      }
      const r = await put(`${fileURL(p)}/taken`, body);
      p.attachment.photo = r.attachment.photo;
      return true;
    });
    if (done) refresh();
  };
  const showBulk = () => {
    bulk.hidden = !choosing;
    if (!choosing) return bulk.replaceChildren();
    const list = [...chosen.values()];
    const any = list.length > 0;
    const labelInput = h("input", { id: "album-bulk-label", list: "album-label-options", placeholder: t("photos.bulkLabel"), "aria-label": t("photos.bulkLabel") });
    const target = h("select", { id: "album-bulk-folder", "aria-label": t("photos.moveTo") }, h("option", { value: "", text: t("photos.moveTo") }));
    target.addEventListener("focus", async () => {
      if (target.options.length > 1) return;
      try { target.append(...(await allFolders()).map((f) => h("option", { value: f.id, text: f.path }))); } catch (err) { fail(err); }
    }, { once: true });
    bulk.replaceChildren(
      h("strong", { id: "album-chosen-count", text: t("photos.chosen", { n: list.length }) }),
      h("button", { type: "button", class: "link", id: "album-choose-all", text: t("photos.chooseAll"), onclick: () => { for (const p of shown) chosen.set(key(p), p); markChosen(); } }),
      h("button", { type: "button", class: "link", id: "album-choose-none", text: t("photos.chooseNone"), onclick: () => { chosen.clear(); markChosen(); } }),
      h("button", { type: "button", class: "secondary", id: "album-bulk-favourite", disabled: !any, text: t("photos.favourite"), onclick: async () => {
        await together("photos.favourited", async (p) => { if (p.favourite) return false; await setFavourite(p, true); refreshFavourite(p); return true; });
      } }),
      h("span", { class: "album-bulk-group" }, labelInput, h("button", { type: "button", class: "secondary", id: "album-bulk-label-add", disabled: !any, text: t("photos.addLabel"), onclick: async () => {
        const add = labelInput.value.split(",").map((l) => l.trim()).filter(Boolean);
        if (!add.length) return;
        await together("photos.labelled", async (p) => {
          if (!p.editable) return false;
          const labels = [...new Set([...(p.attachment.labels || []), ...add])];
          await put(`/api/items/${p.item.id}/attachments/${p.attachment.id}/labels`, { labels });
          p.attachment.labels = await labelsNow(p);
          relabel(p);
          return true;
        });
        await loadLabels();
      } })),
      h("span", { class: "album-bulk-group" }, target, h("button", { type: "button", class: "secondary", id: "album-bulk-move", disabled: !any, text: t("photos.move"), onclick: async () => {
        if (!target.value) return fail(new Error(t("photos.moveChoose")));
        await together("photos.moved", async (p) => { if (p.item.id === target.value) return false; await moveTo(p, target.value); return true; }, undefined, "photos.notMoved");
        chosen.clear();
        refresh();
      } })),
      // One place set by hand for every chosen picture (ADR-0181).
      h("span", { class: "album-bulk-group album-bulk-place" },
        placeInput({ label: t("places.bulkLabel"), id: "album-bulk-place", pick: (pl) => placeAll(pl) }).element,
        h("button", { type: "button", class: "secondary", id: "album-bulk-place-map", disabled: !any, text: t("places.onMap"), onclick: async () => {
          const pl = await pickPlace();
          if (pl) placeAll(pl);
        } })),
      // One time, or one shift, for every chosen picture (ADR-0198).
      h("button", { type: "button", class: "secondary", id: "album-bulk-time", disabled: !any, text: t("photos.timeBulk"), onclick: timeAll }),
      h("button", { type: "button", class: "secondary", id: "album-bulk-zip", disabled: !any, text: t("photos.zip"), onclick: async () => {
        say(t("photos.zipping", { n: list.length }));
        try { await zipPictures(list); say(t("photos.zipped", { n: list.length }), "notice"); } catch (err) { fail(err); }
      } }),
      h("button", { type: "button", class: "danger", id: "album-bulk-delete", disabled: !any, text: t("album.delete"), onclick: async () => {
        const may = list.filter((p) => p.editable && !p.used && !onShare(p));
        if (!confirm(t("photos.deleteConfirm", { n: may.length, kept: list.length - may.length }))) return;
        await together("photos.deleted", async (p) => { if (!p.editable || p.used || onShare(p)) return false; await del(fileURL(p)); return true; });
        chosen.clear();
        await loadLabels();
        refresh();
      } }));
  };

  // placeChoice says what imports do with where photos were taken: keep
  // it for every reader, keep it to oneself (ADR-0178) or remove it
  // (ADR-0177); this browser remembers the choice.
  const placeChoice = () => {
    const text = { "": t("photos.placeKeep"), private: t("photos.placePrivate"), strip: t("photos.placeStrip") };
    const choice = h("select", { id: "album-import-place", "aria-label": t("photos.placeOnImport"), title: t("photos.placeOnImportTitle") },
      PLACES.map((v) => h("option", { value: v, text: text[v] })));
    choice.value = importPlace();
    choice.addEventListener("change", () => importPlace(choice.value));
    return h("label", { class: "field album-import-place" }, h("span", { text: t("photos.placeOnImport") }), choice);
  };

  // dropHint says, on a line of its own below the buttons, that files are
  // also dropped or pasted where one is (ADR-0187).
  const dropHint = () => h("p", { class: "muted album-drop-hint", text: t("photos.dropHint") });

  // takeFiles adds files chosen, dropped or pasted where one is
  // (ADR-0187): into the open folder; in a collection, into a folder
  // chosen now, each picture with the collection's labels; at the top,
  // into a folder chosen now, which then opens. ZIP archives are unpacked
  // here (ADR-0177). A presentation with the pictures of its slides
  // becomes a folder of its own (ADR-0186).
  const takeFiles = async (list) => {
    if (pick || view !== "folders" || !list.length) return;
    const collection = inCollection() ? here() : null;
    const labels = collection ? collection.rule.labels : [];
    let folder = inFolder() ? here() : null;
    if (!folder) {
      // A collection proposes the folder it keeps for everyone, and keeps
      // the one chosen when one may edit it (ADR-0221).
      folder = await chooseFolder(collection
        ? { title: t("album.targetTitleCollection", { name: collection.name, n: list.length }),
          hint: t("album.targetHintCollection", { labels: labels.join(", ") }) + (collection.editable ? " " + t("album.targetKeptCollection") : ""),
          preferred: collection.folder || "" }
        : { title: t("album.targetTitle", { n: list.length }), hint: t("album.targetHint") });
      if (!folder) return;
      if (collection?.editable && collection.folder !== folder.id) {
        try {
          await put(`/api/album/collections/${collection.id}/folder`, { folder: folder.id });
          collection.folder = folder.id;
        } catch (err) { fail(err); }
      }
    }
    const top = !here();
    // Where the pictures went, said once the album shows them; a folder
    // made for a presentation, or the folder chosen at the top, opens.
    let outcome;
    let opens = false;
    try {
      const deck = await importDeck(list, folder.id, (text) => say(text), importPlace(), labels);
      if (deck) {
        const f = deck.folder;
        const made = { id: f.id, key: f.key, name: f.title, space: f.space };
        if (!collection) {
          path = top ? [folder, made] : [...path, made];
          opens = true;
        }
        outcome = [deckText(deck) + (deck.failed.length ? " " + deck.failed.slice(0, 3).join("; ") : ""), deck.failed.length ? "error" : "notice"];
      } else {
        // Duplicates of readable pictures are asked about first (ADR-0196).
        const sum = await importFiles(list, folder.id, (text) => say(text), importPlace(), labels, true);
        if (top) {
          path = [folder];
          opens = true;
        }
        const where = collection || top ? " " + t("album.uploadedTo", { folder: folder.name }) : "";
        outcome = [summaryText(sum) + where + (sum.failed.length ? " " + sum.failed.slice(0, 3).join("; ") : ""), sum.failed.length ? "error" : "notice"];
      }
    } catch (err) { fail(err); return; }
    if (opens) open();
    else await refresh();
    say(...outcome);
    if (labels.length) loadLabels().catch(() => {});
  };

  // chooseFolder asks which folder pictures go into: one of the folders
  // one may add to — preferred first, else the one chosen last in this
  // browser — or a new folder in a room (ADR-0187, ADR-0221). It resolves
  // to the folder, or to null.
  const LAST_FOLDER = "ot.albumUploadFolder";
  const chooseFolder = ({ title, hint, preferred = "" }) => new Promise((resolve) => {
    const NEW = "new";
    let list = [];
    const select = h("select", { id: "album-target-folder", "aria-label": t("album.targetFolder") }, h("option", { value: NEW, text: t("album.targetNew") }));
    const room = h("select", { id: "album-target-space", "aria-label": t("album.folderSpace") },
      h("option", { value: "", text: t("album.folderSpacePick") }), ...roomOptions());
    room.value = space.value || here()?.space || "";
    const name = h("input", { id: "album-target-name", placeholder: t("album.folderName"), "aria-label": t("album.folderName") });
    const fresh = h("div", { class: "album-target-new" }, room, name);
    const sync = () => { fresh.hidden = select.value !== NEW; };
    select.addEventListener("change", sync);
    const error = h("p", { class: "error", role: "alert", hidden: true });
    let done = false;
    const finish = (v) => {
      if (done) return;
      done = true;
      dialog.close();
      dialog.remove();
      resolve(v);
    };
    const form = h("form", { class: "album-target-form" },
      h("p", { class: "muted", text: hint }),
      h("label", { class: "field" }, h("span", { text: t("album.targetFolder") }), select),
      fresh, error,
      h("div", { class: "actions-row" },
        h("button", { type: "submit", class: "primary", id: "album-target-ok", text: t("album.targetOk") }),
        h("button", { type: "button", class: "link", id: "album-target-cancel", text: t("common.cancel"), onclick: () => finish(null) })));
    const dialog = h("dialog", { class: "album-target", id: "album-target", "aria-label": title }, h("h2", { text: title }), form);
    dialog.addEventListener("cancel", (e) => { e.preventDefault(); finish(null); });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      error.hidden = true;
      try {
        let f;
        if (select.value === NEW) {
          if (!room.value) throw new Error(t("album.chooseSpace"));
          if (!name.value.trim()) throw new Error(t("album.targetNameMissing"));
          const { item } = await post("/api/album/folders", { space: room.value, name: name.value.trim() });
          f = { id: item.id, key: item.key, name: item.title, space: item.space };
          loadFolders();
        } else {
          const c = list.find((x) => x.id === select.value);
          f = { id: c.id, key: c.key, name: c.name, space: c.space };
        }
        try { localStorage.setItem(LAST_FOLDER, f.id); } catch { /* chosen afresh next time */ }
        finish(f);
      } catch (err) {
        error.textContent = err.message;
        error.hidden = false;
      }
    });
    document.body.append(dialog);
    dialog.showModal();
    sync();
    select.focus();
    // The folders come while the dialog already stands.
    allFolders().then((all) => {
      list = all.filter((f) => f.editable);
      let last = "";
      try { last = localStorage.getItem(LAST_FOLDER) || ""; } catch { /* none */ }
      select.prepend(...list.map((f) => h("option", { value: f.id, text: f.path })));
      const first = [preferred, last].find((id) => id && list.some((f) => f.id === id));
      select.value = first || (list.length ? list[0].id : NEW);
      sync();
    }).catch((err) => { error.textContent = err.message; error.hidden = false; });
  });

  // addFromAlbum adds pictures already in the album where one is
  // (ADR-0187): into a folder as copies with their source, as inserting
  // does (ADR-0054), or moved with their versions (ADR-0188), as the
  // dialog's choice says, which this browser keeps (ADR-0222); to a
  // collection by giving them its labels, without a copy. The album stays
  // open for one picture after another.
  const ADD_MODE = "ot.albumAddMode";
  const addMode = (value) => {
    try {
      if (value) localStorage.setItem(ADD_MODE, value);
      return localStorage.getItem(ADD_MODE) === "move" ? "move" : "copy";
    } catch {
      return value === "move" ? "move" : "copy";
    }
  };
  const addFromAlbum = () => {
    const target = here();
    if (!target) return;
    const collection = Boolean(target.rule);
    const labels = collection ? target.rule.labels : [];
    const holds = (p) => {
      const own = p.attachment.labels || [];
      return target.rule.match === "all" ? labels.every((l) => own.includes(l)) : labels.some((l) => own.includes(l));
    };
    const add = async (p) => {
      if (collection) {
        if (holds(p)) return t("album.addedAlready");
        if (!p.editable) throw new Error(t("album.addNotEditable", { name: captionOf(p.attachment.name) }));
        await put(`${fileURL(p)}/labels`, { labels: [...new Set([...(p.attachment.labels || []), ...labels])] });
        p.attachment.labels = [...new Set([...(p.attachment.labels || []), ...labels])];
        return t("album.added");
      }
      if (p.item.id === target.id) return t("album.addedAlready");
      if (mode === "move") {
        await moveTo(p, target.id);
        return t("album.addMoved");
      }
      await copyTo(p, target.id);
      return t("album.added");
    };
    // Copy or move, for a folder; a collection takes neither.
    let mode = addMode();
    const hint = h("p", { class: "muted album-add-hint" });
    const showHint = () => {
      hint.textContent = collection ? t("album.addHintCollection", { labels: labels.join(", ") }) : t(mode === "move" ? "album.addHintMove" : "album.addHintFolder");
    };
    const modes = collection ? null : h("div", { class: "segmented album-add-mode", role: "radiogroup", "aria-label": t("album.addMode") },
      ["copy", "move"].map((m) => h("button", { type: "button", class: "segment", role: "radio", id: "album-add-" + m, "data-mode": m,
        "aria-checked": String(m === mode), "aria-pressed": String(m === mode), text: t(m === "move" ? "album.addModeMove" : "album.addModeCopy"),
        onclick: () => {
          mode = addMode(m);
          for (const b of modes.querySelectorAll("button")) {
            b.setAttribute("aria-checked", String(b.dataset.mode === mode));
            b.setAttribute("aria-pressed", String(b.dataset.mode === mode));
          }
          showHint();
        } })));
    showHint();
    let closed = false;
    const close = () => {
      if (closed) return;
      closed = true;
      dialog.close();
      dialog.remove();
      refresh();
      loadLabels().catch(() => {});
    };
    const title = t(collection ? "album.addTitleCollection" : "album.addTitleFolder", { name: target.name });
    const dialog = h("dialog", { class: "album-dialog album-add", id: "album-add", "aria-label": title },
      h("div", { class: "drawing-bar" }, h("strong", { text: title }),
        h("button", { type: "button", class: "primary", id: "album-add-done", text: t("album.addDone"), onclick: close })),
      h("div", { class: "album-add-head" }, modes, hint),
      h("div", { class: "album-dialog-body" }, albumBrowser({ pick: add, pickText: t("album.addOne") })));
    dialog.addEventListener("cancel", (e) => { e.preventDefault(); close(); });
    document.body.append(dialog);
    dialog.showModal();
  };

  // showDropZone fills the invitation of an empty folder or collection.
  const showDropZone = () => {
    const collection = inCollection();
    dropZone.replaceChildren(
      h("strong", { class: "album-drop-title", text: t("album.dropTitle") }),
      h("p", { class: "muted", text: collection ? t("album.dropTextCollection", { labels: here().rule.labels.join(", ") }) : t("album.dropText") }),
      h("div", { class: "actions-row" },
        h("button", { type: "button", class: "primary", id: "album-drop-choose", text: t("album.dropChoose"), onclick: () => files.click() }),
        h("button", { type: "button", class: "secondary", id: "album-drop-existing", text: t("album.addExisting"), onclick: addFromAlbum })));
  };

  // A picture pasted into the album with Ctrl+V is added where one is, as
  // a dropped file is (ADR-0187); in a text field, pasting stays text.
  const pastedName = (f, i) => {
    const d = new Date();
    const pad = (n) => String(n).padStart(2, "0");
    const stamp = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}.${pad(d.getMinutes())}.${pad(d.getSeconds())}`;
    const ext = (f.type.split("/")[1] || "png").replace("jpeg", "jpg");
    return `${t("album.pasted")} ${stamp}${i ? "-" + (i + 1) : ""}.${ext}`;
  };
  const onPaste = (e) => {
    if (!root.isConnected) return document.removeEventListener("paste", onPaste);
    if (view !== "folders" || document.querySelector("dialog[open]")) return;
    if (e.target instanceof Element && e.target.closest("input, textarea, select, [contenteditable]")) return;
    const list = [...(e.clipboardData?.files || [])].filter((f) => f.type.startsWith("image/"));
    if (!list.length) return;
    e.preventDefault();
    // A screenshot comes as "image.png"; it is named by when it was pasted.
    takeFiles(list.map((f, i) => (f.name && !/^image\.\w+$/.test(f.name) ? f : new File([f], pastedName(f, i), { type: f.type }))));
  };
  let stop = false;
  const cancelButton = () => h("button", { type: "button", class: "link", id: "album-import-cancel", text: t("common.cancel"), onclick: () => { stop = true; } });
  const progressOf = (text, link) => status.replaceChildren(h("p", { class: "muted", role: "status" }, text, " ",
    link ? h("a", { href: link, target: "_blank", rel: "noopener", text: t("photos.googleLink") }) : null, " ", cancelButton()));
  const fromGoogle = async () => {
    if (!imports.connected) {
      location.href = "/api/album/google/connect?folder=" + encodeURIComponent(here().key || here().id);
      return;
    }
    stop = false;
    try {
      const sum = await googleImport(here().id, progressOf, () => stop, importPlace());
      say(summaryText(sum), sum.failed.length ? "error" : "notice");
    } catch (err) {
      if (err.status === 401) {
        imports.connected = false;
        say(t("photos.googleAgain"), "error");
      } else fail(err);
    }
    refresh();
  };
  const fromICloud = () => {
    const folder = here();
    const address = h("input", { type: "url", id: "album-icloud-url", value: rememberedAlbum(folder.id), placeholder: "https://www.icloud.com/sharedalbum/#…", required: true,
      "aria-label": t("photos.icloudAddress") });
    const form = h("form", { class: "album-icloud", id: "album-icloud" },
      h("p", { class: "muted", text: t("photos.icloudHint") }),
      h("label", { class: "field" }, h("span", { text: t("photos.icloudAddress") }), address),
      h("div", { class: "actions-row" }, h("button", { type: "submit", class: "primary", text: t("photos.icloudImport") }),
        h("button", { type: "button", class: "link", text: t("common.cancel"), onclick: () => form.remove() })));
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      rememberedAlbum(folder.id, address.value.trim());
      form.remove();
      stop = false;
      try {
        const sum = await icloudImport(address.value.trim(), folder.id, (text) => progressOf(text), () => stop, importPlace());
        say(`${sum.name ? sum.name + ": " : ""}${summaryText(sum)}`, sum.failed.length ? "error" : "notice");
      } catch (err) { fail(err); }
      refresh();
    });
    tools.append(form);
    address.focus();
  };
  const filteredNow = () => Boolean(search.value.trim() || kind.value || label.value || ((view !== "folders" || !inFolder()) && space.value));
  const reset = h("button", { type: "button", class: "link filter-reset", id: "album-filter-reset", text: t("filter.reset"), hidden: true, onclick: () => {
    search.value = kind.value = label.value = "";
    if (!inFolder()) space.value = "";
    loadFolders();
    load(false);
  } });
  let timer = null;
  search.addEventListener("input", () => { clearTimeout(timer); timer = setTimeout(() => load(false), 250); });
  // A choice loads at once, with the text typed so far: a search still
  // waiting would only load the same again, after the reader acted on it.
  kind.addEventListener("change", () => { clearTimeout(timer); load(false); });
  label.addEventListener("change", () => { clearTimeout(timer); load(false); });
  space.addEventListener("change", () => { clearTimeout(timer); loadFolders(); load(false); });
  more.addEventListener("click", () => load(true));
  // One's favourite spaces first, as everywhere a space is chosen (ADR-0145).
  get("/api/spaces").then(({ spaces }) => {
    rooms = [...spaces.filter((s) => s.favourite), ...spaces.filter((s) => !s.favourite)];
    space.append(...roomOptions());
  }).catch(() => {});
  const root = h("div", { class: "album" },
    pick ? null : h("div", { class: "album-bar" }, viewBar),
    h("div", { class: "album-filters filter-bar", role: "search", "aria-label": t("album.title") }, search, kind, space, label, reset), options,
    headRow, tools, status, folders,
    // Right above the pictures: how many there are, their order, choosing
    // several of them and presenting them; the chosen ones' actions follow.
    h("div", { class: "album-count-row" }, count, sortSelect, pick ? null : chooseButton, present, orderReset, orderHint),
    bulk, dropZone, grid, peopleBox, timeline, mapBox, more, files);
  // Files dropped onto the album are added where one is (ADR-0177,
  // ADR-0187); a picture pasted is too.
  if (!pick) {
    document.addEventListener("paste", onPaste);
    root.addEventListener("dragover", (e) => {
      if (dragging || view !== "folders" || !e.dataTransfer?.types?.includes("Files")) return;
      e.preventDefault();
      root.classList.add("dropping");
    });
    root.addEventListener("dragleave", (e) => { if (!root.contains(e.relatedTarget)) root.classList.remove("dropping"); });
    root.addEventListener("drop", (e) => {
      root.classList.remove("dropping");
      if (dragging || view !== "folders" || !e.dataTransfer?.files?.length) return;
      e.preventDefault();
      takeFiles([...e.dataTransfer.files]);
    });
  }
  // Escape closes the open menu of a card and returns to its button.
  root.addEventListener("keydown", (e) => {
    const d = e.target instanceof Element ? e.target.closest("details.album-menu[open]") : null;
    if (e.key === "Escape" && d) { e.preventDefault(); d.open = false; d.querySelector("summary").focus(); }
  });
  loadLabels().catch(() => {});
  const start = async () => {
    if (!pick) {
      imports = await get("/api/album/imports").catch(() => imports);
      // Back from Google, or from a link: the folder named opens.
      const key = params.get("folder");
      if (key) {
        try {
          const { item } = await get("/api/items/" + encodeURIComponent(key));
          path = [{ id: item.id, key: item.key, name: item.title, space: item.space, parent: item.parent || "" }];
        } catch { /* the top */ }
      }
    }
    open();
    const outcome = params.get("photos");
    if (outcome === "google") say(t("photos.googleConnected"), "notice");
    else if (outcome === "google-denied") say(t("photos.googleDenied"), "error");
    else if (outcome === "google-error") say(t("photos.googleError"), "error");
  };
  start();
  return root;
}

// pickFromAlbum shows the album in a dialog and resolves to the chosen
// picture, or to null when it is closed.
export function pickFromAlbum() {
  return new Promise((resolve) => {
    let done = false;
    const finish = (p) => {
      if (done) return;
      done = true;
      dialog.close();
      dialog.remove();
      resolve(p);
    };
    const dialog = h("dialog", { class: "album-dialog", "aria-label": t("album.title") },
      h("div", { class: "drawing-bar" }, h("strong", { text: t("album.title") }), h("button", { type: "button", text: t("draw.cancel"), onclick: () => finish(null) })),
      h("div", { class: "album-dialog-body" }, albumBrowser({ pick: finish })));
    dialog.addEventListener("cancel", () => finish(null));
    document.body.append(dialog);
    dialog.showModal();
  });
}

// pictureFile reads a picture as a file, to be stored in another item:
// one copies only what one may read, into what one may edit.
export async function pictureFile(p) {
  const res = await fetch(fileURL(p), { credentials: "same-origin" });
  if (!res.ok) throw new Error(res.statusText);
  return new File([await res.blob()], p.attachment.name, { type: p.attachment.type });
}

// retentionCard is the album's default rule for older versions of
// pictures, in the administration (ADR-0104): the limits, whether the
// server applies them every day, a preview of what a clean-up would
// remove now, each picture by its own rule or this default, and a
// clean-up started at once.
export async function retentionCard() {
  const { settings } = await get("/api/album/retention");
  const number = (name, value, max) => h("input", { type: "number", name, id: "album-retention-" + name, min: 0, max, step: 1, value: String(value) });
  const keep = number("keep", settings.keep, 1000);
  const days = number("days", settings.days, 36500);
  const automatic = h("input", { type: "checkbox", id: "album-retention-automatic", checked: settings.automatic });
  const preview = h("div", { id: "album-retention-preview", "aria-live": "polite" });
  const status = h("div", { class: "status", role: "status" });
  const fail = (err) => status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
  const rule = () => ({ keep: Number(keep.value) || 0, days: Number(days.value) || 0 });
  // Only the latest preview shows: an earlier answer arriving late does
  // not overwrite it.
  let asked = 0;
  const showPreview = async () => {
    const r = rule();
    const mine = ++asked;
    try {
      const p = await get(`/api/album/retention/preview?keep=${r.keep}&days=${r.days}`);
      if (mine !== asked) return;
      preview.replaceChildren(...(p.versions ? [
        h("p", { text: `${t("retention.previewCount", { n: p.versions })} ${t("retention.inPictures", { n: p.stacks })}` }),
        h("table", { class: "history", id: "album-retention-pictures" }, p.pictures.map((x) => h("tr", {},
          h("td", {}, h("a", { href: "#/item/" + encodeURIComponent(x.item), text: `${x.item} ${x.title}` })),
          h("td", { text: x.name.replace(/(\.drawing)?\.[^.]*$/, "") }),
          h("td", { class: "muted", text: x.own ? t("retention.chip") : "" }),
          h("td", { text: t("retention.removes", { n: x.removes }) })))),
      ] : [h("p", { class: "muted", text: t("retention.previewNone") })]));
    } catch (err) { fail(err); }
  };
  let timer = 0;
  for (const el of [keep, days]) el.addEventListener("input", () => { clearTimeout(timer); timer = setTimeout(showPreview, 250); });
  const save = h("button", { type: "button", class: "primary", id: "album-retention-save", text: t("retention.saveDefault"), onclick: async () => {
    try {
      await put("/api/album/retention", { automatic: automatic.checked, ...rule() });
      status.replaceChildren(h("p", { class: "notice", text: t("retention.saved") }));
      showPreview();
    } catch (err) { fail(err); }
  } });
  const run = h("button", { type: "button", class: "danger", id: "album-retention-run", text: t("retention.runNow"), onclick: async () => {
    if (!confirm(t("retention.runConfirm"))) return;
    try {
      const r = await post("/api/album/retention/run");
      status.replaceChildren(h("p", { class: "notice", text: `${t("retention.ran", { n: r.versions })} ${t("retention.inPictures", { n: r.stacks })}` }));
      showPreview();
    } catch (err) { fail(err); }
  } });
  showPreview();
  return h("div", { id: "album-retention" },
    h("p", { class: "muted", text: t("retention.adminNote") }),
    h("div", { class: "retention-fields" },
      h("label", { class: "field" }, h("span", { text: t("retention.keep") }), keep),
      h("label", { class: "field" }, h("span", { text: t("retention.days") }), days)),
    h("label", { class: "check" }, automatic, t("retention.automatic")),
    h("p", { class: "muted", text: t("retention.always") }),
    h("div", { class: "actions-row" }, save, run),
    status,
    h("h3", { text: t("retention.previewTitle") }),
    preview);
}
