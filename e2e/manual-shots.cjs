// Takes the user manual's pictures (ADR-draft-pictures-and-help-links-in-the-manual)
// from a running server in every language of the manual: views of the UI
// with example content, and the symbols of the start page's modules. It is
// run by hand when what the manual shows has changed, against a fresh
// server, and its pictures are committed:
//
//   OT_ADMIN_PASSWORD=… OT_PUBLIC_URL=http://127.0.0.1:8080 go run ./cmd/orbithek serve -addr :8080 -data-dir /tmp/shots
//   cd e2e && BASE_URL=http://127.0.0.1:8080 ADMIN_PASSWORD=… node manual-shots.cjs
//
// Pictures go to internal/web/static/assets/manual/img: img/<lang>/*.png
// and img/icons/*.png. Not part of the tests; the manual's tests check that
// every picture it shows is there.
const { chromium } = require("playwright");
const path = require("node:path");
const fs = require("node:fs");

const base = process.env.BASE_URL || "http://127.0.0.1:8080";
const password = process.env.ADMIN_PASSWORD;
const out = process.env.OUT || path.join(__dirname, "..", "internal", "web", "static", "assets", "manual", "img");
const languages = ["en", "de"];
if (!password) throw new Error("set ADMIN_PASSWORD");

async function signIn(browser, username, pw) {
  const page = await (await browser.newContext({ viewport: { width: 1024, height: 720 }, deviceScaleFactor: 1, colorScheme: "light", timezoneId: "Europe/Zurich" })).newPage();
  await page.goto(base + "/");
  await page.fill("input[name=username]", username);
  await page.fill("input[name=password]", pw);
  await page.click(".login-submit");
  await page.waitForSelector("#welcome");
  return page;
}

const api = (page, method, url, body) => page.evaluate(async ([m, u, b]) => {
  const r = await fetch(u, { method: m, headers: b ? { "Content-Type": "application/json" } : {}, body: b ? JSON.stringify(b) : undefined });
  const text = await r.text();
  if (!r.ok && r.status !== 409) throw new Error(`${m} ${u}: ${r.status} ${text}`);
  try { return JSON.parse(text); } catch { return {}; }
}, [method, url, body]);

// Example content: a room with pages and tasks, a process engine without a
// shop, the shop's types and a catalogue with a product.
async function seed(page) {
  // A fresh installation starts from the ready-made types.
  const { templates } = await api(page, "GET", "/api/types/templates");
  for (const td of templates || []) await api(page, "POST", "/api/types", td);
  await api(page, "POST", "/api/spaces", { key: "TEAM", name: "Team Platform" });
  const pages = {};
  for (const [title, text] of [["Onboarding", "Everything a new colleague needs in the first week."], ["Release plan", "What ships when, and who is in charge."], ["Meeting notes", "Decisions and open points of our weekly meeting."]]) {
    pages[title] = (await api(page, "POST", "/api/items", { space: "TEAM", type: "page", title, body: [{ kind: "heading", level: 2, text: "Overview" }, { kind: "paragraph", text }, { kind: "bullet", text: "Read the handbook" }, { kind: "bullet", text: "Meet the team" }] })).item;
  }
  const day = (n) => new Date(Date.now() + n * 86400000).toISOString().slice(0, 10);
  for (const [title, due, priority] of [["Prepare the release notes", day(2), "high"], ["Review the onboarding page", day(5), "normal"], ["Book the team event", day(9), "low"]]) {
    await api(page, "POST", "/api/items", { space: "TEAM", type: "task", title, properties: { due, priority } });
  }
  await api(page, "PUT", "/api/engines/atlas", { name: "Atlas", kind: "atlas", baseURL: "https://atlas.invalid" });
  await api(page, "POST", "/api/shop/types", {});
  const cat = (await api(page, "POST", "/api/items", { space: "TEAM", type: "shop-catalogue", title: "IT workplace", properties: { engine: "atlas", languages: "de, en", rank: 1, audience: "staff" } })).item;
  await api(page, "POST", "/api/items", { space: "TEAM", type: "shop-product", title: "Laptop", parent: cat.id, properties: { "provision-process": "laptop-provide", "deprovision-process": "laptop-return", approval: "superior", price: "CHF 1200" } });
  await api(page, "POST", "/api/users", { username: "erika", password: "Erika-Shots-Passwort-1" });
  return { pages, cat };
}

