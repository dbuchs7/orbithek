# ADR-0251: A process engine's shop in the wiki, and its catalogues maintained as items

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-24
- **Deciders:** the maintainer asked whether atlas's shop could be built in the wiki, with forms, decisions and processes staying in atlas and the catalogue and the shop in the wiki; they decided that catalogues are maintained with the wiki's means and shown in it, that both parts are built, and that for now only people with an account order (2026-11-24)
- **Open question:** what of a product the engine knows and this record does not map — actions of a product's lifecycle, targets, a service level, a structured offering, pictures — is lost on the next publish of a catalogue maintained here, and whether the mapping should grow or the engine should keep those fields across an import
- **Question checked:** 2026-11

## Context and problem statement

atlas, the process engine this wiki connects to (ADR-0129, ADR-0144, ADR-0148, ADR-0151, ADR-0152), carries a shop: catalogues of products, each bound to the processes that provision it and take it back; orders placed against a frozen release of a catalogue; approvals; and an inventory of what each person holds.
In atlas these are three models with separate lifetimes — catalogue, order, inventory — and the order and the inventory are part of the engine's own state, written by the same single writer that runs the processes.

The maintainer asked for the shop and its catalogues in the wiki, while forms, decisions and processes stay in atlas.
A catalogue is text that is written, discussed, reviewed and versioned before it is offered — the work the wiki is built for.
An order, an approval and a held right are not: they are facts the engine must be able to rely on while it provisions, for years.

The question: which part lives where, how does the wiki act at the engine for a person, and how does a catalogue written here become one the engine offers?

## Decision drivers

- One model for content (D1): a catalogue and its products are items, not a parallel domain object.
- The engine decides what each person sees, orders and approves, as on its own shop page; the wiki adds no privilege and decides nothing in the engine's place.
- A person's work runs on the person's own grant (ADR-0151, ADR-0152); a system grant never acts for a person it names.
- What is ordered and held must not exist twice: two records of who holds what drift apart silently.
- What a publish sends must be visible before it is sent, and the engine's problems must reach the person who publishes.
- An engine that offers none of this is connected as before.

## Considered options

1. **The shop in the wiki over the engine's API, the catalogue maintained as items and published as the engine's catalogue document**, with a grant of the person's shop.
2. **The shop in the wiki, the catalogue maintained in the engine**: the wiki shows what the engine holds, nothing is maintained here.
3. **Catalogue, orders and fulfilment in the wiki**: the wiki keeps orders as items and starts the engine's processes itself.
4. **A system grant that acts for the people it names**, instead of a grant per person.

## Decision outcome

Chosen option: **1**, because it puts the catalogue where it is written and the order where it is fulfilled, adds no second record of what is held, and keeps every decision about a person at the engine.

### A grant of a person's shop

