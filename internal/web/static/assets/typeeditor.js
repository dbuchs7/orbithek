// The type editor (ADR-0201): a form for a type's name, whether its items
// have a text, its fields with their kinds and options, its workflow and
// the text a new item begins with. Hand-written JSON stays, folded away,
// for what the form does not offer. Administrators use it under
// Administration → Item types for every type; a room's administrators in
// the room's settings for the room's own types. What it sends is checked
// again by the server.
import { h, mount } from "./dom.js";
import { post, del } from "./api.js";
import { t } from "./i18n.js";
import {
  fieldKinds, presetWorkflow, emptyDraft, newField, draftOf, definitionOf, problems, moved, renamedState,
} from "./typeparts.js";

const uniqueKinds = ["text", "url", "number"];

// kindName names a field kind in the reader's language.
export function kindName(kind) {
  const names = {
    text: t("kind.text"), number: t("kind.number"), bool: t("kind.bool"), date: t("kind.date"), datetime: t("kind.datetime"),
    user: t("kind.user"), enum: t("kind.enum"), multi: t("kind.multi"), url: t("kind.url"), item: t("kind.item"),
    rating: t("kind.rating"), file: t("kind.file"),
  };
  return names[kind] || kind;
}

// typeEditor returns the editor of a type (or of a new one, from a
// template if given). space is the room a new type belongs to ("" for the
// whole installation); with spaces, an administrator chooses it. types are
// the types the reader sees, for the targets of item fields. onSaved gets
// the type as the server answers it; onCancel closes the editor.
export function typeEditor({ type = null, template = null, space = "", spaces = null, types = [], onSaved = () => {}, onCancel = null }) {
  let draft = type ? draftOf(type) : template ? draftOf(template, space) : emptyDraft(space);
  const root = h("section", { class: "card type-editor", id: "type-editor" });
  const status = h("div", { class: "status", role: "status" });

  const render = () => {
    mount(root,
      h("h2", { text: draft.existing ? t("types.editTitle", { name: draft.name }) : t("types.newTitle") }),
      h("p", { class: "muted", text: t("types.help") }),
      basics(),
      fieldsPart(),
      workflowPart(),
      draft.body ? startPart() : null,
      jsonPart(),
      status,
      h("div", { class: "actions" },
        h("button", { type: "button", class: "primary", id: "type-save", text: draft.existing ? t("types.saveVersion") : t("types.create"), onclick: save }),
        onCancel ? h("button", { type: "button", class: "secondary", text: t("common.cancel"), onclick: onCancel }) : null),
    );
  };

  const text = (value, attrs, set) => {
    const el = h("input", { value: value ?? "", ...attrs });
    el.addEventListener("input", () => set(el.value));
    return el;
  };
  const check = (on, label, set, attrs = {}) => {
    const el = h("input", { type: "checkbox", checked: on, ...attrs });
    el.addEventListener("change", () => set(el.checked));
    return h("label", { class: "check" }, el, " ", label);
  };
  const small = (label, onclick, attrs = {}) => h("button", { type: "button", class: "link", text: label, onclick, ...attrs });

  function basics() {
    const where = !draft.existing && spaces
      ? h("label", { class: "field" }, h("span", { text: t("types.belongsTo") }),
        (() => {
          const sel = h("select", { name: "type-space", id: "type-space" },
            h("option", { value: "", text: t("types.wholeInstallation"), selected: !draft.space }),
            spaces.map((sp) => h("option", { value: sp.key, text: t("types.room", { name: sp.name, key: sp.key }), selected: sp.key === draft.space })));
          sel.addEventListener("change", () => { draft.space = sel.value; });
          return sel;
        })())
      : h("p", { class: "muted", id: "type-belongs", text: draft.space ? t("types.ofRoom", { key: draft.space }) : t("types.ofInstallation") });
    return h("div", { class: "type-basics" },
      h("label", { class: "field" }, h("span", { text: t("types.name") }),
        text(draft.name, { name: "type-name", id: "type-name", required: true, disabled: draft.existing }, (v) => { draft.name = v; })),
      where,
      check(draft.body, t("types.body"), (on) => { draft.body = on; render(); }, { id: "type-body" }));
  }

  function fieldsPart() {
    const rows = draft.fields.map((f, i) => {
      const kind = h("select", { name: "field-kind", "aria-label": t("types.kind") },
        fieldKinds.map((k) => h("option", { value: k, text: kindName(k), selected: k === f.kind })));
      kind.addEventListener("change", () => { f.kind = kind.value; render(); });
      const values = f.kind === "enum" || f.kind === "multi"
        ? h("label", { class: "field" }, h("span", { text: t("types.values") }),
          (() => {
            const area = h("textarea", { name: "field-values", rows: 3, placeholder: t("types.valuesHint") });
            area.value = (f.values || []).join("\n");
            area.addEventListener("input", () => { f.values = area.value.split("\n"); });
            return area;
          })())
        : null;
      const targets = f.kind === "item"
        ? h("fieldset", { class: "type-targets" }, h("legend", { text: t("types.targets") }),
          h("p", { class: "muted", text: t("types.targetsHint") }),
          types.map((ty) => check((f.targets || []).includes(ty.name), ty.name, (on) => {
            f.targets = on ? [...(f.targets || []), ty.name] : (f.targets || []).filter((n) => n !== ty.name);
          })))
        : null;
      return h("li", { class: "type-field", "data-field": String(i) },
        h("div", { class: "type-field-main" },
          h("label", { class: "field" }, h("span", { text: t("types.fieldName") }),
            text(f.name, { name: "field-name", required: true }, (v) => { f.name = v; })),
          h("label", { class: "field" }, h("span", { text: t("types.kind") }), kind),
          h("span", { class: "type-field-moves" },
            small("↑", () => { draft.fields = moved(draft.fields, i, -1); render(); }, { "aria-label": t("types.up"), disabled: i === 0 }),
            small("↓", () => { draft.fields = moved(draft.fields, i, 1); render(); }, { "aria-label": t("types.down"), disabled: i === draft.fields.length - 1 }),
            small(t("types.remove"), () => { draft.fields.splice(i, 1); render(); }, { class: "link danger" }))),
        h("div", { class: "type-field-options" },
          check(f.required, t("types.required"), (on) => { f.required = on; }),
          check(f.recommended, t("types.recommended"), (on) => { f.recommended = on; }),
          uniqueKinds.includes(f.kind) ? check(f.unique, t("types.unique"), (on) => { f.unique = on; }) : null,
          f.kind === "rating" ? h("span", { class: "muted", text: t("types.ratingHint") }) : null,
          f.kind === "file" ? h("span", { class: "muted", text: t("types.fileHint") }) : null),
        values, targets);
    });
    return h("fieldset", { class: "type-fields" }, h("legend", { text: t("types.fields") }),
      rows.length ? h("ol", { class: "type-field-list" }, rows) : h("p", { class: "muted", text: t("types.noFields") }),
      h("button", { type: "button", class: "secondary", id: "type-add-field", text: t("types.addField"),
        onclick: () => { draft.fields.push(newField()); render(); root.querySelector(".type-field:last-child [name=field-name]")?.focus(); } }));
  }

  function workflowPart() {
    const w = draft.workflow;
    const toggle = check(Boolean(w), t("types.hasWorkflow"), (on) => { draft.workflow = on ? presetWorkflow() : null; render(); }, { id: "type-workflow" });
    if (!w) return h("fieldset", { class: "type-workflow" }, h("legend", { text: t("types.workflow") }), toggle);
    const states = w.states.map((s, i) => {
      const input = h("input", { value: s, name: "state-name", "aria-label": t("types.stateName"), required: true });
      // A state renamed keeps its transitions.
      input.addEventListener("change", () => { draft.workflow = renamedState(draft.workflow, w.states[i], input.value.trim()); render(); });
      return h("li", { class: "type-state" }, input,
        small("↑", () => { w.states = moved(w.states, i, -1); render(); }, { "aria-label": t("types.up"), disabled: i === 0 }),
        small("↓", () => { w.states = moved(w.states, i, 1); render(); }, { "aria-label": t("types.down"), disabled: i === w.states.length - 1 }),
        small(t("types.remove"), () => {
          const gone = w.states[i];
          w.states.splice(i, 1);
          w.transitions = w.transitions.map((tr) => ({ ...tr, from: (tr.from || []).filter((x) => x !== gone), to: tr.to === gone ? "" : tr.to }));
          render();
        }, { class: "link danger" }));
    });
    const initial = h("select", { name: "initial", id: "type-initial" }, w.states.map((s) => h("option", { value: s, text: s, selected: s === w.initial })));
    initial.addEventListener("change", () => { w.initial = initial.value; });
    const transitions = w.transitions.map((tr, i) => {
      const to = h("select", { name: "transition-to", "aria-label": t("types.to") },
        h("option", { value: "", text: "—" }), w.states.map((s) => h("option", { value: s, text: s, selected: s === tr.to })));
      to.addEventListener("change", () => { tr.to = to.value; });
      return h("li", { class: "type-transition" },
        h("label", { class: "field" }, h("span", { text: t("types.transitionName") }), text(tr.name, { name: "transition-name" }, (v) => { tr.name = v; })),
        h("fieldset", { class: "type-from" }, h("legend", { text: t("types.from") }),
          w.states.map((s) => check((tr.from || []).includes(s), s, (on) => { tr.from = on ? [...(tr.from || []), s] : (tr.from || []).filter((x) => x !== s); }))),
        h("label", { class: "field" }, h("span", { text: t("types.to") }), to),
        small(t("types.remove"), () => { w.transitions.splice(i, 1); render(); }, { class: "link danger" }));
    });
    return h("fieldset", { class: "type-workflow" }, h("legend", { text: t("types.workflow") }), toggle,
      h("p", {}, h("button", { type: "button", class: "secondary", id: "type-preset", text: t("types.preset"),
        onclick: () => { draft.workflow = presetWorkflow(); render(); } })),
      h("h4", { text: t("types.states") }), h("p", { class: "muted", text: t("types.statesHint") }),
      h("ol", { class: "type-states" }, states),
      small(t("types.addState"), () => { w.states.push(t("types.newState", { n: w.states.length + 1 })); render(); }, { id: "type-add-state" }),
      h("label", { class: "field" }, h("span", { text: t("types.initial") }), initial),
      h("h4", { text: t("types.transitions") }),
      h("ol", { class: "type-transitions" }, transitions),
      small(t("types.addTransition"), () => { w.transitions.push({ name: "", from: [], to: "" }); render(); }, { id: "type-add-transition" }));
  }

  function startPart() {
    const area = h("textarea", { name: "type-start", id: "type-start", rows: 5 });
    area.value = draft.startText;
    area.addEventListener("input", () => { draft.startText = area.value; });
    return h("label", { class: "field" }, h("span", { text: t("types.start") }), area, h("small", { class: "muted", text: t("types.startHint") }));
  }

  function jsonPart() {
    const area = h("textarea", { name: "type-json", id: "type-json", rows: 10, spellcheck: "false" });
    const details = h("details", { class: "type-json" }, h("summary", { text: t("types.json") }),
      h("p", { class: "muted", text: t("types.jsonHelp") }), area,
      h("button", { type: "button", class: "secondary", id: "type-json-apply", text: t("types.jsonApply"), onclick: () => {
        let def;
        try {
          def = JSON.parse(area.value);
        } catch (err) {
          status.replaceChildren(h("p", { class: "error", role: "alert", text: t("admin.badJson", { error: err.message }) }));
          return;
        }
        const existing = draft.existing;
        draft = draftOf(def, existing ? draft.space : def.space ?? draft.space);
        draft.existing = existing;
        render();
      } }));
    details.addEventListener("toggle", () => { if (details.open) area.value = JSON.stringify(definitionOf(draft), null, 2); });
    return details;
  }

  async function save() {
    const wrong = problems(draft);
    if (wrong.length) {
      status.replaceChildren(h("ul", { class: "error", role: "alert" }, wrong.map((p) => h("li", { text: t(p.key, p.params || {}) }))));
      return;
    }
    status.replaceChildren(h("p", { class: "muted", text: t("common.saving") }));
    try {
      const { type: saved } = await post("/api/types", definitionOf(draft));
      status.replaceChildren(h("p", { class: "notice", text: t("types.saved", { name: saved.name, version: saved.version }) }));
      onSaved(saved);
    } catch (err) {
      status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
    }
  }

  render();
  return root;
}

