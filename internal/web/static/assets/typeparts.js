// The parts of the type editor that need no browser (ADR-0201, ADR-0202):
// a draft of a type as the form holds it, the definition the server takes
// from it and back, what is wrong with it before it is sent, and how a
// value of the new field kinds reads.
// The form itself is typeeditor.js, the inputs of the field kinds fields.js.

// fieldKinds are the kinds a field may have, in the order the editor
// offers them.
export const fieldKinds = ["text", "number", "bool", "date", "datetime", "user", "enum", "multi", "url", "item", "rating", "file"];

// maxRating is the most stars a rating field gives.
export const maxRating = 5;

// richKinds are the kinds whose inputs and views fields.js draws.
export const richKinds = ["item", "multi", "rating", "file"];

// reserved are the names every item has, which no field may take.
const reserved = ["id", "key", "type", "space", "parent", "title", "state", "body"];

// uniqueKinds are the kinds a field may be unique in (ADR-0074).
const uniqueKinds = ["text", "url", "number"];

// usableIn lists the types items of a space may have: those of the whole
// installation and the space's own.
export function usableIn(types, space) {
  return types.filter((ty) => !ty.space || ty.space === space);
}

// presetWorkflow is the simplest workflow: open, then done, and back.
export function presetWorkflow() {
  return {
    initial: "open",
    states: ["open", "done"],
    transitions: [
      { name: "finish", from: ["open"], to: "done" },
      { name: "reopen", from: ["done"], to: "open" },
    ],
  };
}

// emptyDraft is a new type of a space ("" for the whole installation).
export function emptyDraft(space = "") {
  return { name: "", space, body: true, fields: [], workflow: null, review: null, template: "", startText: "", existing: false };
}

// newField is a field as "Add field" adds it.
export function newField() {
  return { name: "", kind: "text", required: false, unique: false, recommended: false, values: [], targets: [] };
}

const clone = (v) => JSON.parse(JSON.stringify(v ?? null));

// draftOf is the draft of a defined type or a template, to edit; space
// names the room a template is added to.
export function draftOf(type, space) {
  const ty = clone(type) || {};
  return {
    name: ty.name || "",
    space: space ?? ty.space ?? "",
    body: Boolean(ty.body),
    fields: (ty.fields || []).map((f) => ({ ...newField(), ...f, values: f.values || [], targets: f.targets || [] })),
    workflow: ty.workflow ? { initial: ty.workflow.initial, states: ty.workflow.states || [], transitions: ty.workflow.transitions || [] } : null,
    review: ty.review || null,
    // The Word template stays as it is (ADR-0167); the start text is
    // edited as Markdown, as the server gives and takes it.
    template: ty.template || "",
    startText: ty.startMarkdown || "",
    existing: Boolean(ty.version),
  };
}

const trimmed = (list) => [...new Set((list || []).map((v) => String(v).trim()).filter(Boolean))];

// definitionOf is the definition the server takes from a draft: what a
// field's kind does not use is left out, the initial state is the first
// one unless another is chosen, and transitions name only states that are
// there.
export function definitionOf(draft) {
  const out = { name: draft.name.trim(), body: Boolean(draft.body), fields: [] };
  if (draft.space) out.space = draft.space;
  for (const f of draft.fields) {
    const field = { name: f.name.trim(), kind: f.kind };
    if (f.required) field.required = true;
    if (f.recommended && !f.required) field.recommended = true;
    if (f.unique && uniqueKinds.includes(f.kind)) field.unique = true;
    if (f.kind === "enum" || f.kind === "multi") field.values = trimmed(f.values);
    if (f.kind === "item" && trimmed(f.targets).length) field.targets = trimmed(f.targets);
    out.fields.push(field);
  }
  if (draft.workflow) {
    const states = trimmed(draft.workflow.states);
    const fieldNames = out.fields.map((f) => f.name);
    out.workflow = {
      initial: states.includes(draft.workflow.initial) ? draft.workflow.initial : states[0] || "",
      states,
      transitions: draft.workflow.transitions
        .map((tr) => {
          const t = { name: String(tr.name || "").trim(), from: trimmed(tr.from).filter((s) => states.includes(s)), to: tr.to };
          const requires = trimmed(tr.requires).filter((r) => fieldNames.includes(r));
          if (requires.length) t.requires = requires;
          return t;
        })
        .filter((tr) => tr.name || tr.from.length),
    };
  }
  if (draft.review && out.fields.some((f) => f.name === draft.review.field)) out.review = draft.review;
  if (draft.template) out.template = draft.template;
  if (draft.body && String(draft.startText || "").trim()) out.startMarkdown = draft.startText;
  return out;
}

