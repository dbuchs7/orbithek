# ADR-DRAFT: A folder or collection is shown in the order of its slides by default

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** Project initiators (request of 2026-11-26: the order of the slides becomes the default view of a folder in the album, so that the order of a presentation is set and seen where the pictures are kept)
- **Open question:** whether the choice should be kept per folder or per person on the server rather than per browser; whether a folder whose pictures come from a camera should still open newest first
- **Question checked:** 2026-11

## Context and problem statement

ADR-0195 lets a folder or a collection keep an order of slides, set by dragging in the album.
The album shows that order only when **Order of the slides** is chosen; **Newest first** stayed the default, "as photos are mostly found by date", and the record left open whether a folder should show its order of slides by itself.
The choice is kept in the browser under `ot.albumSort`, which holds `slides` once that was chosen and the empty string once **Newest first** was chosen.

A person preparing a presentation opens a folder, sees it newest first, and has to switch before every drag; a colleague who opens the same folder sees another order than the one the presentation will have.
Which order should an open folder or collection show when nobody chose one in this browser?

## Decision drivers

- **What is seen is what is presented:** the album is where the order of slides is set (ADR-0195); the default view should show the order the presentation will have.
- **A choice once made holds:** a person who chose **Newest first** before must not lose it.
- **No new persisted data (D4, ADR-0015):** a view preference is no content and needs no event.
- **The same for every folder:** one rule is easier to predict than a default that depends on what a folder holds.

## Considered options

1. **The order of the slides is the default for every folder and collection;** **Newest first** holds only where it was chosen in this browser.
2. **The order of the slides is the default only for a folder or collection that has an order set by hand** (the open question of ADR-0195), newest first otherwise.
3. **Newest first stays the default** (ADR-0195 as it is).
4. **The choice is kept per folder** in the browser, defaulting to the order of the slides.

## Decision outcome

Chosen option: **1**, as requested.

- `sortKept` in `assets/album.js` answers `slides` unless `ot.albumSort` holds the empty string, which only an earlier choice of **Newest first** writes; where the browser keeps nothing, or storage cannot be read, the album shows the order of the slides.
- The value written for each choice is unchanged, so a browser in which **Newest first** was chosen before keeps it, and one in which **Order of the slides** was chosen keeps that.
- A folder without an order set by hand is shown, as before under **Order of the slides**, by name, numbers by their value (ADR-0195); whoever may edit it can drag at once.
- Choosing a picture to insert into a text (**Insert… → From the album**, ADR-0054) still lists pictures newest first, as it shows no order of slides.
- The timeline and the other views are not changed; they belong to no folder.
- The manual's section on ordering slides by hand says that the order of the slides is the default view.

### Consequences

- **Positive:** the album opens a folder or collection in the order it will be presented, and the order can be dragged without a choice first.
- **Positive:** no server change, no event and no codec version; earlier choices in a browser are kept.
- **Negative / trade-offs accepted:** a folder of photos from a camera, named like `IMG_1234`, now opens by name, which is mostly oldest first, rather than newest first; whoever wants newest first chooses it once per browser.
- **Negative / trade-offs accepted:** the choice is still per browser, not per folder; a person who switches to **Newest first** for one folder sees every folder so until switching back.
- **Follow-ups / risks to watch:** the open questions above.

## Pros and cons of the options

### Option 2: the order of the slides only where one was set

Strongest form: folders that are presentations get their order, folders of photos keep the date order in which photos are found; nobody sees an order by file name they never asked for.

- Good: no change for folders of photos.
- Bad: the default changes with the first drag, so a person who wants to order a folder must first switch to a view that the folder does not show yet; the request is exactly about that first step.
- Bad: the listing would have to tell, before the view is chosen, whether a folder has an order, which needs a request or a field it does not give today.

### Option 3: newest first stays the default

Strongest form: most folders hold photos, and photos are found by date; ADR-0195 chose it for that reason.

- Bad: it keeps the extra step the request is about, and shows a colleague another order than the one presented.

### Option 4: a choice per folder

Strongest form: a folder of photos and a folder of slides each keep the view that suits them, in the same browser.

- Bad: one more key per folder in the browser's storage, with no bound and no clean-up; the album would need to forget keys of folders that are gone.
- Bad: it does not answer the request any better for a folder opened for the first time, which is the case asked about.

## Links

- amends [ADR-0195](0195-the-slides-of-a-folder-or-collection-are-ordered-by-hand.md) (newest first as the default view of a folder)
- relates to ADR-0054 (pictures inserted from the album), ADR-0175 (collections), ADR-0176 (slides), ADR-0177 (the album's views)
