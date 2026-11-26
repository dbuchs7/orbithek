// A slide deck in a text (ADR-0260): a block of the kind slides keeps the
// slides as they were when it was saved — copies of the pictures of an
// album folder or collection, made here with their speaker's notes and
// kept as files of the item (ADR-0054, ADR-0055) — and names the folder
// they came from. The reader steps through them in the text, shows them
// full screen, or presents them: the album's presenter's view and
// presentation window (ADR-0176), which return to the text when the
// presentation ends. When the folder's slides differ from those kept, the
// deck says so, and whoever edits the text updates it from the folder.
//
// A deck written before slides were kept names only its folder; it shows
// the folder as it is now until the text is next edited.
//
// The page shares what this module needs once (configureSlides): its
// dialog. The decisions that need no browser are in slidesparts.js.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get, post, put, upload } from "./api.js";
import { slidesOf } from "./present-parts.js";
import { startPresentation } from "./present.js";
import { CHOICE_DEPTH, MAX_CHOICES, choiceLabel, deckChanged, deckFilter, keptSlides, keyStep, slideURLs, turnBetween } from "./slidesparts.js";

let shared = null;

// configureSlides takes what the page shares: formDialog(title, id,
// fields, submit, ok), the page's dialog.
export function configureSlides(helpers) { shared = helpers; }
export const slidesReady = () => shared !== null;

// albumPictures reads every picture the album lists for a query, page by
// page.
async function albumPictures(q) {
  const all = [];
  let offset = 0;
  for (;;) {
    q.set("limit", "200");
    q.set("offset", String(offset));
    const page = await get("/api/album?" + q);
    all.push(...page.pictures);
    if (!page.next) return all;
    offset = page.next;
  }
}

// sourceDeck reads the folder or collection a deck comes from and its
// pictures in its order of slides. It answers null when the reader does
// not read it or it is gone: unreadable is non-existent.
async function sourceDeck(ref) {
  if (!ref) return null;
  let target;
  try {
    target = (await get("/api/items/" + encodeURIComponent(ref))).item;
  } catch {
    return null;
  }
  if (!target || target.presence && target.presence !== "live") return null;
  const q = deckFilter(target);
  q.set("order", "slides");
  return { target, pictures: await albumPictures(q) };
}

// keptPictures reads the slides a deck keeps: the pictures of its item, in
// the deck's order.
async function keptPictures(host, files) {
  return keptSlides(await albumPictures(new URLSearchParams({ folder: host })), files);
}

// hostRef is the item whose text is shown now, which a presentation
// started from it returns to: the key or ID in the address.
function hostRef() {
  const m = /^#\/item\/([A-Za-z0-9-]{1,64})(?:[?/]|$)/.exec(location.hash);
  return m ? m[1] : "";
}

// hostID is the ID of the item whose text holds a deck: here's, or the
// item of the address.
export async function hostID(here) {
  if (here?.id) return here.id;
  return (await get("/api/items/" + encodeURIComponent(hostRef()))).item.id;
}

// keepSlides copies the slides of a deck's source into the item host, as
// they are now: each picture's file under its name, as a copy of it, with
// its speaker's notes, a video's poster and a turn made by hand. progress
// is told (done, total) as it goes. It answers the copies' IDs in the
// order of the slides.
export async function keepSlides(ref, host, progress = () => {}) {
  const deck = await sourceDeck(ref);
  if (!deck) throw new Error(t("slides.unavailable"));
  if (!deck.pictures.length) throw new Error(t("slides.empty"));
  const files = [];
  progress(0, deck.pictures.length);
  for (const p of deck.pictures) {
    const url = `/api/items/${p.item.id}/attachments/${p.attachment.id}`;
    const res = await fetch(url, { credentials: "same-origin" });
    if (!res.ok) throw new Error(t("slides.copyFailed", { name: p.attachment.name }));
    const file = new File([await res.blob()], p.attachment.name, { type: p.attachment.type });
    const copy = await upload(`/api/items/${host}`, file, { item: p.item.id, attachment: p.attachment.id }, p.video ? { video: "1" } : {});
    const copyURL = `/api/items/${host}/attachments/${copy.id}`;
    if (p.notes) await put(copyURL + "/notes", { notes: p.notes });
    if (p.video && p.poster) {
      const poster = await fetch(url + "/thumbnail?edge=1600", { credentials: "same-origin" });
      if (poster.ok) {
        await fetch(copyURL + "/poster", { method: "PUT", credentials: "same-origin",
          headers: { "Content-Type": "application/octet-stream" }, body: await poster.blob() });
      }
    }
    const photo = p.attachment.photo;
    const by = photo?.orientationManual ? turnBetween(copy.photo?.orientation, photo.orientation) : 0;
    if (by) await post(copyURL + "/turn", { by });
    files.push(copy.id);
    progress(files.length, deck.pictures.length);
  }
  return files;
}

