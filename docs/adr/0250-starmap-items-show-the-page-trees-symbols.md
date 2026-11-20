# ADR-0250: Items on the starmap show the page tree's symbols, finished tasks in green

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-20
- **Deciders:** Project initiators. Request of 2026-11-20: on the starmap, draw symbols instead of circles, the same symbols by type as the navigation of a room shows on its left, and draw a task green when it is done.
- **Open question:** whether "done" should mean the state named `done` alone rather than every finished state; this record takes every finished state of a task (ADR-0082).
- **Question checked:** 2026-11

## Context and problem statement

The starmap draws every item as a circle coloured by its type (ADR-0157).
The page tree in a room's navigation draws a sheet with lines of text for a page and a ticked box for a task, on a stack when items lie below it (`assets/treeicons.js`).
The same item thus looks different in the two places, and the map does not show whether a task is still open.
The requester wants the map's items to show the page tree's symbols and a finished task to be green.

How is an item drawn on the starmap, and which tasks count as done?

## Decision drivers

- An item looks the same on the map as in the navigation.
- Kinds are told apart by shape and outline, not colour alone (ADR-0157); a finished task is told by more than its hue and says so in its name.
- Every colour stays a token of the look (ADR-0156), so the light, dark and cockpit looks keep working.
- The server, the budget, the layout and the simulation stay as they are; the change is confined to the drawing.
- What can be tested without a browser is.

## Considered options

1. **The page tree's symbols, from `treeicons.js`, with the outline filled in the type's tone; a task of a finished state filled green with its tick in the surface's colour; green kept for it.**
2. Keep the circle and draw the symbol small inside it.
3. Let the server mark each node as a task and as finished.

## Decision outcome

Chosen option: **1**, because it is what was asked for, reuses the symbols the navigation already draws and needs no change on the server.

- `treeIconLayers(kind, stacked)` in `assets/treeicons.js` splits a symbol into the outlines of its stack (`behind`), its outline (`outline`, closed, so it can be filled) and its marks (`marks`), the same paths `treeIconParts` draws.
- `itemSymbols(graph, { taskTypes, finished })` in `assets/starmap-graph.js` gives each item of the map its symbol: `task` for an item of a task type and `page` for every other, `stacked` when the map holds an item below it by a `parent` edge, `done` when it is a task whose state is finished.
  Pictures, labels and clouds keep their shapes (ADR-0157, ADR-0230).
- The page that shows the map passes the task types and the finished states, as the navigation's tree gets the task types: a task type is one `isTaskType` names (ADR-0049), and a state is finished as `finished` decides it (ADR-0082) — the workflow's last state, or one no transition leaves.
  Without them every item shows a page's symbol.
- An item is drawn as `g.star-symbol` with `data-symbol` and, on a stack, `data-stacked`, its outline being the node's `.star-shape`, so hover, focus, matches, the chosen node and outside items keep their styles.
  The symbol is drawn at 1.25 times the navigation's 24 by 24 box, about the room the circle took.
- A finished task carries the class `done`: its outline is filled with `--success` and its tick drawn in `--surface`; its name ends in ", finished" (de: "erledigt").
  The type of the second tone, which was drawn in `--success`, is drawn in a mix of `--accent` and `--danger` instead, so green on an item means a finished task alone.
- The legend explains the page's sheet, the task's box, the green box and the stack; the manual says the same.

### Consequences

- **Positive:** an item looks alike in the navigation and on the map; open and finished tasks differ on the map by fill and in their names; the change has unit tests and a step in the smoke test.
- **Negative / trade-offs accepted:** a page's sheet is narrower than the circle was, so it is a slightly smaller target for the pointer; whether an item is stacked depends on what the map holds, so an item whose children were left out by the budget or by access shows no stack on the map while the navigation shows one; one more request (`/api/types`) is made when the map opens.
- **Follow-ups / risks to watch:** the starmap of the labels (ADR-0165) draws its stars as before; the edge markers keep the distance they had from a circle.

## Pros and cons of the options

### Option 2: keep the circle, the symbol inside it

Strongest form: the circle keeps the largest target and the layout's spacing exactly as tuned, and a symbol inside it still tells pages from tasks.

- Good: nothing about size or hit area changes.
- Bad: not what was asked for ("symbols instead of circles"); a symbol inside a circle of the same size is small at the zooms the map is mostly read at.

### Option 3: the server marks tasks and finished items

Strongest form: the server knows every type and its workflow, so the answer would be complete for items of every room, and the client would not need to read the types.

- Good: one source of truth; works for any client of the API.
- Bad: which types are tasks is a convention of the UI (ADR-0049), not of the model; marking it on the server would move that convention into the API and widen the starmap's answer for one drawing; the client already reads the types for the navigation.

## Links

- amends ADR-0157 (items were circles)
- relates to ADR-0049 (task types), ADR-0082 (finished states), ADR-0156 (looks), ADR-0230 (clouds), ADR-0041 (page tree)