// roomTypesCard is the card of a room's own types in the room's settings,
// for its administrators (ADR-0201): each with Edit and Remove, a new type,
// and the ready-made templates to add as types of the room. types are the
// types the room's items may have; onChange shows the page again.
export function roomTypesCard({ space, types, templates = [], onChange }) {
  const own = types.filter((ty) => ty.space === space);
  const box = h("div", { id: "room-type-editor" });
  const status = h("div", { class: "status", role: "status" });
  const open = (opts) => {
    box.replaceChildren(typeEditor({ space, types, ...opts, onSaved: onChange, onCancel: () => box.replaceChildren() }));
    box.querySelector("input:not([disabled])")?.focus();
  };
  const remove = async (ty) => {
    if (!confirm(t("admin.removeTypeConfirm", { name: ty.name }))) return;
    try {
      await del(`/api/types/${encodeURIComponent(ty.name)}`);
      onChange();
    } catch (err) {
      status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
    }
  };
  // A template whose name is taken is added under another name, chosen
  // in the editor.
  const taken = (name) => types.some((ty) => ty.name === name);
  return h("section", { class: "card", id: "room-types" },
    h("h2", { text: t("types.roomTitle") }),
    h("p", { class: "muted", text: t("types.roomHelp") }),
    own.length
      ? h("table", { id: "room-type-list" }, own.map((ty) => h("tr", {},
        h("td", {}, h("strong", { text: ty.name }), ` v${ty.version}`),
        h("td", { class: "muted", text: [ty.fields.map((f) => f.name).join(", "), ty.workflow ? ty.workflow.states.join(" → ") : ""].filter(Boolean).join(" · ") }),
        h("td", {},
          h("button", { type: "button", class: "link", text: t("common.edit"), onclick: () => open({ type: ty }) }), " ",
          h("button", { type: "button", class: "link danger", text: t("admin.removeType"), onclick: () => remove(ty) })))))
      : h("p", { class: "empty", text: t("types.roomNone") }),
    status,
    h("div", { class: "actions" },
      h("button", { type: "button", class: "secondary", id: "room-new-type", text: t("types.new"), onclick: () => open({}) }),
      templates.map((tt) => h("button", { type: "button", class: "secondary", "data-template": tt.name,
        text: t("types.addTemplate", { name: tt.name }),
        onclick: () => open({ template: taken(tt.name) ? { ...tt, name: "" } : tt }) }))),
    box);
}