// slidesElement shows a slide deck. While reading, the reader steps
// through it, shows it full screen and presents it; while editing
// (editing), the slides show without presenting, beside a button that
// changes the deck (onChange) and, when its folder's slides differ, one
// that updates it (onUpdate). here names the item the text is on;
// deckIndex() answers the deck's place among the decks of the text.
export function slidesElement(block, { here = null, deckIndex = () => 0, editing = false, onChange = null, onUpdate = null } = {}) {
  const title = h("span", { class: "pm-slides-title" }, h("strong", { text: t("slides.deck") }));
  const tools = h("span", { class: "pm-slides-tools" });
  const bar = h("div", { class: "pm-slides-bar" }, title, tools);
  const notice = h("p", { class: "pm-slides-newer", role: "status", hidden: true });
  const stage = h("div", { class: "pm-slides-stage", tabindex: "0", role: "group", "aria-roledescription": t("slides.deck"), "aria-busy": "true" },
    h("p", { class: "muted", text: "…" }));
  const strip = h("ol", { class: "pm-slides-strip", "aria-label": t("slides.strip") });
  const caption = block.text ? h("figcaption", { class: "pm-slides-caption", text: block.text }) : null;
  const root = h("figure", { class: "pm-slides", contenteditable: "false" }, bar, notice, stage, strip, caption);
  if (editing && onChange) {
    tools.append(h("button", { type: "button", class: "link pm-slides-change", text: t("slides.change"), onclick: onChange }));
  }
  const unavailable = (key) => {
    stage.replaceChildren(h("p", { class: "muted pm-slides-message", text: t(key) }));
    stage.removeAttribute("aria-busy");
  };
  const kept = !!block.files?.length;
  const albumLink = (target) => h("a", { href: target.type === "collection" ? `#/item/${encodeURIComponent(target.key)}` : `#/album?folder=${encodeURIComponent(target.key)}`,
    class: "pm-slides-album", title: t("slides.openAlbum"), text: target.title || target.key });

  (async () => {
    let pictures, source = null, host = "";
    if (kept) {
      host = await hostID(here);
      pictures = await keptPictures(host, block.files);
      source = await sourceDeck(block.ref).catch(() => null);
    } else {
      source = await sourceDeck(block.ref);
      if (!source) return unavailable("slides.unavailable");
      pictures = source.pictures;
    }
    const slides = slidesOf(pictures);
    title.append(" ", h("span", { class: "muted" }, t("slides.count", { n: slides.length }), source ? [" · ", albumLink(source.target)] : null));
    // The folder's slides differ from those kept: the deck says so, and
    // while editing offers to update it.
    if (kept && source && deckChanged(pictures, source.pictures)) {
      notice.hidden = false;
      notice.append(t(editing ? "slides.newerEdit" : "slides.newer"));
      if (editing && onUpdate) notice.append(" ", h("button", { type: "button", class: "link pm-slides-update", text: t("slides.update"), title: t("slides.updateTitle"), onclick: onUpdate }));
    }
    if (!slides.length) return unavailable(kept ? "slides.unavailable" : "slides.empty");
    let index = 0;
    const counter = h("span", { class: "pm-slides-counter", "aria-live": "polite" });
    const prev = h("button", { type: "button", class: "secondary pm-slides-prev", title: t("slides.prev"), "aria-label": t("slides.prev"), text: "‹", onclick: () => go(index - 1) });
    const next = h("button", { type: "button", class: "secondary pm-slides-next", title: t("slides.next"), "aria-label": t("slides.next"), text: "›", onclick: () => go(index + 1) });
    tools.prepend(prev, counter, next);
    if (!editing) {
      const deck = () => (kept ? new URLSearchParams({ deck: host, n: String(deckIndex()) }) : deckFilter(source.target));
      tools.append(
        h("button", { type: "button", class: "primary pm-slides-present", title: t("slides.presentTitle"), text: t("slides.present"),
          onclick: () => startPresentation(deck(), hostRef()) }),
        h("button", { type: "button", class: "secondary pm-slides-full", title: t("slides.fullTitle"), text: t("slides.full"),
          onclick: () => (document.fullscreenElement ? document.exitFullscreen() : stage.requestFullscreen?.())?.catch?.(() => {}) }));
    }
    const thumbs = slides.map((s, i) => {
      const { thumb } = slideURLs(s);
      return h("li", {}, h("button", { type: "button", class: "pm-slides-thumb", "aria-label": t("present.goTo", { n: i + 1, name: s.name }), title: s.name, onclick: () => go(i) },
        thumb ? h("img", { src: thumb, alt: "", loading: "lazy", draggable: "false" }) : h("span", { class: "present-thumb-video", "aria-hidden": "true" }),
        h("span", { text: String(i + 1) })));
    });
    strip.replaceChildren(...thumbs);
    const show = () => {
      const s = slides[index];
      const { file, stage: picture } = slideURLs(s);
      const media = s.video
        ? h("video", { class: "pm-slides-media", src: file, controls: true, preload: "metadata", ...(s.poster ? { poster: picture } : {}), "aria-label": s.name })
        : h("img", { class: "pm-slides-media", src: picture, alt: s.name, draggable: "false" });
      stage.replaceChildren(media);
      stage.removeAttribute("aria-busy");
      counter.textContent = t("slides.counter", { n: index + 1, total: slides.length });
      prev.disabled = index === 0;
      next.disabled = index === slides.length - 1;
      thumbs.forEach((li, i) => li.firstChild.classList.toggle("on", i === index));
      thumbs[index].firstChild.scrollIntoView?.({ block: "nearest", inline: "nearest" });
    };
    const go = (i) => {
      const to = Math.min(slides.length - 1, Math.max(0, i));
      if (to !== index) { index = to; show(); }
    };
    stage.addEventListener("keydown", (e) => {
      const to = keyStep(e.key, index, slides.length);
      if (to < 0 || e.target instanceof HTMLVideoElement && e.key === " ") return;
      e.preventDefault();
      go(to);
    });
    show();
  })().catch(() => unavailable("slides.unavailable"));
  return root;
}

