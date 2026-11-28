// The app's own questions (ADR-0031): a confirmation, a line to type and a
// notice, each a modal dialog of one look and one behaviour, in place of
// the browser's confirm, prompt and alert. Those block the page, look as
// the browser pleases and differ from every other dialog of the app.
//
// One layout for all: the text, a field where one is asked for, then the
// button that goes on, named by what it does where the caller says so,
// and Cancel beside it as a link. Escape and Cancel answer no; Enter goes
// on. The focus starts in the field, or on the button that goes on, and
// returns to where it was when the dialog closes.
import { h } from "./dom.js";
import { t } from "./i18n.js";

let count = 0;

// open shows a dialog with text, an optional field and its buttons, and
// resolves to answer() when the person goes on, or to none otherwise.
function open(text, { field = null, ok = t("common.ok"), danger = false, cancel = true, answer, none }) {
  return new Promise((resolve) => {
    const back = document.activeElement;
    const id = "ask-" + ++count;
    let result = none;
    const go = h("button", { type: "submit", class: danger ? "danger" : "primary", "data-ask": "ok", text: ok });
    const form = h("form", { method: "dialog", class: "note-form" },
      h("p", { id: id + "-text", class: "ask-text", text }),
      field,
      h("div", { class: "actions-row" }, go,
        cancel ? h("button", { type: "button", class: "link", "data-ask": "cancel", text: t("common.cancel"), onclick: () => dialog.close() }) : null));
    form.addEventListener("submit", (ev) => {
      ev.preventDefault();
      result = answer();
      dialog.close();
    });
    const dialog = h("dialog", { class: "note-dialog ask-dialog", id, role: "alertdialog", "aria-describedby": id + "-text" }, form);
    dialog.addEventListener("close", () => {
      dialog.remove();
      if (back && back.isConnected && typeof back.focus === "function") back.focus();
      resolve(result);
    });
    document.body.append(dialog);
    dialog.showModal();
    (field?.querySelector("input") || go).focus();
  });
}

// askConfirm asks a yes-or-no question and resolves to whether the person
// went on. ok names the button, e.g. "Delete"; danger marks what cannot be
// undone.
export function askConfirm(text, { ok, danger = false } = {}) {
  return open(text, { ok, danger, answer: () => true, none: false });
}

// askText asks for a line of text and resolves to it, or to null when the
// person cancels, as prompt does. value is what the field holds at first;
// password hides what is typed.
export function askText(text, { value = "", ok, password = false } = {}) {
  const input = h("input", { type: password ? "password" : "text", name: "answer", value, "aria-label": text,
    autocomplete: password ? "current-password" : "off" });
  return open(text, { field: h("label", { class: "field" }, input), ok, answer: () => input.value, none: null });
}

// tell says something that needs no answer, such as a failure with no
// place of its own on the page, and resolves when it is closed.
export function tell(text) {
  return open(text, { ok: t("common.close"), cancel: false, answer: () => undefined, none: undefined });
}
