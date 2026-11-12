# ADR-0209: Capturing into the wiki from anywhere

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators (request of 2026-11-12 for an installable web app with a share target, a capture view and a bookmarklet)
- **Open question:** whether mail sent to an address of the wiki, and messages to a bot in a messenger, should become captures; whether a service worker becomes necessary for browsers that offer installing or sharing only with one; whether one's note of the day should be the first target once it exists; whether the share target should also accept files, such as photos
- **Question checked:** 2026-11

## Context and problem statement

Most of what people want to keep comes to them outside the wiki: a page in the browser, a passage they selected, a link a colleague sent to their phone. Opening the wiki, finding the right page, entering edit mode and pasting is enough friction that much is never kept.

How does content reach the wiki from wherever a person is — another web page on the desktop, any app on the phone — with their own rights, without an extension to install, without a native app, and without weakening authentication or the content security policy?

## Decision drivers

- **Their own rights (D10):** a capture is saved through the API as the person, after they chose where.
- **One binary, no store (ADR-0009):** no browser extension, no native app, nothing from a CDN.
- **No content outside the item store:** captures are not kept offline in the browser, nor queued on the server.
- **Authentication stays as it is:** routes are authenticated unless declared public; the session cookie stays SameSite=Strict (ADR-0027).
- **Privacy of what is shared:** a shared text should not end up in access logs of the server or of a reverse proxy.
- **Extensible:** one's note of the day, built in parallel, and later mail or bots, are more targets, not another view.
- **Live editing is respected (ADR-0010):** adding to an item does not overwrite a body being edited.

## Considered options

1. **An installable web app with a share target, a capture view at `#/capture`, and a bookmarklet;** additions to an item through a new route that appends blocks.
2. **A browser extension** for the desktop and native share extensions for phones.
3. **Capture by mail or a messenger bot only.**
4. **Adding to an item by reading and writing its whole body from the browser** (`PUT /api/items/{ref}/body`).

## Decision outcome

Chosen option: **1**.

- **The installable app.** `GET /manifest.webmanifest` (public: browsers fetch a manifest without the session cookie) names the installation as the corporate design does (ADR-0052), with the product's icons (192, 512 and a maskable 512, copied from `docs/brand/favicon`), start address `/`, display `standalone`, the header colour as theme colour, a shortcut to the capture view, and a share target: `POST /capture` with `title`, `text` and `url` as a form. The page links it. **No service worker** is served: the browsers that install web apps and offer them as share targets do so without one, and the decision not to keep items in the browser makes caching pointless.
- **The share target** `GET` and `POST /capture` (public) take the three fields and nothing else, bounded by `RequestBody`, and redirect (303) to `/#/capture?…`. The content travels on in the fragment, which browsers do not send, so after this one request neither this server's logs nor a proxy's see it again; the redirect itself reveals and stores nothing, so it needs no principal. The view then signs in as any view does (a capture, like an assistant's consent page, survives signing in), and its API calls are same-site requests that carry the Strict cookie even when the share came from another app.
- **The capture view** `#/capture?title=&text=&url=` shows the three fields, editable, and the targets that take them: **a web link** when there is an address (ADR-0208; the page's text on request), **a new item** of a type that has a body and no required field, in a room and below an item chosen, and **an addition to an item** found by searching. A text becomes quotes, one per paragraph, and the address follows as a paragraph naming the source, linked. An address alone at the end of a shared text is taken as the url, as many phones share it so. Nothing is saved before *Save*. It is reached from *New*, from the app's shortcut and from shares; a window the bookmarklet opened offers to close after saving.
- **Targets are data.** `TARGETS` in `assets/capture.js` says which targets take a capture; `TARGET_VIEWS` in `assets/captureview.js` gives each its label, fields and action, which answers the saved item. One's note of the day becomes one entry in each: it takes every capture, has no fields, and its action asks the daily note's route for today's note and appends to it with the route below.
- **Appending.** `POST /api/items/{ref}/body/blocks` adds blocks at the end of a body in one step: the body as the caller reads it, extended, committed under the store's lock, so nothing written between reading and writing is lost. It is refused (409) while a live session holds a block or an unsaved draft, as a whole body is; protected passages stay protected, and who does not read every passage cannot change the text (ADR-0088). MCP offers it as `append_to_body`.
- **The bookmarklet** is offered on the account page: a button to drag to the bookmarks and the code to copy. It opens the capture view of this site in a small window with the page's title, address and the selected text, each cut to the capture's limits. It is the one `javascript:` address the UI shows, a constant around the site's origin quoted as a string, set outside `h()`'s link filter on purpose; the UI's content security policy keeps it from running on Orbithek's own page.

### Consequences

- **Positive:** a page, a quote or a thought is kept in two steps from any page or app; no extension, no native app, no change to authentication; the capture's content stays out of logs after the first request; appending is safe against concurrent writes.
- **Negative / trade-offs accepted:** installing and sharing depend on the browser: some desktop browsers and some phone browsers do not offer share targets, where the bookmarklet or copying remains; a reverse proxy or sign-on gateway in front must let `/manifest.webmanifest` and `/capture` through without signing in; the first request of a share still carries its content to the server and any proxy before it.
- **Follow-ups / risks to watch:** the daily note's target at integration; mail-in and messenger bots as further targets.

## Pros and cons of the options

### Option 1: web app, share target, capture view, bookmarklet

- Good: one binary; works with the person's session and rights; one view for every way in.
- Bad: depends on browser support for installing and share targets.

### Option 2: browser extension and native share extensions

Strongest form: an extension can read the page's metadata and selection precisely, capture in one click without a window, work offline and offer a context menu; native share extensions are offered by every app on a phone, in every browser.

- Bad: separate artifacts to build, sign, review in stores and keep in step with the server, against ADR-0009; each holds a credential of its own outside the browser session; an organisation must approve and distribute them.

### Option 3: mail or a bot only

Strongest form: everyone can send mail from anywhere, with no installation at all, and a bot meets people in the messenger they use all day.

- Bad: a mail address or a bot needs an identity check of its own (who sent this?), an inbound mail server or a messenger's credentials, and choosing where a capture goes is awkward by mail; left as open question, possibly a later target.

### Option 4: whole-body writes from the browser

Strongest form: no new route; the existing body API already replaces a body, and the browser has the body in hand.

- Bad: reading and writing back loses what someone wrote in between; an agent must send a long body back to add one line.

## Links

- relates to ADR-0009, ADR-0010, ADR-0012, ADR-0014, ADR-0027, ADR-0037, ADR-0052, ADR-0068, ADR-0088, ADR-0208
