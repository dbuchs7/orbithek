// The depths a list of child pages offers and their names
// (ADR-0042, ADR-0239). A block keeps 0
// for one level, 2 to 5 for that many and -1 for every level; the page
// tree's API is asked for 1 to 5 or all.
import { t } from "./i18n.js";

// childDepths are the depths offered, in the order of the choice.
export const childDepths = [0, 2, 3, 4, 5, -1];

// childDepthName names a depth of a list of child pages.
export function childDepthName(depth) {
  if (!depth || depth === 1) return t("editor.childrenDepthOne");
  if (depth === -1) return t("editor.childrenDepthAll");
  return t("editor.childrenDepth", { n: depth });
}

// childDepthQuery is the depth the page tree's API is asked for.
export function childDepthQuery(depth) {
  if (depth === -1) return "all";
  return String(depth > 1 ? depth : 1);
}
