# ADR-0199: Faces suggested for a person across the album, confirmed together

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-11
- **Deciders:** Project initiators. Request of 2026-11-11: faces are recognised only once, so a person named later is never assigned on the other pictures that show them. Offered three remedies (search pictures again that have no vectors, point to that after recognition is turned on, suggest a person's faces across the album), the requester chose the last. The rest are defaults proposed with the implementation.
- **Open question:** whether a suggestion can be dismissed for good ("not this person"), which needs a mark kept with the face; whether unnamed faces that resemble each other should be grouped so that a person never named can be named at once; whether pictures searched without recognition, before it was turned on or after it was turned off and on again, should be searched again by **Find faces in all pictures**; whether agents should read these suggestions over MCP; whether the threshold of 0.55 suits suggestions across many pictures as well as within one
- **Question checked:** 2026-11

## Context and problem statement

ADR-0194 suggests a name for an unnamed face only in the viewer of one picture, and only when that picture is opened. The comparison itself is made afresh on each request, so a person named later is suggested on a picture opened later; but nobody opens thousands of pictures one by one to find where a person appears. In practice a person named on one picture stays unnamed on the others, and their album shows only the pictures named by hand.

How can the faces that resemble a person be found and named across the album, without naming anyone without a person's decision?

## Decision drivers

- **A person decides every name** (ADR-0194): a name is seen by every reader and is not biometric data only because a person typed or chose it.
- **Access unchanged** (D10, ADR-0022): faces are compared only with faces of pictures one may read, and suggested only on pictures one may edit.
- **Bounded server load** (ADR-0015): one request compares in memory and answers a bounded page.
- **Nothing new is kept**: no codec version, no new file; the vectors of ADR-0194 suffice.

## Considered options

1. **A person's album lists the unnamed faces that resemble that person more than anyone else named, most alike first; whoever may edit chooses those that show the person and confirms them together.**
2. **Faces very close to a named face are named automatically.**
3. **Unnamed faces are grouped by resemblance, and a group is named at once.**
4. **Suggestions stay in the viewer of one picture** (as ADR-0194 has it).

## Decision outcome

Chosen option: **1** (requested; the details are defaults proposed with the implementation).

### What is suggested

- `GET /api/album/people/suggestions?person=&space=&limit=` answers the pictures whose faces are suggested for the person, most alike first: the picture, all its faces (boxes and names, never vectors) and the indices and distances of the faces suggested, at most `limit` pictures (60 by default, at most 200) and their total.
- A face is suggested when one of the person's named faces lies within 0.55 of it (the Euclidean distance of ADR-0194) and no face named otherwise lies nearer: the same rule as the suggestion of one picture, which chooses the nearest name. A face is therefore never suggested for two persons.
- The named faces compared are those of the pictures the caller may read; the faces suggested are on the newest version of each stack, as the album shows it, of pictures the caller may edit. The answer is only given while recognition is on, as ADR-0194's suggestions.
- The work is bounded: each unnamed face is compared with the person's faces first, and only those within the threshold with every other named face. Vectors are read through the bounded memory of ADR-0194.

### What is done

- In a person's album, whoever may edit pictures sees **Is this {name}?** with the faces suggested, cut from their pictures. A click chooses a face; **Choose all** chooses every face shown; **Confirm chosen** names them, one picture after another, through `PUT …/faces`. A face sent without a vector keeps the vector of its box (ADR-0194), so a confirmed face recognises the person in turn.
- Faces not chosen stay unnamed and are suggested again; once the shown ones are named, the next most alike follow.

### Consequences

- **Positive:** a person named on a few pictures is found on the rest with a few clicks, and each confirmed face improves the next suggestions.
- **Positive:** no name is given without a person's choice, nothing new is stored, and access is that of the folders.
- **Negative / trade-offs accepted:**
  - A wrong suggestion that is not chosen comes back every time the album of that person is opened.
  - Only pictures with vectors are suggested: pictures searched while recognition was off are not, until each is searched again with **Find faces**.
  - A request reads the vectors of every picture one may read that names someone; after a start, the first request is slower with very many pictures.
  - The faces of a picture are sent whole, as they were when suggested; a change made to them by someone else in the meantime is overwritten.
- **Follow-ups / risks to watch:** the open questions above; whether 200 pictures per page suffice.

## Pros and cons of the options

### Option 2: named automatically

Strongest form: no clicks at all; very close faces (well below 0.55) are almost always the same person, and photo services name them so.

- Good: the album of a person fills itself.
- Bad: a wrong name is seen by every reader of the picture. A name derived from biometric data without a person's choice is no longer just a name someone typed, which ADR-0194 relies on to keep names when recognition is turned off. A threshold low enough to be safe still errs on siblings and children.

### Option 3: groups of unnamed faces

Strongest form: it also finds people never named, and names a whole group with one word.

- Good: it covers what option 1 cannot, the first naming.
- Bad: grouping compares every unnamed face with every other, which is unbounded work on the server for a large album, and keeps groups that change with each new picture. It can be added later on top of option 1, and is left as an open question.

### Option 4: in the viewer only

Strongest form: no new endpoint and no new view; each suggestion is seen in the context of its whole picture.

- Bad: it is the problem this record answers; at scale nobody opens each picture.

## Links

- amends [ADR-0194](0194-faces-and-person-albums.md) (faces and person albums)
- relates to ADR-0015 (budgets), ADR-0022 (permissions), ADR-0103 (stacks of versions), ADR-0177 (the album as a photo library)
