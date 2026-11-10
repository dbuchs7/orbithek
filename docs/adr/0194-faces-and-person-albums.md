# ADR-0194: Faces found in the browser, named by people, and recognised only where administrators allow it

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-10
- **Deciders:** Project initiators. Request of 2026-11-10: faces in the album's pictures are found and named, so that an album per person can be shown. The requester chose the same day:
  - recognition included, the person suggested on other pictures, with its model and licence settled in this record before it is built;
  - names seen by every reader of the picture;
  - faces found in the browser.

  The rest are defaults proposed with the implementation, confirmed by the requester; accepted 2026-11-10.
- **Open question:** whether recognition should also need each person's own consent in the product, not only the administrators' setting; whether a face can be drawn by hand where none was found; whether faces should be found on videos' posters; whether the more accurate SSD detector (5.4 MB) should replace the tiny one; whether agents should read persons over MCP
- **Question checked:** 2026-11

## Context and problem statement

The album (ADR-0177) finds pictures by when and where they were taken, by labels and folders. People also look for pictures by who is in them. Naming the faces on a picture by hand is personal data; recognising a person from their face is **biometric data** under the Swiss Data Protection Act (revDSG Art. 5 lit. c Ziff. 4: "biometrische Daten, die eine natürliche Person eindeutig identifizieren") and under the GDPR (Art. 4(14), Art. 9(1)). Both count it as sensitive. The exemption for purely personal use (revDSG Art. 2 Abs. 2 lit. a; GDPR Art. 2(2)(c), Recital 18) covers a person running an instance for their own family photos; it does not cover an instance of an association or a company. *These legal readings were taken from search snippets and memory, not from the primary texts, which could not be reached while this record was written; they are to be checked on fedlex and EUR-Lex.*

The binary is pure Go without C (ADR-0009, ADR-0024): it cannot run a neural network as a library does, and a second executable is excluded.

How are faces found, named and recognised within these limits, and so that biometric data exist only where they are wanted and can be removed?

## Decision drivers

- **One binary, nothing from a CDN** (ADR-0009): whatever runs a network is embedded and served by this server.
- **Licences of code and weights** compatible with AGPL-3.0, and no weights for research only.
- **Data minimisation:** biometric data off by default, kept apart from the history, deletable at once and never backed up.
- **Access unchanged** (D10, ADR-0022): a reader sees only what they may read; naming needs the edit right, as labels do (ADR-0090).
- **Bounded server load** (ADR-0015).

## Considered options

1. **The browser finds faces and, where recognition is on, computes a vector of each with the vendored face-api; the server keeps boxes and names in the journal and the vectors in a companion file, and compares them to suggest names.**
2. **The server finds and recognises faces in pure Go** (a Go detector such as pigo; a network evaluated in Go).
3. **Faces drawn and named by hand only**, without any network.
4. **A photo service recognises faces** (Google Photos, iCloud).

## Decision outcome

Chosen option: **1** (defaults proposed with the implementation, confirmed by the requester).

### What runs where

- **Library:** `@vladmandic/face-api` 1.7.15 (MIT), which bundles TensorFlow.js 4.22.0 (Apache-2.0). It is vendored as `vendor/face-api.js` and loaded only when a face is looked for. It runs on the graphics card (WebGL) where the browser offers it, else on the processor. It needs no `eval` and no change to the content security policy; nothing is fetched from elsewhere.
- **Networks**, in `vendor/face-models/`:

  | Network | Use | Size | Weights |
  |---|---|---|---|
  | tiny_face_detector | finds faces | 0.19 MB | MIT (face-api.js) |
  | face_landmark_68_tiny | aligns a face before recognising it | 0.08 MB | MIT (face-api.js) |
  | face_recognition | 128 numbers per face | 6.4 MB | dlib's `dlib_face_recognition_resnet_model_v1`, released by Davis King into the public domain |

  The recognition network is loaded only where recognition is on. The binary grows by about 8.1 MB.
- **Rejected models:** InsightFace/ArcFace weights are for non-commercial research only, a further restriction AGPL-3.0 does not admit. MediaPipe's tasks from version 1.0 send usage data to Google. FaceNet's and Human's FaceRes weights have unclear terms.

### What is kept

