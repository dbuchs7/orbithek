// When a photo was taken, set by hand (ADR-0196): the fields for a date and
// time and its offset from UTC, in the viewer and in the dialog that sets
// or shifts the time of every chosen picture. The server records the time
// and writes it into every file it gives out; the parts that need no
// browser are in photoparts.js.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { OFFSETS, shiftMinutes, takenValue } from "./photoparts.js";

// timeFields are a field for the date and time a photo was taken and a
// choice of its offset from UTC, "not known" first. value() answers what
// they hold, taken "" when it is no time.
export function timeFields({ id, taken = "", offset = "" }) {
  const when = h("input", { type: "datetime-local", step: "1", id: `${id}-input`, value: taken || "", min: "1800-01-01T00:00", max: "2200-12-31T23:59:59" });
  const zone = h("select", { id: `${id}-offset` },
    h("option", { value: "", text: t("photos.timeOffsetUnknown") }),
    OFFSETS.map((o) => h("option", { value: o, text: `UTC${o}`, selected: o === offset })));
  const element = h("div", { class: "photo-time-fields" },
    h("label", { class: "field", for: when.id }, h("span", { text: t("photos.timeLabel") }), when),
    h("label", { class: "field", for: zone.id }, h("span", { text: t("photos.timeOffset") }), zone));
  return { element, value: () => ({ taken: takenValue(when.value), offset: zone.value }) };
}

// askTime asks how the time of count chosen pictures is set: every one to
// the same time, or each shifted by days, hours and minutes, later or
// earlier. It resolves to { taken, offset }, to { minutes }, or to null.
export function askTime(count) {
  return new Promise((resolve) => {
    let mode = "set";
    const set = timeFields({ id: "album-time" });
    const num = (name, max) => h("input", { type: "number", id: `album-shift-${name}`, min: "0", max: String(max), step: "1", value: "0", inputmode: "numeric" });
    const days = num("days", 100000);
    const hours = num("hours", 23);
    const minutes = num("minutes", 59);
    const direction = h("select", { id: "album-shift-direction" },
      h("option", { value: "later", text: t("photos.timeLater") }), h("option", { value: "earlier", text: t("photos.timeEarlier") }));
    const field = (label, input) => h("label", { class: "field", for: input.id }, h("span", { text: label }), input);
    const shift = h("div", { class: "photo-time-fields", hidden: true },
      field(t("photos.timeDays"), days), field(t("photos.timeHours"), hours), field(t("photos.timeMinutes"), minutes), field(t("photos.timeDirection"), direction));
    const error = h("p", { class: "error", role: "alert", hidden: true });
    const modes = h("div", { class: "segmented", role: "group", "aria-label": t("photos.timeTitle") },
      [["set", t("photos.timeModeSet")], ["shift", t("photos.timeModeShift")]].map(([m, text]) =>
        h("button", { type: "button", class: "segment", id: `album-time-mode-${m}`, "aria-pressed": String(m === mode), text, onclick: () => {
          mode = m;
          for (const b of modes.querySelectorAll("button")) b.setAttribute("aria-pressed", String(b.id === `album-time-mode-${m}`));
          set.element.hidden = m !== "set";
          shift.hidden = m !== "shift";
          error.hidden = true;
        } })));
    let done = false;
    const finish = (value) => {
      if (done) return;
      done = true;
      dialog.close();
      dialog.remove();
      resolve(value);
    };
    const ok = h("button", { type: "button", class: "primary", id: "album-time-ok", text: t("photos.timeApply"), onclick: () => {
      if (mode === "set") {
        const v = set.value();
        if (!v.taken) return say(t("photos.timeInvalid"));
        return finish(v);
      }
      const n = shiftMinutes({ days: days.value, hours: hours.value, minutes: minutes.value, later: direction.value === "later" });
      if (!n) return say(t("photos.timeNoShift"));
      finish({ minutes: n });
    } });
    const say = (text) => {
      error.textContent = text;
      error.hidden = false;
    };
    const dialog = h("dialog", { class: "photo-time-dialog", id: "album-time-dialog", "aria-label": t("photos.timeTitle") },
      h("div", { class: "drawing-bar" }, h("strong", { text: t("photos.timeTitle") }),
        h("button", { type: "button", id: "album-time-cancel", text: t("common.cancel"), onclick: () => finish(null) })),
      h("div", { class: "photo-time-body" },
        h("p", { class: "muted", text: t("photos.timeFor", { n: count }) }), modes, set.element, shift, error,
        h("p", { class: "muted", text: t("photos.timeFiles") }),
        h("div", { class: "actions-row" }, ok)));
    dialog.addEventListener("cancel", (e) => { e.preventDefault(); finish(null); });
    document.body.append(dialog);
    dialog.showModal();
  });
}
