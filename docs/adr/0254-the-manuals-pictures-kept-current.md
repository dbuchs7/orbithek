# ADR-0254: The manual's pictures are kept current by a check, as its text is

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-26
- **Deciders:** the maintainer asked for a decision so that the manual and its pictures are updated with every change (2026-11-26); the means below were proposed with this record; accepted by the maintainer, who switched Checks B and C to enforce (2026-11-26) and asked for the pictures to be taken in CI on demand (2026-11-26)
- **Open question:** whether a change to the look (`style.css`) should call for every picture to be taken again
- **Question checked:** 2026-11

## Context and problem statement

ADR-0185 keeps the manual's text current with two checks:

- Check A, a test, fails when the manual names a label the UI does not have.
- Check B, in CI, flags a pull request that changes what people see but not every manual.

ADR-0253 added screenshots of the UI and the symbols of the modules: twelve pictures per language and ten symbols. It left open how a picture that no longer shows the UI gets noticed. Neither check sees pictures:

- Check A reads only bold text.
- Check B is satisfied by a change to the manuals' text, though the pictures beside it stay as they were.

A stale picture does more harm than a stale sentence. Readers trust what they see, and the people the pictures are for are the ones least able to tell that a button has moved since. A picture also goes stale without any new string. A change to the order of the start page's tiles adds no catalogue value, yet every picture of the start page is then wrong.

How does the project notice, before a change is merged, that a picture of the manual no longer shows the UI, without asking for new pictures where nothing they show has changed?

## Decision drivers

- **Visible at review:** as in ADR-0185, a stale picture is found while the author still knows what changed.
- **No busywork:** a change elsewhere in a large file such as `app.js` must not call for every picture taken from it.
- **Checks over memory:** agents write most changes and follow what CI says; a rule in prose is skipped.
- **Deterministic:** the check must give the same answer on every runner; fonts, timing and antialiasing must not decide it.
- **One mechanism:** pictures are answered the way the text is, in the same job, with the same mode and the same kind of escape line.

## Considered options

1. **A list of what each picture shows, compared between base and head by the manual check (Check C).**
2. **Take every picture again in CI and compare it with the committed one, pixel by pixel.**
3. **Take every picture again before each release**, by hand or in the release workflow.
4. **A rule in `AGENTS.md` only.**

## Decision outcome

Chosen option: **1**. Option 3 stays welcome as an occasional full pass, not as the guarantee.

### What each picture shows: `manual/img/sources.txt`

- One line per picture: its name without language, then what draws it.
  ```
  home            app.js#homeView key:home.
  shop-setup      shop.js#setupCard key:shop.setup key:shop.adminNo key:shop.toEngines
  icons/shop      app.js#moduleGlyphs app.js#moduleIcon
  ```
- What draws a picture is one of three kinds of source:
  - a file of the assets directory (`calendarview.js`);
  - a top-level function or constant of one (`app.js#homeView`), from its first line to the closing brace at the start of a line;
  - a prefix of catalogue keys (`key:shop.setup`), since a picture shows the UI's words.
- A name stands for the picture in every language (`img/en/home.png`, `img/de/home.png`); a name under `icons/` is language-neutral.
- The list sits beside the pictures, so whoever adds a picture sees where to say what it shows.
- A test (`TestManualPictureSources`) keeps the list true to the tree. It fails when:
  - a picture has no line;
  - a line names a picture missing in a language;
  - a source names a file, function, constant or key prefix that no longer exists.
  A rename therefore cannot let a picture drop out of the check unnoticed.

### Check C: a change to what a picture shows takes the picture again

- `cmd/manualcheck` runs Check C beside Check B, in the job `Manual (user-visible changes)`; its rules are in `internal/manualcheck/pictures.go`.
- A picture is **stale** when the pull request changes one of its sources but not every file of the picture:
  - for a function or constant, its text at the head differs from its text at the base; a change elsewhere in the file does not count;
  - for a key prefix, a catalogue value with that prefix is added or changed.
- Such a pull request passes when it takes the stale pictures again (`e2e/manual-shots.cjs`, then `e2e/shrink-pictures.py`), or when its description holds a line of its own:
  `Pictures: not needed — <reason>`
  Examples of a reason: a tooltip the picture does not show; a refactoring that draws the same.
- The report names each stale picture, the sources that made it stale and the files left untouched.
- The check runs in the mode of Check B. Both are **enforced**: the maintainer switched them from advisory on 2026-11-26, with this record. A pull request with a stale picture fails the job, a required check, and cannot be merged until it takes the picture again or its description gives the reason; a description edited later re-runs the job. Switching back is the maintainer's decision too, never an agent's own change.

### Taking the pictures on demand in CI

Since Check C blocks the merge, a pull request that changes a view the manual shows needs new pictures, and taking them needs a browser and a fresh server. An agent in the cloud has both; a person contributing from a laptop often has neither. So the pictures are also taken by a workflow, at the press of a button.

