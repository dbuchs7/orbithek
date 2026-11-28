// A process engine's shop in the wiki
// (ADR-0251): the catalogues one may
// order from as tiles, a product with its order form, a basket, one's
// orders and the approvals one holds. Everything is the engine's answer to
// one's own grant of one's shop; the engine decides what one sees, orders
// and approves, and its refusals are shown in its words. A catalogue
// written in the wiki is published from its page (catalogueCard), and
// administrators set up the types it is written in (shopTypesCard).
import { h, when } from "./dom.js";
import { get, post } from "./api.js";
import { t, language } from "./i18n.js";
import { helpLink } from "./help.js";
import { loadForm } from "./engineviews.js";
import { engineOnly } from "./taskparts.js";
import { textOf, offered, partsOf, answers, orders as basketOrders, settled, listOf, statuses } from "./shopparts.js";
import { askConfirm, askText } from "./dialogs.js";

const errorBox = (err) => h("p", { class: "error", role: "alert", text: err instanceof Error ? err.message : String(err) });
const enc = encodeURIComponent;
const base = (engine) => "/api/engines/" + enc(engine) + "/shop";

// The basket is kept in the browser, per engine, as a convenience of one
// viewer; an order placed is the engine's.
const basketKey = (engine) => "ot.shop.basket." + engine;
function loadBasket(engine) {
  try {
    const v = JSON.parse(localStorage.getItem(basketKey(engine)) || "[]");
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}
function saveBasket(engine, basket) {
  try { localStorage.setItem(basketKey(engine), JSON.stringify(basket)); } catch { /* kept for this page only */ }
}

// statusText names a position's status in the UI's language.
function statusText(s) {
  const names = {
    pending: t("shop.status.pending"), blocked: t("shop.status.blocked"), running: t("shop.status.running"), done: t("shop.status.done"),
    skipped: t("shop.status.skipped"), failed: t("shop.status.failed"), rejected: t("shop.status.rejected"), cancelled: t("shop.status.cancelled"),
    returning: t("shop.status.returning"), returned: t("shop.status.returned"), abandoned: t("shop.status.abandoned"),
  };
  return statuses.includes(s) ? names[s] : s;
}

// connectCard asks one to connect one's account for the shop: through the
// engine, where one approves it, and back to this page.
function connectCard(st, admin) {
  const href = "/api/engines/" + enc(st.engine.id) + "/connect?" + new URLSearchParams({ shop: "1", back: "#/shop?engine=" + st.engine.id }).toString();
  const name = st.engine.name;
  if (!st.connectable) {
    // Nobody can connect yet: this server is not known to the engine.
    return h("section", { class: "card", id: "shop-connect" },
      h("h2", { text: t("shop.notReadyTitle") }),
      h("p", { text: admin ? t("shop.adminNotConnectable", { engine: name }) : t("shop.userNotReady") }),
      admin ? h("p", {}, h("a", { class: "button primary", id: "shop-admin-link", href: adminEngines, text: t("shop.toEngines") })) : "");
  }
  return h("section", { class: "card", id: "shop-connect" },
    h("h2", { text: t("shop.connectTitle", { engine: name }) }),
    h("p", { text: st.connected ? t("shop.connectAgain", { engine: name }) : t("shop.connectFirst", { engine: name }) }),
    h("ol", { class: "shop-steps" },
      h("li", { text: t("shop.connectStep1", { engine: name }) }),
      h("li", { text: t("shop.connectStep2", { engine: name }) }),
      h("li", { text: t("shop.connectStep3") })),
    h("p", {}, h("a", { class: "button primary", id: "shop-connect-link", href, text: t("shop.connect") })),
    h("p", { class: "muted", text: t("shop.connectOnce") }));
}

// adminEngines is where administrators set up process engines.
const adminEngines = "#/admin?section=engines";

// setupCard says the shop is not available yet: to everyone in plain
// words, and to administrators what to set up, with a way there.
async function setupCard(admin, states) {
  if (!admin) {
    return h("section", { class: "card", id: "shop-none" },
      h("h2", { text: t("shop.notReadyTitle") }),
      h("p", { text: t("shop.userNotReady") }));
  }
  let all = [];
  try {
    ({ engines: all } = await get("/api/engines"));
  } catch { /* the steps say it all */ }
  const known = new Map(states.map((s) => [s.engine.id, s]));
  const atlas = (all || []).filter((e) => !e.kind || e.kind === "atlas");
  const status = !atlas.length ? t("shop.adminNoEngine")
    : t("shop.adminNoShop", { engines: atlas.map((e) => known.get(e.id)?.engine.name || e.name || e.id).join(", ") });
  return h("section", { class: "card", id: "shop-none" },
    h("h2", {}, t("shop.setupTitle"), " ", helpLink("process-engines")),
    h("p", { class: "notice", id: "shop-setup-status", text: status }),
    h("p", { text: t("shop.setupLead") }),
    h("ol", { class: "shop-steps" },
      h("li", { text: t("shop.setupStep1") }),
      h("li", { text: t("shop.setupStep2") }),
      h("li", { text: t("shop.setupStep3") }),
      h("li", { text: t("shop.setupStep4") })),
    h("p", {}, h("a", { class: "button primary", id: "shop-admin-link", href: adminEngines, text: t("shop.toEngines") })),
    h("p", { class: "muted", text: t("shop.setupDelay") }));
}

// shopView draws the shop into section. params: engine, tab; admin says
// the reader may set up engines, so the help says how.
export async function shopView(section, params, { admin = false } = {}) {
  section.replaceChildren(h("p", { class: "muted", text: t("autobackup.loading") }));
  let engines;
  try {
    ({ engines } = await get("/api/me/engines"));
  } catch (err) {
    return section.replaceChildren(errorBox(err));
  }
  const every = (await Promise.all((engines || []).map((e) => get(base(e.id)).catch(() => null)))).filter(Boolean);
  const states = every.filter((s) => s.offered);
  if (!states.length) return section.replaceChildren(await setupCard(admin, every));
  const st = states.find((s) => s.engine.id === params.get("engine")) || states[0];
  const choose = states.length > 1 ? h("p", { class: "actions-row", id: "shop-engines" }, states.map((s) =>
    h("a", { class: s === st ? "button primary" : "button secondary", href: "#/shop?engine=" + enc(s.engine.id), text: s.engine.name }))) : "";
  if (!st.shop) return section.replaceChildren(choose, connectCard(st, admin));

  const tab = ["catalogues", "orders", "approvals"].includes(params.get("tab")) ? params.get("tab") : "catalogues";
  const tabLink = (id, label) => h("a", { class: id === tab ? "tab active" : "tab", href: "#/shop?" + new URLSearchParams({ engine: st.engine.id, tab: id }).toString(),
    id: "shop-tab-" + id, "aria-current": id === tab ? "page" : null, text: label });
  const body = h("div", { id: "shop-body" }, h("p", { class: "muted", text: t("autobackup.loading") }));
  section.replaceChildren(choose,
    h("nav", { class: "tabs", "aria-label": t("shop.title") }, tabLink("catalogues", t("shop.tabCatalogues")), tabLink("orders", t("shop.tabOrders")), tabLink("approvals", t("shop.tabApprovals"))),
    body);
  const ctx = { engine: st.engine.id, name: st.engine.name, lang: language(), body, admin, reconnect: () => section.replaceChildren(choose, connectCard({ ...st, shop: false, connected: true }, admin)) };
  try {
    if (tab === "orders") await ordersTab(ctx);
    else if (tab === "approvals") await approvalsTab(ctx);
    else await cataloguesTab(ctx);
  } catch (err) {
    if (err?.body?.connect) return ctx.reconnect();
    body.replaceChildren(errorBox(err));
  }
}

// releases reads the catalogues one may order from with their newest
// releases.
async function releases(ctx) {
  const catalogs = listOf(await get(base(ctx.engine) + "/catalogs"));
  const got = await Promise.all(catalogs.map(async (c) => ({ catalog: c, release: (await get(base(ctx.engine) + "/catalogs/" + enc(c.id) + "/release")).release })));
  return got.filter((x) => x.release);
}

async function cataloguesTab(ctx) {
  const all = await releases(ctx);
  const basket = loadBasket(ctx.engine).filter((l) => all.some((x) => x.release.id === l.releaseId));
  const basketBox = h("section", { class: "card", id: "shop-basket" });
  const status = h("div", { class: "status", role: "status" });
  const itemOf = (releaseId, id) => all.find((x) => x.release.id === releaseId)?.release.items?.find((it) => it.id === id);
  const drawBasket = () => {
    saveBasket(ctx.engine, basket);
    if (!basket.length) return basketBox.replaceChildren(h("h2", { text: t("shop.basket") }), h("p", { class: "muted", text: t("shop.basketEmpty") }));
    basketBox.replaceChildren(h("h2", { text: t("shop.basket") }),
      h("ul", { class: "shop-basket-lines" }, basket.map((l, i) => {
        const it = itemOf(l.releaseId, l.itemId);
        const opts = (l.options || []).map((o) => textOf(itemOf(l.releaseId, o)?.texts, ctx.lang) || o);
        return h("li", {}, h("strong", { text: textOf(it?.texts, ctx.lang) || l.itemId }),
          l.variantId ? " · " + (textOf(it?.variants?.find((v) => v.id === l.variantId)?.texts, ctx.lang) || l.variantId) : "",
          opts.length ? " · " + t("shop.withOptions", { options: opts.join(", ") }) : "", " ",
          h("button", { type: "button", class: "link danger", text: t("shop.remove"), onclick: () => { basket.splice(i, 1); drawBasket(); } }));
      })),
      h("p", { class: "actions-row" }, h("button", { type: "button", class: "primary", id: "shop-place", text: t("shop.place"), onclick: async (ev) => {
        ev.currentTarget.disabled = true;
        status.replaceChildren(h("p", { class: "muted", text: t("shop.placing") }));
        const placed = [];
        try {
          for (const o of basketOrders(basket)) {
            placed.push(await post(base(ctx.engine) + "/orders", o));
            const left = basket.filter((l) => l.releaseId !== o.releaseId);
            basket.splice(0, basket.length, ...left);
          }
          drawBasket();
          status.replaceChildren(h("p", { class: "notice success", id: "shop-placed" }, t("shop.placed", { n: placed.length }), " ",
            h("a", { href: "#/shop?" + new URLSearchParams({ engine: ctx.engine, tab: "orders" }).toString(), text: t("shop.toOrders") })));
        } catch (err) {
          drawBasket();
          status.replaceChildren(errorBox(err), problemsOf(err));
        }
      } })));
  };
  drawBasket();
  if (!all.length) {
    return ctx.body.replaceChildren(h("section", { class: "card", id: "shop-empty" },
      h("h2", {}, t("shop.noCatalogueTitle"), " ", helpLink("shop-catalogue")),
      h("p", { text: t("shop.noCatalogue", { engine: ctx.name }) }),
      ctx.admin ? h("details", { class: "shop-help" }, h("summary", { text: t("shop.howCatalogue") }),
        h("ol", { class: "shop-steps" },
          h("li", { text: t("shop.catalogueStep1") }),
          h("li", { text: t("shop.catalogueStep2") }),
          h("li", { text: t("shop.catalogueStep3") }),
          h("li", { text: t("shop.catalogueStep4") }))) : ""));
  }
  ctx.body.replaceChildren(status, basketBox, ...all.map(({ catalog, release }) =>
    h("section", { class: "shop-catalogue", "data-catalogue": catalog.id },
      h("h2", { text: textOf(catalog.texts, ctx.lang) || catalog.id }),
      h("div", { class: "shop-tiles" }, offered(release).map((it) => productTile(ctx, release, it, (line) => {
        basket.push(line);
        drawBasket();
        status.replaceChildren(h("p", { class: "notice success", text: t("shop.added", { name: textOf(it.texts, ctx.lang) || it.id }) }));
      }))))));
}

// problemsOf lists the problems the engine named with its refusal.
function problemsOf(err) {
  const problems = err?.body?.engine?.problems || err?.body?.problems;
  if (!Array.isArray(problems) || !problems.length) return "";
  return h("ul", { class: "error-list" }, problems.map((p) => h("li", { text: [p.subject || p.item || p.catalog || "", p.problem || p.message || ""].filter(Boolean).join(": ") })));
}

// picture is a product's picture, left out when it has none.
function picture(ctx, it) {
  const img = h("img", { class: "shop-picture", alt: "", loading: "lazy", src: base(ctx.engine) + "/products/" + enc(it.id) + "/picture" });
  img.addEventListener("error", () => img.remove());
  return img;
}

function productTile(ctx, release, it, add) {
  const desc = textOf(it.descriptions, ctx.lang);
  return h("article", { class: "shop-tile card", "data-product": it.id },
    picture(ctx, it),
    h("h3", { text: textOf(it.texts, ctx.lang) || it.id }),
    it.category ? h("p", { class: "chip", text: it.category }) : "",
    desc ? h("p", { class: "shop-description", text: desc.length > 220 ? desc.slice(0, 220) + "…" : desc }) : "",
    it.price ? h("p", { class: "shop-price", text: it.price }) : "",
    h("p", {}, h("button", { type: "button", class: "secondary", text: t("shop.choose"), onclick: () => productDialog(ctx, release, it, add) })));
}

// productDialog shows a product with what comes with it, its options, its
// shapes and its order form, and puts it in the basket.
function productDialog(ctx, release, it, add) {
  let form = null;
  const body = h("div", { class: "shop-dialog-body" });
  const status = h("div", { class: "status", role: "status" });
  const actions = h("div", { class: "actions-row" });
  const dialog = h("dialog", { class: "shop-dialog", id: "shop-dialog" }, h("h2", { text: textOf(it.texts, ctx.lang) || it.id }), body, status, actions);
  const close = () => { form?.destroy(); dialog.close(); dialog.remove(); };
  dialog.addEventListener("cancel", (ev) => { ev.preventDefault(); close(); });
  document.body.append(dialog);
  dialog.showModal();
  const { included, options } = partsOf(release, it.id);
  const variant = it.variants?.length ? h("select", { name: "variant", id: "shop-variant" }, it.variants.map((v) => h("option", { value: v.id, text: textOf(v.texts, ctx.lang) || v.id }))) : null;
  const optionBoxes = options.map((o) => h("label", { class: "checkbox" }, h("input", { type: "checkbox", value: o.id }), " " + (textOf(o.texts, ctx.lang) || o.id)));
  const canvas = h("div", { class: "pm-form-canvas shop-form" });
  body.append(
    picture(ctx, it),
    textOf(it.descriptions, ctx.lang) ? h("p", { class: "shop-description", text: textOf(it.descriptions, ctx.lang) }) : "",
    it.price ? h("p", { class: "shop-price", text: it.price }) : "",
    included.length ? h("p", {}, t("shop.includes", { parts: included.map((p) => textOf(p.texts, ctx.lang) || p.id).join(", ") })) : "",
    variant ? h("label", {}, t("shop.variant"), " ", variant) : "",
    optionBoxes.length ? h("fieldset", {}, h("legend", { text: t("shop.options") }), optionBoxes) : "",
    canvas);
  actions.append(h("button", { type: "button", class: "secondary", text: t("common.cancel"), onclick: close }));
  const put = (config) => {
    add({ releaseId: release.id, itemId: it.id, variantId: variant?.value || "", config,
      options: optionBoxes.map((l) => l.querySelector("input")).filter((i) => i.checked).map((i) => i.value) });
    close();
  };
  const addButton = (onclick) => actions.prepend(h("button", { type: "button", class: "primary", id: "shop-add", text: t("shop.add"), onclick }));
  if (!it.configForm) return addButton(() => put({}));
  canvas.append(h("p", { class: "muted", text: t("autobackup.loading") }));
  (async () => {
    try {
      const [schema, Form] = await Promise.all([get(base(ctx.engine) + "/forms/" + enc(it.configForm)), loadForm()]);
      canvas.replaceChildren();
      form = new Form({ container: canvas });
      const kinds = form.get("formFields");
      const own = engineOnly(schema.schema, (type) => Boolean(type && kinds.get(type)));
      if (own.length) {
        form.destroy();
        form = null;
        return canvas.replaceChildren(h("p", { class: "notice", text: t("shop.engineForm", { fields: own.join(", "), engine: ctx.name }) }));
      }
      await form.importSchema(schema.schema, {});
      addButton(() => {
        const { data, errors } = form.submit();
        if (errors && Object.keys(errors).length) return; // form-js marks the fields
        put(answers(data));
      });
    } catch (err) {
      canvas.replaceChildren(errorBox(err));
    }
  })();
}

async function ordersTab(ctx) {
  const list = listOf(await get(base(ctx.engine) + "/orders"));
  const names = new Map();
  try {
    for (const { release } of await releases(ctx)) for (const it of release.items || []) names.set(it.id, textOf(it.texts, ctx.lang));
  } catch { /* names fall back to the product IDs */ }
  if (!list.length) return ctx.body.replaceChildren(h("p", { class: "muted", id: "shop-no-orders", text: t("shop.noOrders") }));
  const status = h("div", { class: "status", role: "status" });
  // ok names the button that goes on, as the button that asked is named.
  const act = (question, ok, fn, done) => async (ev) => {
    // Kept before the question: an event names its current target only
    // while it is handled.
    const button = ev.currentTarget;
    if (!(await askConfirm(question, ok))) return;
    button.disabled = true;
    try {
      await fn();
      done();
    } catch (err) {
      ev.target.disabled = false;
      status.replaceChildren(errorBox(err));
    }
  };
  const reload = () => ordersTab(ctx).catch((err) => ctx.body.replaceChildren(errorBox(err)));
  const sorted = [...list].sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
  ctx.body.replaceChildren(status, h("ul", { class: "shop-orders" }, sorted.map((o) => {
    const open = (o.lines || []).some((l) => !settled(l.status));
    return h("li", { class: "card", "data-order": o.id },
      h("h3", {}, t("shop.order", { id: o.id }), o.createdAt ? h("span", { class: "muted", text: " · " + when(new Date(o.createdAt * 1000).toISOString()) }) : ""),
      h("ul", {}, (o.lines || []).map((l) => h("li", { "data-line": l.itemId },
        h("strong", { text: names.get(l.itemId) || l.itemId }), " ",
        h("span", { class: "chip", "data-status": l.status, text: statusText(l.status) }),
        l.reason ? h("span", { class: "muted", text: " " + l.reason }) : "", " ",
        l.status === "done" ? h("button", { type: "button", class: "link", text: t("shop.return"), onclick: act(t("shop.returnConfirm", { name: names.get(l.itemId) || l.itemId }), { ok: t("shop.return") }, async () => {
          await post(base(ctx.engine) + "/orders/" + enc(o.id) + "/lines/" + enc(l.itemId) + "/return", {});
        }, reload) }) : ""))),
      open ? h("p", {}, h("button", { type: "button", class: "link danger", text: t("shop.cancel"), onclick: act(t("shop.cancelConfirm"), { ok: t("shop.cancel"), danger: true }, async () => {
        await post(base(ctx.engine) + "/orders/" + enc(o.id) + "/cancel", {});
      }, reload) })) : "");
  })));
}

async function approvalsTab(ctx) {
  const list = listOf(await get(base(ctx.engine) + "/approvals"));
  if (!list.length) return ctx.body.replaceChildren(h("p", { class: "muted", id: "shop-no-approvals", text: t("shop.noApprovals") }));
  const status = h("div", { class: "status", role: "status" });
  const reload = () => approvalsTab(ctx).catch((err) => ctx.body.replaceChildren(errorBox(err)));
  const decide = (a, approved) => async (ev) => {
    const button = ev.currentTarget;
    let reason = "";
    if (!approved) {
      reason = await askText(t("shop.refuseReason"), { ok: t("shop.refuse") }) || "";
      if (!reason.trim()) return;
    }
    button.disabled = true;
    try {
      await post(base(ctx.engine) + "/approvals/decide", { approved, reason, taskKeys: [a.task.key] });
      reload();
    } catch (err) {
      ev.target.disabled = false;
      status.replaceChildren(errorBox(err));
    }
  };
  ctx.body.replaceChildren(status, h("ul", { class: "shop-approvals" }, list.map((a) => h("li", { class: "card", "data-task": String(a.task?.key) },
    h("h3", { text: textOf(a.texts, ctx.lang) || a.itemId }),
    h("p", { text: t("shop.approvalFor", { recipient: a.recipientName || a.recipient || "", orderer: a.ordererName || a.orderer || "" }) }),
    a.price ? h("p", { class: "shop-price", text: a.price }) : "",
    (a.answers || []).length ? h("dl", { class: "shop-answers" }, a.answers.map((x) => [h("dt", { text: x.label || x.key || "" }), h("dd", { text: String(x.value ?? "") })])) : "",
    h("p", { class: "actions-row" },
      h("button", { type: "button", class: "primary", text: t("shop.approve"), onclick: decide(a, true) }),
      h("button", { type: "button", class: "secondary", text: t("shop.refuse"), onclick: decide(a, false) }))))));
}

// catalogueCard publishes a catalogue item at its engine: it shows what
// would be sent or what stops it, asks the engine for a dry run, and
// publishes.
export function catalogueCard(it) {
  const path = "/api/items/" + enc(it.id) + "/shop";
  const out = h("div", { class: "status", role: "status" });
  const card = h("section", { class: "side-card", id: "shop-catalogue" }, h("h2", {}, t("shopcat.title"), " ", helpLink("shop-catalogue")),
    h("p", { class: "muted", text: t("shopcat.note") }),
    fieldHelp("shopcat-help", t("shopcat.helpTitle"), catalogueFields, [t("shopcat.helpProducts"), t("shopcat.helpPublish")]), out);
  const show = (got, dryRun) => {
    const r = got.result || {};
    const names = (xs) => (xs || []).join(", ") || "–";
    out.replaceChildren(h("p", { class: "notice success", id: "shopcat-done", text: dryRun ? t("shopcat.dryRunDone") : t("shopcat.published", { engine: got.engine }) }),
      h("p", { text: t("shopcat.created", { ids: names(r.created) }) }), h("p", { text: t("shopcat.updated", { ids: names(r.updated) }) }),
      (r.releases || []).length ? h("p", { text: t("shopcat.releases", { ids: names(r.releases.map((x) => x.id)) }) }) : "",
      (r.warnings || []).length ? h("ul", { class: "notice warning" }, r.warnings.map((w) => h("li", { text: [w.subject, w.problem].filter(Boolean).join(": ") }))) : "");
  };
  const run = (dryRun) => async (ev) => {
    const button = ev.currentTarget;
    if (!dryRun && !(await askConfirm(t("shopcat.confirm"), { ok: t("shopcat.publish") }))) return;
    button.disabled = true;
    out.replaceChildren(h("p", { class: "muted", text: t("shopcat.sending") }));
    try {
      show(await post(path + "/publish", { dryRun }), dryRun);
    } catch (err) {
      const connect = err?.body?.connect ? h("p", {}, h("a", { href: "#/shop", text: t("shopcat.connect") })) : "";
      out.replaceChildren(errorBox(err), problemsOf(err), connect);
    } finally {
      button.disabled = false;
    }
  };
  const check = async () => {
    try {
      const got = await get(path + "/document");
      if (got.problems?.length) return out.replaceChildren(h("p", { class: "error", text: t("shopcat.problems") }), problemsOf({ body: got }));
      const doc = got.document;
      out.replaceChildren(h("p", { id: "shopcat-summary", text: t("shopcat.summary", { engine: got.engine, catalogue: doc.catalogs[0].id, n: doc.catalogs[0].items.length, all: doc.products.length }) }));
    } catch (err) {
      out.replaceChildren(errorBox(err));
    }
  };
  card.append(h("p", { class: "actions-row" },
    h("button", { type: "button", class: "secondary", id: "shopcat-check", text: t("shopcat.check"), onclick: check }),
    h("button", { type: "button", class: "secondary", id: "shopcat-dry", text: t("shopcat.dryRun"), onclick: run(true) }),
    h("button", { type: "button", class: "primary", id: "shopcat-publish", text: t("shopcat.publish"), onclick: run(false) })));
  check();
  return card;
}

// The fields of a catalogue and of a product, each with what belongs in
// it in plain words; the keys name the help texts.
const catalogueFields = ["engine", "languages", "rank", "audience", "catalogue-id", "names"];
const productFields = ["provision-process", "deprovision-process", "approval", "approver", "order-form", "price", "category", "multiple", "max-days", "orderable", "eligible", "product-id", "names"];

// fieldHelp is a folded list of what belongs in each field, with notes
// below it.
function fieldHelp(id, title, fields, notes = []) {
  const label = (f) => f === "names" ? "name-de, name-fr, name-it, name-en" : f === "orderable" ? "orderable-from, orderable-until" : f;
  const text = {
    engine: t("shopfield.engine"), languages: t("shopfield.languages"), rank: t("shopfield.rank"), audience: t("shopfield.audience"),
    "catalogue-id": t("shopfield.catalogueId"), names: t("shopfield.names"), "provision-process": t("shopfield.provision"),
    "deprovision-process": t("shopfield.deprovision"), approval: t("shopfield.approval"), approver: t("shopfield.approver"),
    "order-form": t("shopfield.orderForm"), price: t("shopfield.price"), category: t("shopfield.category"), multiple: t("shopfield.multiple"),
    "max-days": t("shopfield.maxDays"), orderable: t("shopfield.orderable"), eligible: t("shopfield.eligible"), "product-id": t("shopfield.productId"),
  };
  return h("details", { class: "shop-help", id },
    h("summary", { text: title }),
    h("dl", { class: "shop-fields" }, fields.map((f) => [h("dt", {}, h("code", { text: label(f) })), h("dd", { text: text[f] })])),
    notes.map((n) => h("p", { class: "muted", text: n })));
}

// productCard explains a product's fields on its page and leads to the
// catalogue it is published with.
export function productCard(it) {
  const to = h("p", {});
  const card = h("section", { class: "side-card", id: "shop-product" }, h("h2", {}, t("shopprod.title"), " ", helpLink("shop-catalogue")),
    h("p", { class: "muted", text: t("shopprod.note") }),
    h("p", { class: "muted", text: it.state === "active" ? t("shopprod.active") : it.state === "withdrawn" ? t("shopprod.withdrawn") : t("shopprod.draft") }),
    fieldHelp("shopprod-help", t("shopcat.helpTitle"), productFields), to);
  // The catalogue is the nearest shop-catalogue above it in the page tree.
  (async () => {
    let id = it.parent;
    for (let i = 0; id && i < 20; i++) {
      const up = (await get("/api/items/" + enc(id)).catch(() => null))?.item;
      if (!up) break;
      if (up.type === "shop-catalogue") {
        return to.replaceChildren(h("a", { href: "#/item/" + enc(up.key), id: "shopprod-catalogue", text: t("shopprod.toCatalogue", { title: up.title || up.key }) }));
      }
      id = up.parent;
    }
    to.replaceChildren(h("span", { class: "notice warning", text: t("shopprod.noCatalogue") }));
  })();
  return card;
}

// shopTypesCard lets administrators set up the types catalogues are
// written in.
export function shopTypesCard() {
  const out = h("div", { class: "status", role: "status" });
  return h("section", { class: "card", id: "shop-types" }, h("h3", {}, t("shopcat.typesTitle"), " ", helpLink("shop-catalogue")),
    h("p", { class: "muted", text: t("shopcat.typesNote") }),
    h("ol", { class: "shop-steps" },
      h("li", { text: t("shopcat.typesStep1") }),
      h("li", { text: t("shopcat.typesStep2") }),
      h("li", { text: t("shopcat.typesStep3") })),
    h("p", {}, h("button", { type: "button", class: "secondary", id: "shop-types-define", text: t("shopcat.typesDefine"), onclick: async () => {
      try {
        const got = await post("/api/shop/types", {});
        out.replaceChildren(h("p", { class: "notice success", text: (got.defined.length ? t("shopcat.typesDefined", { types: got.defined.join(", ") }) : t("shopcat.typesExisting")) + " " + t("shopcat.typesNext") }));
      } catch (err) {
        out.replaceChildren(errorBox(err));
      }
    } })), out);
}