- **Faces** (`item.Face`: box as fractions of the picture, person) on the version of a picture they were found on, with `FacesSeen`, by the kind `AttachmentFacesSet`, sealed with the item's key (ADR-0026). Codec version 51. A deleted version forgets them; a moved file keeps them (ADR-0188).
- **Vectors** in `<id>.faces` beside the file, sealed like it, aligned with its faces. They are moved and removed with the file and healed at the start like posters (ADR-0189). They are **never backed up** and never leave the server again.
- **The setting** `site.Faces{Recognition}`, off unless administrators turn it on (`PUT /api/settings/faces`, audited as `faces.change`). Turning it off removes every `.faces` file at once. The names stay: a name typed by a person is no biometric data.

### What is done

- Opening a picture one may edit that was never searched looks for faces; **Find faces** looks again. Each picture uploaded in the album is searched from the file in the browser right after its upload; **Find faces in all pictures** searches what the album shows. A search keeps the names of the faces found again where they were and never drops a named face.
- `PUT /api/items/{ref}/attachments/{id}/faces` sets the faces, for whoever may edit the item. A face sent without a vector keeps the vector of the face of the same box, so naming a face keeps what recognises it.
- `GET …/faces/suggestions` compares the vectors of the unnamed faces with those of the named faces of every picture the caller may read. It uses the Euclidean distance, below 0.55. It answers only names, for whoever may edit the picture, and only while recognition is on. The vectors read are kept in memory, bounded, and dropped when they change.
- `GET /api/album/people` lists the persons named on the pictures one may read, with a picture count and a cover. `GET /api/album?person=` is a person's album, presentable as slides. The album's words find persons too.
- The album gets the view **People**; administration gets **Recognise faces again** with a warning that the numbers are biometric data and need the consent of the people shown.

### Consequences

- **Positive:** pictures are found by who is in them, with the access of their folders, without a service elsewhere.
- **Positive:** biometric data exist only where administrators turned them on, separate from the history, and are gone as a whole when it is turned off.
- **Negative / trade-offs accepted:**
  - About 8 MB more in the binary. The first search in a browser loads 1.5 MB, recognition 6.5 MB more.
  - Faces are found only where a browser looks: pictures nobody who may edit them opens stay unsearched until **Find faces in all pictures** runs.
  - The tiny detector misses small and turned faces. A face it misses cannot be drawn by hand yet.
  - Comparing reads the vectors of every named picture one may read; with very many pictures the first suggestion after a start is slower.
  - Codec version 51: a data directory written by this build cannot be opened by an older one.
- **Follow-ups / risks to watch:**
  - The upstream library is stagnant (last release February 2025, TensorFlow.js October 2024); it is vendored, so nothing breaks, but nothing is fixed either.
  - The dlib weights were trained partly on the VGG Face and FaceScrub datasets, whose terms could not be checked; whether such terms bind users of the trained weights is unsettled. A public-domain declaration is not a formal CC0 and may not waive every right under Swiss or German law.
  - The legal readings above are to be confirmed from the primary texts.

## Pros and cons of the options

### Option 2: on the server, in pure Go

Strongest form: every picture is searched, also those nobody opens, and the server alone decides; no browser needs a graphics card.

- Good: complete coverage; pigo is MIT and pure Go.
- Bad: pigo finds frontal faces only. A recognition network evaluated in Go is code of our own for convolutional networks, unbounded CPU per picture on the server, and a new dependency or a large piece of numeric code to maintain.

### Option 3: by hand only

Strongest form: no biometric data at all, no 8 MB, no model licences; people name exactly who they mean.

- Good: the simplest and the most private.
- Bad: drawing a box around every face of thousands of photos is not done; the requester chose recognition.

### Option 4: a photo service

Strongest form: Google and Apple recognise faces well and keep improving.

- Bad: the pictures and the biometric data would live at another service, outside the access, encryption and erasure of their folder (D1, ADR-0026), as ADR-0177 found for photo libraries; their imports carry no faces anyway.

## Links

- amends [ADR-0177](0177-photo-management-in-the-album.md) (the album as a photo library)
- relates to ADR-0009 and ADR-0024 (one binary), ADR-0015 (budgets), ADR-0026 (sealed content), ADR-0090 (labels of pictures), ADR-0176 (presentation), ADR-0178 (a photo's place kept to its uploader), ADR-0188 (moving files), ADR-0189 (companion files of videos)
