// The album (ADR-0054): the images and drawings of every item one may
// read, in one place. Inserting a picture into a body copies it into that
// body's item, where it takes on the item's key and access; the original
// stays where it is, unchanged. Folders (ADR-0090) are items of their own
// that hold the pictures uploaded into them; pictures carry labels of
// their own, which only narrow the album. Every picture one sees here is
// exported as PNG or JPEG, a drawing also as SVG (ADR-0173). Collections
// (ADR-0175) sit beside folders but hold nothing: their rule names labels,
// and they gather the matching pictures of every room one may read.
import { h, when } from "./dom.js";
import { t } from "./i18n.js";
import { get, post, put, del, upload } from "./api.js";
import { DRAWING } from "./drawing.js";
import { exportFormats, exportPicture } from "./picexport.js";

const fileURL = (p) => `/api/items/${p.item.id}/attachments/${p.attachment.id}`;

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
export function albumBrowser({ pick } = {}) {
  const search = h("input", { type: "search", placeholder: t("album.search"), "aria-label": t("album.search") });
  const kind = h("select", { "aria-label": t("album.kind") },
    [["", t("album.kindAll")], ["image", t("album.kindImages")], ["drawing", t("album.kindDrawings")]].map(([v, label]) => h("option", { value: v, text: label })));
  const space = h("select", { "aria-label": t("album.space") }, h("option", { value: "", text: t("album.spaceAll") }));
  // The rooms one may see, for the filter and the room of a new folder.
  let rooms = [];
  const roomOptions = () => rooms.map((s) => h("option", { value: s.key, text: `${s.key} ${s.name}` }));
  const label = h("select", { "aria-label": t("album.label"), id: "album-label" }, h("option", { value: "", text: t("album.labelAll") }));
  const options = h("datalist", { id: "album-label-options" });
  const trail = h("nav", { class: "album-trail", "aria-label": t("album.folders") });
  const tools = h("div", { class: "album-tools" });
  const folders = h("ul", { class: "album-folders" });
  const grid = h("ul", { class: "album-grid" });
  const count = h("p", { class: "muted", role: "status" });
  const more = h("button", { type: "button", text: t("album.more"), hidden: true });
  const status = h("div", { class: "status" });
  let next = 0;
  let generation = 0;
  // path is the open folder and those above it; empty at the top. Its last
  // step may be a collection, which has no folders below it.
  let path = [];
  const here = () => path[path.length - 1];
  const inCollection = () => Boolean(here()?.rule);
  const inFolder = () => Boolean(here()) && !inCollection();
  const fail = (err) => status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));

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

  // labelsOf shows a picture's labels; each narrows the album to it. One
  // who may edit the picture opens a field for them with one button, so
  // the field does not stand open on every card.
  const labelsOf = (p) => {
    const name = captionOf(p.attachment.name);
    const labels = p.attachment.labels || [];
    const chips = () => labels.map((l) => h("button", { type: "button", class: "chip album-label", text: l,
      onclick: () => { label.value = l; load(false); } }));
    if (pick || !p.editable) return labels.length ? h("p", { class: "album-labels" }, ...chips()) : null;
    const row = h("div", { class: "album-labels" });
    const show = () => row.replaceChildren(...chips(), h("button", { type: "button", class: "link album-labels-edit",
      text: labels.length ? t("album.labelsEdit") : t("album.labelsAdd"), "aria-label": t("album.labelsOf", { name }),
      onclick: edit }));
    const edit = () => {
      const input = h("input", { class: "album-labels-input", list: "album-label-options", value: labels.join(", "),
        placeholder: t("album.labelsPlaceholder"), "aria-label": t("album.labelsOf", { name }) });
      let saving = false;
      input.addEventListener("change", async () => {
        saving = true;
        try {
          await put(`/api/items/${p.item.id}/attachments/${p.attachment.id}/labels`, { labels: input.value.split(",").map((l) => l.trim()).filter(Boolean) });
          status.replaceChildren();
          await loadLabels();
          load(false);
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
    const m = menu("album-more", h("summary", { class: "album-more-button", "aria-label": t("album.moreOf", { name }), title: t("album.moreTitle") }), [
      stacked ? h("button", { type: "button", class: "link album-retention", text: t("retention.open"),
        onclick: () => { m.open = false; cleanUp(p); } }) : null,
      h("button", { type: "button", class: "link danger album-delete", text: t("album.delete"), disabled: p.used,
        onclick: async () => {
          m.open = false;
          if (!confirm(t("album.deleteConfirm", { name: p.attachment.name }))) return;
          try {
            await del(`/api/items/${p.item.id}/attachments/${p.attachment.id}`);
            await loadLabels();
            load(false);
          } catch (err) { fail(err); }
        } }),
      p.used ? h("p", { class: "muted album-used", text: t("album.used") }) : null,
    ]);
    return m;
  };

  // actions is the row at a card's foot: Insert while choosing a picture,
  // otherwise Copy and Export, of one width.
  const actions = (p) => {
    if (pick) return h("div", { class: "album-actions album-actions-pick" },
      h("button", { type: "button", class: "primary album-pick", text: t("album.insert"), onclick: () => pick(p) }));
    return h("div", { class: "album-actions" },
      h("button", { type: "button", class: "secondary album-copy", text: t("album.copy"), title: t("album.copyTitle"), onclick: async () => {
        const system = await copyPicture(p);
        status.replaceChildren(h("p", { class: "notice", role: "status", text: t(system ? "album.copied" : "album.copiedHere", { name: captionOf(p.attachment.name) }) }));
      } }),
      exportMenu(p));
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
    return h("li", { class: "album-card", "data-attachment": p.attachment.id },
      h("figure", {},
        h("div", { class: "album-thumb" }, picture || h("img", { src: fileURL(p), alt: name, loading: "lazy" })),
        h("figcaption", {},
          h("div", { class: "album-head" },
            h("span", { class: "album-name", text: name, title: name }),
            !pick && p.editable ? moreMenu(p, stacked) : null),
          h("span", { class: "album-chips" },
            p.drawing ? h("span", { class: "chip", text: t("album.drawing") }) : null,
            version ? h("span", { class: "chip album-version", text: t("album.version", { n: version }) }) : null,
            stacked && p.retention ? h("span", { class: "chip album-own-rule", text: t("retention.chip") }) : null,
            h("span", { class: "muted album-when", text: when(p.attachment.created) })),
          h("a", { href: "#/item/" + encodeURIComponent(p.item.key), class: "muted album-item", text: `${p.item.key} ${p.item.title}`,
            title: `${p.item.key} ${p.item.title}`, target: pick ? "_blank" : null, rel: pick ? "noopener" : null }))),
      labelsOf(p),
      actions(p));
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
        load(false);
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
      versions.slice(0, 2).map((v, i) => h("img", { class: `album-behind album-behind-${i + 1}`, src: fileURL(v), alt: "", loading: "lazy", "aria-hidden": "true" })).reverse(),
      h("img", { class: "album-front", src: fileURL(p), alt: captionOf(p.attachment.name), loading: "lazy" }),
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
          h("ul", { class: "album-grid" }, versions.map((v, i) => card(v, n - 1 - i))));
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

  const showTrail = () => {
    const step = (name, to) => h("button", { type: "button", class: "link", text: name, onclick: () => { path = path.slice(0, to); open(); } });
    trail.replaceChildren(step(t("album.top"), 0), ...path.flatMap((f, i) => [" › ", i === path.length - 1 ? h("strong", { text: f.name }) : step(f.name, i + 1)]));
  };

  // showCollection is the head of an open collection: which pictures it
  // gathers, and for one who may edit it a form to change that.
  const showCollection = () => {
    const c = here();
    const about = h("p", { class: "muted album-rule-text", id: "album-rule-text", text: ruleText(c.rule) });
    if (pick || !c.editable) return tools.replaceChildren(about);
    const { form, first } = ruleForm({ id: "album-rule-form", rule: c.rule, submit: t("album.ruleSave"), done: async (rule) => {
      c.rule = (await put(`/api/album/collections/${c.id}`, rule)).rule;
      showCollection();
      load(false);
    } });
    tools.replaceChildren(about,
      h("button", { type: "button", class: "secondary", id: "album-rule-edit", text: t("album.ruleEdit"), onclick: () => {
        form.hidden = !form.hidden;
        if (!form.hidden) first().focus();
      } }),
      form);
  };

  const showTools = () => {
    if (inCollection()) return showCollection();
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
    const files = h("input", { type: "file", accept: "image/png,image/jpeg,image/gif,image/webp", multiple: true, hidden: true, id: "album-files" });
    files.addEventListener("change", async () => {
      try {
        for (const f of files.files) await upload(`/api/items/${here().id}`, f);
        status.replaceChildren();
      } catch (err) { fail(err); }
      files.value = "";
      load(false);
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
    // replaceChildren, unlike h, writes null as text.
    tools.replaceChildren(...[
      h("button", { type: "button", class: "secondary", id: "album-new-folder", text: t("album.newFolder"), onclick: () => {
        make.hidden = !make.hidden;
        collect.form.hidden = true;
        if (make.hidden) return;
        if (where) fillWhere();
        (where && !where.value ? where : name).focus();
      } }),
      h("button", { type: "button", class: "secondary", id: "album-new-collection", text: t("album.newCollection"), title: t("album.newCollectionTitle"), onclick: () => {
        collect.form.hidden = !collect.form.hidden;
        make.hidden = true;
        if (collect.form.hidden) return;
        if (cwhere) {
          cwhere.replaceChildren(h("option", { value: "", text: t("album.collectionSpacePick") }), ...roomOptions());
          cwhere.value = space.value;
        }
        collect.first().focus();
      } }),
      here() ? h("button", { type: "button", class: "primary", id: "album-upload", text: t("album.upload"), onclick: () => files.click() }) : null,
      here() ? null : h("span", { class: "muted", text: t("album.uploadHint") }),
      files, make, collect.form].filter(Boolean));
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
        h("button", { type: "button", class: "album-folder", "data-folder": f.id, onclick: () => { path = [...path, f]; open(); } },
          h("span", { class: "album-folder-icon", "aria-hidden": "true" }), h("span", { text: f.name })))),
      ...collections.map((c) => h("li", {},
        h("button", { type: "button", class: "album-folder album-collection", "data-collection": c.id, title: ruleText(c.rule),
          onclick: () => { path = [...path, c]; open(); } },
          h("span", { class: "album-collection-icon", "aria-hidden": "true" }),
          h("span", { class: "album-collection-what" }, h("span", { text: c.name }),
            h("span", { class: "muted album-collection-labels", text: c.rule.labels.join(", ") }))))));
    } catch (err) { fail(err); }
  };

  // load shows the first page of the filtered album, or with append the
  // next. A later filter replaces an answer still on its way.
  const load = async (append) => {
    const mine = ++generation;
    const q = new URLSearchParams({ offset: String(append ? next : 0) });
    if (search.value.trim()) q.set("q", search.value.trim());
    if (kind.value) q.set("kind", kind.value);
    // A collection gathers from every room; the room filter narrows it.
    if (inFolder()) q.set("folder", here().id);
    else if (space.value) q.set("space", space.value);
    if (inCollection()) q.set("collection", here().id);
    if (label.value) q.set("label", label.value);
    try {
      const page = await get("/api/album?" + q);
      if (mine !== generation) return;
      if (!append) grid.replaceChildren();
      grid.append(...page.pictures.map((p) => stack(p)));
      next = page.next;
      more.hidden = !page.next;
      // The album's filters work as every list's do (ADR-0098).
      const on = filteredNow();
      count.textContent = page.total ? t("album.count", { n: page.total }) : on ? t("filter.none") : t("album.none");
      reset.hidden = !on;
    } catch (err) { fail(err); }
  };

  // open shows the folder path ends in, or the top.
  const open = () => {
    status.replaceChildren();
    space.disabled = inFolder();
    showTrail();
    showTools();
    loadFolders();
    load(false);
  };
  const filteredNow = () => Boolean(search.value.trim() || kind.value || label.value || (!inFolder() && space.value));
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
  const root = h("div", { class: "album" }, h("div", { class: "album-filters filter-bar", role: "search", "aria-label": t("album.title") }, search, kind, space, label, reset), options,
    trail, tools, status, folders, count, grid, more);
  // Escape closes the open menu of a card and returns to its button.
  root.addEventListener("keydown", (e) => {
    const d = e.target instanceof Element ? e.target.closest("details.album-menu[open]") : null;
    if (e.key === "Escape" && d) { e.preventDefault(); d.open = false; d.querySelector("summary").focus(); }
  });
  loadLabels().catch(() => {});
  open();
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