// problems names what is wrong with a draft, as catalogue keys with their
// values, before it is sent; the server checks again.
export function problems(draft) {
  const out = [];
  const def = definitionOf(draft);
  if (!def.name) out.push({ key: "types.problemName" });
  const seen = new Set();
  for (const [i, f] of def.fields.entries()) {
    const n = i + 1;
    if (!f.name) out.push({ key: "types.problemFieldName", params: { n } });
    else if (reserved.includes(f.name.toLowerCase())) out.push({ key: "types.problemReserved", params: { name: f.name } });
    else if (seen.has(f.name)) out.push({ key: "types.problemTwice", params: { name: f.name } });
    seen.add(f.name);
    if ((f.kind === "enum" || f.kind === "multi") && !f.values.length) out.push({ key: "types.problemValues", params: { name: f.name || n } });
  }
  if (def.workflow) {
    const w = def.workflow;
    if (!w.states.length) out.push({ key: "types.problemStates" });
    if (new Set(draft.workflow.states.map((s) => String(s).trim()).filter(Boolean)).size !== draft.workflow.states.filter((s) => String(s).trim()).length) {
      out.push({ key: "types.problemStateTwice" });
    }
    const names = new Set();
    for (const [i, tr] of w.transitions.entries()) {
      const n = i + 1;
      if (!tr.name) out.push({ key: "types.problemTransitionName", params: { n } });
      else if (names.has(tr.name)) out.push({ key: "types.problemTransitionTwice", params: { name: tr.name } });
      names.add(tr.name);
      if (!tr.from.length || !w.states.includes(tr.to)) out.push({ key: "types.problemTransitionStates", params: { name: tr.name || n } });
    }
  }
  return out;
}

// stars writes a rating as filled and empty stars.
export function stars(n) {
  const k = Math.max(0, Math.min(maxRating, Math.round(Number(n) || 0)));
  return "★".repeat(k) + "☆".repeat(maxRating - k);
}

// valueText is a value of a field as one line of text, as a table or card
// shows it: the values of a multi field joined by ", ", a rating as stars,
// a referenced item by what ref names it (its key and title), a file by
// what file names it; other values as they are.
export function valueText(f, v, { ref = (id) => id, file = (id) => id } = {}) {
  if (v === null || v === undefined || v === "") return "";
  switch (f?.kind) {
    case "multi": return (Array.isArray(v) ? v : [v]).join(", ");
    case "rating": return stars(v);
    case "item": return ref(v);
    case "file": return file(v);
    case "bool": return v ? "✓" : "";
    default: return String(v);
  }
}

// moved returns a list with the entry at i moved by step (-1 up, 1 down),
// or the list itself where it cannot move.
export function moved(list, i, step) {
  const j = i + step;
  if (j < 0 || j >= list.length) return list;
  const out = list.slice();
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}

// renamedState renames a state of a workflow, in its transitions too.
export function renamedState(workflow, from, to) {
  const swap = (s) => (s === from ? to : s);
  return {
    initial: swap(workflow.initial),
    states: workflow.states.map(swap),
    transitions: workflow.transitions.map((tr) => ({ ...tr, from: (tr.from || []).map(swap), to: swap(tr.to) })),
  };
}