- `.github/workflows/manual-pictures.yml`, *Manual pictures*, is started from the Actions tab (`workflow_dispatch`) on the branch of the pull request.
- It builds the branch, starts a fresh server with a password made for the run, takes every picture with `e2e/manual-shots.cjs` and shrinks them with `e2e/shrink-pictures.py`: the same two commands as by hand, so the pictures do not depend on who took them.
- The pictures are always kept as an artifact of the run, for 14 days.
- On a branch other than the default one, and unless the input `commit` is turned off, the pictures that changed are committed to the branch by `github-actions[bot]`. The commit names the run that made it; it carries no sign-off, as bots never sign (ADR-0018). On the default branch the workflow only keeps the artifact, since `main` takes changes through pull requests only (ADR-0003).
- A push made with the workflow's default token starts no workflow; that is GitHub's rule against loops. The pull request's checks would then not run on the new commit, and as required checks they would hold the merge. The secret `PICTURES_PUSH_TOKEN`, a fine-grained token limited to this repository with *Contents: read and write*, pushes so that the checks run. Without it the workflow pushes with the default token and warns that the next push of the branch runs the checks; this follows `SETTINGS_READ_TOKEN` of the settings drift check.
- Only people with write access can start a workflow, and it runs the workflow file of the branch chosen. It runs no code that the branch does not already hold, and it gains no access that a push to the branch would not have.
- It is no pull request check and no required one: it changes the branch, which a check must not do.
- Pictures taken on a runner differ from those taken elsewhere in fonts and antialiasing. The first run after this change therefore renews every picture, and later runs renew those that changed. Pictures are compared by eye in review, not by pixels (option 2 below).

### What it does not cover, and how that is answered

- **The look as a whole.** `style.css` changes every picture a little and is named by no picture; otherwise every change to a margin would call for all pictures. When the look changes visibly (colours, fonts, the layout of the shell), all pictures are taken again in one change; the script takes them all in one run.
- **Meaning.** The check judges by sources, not by pixels. A source missing from a line makes a picture's change go unseen. The list is kept generous, naming the view's function and its key prefix, and a reviewer who sees a picture go stale adds the missing source.

### What stays as it is

- Checks A and B of ADR-0185, and the rule in `AGENTS.md` that a change to what people see updates both manuals in the same change.
- The manual's tests of ADR-0253: every manual shows the English one's pictures and anchors, every picture shown exists, every embedded picture is shown.

### Consequences

- **Positive:** a change to a view the manual shows reaches review with the question whether its picture still holds, answered in the open, as the text's question is.
- **Positive:** the answer is the same on every runner; no browser runs in CI for it.
- **Negative / trade-offs accepted:** the list must be kept; a new picture needs a line, and a view drawn by a new function needs that function named. The test catches names that have gone, not sources that were never listed.
- **Negative / trade-offs accepted:** taking pictures again needs a browser and a few minutes; the workflow *Manual pictures* does it on demand, but someone has to start it.
- **Negative / trade-offs accepted:** without `PICTURES_PUSH_TOKEN` the workflow's commit starts no checks until the branch is pushed again; with it, a token that can write the repository is stored as a secret and must be renewed when it expires.
- **Follow-ups / risks to watch:** if the escape line becomes a habit rather than an exception, review the reasons given; starting *Manual pictures* by a label or a comment on the pull request, if the Actions tab proves too far away.

## Pros and cons of the options

### Option 2: compare pictures taken in CI

Strongest form: it sees exactly what readers would see, including changes to the look, with no list to keep.

- Good: no source can be forgotten; `style.css` is covered.
- Bad: a picture differs with every font version, antialiasing and timing of the runner. A threshold large enough to stay quiet misses a moved button; a small one fails on noise. A failing check that no one can make pass by a change of their own is ignored within days.
- Bad: every pull request would start a server and a browser for minutes.

### Option 3: take every picture again before each release

Strongest form: one run renews all pictures at a known moment, whatever changed, including the look.

- Good: catches what Check C cannot: the look, and sources missing from the list.
- Bad: releases follow each other within hours (ADR-0185); the run would not happen every time, and a picture renewed then belongs to a change long merged, unreviewed. It stays welcome as a pass now and then.

### Option 4: a rule in AGENTS.md only

Strongest form: no list, no tooling; authors know which views they changed.

- Bad: authors do not know which views the manual shows a picture of; the day the pictures came, the start page's tiles were reordered in the same branch, and only a fresh run of the script caught it.

### Pictures on demand: a commit to the branch, or only an artifact

Strongest form of an artifact only: the workflow writes nothing; a reviewer sees what it took and the author commits it, so nothing reaches the branch unseen.

- Good: needs no write access and no token, and no commit escapes the author.
- Bad: the author still has to download, unpack and commit pictures. The point of the button is to spare that, and the commit is as visible in the pull request as the author's own. So the commit is the default, the artifact is always kept, and the input `commit` turns the commit off.

## Links

- amends [ADR-0185](0185-the-user-manual-is-kept-current-by-checks.md) (the manual kept current by checks) with a third check
- relates to ADR-0253 (the pictures), ADR-0183 (the user manual), ADR-0018 and ADR-0019 (an advisory check first, then enforced)
