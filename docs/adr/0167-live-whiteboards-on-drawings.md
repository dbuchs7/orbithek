# ADR-0167: Live whiteboards: a drawing edited by up to twenty people at once, with a lock per element

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09: a whiteboard for workshops of up to 20 people, built on the drawings, with the cursors of the others, a lock per element, and AI agents working on it over MCP)
- **Open question:** whether a picture kept current only by the browsers of the participants is enough, or whether pages need a picture drawn on the server; this record keeps the browser as the only renderer and marks pictures written without one as stale (see *Checkpoints*). It also leaves open whether the version of a checkpoint should name every participant rather than the one whose browser saved it.
- **Question checked:** 2026-11

## Context and problem statement

A drawing (ADR-0053, ADR-0126) is a PNG attachment that carries its Excalidraw scene; the picture is the source, and every saved edit is a new version of the file (ADR-0055). One person edits it at a time: whoever saves last wins.

Workshops need the opposite: up to twenty people working on one surface at the same time, seeing each other's cursors and changes as they happen, and AI agents taking part. Live editing exists for text (ADR-0010): a session per item on the server, events as server-sent events, operations as POSTs, a lock per block, a commit when a lock is released.

How does a drawing become such a whiteboard without a second kind of content, without a second server runtime, and without giving up that a drawing is a picture that shows wherever pictures show?

## Decision drivers

- A whiteboard *is* a drawing: one stack of PNG versions in one item, shown in pages, exports, the album and other clients as a picture, opened in the drawing editor as before (ADR-0053). No new domain object (D1), no new body block.
- Several people edit at once without overwriting each other; no change is silently lost or attributed to the wrong person.
- One binary, standard library, no CGO, no runtime beside the server (ADR-0004, ADR-0009); the transport is the one of ADR-0010.
- Access is the item's: reading joins, editing changes; nothing reveals an item to one who may not read it (D9, D10).
- Content never reaches disk unencrypted (ADR-0026); budgets bound what a session holds (ADR-0015).
- Agents work through the public API with their caller's rights, over MCP (ADR-0012).

## Considered options

