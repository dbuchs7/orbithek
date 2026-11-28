// One's sign-ins at other providers (ADR-0229): an existing account is
// linked to one's identity at a configured provider, after the password is
// given again, and the identity then signs in to this account. A link is
// removed here too; the only way an account without a password signs in
// stays.
import { h } from "./dom.js";
import { get, post, del } from "./api.js";
import { t } from "./i18n.js";
import { askConfirm } from "./dialogs.js";

const errorBox = (err) => h("p", { class: "error", role: "alert", text: err instanceof Error ? err.message : String(err) });

// returned is what the provider's way back said, once: the address is
// cleaned so a reload does not say it again.
function returned() {
  const [route, query = ""] = location.hash.split("?");
  const q = new URLSearchParams(query);
  const out = { linked: q.get("linked"), error: q.get("link_error") };
  if (out.linked || out.error) history.replaceState(null, "", location.pathname + location.search + route);
  return out;
}

// linkErrorText says why a link did not come about.
function linkErrorText(code) {
  return {
    taken: t("identities.errorTaken"),
    linked: t("identities.errorLinked"),
    domain: t("login.domain"),
    expired: t("identities.errorExpired"),
  }[code] || t("identities.errorProvider");
}

// refusalText words a refusal of the server by its code.
function refusalText(err) {
  return {
    password: t("identities.wrongPassword"),
    reauth: t("identities.reauth"),
    linked: t("identities.errorLinked"),
    last: t("identities.last"),
    throttled: t("identities.throttled"),
  }[err?.code] || (err instanceof Error ? err.message : String(err));
}

export function identitiesCard() {
  const card = h("section", { class: "card", id: "identities", hidden: true });
  const back = returned();
  const status = h("div", { class: "status", role: "status" });
  const load = async () => {
    let data;
    try {
      data = await get("/api/me/identities");
    } catch (err) {
      card.hidden = false;
      card.replaceChildren(h("h2", { text: t("identities.title") }), errorBox(err));
      return;
    }
    if (!data.providers.length) return;
    card.hidden = false;
    const named = data.providers.find((p) => p.id === back.linked);
    card.replaceChildren(
      h("h2", { text: t("identities.title") }),
      h("p", { class: "muted", text: t("identities.note") }),
      named ? h("p", { class: "notice success", id: "identity-linked", text: t("identities.linked", { name: named.name }) }) : "",
      back.error ? h("p", { class: "error", role: "alert", id: "identity-error", text: linkErrorText(back.error) }) : "",
      h("ul", { class: "identity-list" }, data.providers.map((p) => row(p, data.password))),
      status);
    back.linked = null;
    back.error = null;
  };
  const row = (p, hasPassword) => {
    const li = h("li", { "data-provider": p.id },
      h("strong", { text: p.name }), " ",
      h("span", { class: "muted", text: p.linked ? t("identities.isLinked") : t("identities.notLinked") }), " ");
    if (p.linked) {
      // The calendars of the same account, a grant of their own that Google
      // asks for (ADR-0229, ADR-0174); Google is proposed this account.
      if (p.calendar) li.append(h("a", { class: "button secondary identity-calendar", href: "/api/me/calendar-accounts/google/connect?linked=1",
        text: t("identities.calendar") }), " ");
      li.append(h("button", { type: "button", class: "link identity-unlink", text: t("identities.unlink"),
        onclick: async () => {
          if (!(await askConfirm(t("identities.unlinkConfirm", { name: p.name })))) return;
          status.replaceChildren();
          try {
            await del("/api/me/identities/" + encodeURIComponent(p.id));
            load();
          } catch (err) {
            status.replaceChildren(errorBox(refusalText(err)));
          }
        } }));
      return li;
    }
    const start = async (password) => {
      status.replaceChildren();
      try {
        const { url } = await post("/api/me/identities/" + encodeURIComponent(p.id), hasPassword ? { password } : {});
        location.assign(url);
      } catch (err) {
        status.replaceChildren(errorBox(refusalText(err)));
      }
    };
    if (!hasPassword) {
      li.append(h("button", { type: "button", class: "identity-link", text: t("identities.link"), onclick: () => start("") }));
      return li;
    }
    const pw = h("input", { type: "password", name: "password", autocomplete: "current-password", required: true,
      "aria-label": t("identities.passwordFor", { name: p.name }) });
    li.append(h("form", { class: "identity-link-form", onsubmit: (ev) => { ev.preventDefault(); start(pw.value); } },
      h("label", {}, t("identities.password"), " ", pw), " ",
      h("button", { type: "submit", class: "identity-link", text: t("identities.link") })));
    return li;
  };
  load();
  return card;
}