// shot takes a picture of an element, or of the window as one sees it
// when given a page.
const shot = async (target, file) => {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  await target.screenshot({ path: file, animations: "disabled" });
  console.log("wrote", path.relative(process.cwd(), file));
};

(async () => {
  const browser = await chromium.launch({ ...(process.env.CHROMIUM ? { executablePath: process.env.CHROMIUM } : {}) });
  const admin = await signIn(browser, "admin", password);
  const { pages, cat } = await seed(admin);
  const user = await signIn(browser, "erika", "Erika-Shots-Passwort-1");

  // The symbols of the modules, the same in every language, sharp.
  const icons = await (await browser.newContext({ viewport: { width: 1024, height: 720 }, deviceScaleFactor: 2, storageState: await admin.context().storageState() })).newPage();
  await icons.goto(base + "/#/");
  await icons.waitForSelector(".home-tiles a .module-icon");
  for (const tile of await icons.locator(".home-tiles a").all()) {
    const id = (await tile.getAttribute("id")).replace(/^home-/, "");
    await shot(tile.locator(".module-icon"), path.join(out, "icons", id + ".png"));
  }

  for (const lang of languages) {
    for (const p of [admin, user]) {
      await api(p, "PUT", "/api/me/language", { language: lang });
      await p.goto(base + "/#/");
      await p.reload();
      await p.waitForSelector("#welcome");
    }
    const dir = path.join(out, lang);
    await shot(admin.locator(".home-tiles"), path.join(dir, "home.png"));
    await admin.click("#app-switcher");
    await admin.waitForSelector("#drawer:not([hidden])");
    await shot(admin.locator("#drawer"), path.join(dir, "modules.png"));
    await admin.keyboard.press("Escape");

    await admin.goto(base + "/#/item/" + pages.Onboarding.key);
    await admin.waitForSelector(".item-layout");
    await admin.waitForTimeout(500);
    await shot(admin, path.join(dir, "page.png"));

    await admin.goto(base + "/#/planning");
    await admin.waitForSelector("#tasks");
    await admin.waitForTimeout(500);
    await shot(admin, path.join(dir, "tasks.png"));

    await admin.goto(base + "/#/calendar");
    await admin.waitForSelector("#calendar");
    await admin.waitForTimeout(800);
    await shot(admin, path.join(dir, "calendar.png"));

    await admin.goto(base + "/#/shop");
    await admin.waitForSelector("#shop-none");
    await shot(admin.locator("#shop-none"), path.join(dir, "shop-setup.png"));
    await user.goto(base + "/#/shop");
    await user.waitForSelector("#shop-none");
    await shot(user.locator("#shop-none"), path.join(dir, "shop-user.png"));

    await admin.goto(base + "/#/item/" + cat.key);
    await admin.waitForSelector("#shop-catalogue");
    await admin.click("#shopcat-help summary");
    await admin.waitForTimeout(400);
    await shot(admin.locator("#shop-catalogue"), path.join(dir, "shop-catalogue.png"));

    await admin.goto(base + "/#/admin?section=engines");
    await admin.waitForSelector("#engine-list");
    await admin.click("#engine-list tr[data-engine=atlas] button");
    await admin.waitForSelector("#shop-types");
    await admin.waitForTimeout(400);
    await shot(admin.locator("#engine-list"), path.join(dir, "engines.png"));
    await shot(admin.locator("#engine-settings"), path.join(dir, "engine-form.png"));
    await shot(admin.locator("#shop-types"), path.join(dir, "shop-types.png"));
    await shot(admin.locator("#engine-system"), path.join(dir, "engine-connect.png"));
  }
  await browser.close();
})().catch((err) => { console.error(err); process.exit(1); });
