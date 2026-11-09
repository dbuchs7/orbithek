# ADR-0169: Workshop tools on a whiteboard: notes, voting, a timer and a presentation

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-09
- **Deciders:** Project initiators (request of 2026-11-09: on the whiteboard, the cursors of the others, notes, voting, a timer, comments and a presentation mode; external participants not yet)
- **Open question:** whether a vote, a timer or a presentation needs a person in charge (a facilitator) who alone may start and end them; this record lets everyone who may edit do so. Whether the result of a vote should stay on the whiteboard itself, rather than in a comment, is open too.
- **Question checked:** 2026-11

## Context and problem statement

A live whiteboard (ADR-0168) lets twenty people draw on one surface. A workshop needs more than drawing: people write ideas on notes, choose among them by votes, work against a time box, and follow the one who leads through the results, frame by frame. Comments on elements are recorded apart (ADR-0170).

Which of these are part of the drawing, which belong to the session, and what is kept when the session ends?

## Decision drivers

- What is content stays in the drawing; what lasts only while people meet stays in the session and is never written into a picture by accident.
- The result of a decision taken together (a vote) is kept, readable later and attributable.
- Nothing new in the item model (D1, D4); no new event kinds for what is momentary.
- Agents take part with their caller's rights (ADR-0012): they may write notes and vote.
- The UI's strings come from the catalogue (ADR-0017); nothing is worded by the server.

## Considered options

1. **Notes as elements of the scene; voting, timer and presentation as state of the session; a vote's result kept as a comment on the item.**
2. **Everything as state of the session**, nothing kept.
3. **Everything in the scene**: votes as counters in the elements' data, the timer and the presentation as elements.

## Decision outcome

Chosen option: **1**.

### Notes

- A note is an Excalidraw rectangle filled with one of six colours, with rounded corners and text bound to it, marked `customData.wikiwikibangbang.kind = "note"`. It is content like any element: it is saved in the picture and opens in the drawing editor as a shape with text.
- The whiteboard's tool bar places a note in the middle of the view in the chosen colour, selects it and opens its text for typing.

### Voting

- Whoever may edit starts a vote with a title and a number of votes per person (1 to 20, three by default). While it is open each participant places their votes on notes and shapes with text, several on one if they like, and takes them back; each sees only their own votes and how many are left, and everyone sees how many people have voted.
- Ending the vote shows the counts to everyone, on the elements, until someone clears them or starts the next vote. The browser that ended it writes the result as a comment on the item — the title and the elements by votes, in the language of the person who ended it — so the decision is kept and attributed (ADR-0035).
- A vote is counted per person, not per browser; an agent may vote, but neither start nor end a vote.

### Timer

- Whoever may edit starts a countdown of 10 seconds to 2 hours, pauses, resumes, adds a minute or stops it. Everyone sees the same time: the server sends the moment it ends and its own clock, and each browser counts from there. At the end the whiteboard says so.

### Presentation

- One participant at a time presents. The frames of the scene are the slides, in reading order (top to bottom, then left to right); the presenter steps through them, and everyone else's view follows, fitted to the frame and read-only while following. A scene without frames follows the presenter's view instead.
- A follower may stop following and look around, and follow again. The presentation ends when the presenter ends it or leaves.

### What is kept

| Tool | Kept in | After the session |
|---|---|---|
| Notes | the scene | in every saved version |
| Votes while open | the session | gone |
| Result of a vote | a comment on the item | kept, attributed |
| Timer | the session | gone |
| Presentation | the session | gone |

### Consequences

- **Positive:** no new event kind, no change to the codec for any of the four; a picture never holds a stray counter or timer.
- **Positive:** a vote's result is found where the item's other discussion is, and its author answers for it.
- **Negative / trade-offs accepted:** only a person's browser ends a vote and writes its result, since the result is worded in a person's language; a vote still open when the session closes is lost.
- **Negative / trade-offs accepted:** everyone who may edit may stop another's timer or vote; workshops agree on that among themselves until a facilitator role exists.

## Pros and cons of the options

### Option 2: everything in the session

Strongest form: the simplest; nothing to clean up.

- Bad: a vote is a decision; losing its result when the last person closes the tab defeats its purpose.

### Option 3: everything in the scene

Strongest form: one place for all; results travel with copies of the drawing.

- Bad: anonymous votes cannot be kept in data every reader of the picture can see; a running timer in a saved picture is wrong the moment it is saved; every vote would be a new version of the PNG.

## Links

- builds on [ADR-0168](0168-live-whiteboards-on-drawings.md) (live whiteboards)
- relates to [ADR-0035](0035-comments-are-part-of-their-item.md) (comments), [ADR-0170](0170-comments-on-the-elements-of-a-drawing.md) (comments on elements), [ADR-0017](0017-internationalisation-via-message-catalogue.md) (UI strings)