- The engine publishes a resource `…/shop` beside `…/tasks` (atlas's record of the same date).
  A grant of it reaches the person's tasks as before and, besides them, the catalogues the person may order from, their orders and approvals, and — for those the engine lets maintain catalogues — the import of a catalogue document.
- A person connects for the shop from the shop page (`GET /api/engines/{id}/connect?shop=1`).
  The connection records that its grant is one of the shop (`engineconn.Profile.Shop`, in the connections' journal as an optional field, so older records read as before).
  Connecting again from anywhere asks for the shop again, so a grant never narrows by reconnecting.
- An engine that does not publish the resource is connected for tasks alone; the shop page says that the engine offers no shop.
- Pre-consent by the engine's administrator (ADR-0152) applies to the shop's grant as to the tasks' grant; the engine decides that.

### The shop

- Routes under `/api/engines/{id}/shop/…` name each engine route they call: the catalogues, a catalogue's newest release, a product's order form and the searches of its fields, pictures and logos, placing, reading, withdrawing and returning orders, the approvals one holds and deciding them, and the open tasks of one's orders.
  Each runs with the caller's access token; the engine's JSON is handed on as it is given, and its refusals come back with its status and its words.
- A request body is one JSON object sent as `application/json`, which keeps a cross-site form out as the rest of the API does.
- Pictures are handed on only as PNG, JPEG, WebP or GIF, never SVG, with `nosniff` and a content security policy that allows nothing.
- Only people with an account here reach the shop, connected to the engine with their own grant; there is no anonymous ordering.
- The shop is reached from the module switcher and from a quick access of its own on the start page, with a shopping cart as its symbol.
- The shop page (`#/shop`) shows the catalogues as tiles, a product with its order form, a basket per catalogue release, one's orders with the state of each position, and the approvals one holds.

### Catalogues as items

- Two installation types, set up by an administrator (`POST /api/shop/types`): `shop-catalogue` and `shop-product`.
  A product's workflow is the engine's: draft, active, withdrawn.
  The fields carry what the engine keeps of a catalogue and a product: the engine, the IDs, languages and rank, the audience, the processes that provision and take back, the approval, the order form, price, category, whether a product may be held more than once, how long, when it may be ordered, who may order it, and a name per language.
- A catalogue's products are the `shop-product` items below it in the page tree, at any depth (D5: the catalogue is the product's home).
  Structure and precedence are relations made by hand with the engine's own words: `composition`, `aggregation`, `requires` and `excludes`.
  A product's labels are its keywords; its text, without protected passages (ADR-0088), is its description in the catalogue's first language.
- An ID left empty is made of the item's ID (`ot-<id>`), which never changes (D2); keys are not used.
- `GET /api/items/{ref}/shop/document` shows the document a catalogue would be published as, or every problem the wiki can see.
- `POST /api/items/{ref}/shop/publish` sends it to the catalogue's engine with the publisher's grant of the shop, replacing what the engine held of the catalogue and its products (`overwrite=*`) and asking for a release; `{"dryRun": true}` asks the engine what it would do.
  Who may edit the catalogue item may publish it; whether they maintain catalogues at the engine, the engine decides.
- The document says where the catalogue is maintained (`maintainedIn.url`, the address of its item, sent only when it is https, as the engine requires); the engine marks the catalogue as maintained by this server's grant either way and then refuses changes made in its own catalogue editor, so the next publish cannot overwrite them unseen.
- Mapping and validation are a pure function of the items (`internal/shopcat`), so what is sent is what is shown.

### What stays in the engine

Forms, decisions and processes are modelled and run in atlas.
Releases, orders, approvals, fulfilment and the inventory of held rights are atlas's records; the wiki shows them and asks for changes to them, it never keeps them.

### Consequences

- **Positive:** a catalogue is written with versions, comments, reviews, protections, labels and boards like any other page.
- **Positive:** nothing is ordered or held twice; the engine's rules for ordering and approving apply unchanged, wherever a person orders.
- **Positive:** the engine's problems reach the publisher before anything changes, through the dry run.
- **Negative / trade-offs accepted:** a person who wants to shop connects again once, since their earlier grant was one of tasks alone.
- **Negative / trade-offs accepted:** only the fields this record maps survive a publish; what is set at the engine besides is replaced (see the open question).
- **Negative / trade-offs accepted:** a product the publisher may not read is not in what they publish (D10); a catalogue whose products are protected from some of its editors should be published by one who reads them all.
- **Negative / trade-offs accepted:** a description is sent in one language; names may be given in four.
- **Follow-ups / risks to watch:** pictures of products from a file field; descriptions in further languages; the shop's tasks and approvals as linked tasks (ADR-0151); MCP tools for the shop.

## Pros and cons of the options

### Option 2: the catalogue maintained in the engine

Strongest form: the engine already has a catalogue editor with every field, structure checks and a preview; the wiki would only show the shop, and no field could be lost between two systems.

- Good: nothing to map, nothing to lose on a publish.
- Bad: the catalogue is not written with the wiki's means — versions, comments, reviews, protections — which is what the maintainer asked for.
- Bad: product owners who work in the wiki would need an account and a role in the engine to change a text.

### Option 3: orders and fulfilment in the wiki

Strongest form: the engine would be a pure process runtime and every business object would be an item, queryable, on boards and in the starmap, with one permission model.

- Good: one place for every object a person sees.
- Bad: atlas refuses to start a process bound to a catalogue outside an order, precisely so that nothing is provisioned past its inventory; the wiki would have to be granted a way around that.
- Bad: the hard parts — fulfilment in dependency order that survives a crash, failed, rejected and blocked positions, approvals, the inventory and its reconciliation with target systems — live in the engine's state and would be rebuilt here, with two records of who holds what.

### Option 4: a system grant acting for people

Strongest form: one credential, no consent per person, no second connection.

- Good: simpler to set up.
- Bad: the engine would decide on the wiki's word who a person is; ADR-0152 and atlas's record of connected systems rule this out, and an order or an approval would no longer be the person's own act at the engine.

## Links

- relates to ADR-0129, ADR-0144 and ADR-0148 (processes, forms and decisions from an engine)
- relates to ADR-0151 and ADR-0152 (people's grants and the system grant)
- relates to ADR-0001 (one model for content), ADR-0088 (protected passages)
- relates to atlas ADR-0312 (catalogue, order and inventory as three models), ADR-0436 (a catalogue imported as one document), ADR-0510 and ADR-0512 (grants of connected systems), ADR-0550 (the shop's customer side over MCP)
