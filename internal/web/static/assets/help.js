// A help link: a small "?" that opens the user manual at the chapter about
// the place it stands in (ADR-draft-pictures-and-help-links-in-the-manual).
// The chapter is named by its anchor, the same in every language, so the
// link reads the manual in the UI's language.
import { h } from "./dom.js";
import { t } from "./i18n.js";

// helpHref is the address of a chapter of the manual.
export const helpHref = (section) => "#/manual?section=" + encodeURIComponent(section);

// helpLink is a link to a chapter of the manual.
export function helpLink(section) {
  return h("a", { class: "help-link", href: helpHref(section), title: t("help.manual"), "aria-label": t("help.manual"), "data-section": section }, "?");
}
