// Importing a ZIP archive of notes into a room (ADR-0211): a folder of
// Markdown notes or a workspace export of Markdown and CSV, rehearsed by a
// dry run, run under a name that a re-run repeats to update what it made.
import { h } from "./dom.js";
import { get, ApiError } from "./api.js";
import { t } from "./i18n.js";

const errorBox = (err) => h("p", { class: "error", role: "alert", text: err instanceof Error ? err.message : String(err) });
const field = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);

// importName is the name an import gets from its archive's file name: the
// name without .zip, and without the export's own ID, which changes with
// every export, so the next export updates the same import.
export function importName(fileName) {
  let n = String(fileName || "").replace(/\.zip$/i, "").trim();
  n = n.replace(/[-_ ]?[0-9a-f]{8}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{12}$/i, "").replace(/[-_ ]?[0-9a-f]{32}$/i, "").trim();
  return n.slice(0, 80) || "notes";
}

// importQuery is the query of an import's request.
export function importQuery({ name, type, rowType, dryRun, lang }) {
  const q = new URLSearchParams({ name: String(name || "").trim() });
  if (type) q.set("type", type);
  if (rowType && rowType !== type) q.set("rowType", rowType);
  if (dryRun) q.set("dryRun", "true");
  if (lang) q.set("lang", lang);
  return q;
}

// send posts the archive as the request's body, a type a cross-site form
// cannot send.
async function send(space, file, query) {
  const res = await fetch(`/api/spaces/${encodeURIComponent(space)}/imports/notes?${query}`, {
    method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream" }, body: file,
  });
  let data = null;
  try { data = await res.json(); } catch { /* not JSON */ }
  if (!res.ok) throw new ApiError(res.status, (data && data.error) || res.statusText);
  return data;
}

// reportOf shows what a run did, or as a dry run would do.
function reportOf(rep, keyLink) {
  const counts = { pages: rep.pages, created: rep.new, changed: rep.changed, unchanged: rep.unchanged, pictures: rep.pictures, kept: rep.kept };
  const kinds = { new: t("notesImport.kind.new"), changed: t("notesImport.kind.changed"), kept: t("notesImport.kind.kept") };
  return h("div", { class: "note-import-report", id: "note-import-report", "data-dry": String(!!rep.dryRun) },
    h("p", { class: "notice", text: t(rep.dryRun ? "notesImport.dryCounts" : "notesImport.counts", counts) }),
    h("p", { class: "muted", text: rep.format === "notion" ? t("notesImport.format.workspace") : t("notesImport.format.folder") }),
    rep.examples.length ? h("ul", { class: "note-import-examples" }, rep.examples.map((x) => h("li", {},
      h("span", { class: "chip", text: kinds[x.kind] || x.kind }), " ",
      x.key ? keyLink(x.key, `${x.key} ${x.title}`) : x.title, " ", h("span", { class: "muted", text: x.path })))) : "",
    rep.problems.length ? h("details", { class: "note-import-problems", open: rep.dryRun },
      h("summary", { text: t("notesImport.problems", { n: rep.problems.length }) }),
      h("ul", {}, rep.problems.map((p) => h("li", { class: "muted", text: p })))) : "");
}

// noteImportCard is the form of an import: into one room, or with a room
// to choose among those the caller administers. types are the types with
// a text; keyLink makes a link to an item by its key.
export function noteImportCard({ spaces, space, types, lang, keyLink }) {
  const status = h("div", { class: "status", id: "note-import-status" });
  const earlier = h("datalist", { id: "note-import-names" });
  const earlierText = h("p", { class: "muted", id: "note-import-earlier" });
  const fileInput = h("input", { type: "file", name: "archive", accept: ".zip,application/zip", id: "note-import-file", required: true });
  const nameInput = h("input", { name: "name", id: "note-import-name", maxlength: 80, list: "note-import-names", required: true });
  const typeOptions = (chosen) => types.map((ty) => h("option", { value: ty.name, text: ty.name, selected: ty.name === chosen }));
  const pageDefault = types.some((ty) => ty.name === "page") ? "page" : types[0]?.name;
  const typeSelect = h("select", { name: "type", id: "note-import-type" }, typeOptions(pageDefault));
  const rowSelect = h("select", { name: "rowType", id: "note-import-row-type" }, typeOptions(pageDefault));
  const spaceSelect = space ? null : h("select", { name: "space", id: "note-import-space", required: true },
    spaces.map((sp) => h("option", { value: sp.key, text: `${sp.name} (${sp.key})` })));
  const room = () => space || spaceSelect.value;
  // The earlier imports of the room are offered as names to repeat.
  const loadEarlier = async () => {
    earlier.replaceChildren();
    earlierText.textContent = "";
    if (!room()) return;
    try {
      const { imports } = await get(`/api/spaces/${encodeURIComponent(room())}/imports`);
      earlier.replaceChildren(...imports.map((i) => h("option", { value: i.name })));
      if (imports.length) earlierText.textContent = t("notesImport.earlier", { list: imports.map((i) => `${i.name} (${i.pages})`).join(", ") });
    } catch { /* the names are a help only */ }
  };
  fileInput.addEventListener("change", () => {
    if (fileInput.files[0] && !nameInput.value) nameInput.value = importName(fileInput.files[0].name);
  });
  spaceSelect?.addEventListener("change", loadEarlier);
  const go = (dryRun) => async () => {
    status.replaceChildren();
    const file = fileInput.files[0];
    if (!file) return status.replaceChildren(errorBox(t("notesImport.chooseFile")));
    if (!nameInput.value.trim()) nameInput.value = importName(file.name);
    for (const b of buttons) b.disabled = true;
    status.replaceChildren(h("p", { class: "muted", text: t("notesImport.running") }));
    try {
      const rep = await send(room(), file, importQuery({ name: nameInput.value, type: typeSelect.value, rowType: rowSelect.value, dryRun, lang }));
      status.replaceChildren(reportOf(rep, keyLink));
      if (!dryRun) loadEarlier();
    } catch (err) {
      status.replaceChildren(errorBox(err));
    } finally {
      for (const b of buttons) b.disabled = false;
    }
  };
  const buttons = [
    h("button", { type: "button", class: "secondary", id: "note-import-dry-run", text: t("notesImport.dryRun"), onclick: go(true) }),
    h("button", { type: "button", class: "primary", id: "note-import-run", text: t("notesImport.run"), onclick: go(false) }),
  ];
  loadEarlier();
  return h("div", { class: "card note-import", id: "note-import" },
    h("p", { class: "muted", text: t("notesImport.help") }),
    spaceSelect ? field(t("notesImport.space"), spaceSelect) : "",
    field(t("notesImport.file"), fileInput),
    field(t("notesImport.name"), nameInput), earlier,
    h("p", { class: "muted", text: t("notesImport.nameHelp") }),
    earlierText,
    field(t("notesImport.type"), typeSelect),
    field(t("notesImport.rowType"), rowSelect),
    h("p", { class: "muted", text: t("notesImport.typeHelp") }),
    h("div", { class: "actions-row" }, buttons),
    status);
}