1. **A live session per drawing on the server, a lock per element, the picture saved in checkpoints rendered by a participant's browser, the scene written by the server.**
2. **A CRDT of the scene** (e.g. a Yjs document), merged in every browser and on the server.
3. **The whiteboard as content of its own**: the scene as structured data in the event log, the picture only a derived preview.
4. **A collaboration server beside the product** (Excalidraw's own room server, Node.js).
5. **A save per change**: every change is a new version of the PNG.

## Decision outcome

Chosen option: **1**, because it is the only one that keeps a drawing a picture (driver one) while letting twenty people work at once with the transport, the access checks and the locking discipline the product already has.

### What a whiteboard is

- A whiteboard is a drawing opened live. It is named by its item and its first version (`Picture.First`, ADR-0103); any version's ID leads to it. The address is `#/whiteboard/{item}/{file}`, the API `/api/items/{ref}/whiteboards/{file}`.
- `POST /api/items/{ref}/whiteboards` makes a new one in an item one may edit: an empty scene in a white placeholder picture, marked stale (see below), as a file `<title>.drawing.png`, and an image block at the end of the body.
- A drawing among a page's files shows *Open as whiteboard*; the module *Whiteboards* behind the nine dots lists the drawings one may read, those with a live session first, and makes new ones.

### The session

- `internal/whiteboard` holds one session per whiteboard in memory, as `internal/collab` does for text: the scene's elements (each kept as the JSON the browser sent, read only for its ID, version, deletion, type and the few fields an agent sees), its image files, a lock per element and the participants.
- `GET …/live` joins with a stream of server-sent events, starting with a snapshot; `POST …/live` sends operations. Reading the item and the drawing's newest version is enough to join; changing needs the right to edit. Who joins without it follows read-only.
- **A lock per element.** An element is changed only by the client holding its lock. Selecting elements locks them (with the text bound to a container, and the container of a selected text); a change to a free element takes its lock too; deselecting, leaving and two minutes without any operation release it. The others see a held element locked, outlined in its holder's colour, and cannot select it.
- **Versions decide.** Excalidraw counts a version per element. The server takes a change only with a higher version than it holds; a stale change, or one to an element held by someone else (an arrow bound to a moved shape, say), is refused for that element alone and answered with the element as the server holds it, so the sender falls back in step.
- **Cursors.** Pointer positions are sent at most ten times a second and broadcast together every 100 ms; they are not kept. Each person has a colour of their own for cursor and selection.
- **Budgets** (ADR-0015): `WhiteboardSessions`, `WhiteboardClients` (20 by default: the workshop size asked for), `WhiteboardElements`, `WhiteboardSceneBytes` (elements and images held by one session) and `WhiteboardRequestBytes` (one request of operations, which may carry an image).

### Checkpoints

The browser is the only renderer: the server can write a PNG's chunks, not draw Excalidraw's shapes and fonts.

- **Who saves.** The editing browser that joined first is the session's *keeper*. When the scene changed and was quiet for ten seconds, or changed for two minutes on end, or someone asks to save, the server sends the keeper a `checkpoint` with a token and keeps a copy of the scene as it is at that event. The keeper renders exactly that state — the events reach it in order — and uploads the picture to `POST …/checkpoints`.
- **The server writes the scene.** It does not trust the scene of an uploaded picture: it removes Excalidraw's text chunk and writes its own copy, so what a version holds is what the session held, whatever a browser sent. The picture is stored as a new version of the drawing (source: the newest version), attributed to the keeper, and the image blocks of the item's body that show any version of it are pointed at it, unless a live text session holds locks on that body (ADR-0010), when the next checkpoint does it.
- **Without a browser.** When no editing browser is in the session — an agent changed it alone, everyone left, or the server shuts down — the server writes the new scene into the newest picture as it is and marks the PNG stale with a private chunk `wkSt`. The scene is never lost; the picture lags. The next keeper renders a stale picture again at once.
- **Versions pile up.** A two-hour workshop leaves dozens of versions. The retention of picture versions (ADR-0104) removes them by its rules; the current version and the versions the body shows stay.

### Agents

- `GET /api/items/{ref}/whiteboards/{file}` reads a whiteboard as an agent needs it: each element's ID, kind, text, position, size, colour, frame, container and the ends of an arrow, who holds it, and the session's participants, timer, vote and presentation (ADR-0168). Without a session it reads the newest picture's scene.
- `POST …/changes` takes simple changes: a note, a text, a shape, a frame, an arrow between two elements, new text, a move, a colour, a deletion, a vote. The server makes Excalidraw's elements from them, with text measured by estimate and marked, so the keeper's browser measures them again; each change takes and releases its locks within the request, and an element someone holds is refused with their name. An agent shows among the participants for two minutes after its last change.
- MCP offers `create_whiteboard`, `get_whiteboard` and `change_whiteboard`; nothing destructive beyond deleting elements, which editing a body already allows.

### Consequences

- **Positive:** a whiteboard is a drawing everywhere: pages, exports, the album, copies (ADR-0055), backups and erasure need nothing new; access, encryption and history are the item's.
- **Positive:** twenty people and agents work at once; a lock per element means no merge can go wrong, and a refused change corrects itself.
- **Negative / trade-offs accepted:** the scene of a live session lives in memory between checkpoints. A crash loses at most the changes since the last checkpoint (ten seconds of quiet, two minutes at most while someone works); a graceful shutdown loses nothing but the freshness of the picture.
- **Negative / trade-offs accepted:** a version is attributed to the participant whose browser saved it, though others drew in it.
- **Negative / trade-offs accepted:** two people cannot change the same element at once; an arrow bound to a shape someone else holds does not follow until that shape is free.
- **Negative / trade-offs accepted:** text in a whiteboard is not found by the full-text search, as for every drawing (ADR-0033).
- **Follow-ups / risks to watch:** the performance of Excalidraw with thousands of elements and twenty cursors is not measured; elements made by agents are measured by estimate until a browser measures them.

## Implementation (2026-11-09)

Built: `internal/whiteboard` (the session, element locks, cursors, checkpoints, the scene in the PNG, the server's stale save, agents' reading and changes, the workshop tools of ADR-0168), the routes in `internal/server/api_whiteboards.go`, the MCP tools, the drawing page's live mode (`assets/whiteboard.js`, its browser-free parts in `assets/whiteboard-parts.js`), the module *Whiteboards*, *Open as whiteboard* beside every drawing among a page's files and *New whiteboard* below them, and a step of the browser smoke test with two people and an agent.

Not built yet:

- the process chooser of ADR-0142 on a live whiteboard: its dialogs belong to the UI around the drawing page, which a live whiteboard does not talk to;
- operational metrics of the live whiteboards (ADR-0016);
- a measurement of Excalidraw with thousands of elements and twenty cursors.

## Pros and cons of the options

### Option 2: a CRDT of the scene

Strongest form: no locks at all, offline edits merge, and Excalidraw's own collaboration already reconciles elements by version, which is close to a CRDT per element.

- Good: the most fluid editing; nobody is ever refused.
- Bad: a merge is code in two languages that must agree on every element type; the Go side has no mature implementation (ADR-0010); a whiteboard element is small, so a lock costs little; and agents would have to speak the CRDT's protocol rather than make plain requests.

### Option 3: content of its own

Strongest form: elements as events of the item would be searchable, could carry comments and votes as properties, and would need no browser to save.

- Good: no picture lag, no checkpoint protocol; text searchable.
- Bad: a second kind of drawing beside ADR-0053's, a new body form and codec, a preview that still needs a renderer; the request was a whiteboard built on the drawings.

### Option 4: a collaboration server beside the product

Strongest form: proven, maintained, and finished today.

- Bad: a second runtime to ship and run (ADR-0009); its rooms know nothing of the item's access, encryption, budgets or audit; agents could not reach it through MCP.

### Option 5: a save per change

Strongest form: nothing in memory can be lost.

- Bad: a PNG of megabytes per stroke, rendered by whichever browser moved last, for twenty people at once; thousands of versions an hour.

## Links

- builds on [ADR-0053](0053-drawings-with-excalidraw-on-a-page-of-their-own.md) (drawings), [ADR-0010](0010-real-time-collaboration-with-block-locks.md) (live sessions, locks), [ADR-0055](0055-copied-files-carry-their-source.md) (versions), [ADR-0104](0104-rules-for-the-older-versions-of-pictures.md) (retention)
- relates to [ADR-0012](0012-mcp-adapter-over-http-api.md) (MCP), [ADR-0015](0015-resource-budgets-and-paged-listings.md) (budgets), [ADR-0022](0022-permission-model.md) (access), [ADR-0026](0026-crypto-shredding-of-item-content.md) (encryption), [ADR-0142](0142-processes-in-a-drawing.md) (the private chunk of a drawing's PNG)
- extended by [ADR-0168](0168-workshop-tools-on-a-whiteboard.md) (notes, voting, timer, presentation) and [ADR-0169](0169-comments-on-the-elements-of-a-drawing.md) (comments on elements)
- adapted from atlas [ADR-0140](https://github.com/pblumer/atlas/blob/main/docs/adr/0140-live-collaborative-modeling-sessions.md) (element locks for diagrams, agents as participants)