// folderChoices lists the folders the reader reads, down to CHOICE_DEPTH
// below the top, and the collections, each with its label, at most
// MAX_CHOICES.
async function folderChoices() {
  const out = [];
  const visit = async (parent, path, depth) => {
    if (out.length >= MAX_CHOICES) return;
    const { folders = [], collections = [] } = await get("/api/album/folders" + (parent ? "?parent=" + encodeURIComponent(parent) : ""));
    for (const c of collections) {
      if (out.length < MAX_CHOICES) out.push({ id: c.id, label: choiceLabel({ space: c.space, path: [...path, c.name], collection: true }, t("slides.collection")) });
    }
    for (const f of folders) {
      if (out.length >= MAX_CHOICES) return;
      out.push({ id: f.id, label: choiceLabel({ space: f.space, path: [...path, f.name] }) });
      if (depth + 1 < CHOICE_DEPTH) await visit(f.id, [...path, f.name], depth + 1);
    }
  };
  await visit("", [], 0);
  return out.sort((a, b) => a.label.localeCompare(b.label));
}

// pickSlides asks for the folder or collection of a slide deck and its
// caption, starting from initial (a block), and answers { ref, text }, or
// null when cancelled.
export async function pickSlides(initial = {}, helpers = shared) {
  const choices = await folderChoices();
  const folder = h("select", { id: "slides-pick-folder", name: "folder", required: true },
    h("option", { value: "", text: t("slides.chooseFolder") }),
    choices.map((c) => h("option", { value: c.id, text: c.label })));
  if (initial.ref && !choices.some((c) => c.id === initial.ref)) folder.append(h("option", { value: initial.ref, text: t("slides.current") }));
  folder.value = initial.ref || "";
  const preview = h("div", { class: "slides-pick-preview", id: "slides-pick-preview", "aria-live": "polite" });
  const showPreview = async () => {
    preview.replaceChildren();
    if (!folder.value) return;
    const deck = await sourceDeck(folder.value).catch(() => null);
    if (!deck || folder.value !== deck.target.id) return;
    const slides = slidesOf(deck.pictures);
    preview.replaceChildren(h("p", { class: "muted", text: t("slides.count", { n: slides.length }) }),
      h("div", { class: "slides-pick-thumbs" }, slides.slice(0, 8).map((s) => {
        const { thumb } = slideURLs(s);
        return thumb ? h("img", { src: thumb, alt: s.name, title: s.name, loading: "lazy" }) : h("span", { class: "present-thumb-video", title: s.name });
      })));
  };
  folder.addEventListener("change", showPreview);
  showPreview();
  const caption = h("input", { id: "slides-pick-caption", name: "caption", type: "text", value: initial.text || "", maxlength: 500 });
  const choice = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);
  return helpers.formDialog(t("slides.insert"), "slides-dialog", [
    choice(t("slides.folder"), folder),
    preview,
    choice(t("slides.caption"), caption),
    h("p", { class: "muted", text: t("slides.hint") }),
    choices.length ? null : h("p", { class: "muted", id: "slides-pick-none", text: t("slides.noFolders") }),
  ].filter(Boolean), async () => {
    if (!folder.value) throw new Error(t("slides.chooseFolder"));
    return { ref: folder.value, text: caption.value.trim() };
  }, t("slides.ok"));
}
