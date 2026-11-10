// renameInPlace lets one rename a file where its name is shown: in the
// album's card and in an item's file list (ADR-0180, ADR-0182). The name
// becomes a field holding value, the name without its extension, which the
// server keeps; Enter or leaving the field saves through save(next),
// Escape keeps the name. A name unchanged or emptied saves nothing. When
// save fails the field stays, so the name can be corrected; save shows the
// failure itself.
import { h } from "./dom.js";

export function renameInPlace(label, { value, ariaLabel, className, save }) {
  if (!label) return;
  const input = h("input", { class: className, value, maxlength: "200", "aria-label": ariaLabel });
  let done = false;
  const close = () => { done = true; input.replaceWith(label); };
  const commit = async () => {
    if (done) return;
    const next = input.value.trim();
    if (!next || next === value) return close();
    done = true;
    try {
      await save(next);
    } catch {
      done = false;
      input.focus();
    }
  };
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); commit(); }
    if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); close(); }
  });
  input.addEventListener("blur", () => commit());
  label.replaceWith(input);
  input.focus();
  input.select();
}
