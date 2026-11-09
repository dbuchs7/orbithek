package whiteboard

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// Agents read a whiteboard as a list of its elements in plain terms and
// change it with simple changes (ADR-0169); the server makes Excalidraw's
// elements from them. An agent holds no lock across requests: each change
// is refused when a browser holds what it touches.

// AgentElement is an element as an agent reads it.
type AgentElement struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	Text   string  `json:"text,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Colour string  `json:"colour,omitempty"`
	Frame  string  `json:"frame,omitempty"`
	From   string  `json:"from,omitempty"`
	To     string  `json:"to,omitempty"`
	HeldBy string  `json:"heldBy,omitempty"`
	Votes  int     `json:"votes,omitempty"`
}

// View is a whiteboard as an agent reads it. Live says whether a session
// is open; without one, it is the newest saved version.
type View struct {
	Live         bool               `json:"live"`
	Elements     []AgentElement     `json:"elements"`
	Participants []Participant      `json:"participants"`
	Timer        *TimerState        `json:"timer,omitempty"`
	Vote         *VoteState         `json:"vote,omitempty"`
	Presentation *PresentationState `json:"presentation,omitempty"`
	Saved        *Saved             `json:"saved,omitempty"`
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// agentView lists elements in plain terms; the text bound to a shape or
// an arrow is told as the shape's.
func agentView(order []string, get func(string) (meta, bool), holder func(string) string, votes map[string]int) []AgentElement {
	out := []AgentElement{}
	for _, id := range order {
		m, _ := get(id)
		if m.IsDeleted || (m.Type == "text" && str(m.ContainerID) != "") {
			continue
		}
		a := AgentElement{ID: m.ID, Kind: m.Type, Text: textOf(m, get), X: round1(m.X), Y: round1(m.Y), Width: round1(m.Width), Height: round1(m.Height),
			Frame: str(m.FrameID), HeldBy: holder(id), Votes: votes[id]}
		if m.isNote() {
			a.Kind = "note"
		}
		switch m.Type {
		case "text", "arrow", "line", "freedraw":
			a.Colour = m.StrokeColor
		case "frame", "magicframe", "image":
		default:
			if m.BackgroundColor != "transparent" {
				a.Colour = m.BackgroundColor
			}
		}
		if m.StartBinding != nil {
			a.From = m.StartBinding.ElementID
		}
		if m.EndBinding != nil {
			a.To = m.EndBinding.ElementID
		}
		out = append(out, a)
	}
	return out
}

// Read returns the whiteboard as principal may read it, from its session
// or, with none open, from its newest version.
func (h *Hub) Read(k Key, principal string) (View, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if read, _ := h.b.Access(k, principal); !read {
		return View{}, fmt.Errorf("%w: whiteboard %s/%s", item.ErrNotFound, k.Item, k.First)
	}
	if s := h.sessions[k]; s != nil {
		get := func(id string) (meta, bool) {
			e := s.elements[id]
			if e == nil {
				return meta{}, false
			}
			return e.m, true
		}
		holder := func(id string) string {
			if e := s.elements[id]; e != nil && e.holder != nil {
				return e.holder.name
			}
			return ""
		}
		v := h.voteFor(s, principal)
		timer, p := s.timer, h.presentation(s)
		return View{Live: true, Elements: agentView(s.order, get, holder, v.Results), Participants: h.participants(s),
			Timer: &timer, Vote: &v, Presentation: &p, Saved: s.lastSaved}, nil
	}
	p, err := h.b.Load(k)
	if err != nil {
		return View{}, err
	}
	sc, _, err := ReadScene(p, budget.Get().WhiteboardSceneBytes)
	if err != nil {
		return View{}, err
	}
	byID := map[string]meta{}
	var order []string
	for _, raw := range sc.Elements {
		if m, err := parseElement(raw); err == nil {
			byID[m.ID] = m
			order = append(order, m.ID)
		}
	}
	get := func(id string) (meta, bool) { m, ok := byID[id]; return m, ok }
	return View{Elements: agentView(order, get, func(string) string { return "" }, nil), Participants: []Participant{}}, nil
}

// Change is one change of an agent.
//
//   - note, shape (rectangle, ellipse or diamond), text, frame: a new
//     element with text (a frame's name), at x and y or placed below the
//     scene, in frame if given, of width and height, in colour;
//   - connect: an arrow from one element to another, text its label;
//   - edit: new text for id; move: id to x and y; colour: id in colour;
//     delete: id;
//   - vote, unvote: cast or take back a vote on id (ADR-0170).
type Change struct {
	Op     string   `json:"op"`
	ID     string   `json:"id,omitempty"`
	Text   string   `json:"text,omitempty"`
	Shape  string   `json:"shape,omitempty"`
	Colour string   `json:"colour,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	Width  float64  `json:"width,omitempty"`
	Height float64  `json:"height,omitempty"`
	Frame  string   `json:"frame,omitempty"`
	From   string   `json:"from,omitempty"`
	To     string   `json:"to,omitempty"`
}

// work is a batch of agent changes being made: elements changed or made,
// checked against the session, written only when all succeed.
type work struct {
	s       *session
	changed map[string]json.RawMessage
	metas   map[string]meta
	order   []string
	now     int64
	last    string // the highest index so far
	placed  float64
}

func (w *work) get(id string) (meta, bool) {
	if m, ok := w.metas[id]; ok {
		return m, true
	}
	if e := w.s.elements[id]; e != nil {
		return e.m, true
	}
	return meta{}, false
}

func (w *work) raw(id string) json.RawMessage {
	if r, ok := w.changed[id]; ok {
		return r
	}
	return w.s.elements[id].raw
}

// live returns an element the agent may change: there, not deleted, and
// held by nobody.
func (w *work) live(id string) (meta, error) {
	m, ok := w.get(id)
	if !ok || m.IsDeleted {
		return meta{}, fmt.Errorf("%w: no element %q", ErrInvalid, id)
	}
	if e := w.s.elements[id]; e != nil && e.holder != nil {
		return meta{}, fmt.Errorf("%w: %s is working on element %s", ErrLocked, e.holder.name, id)
	}
	return m, nil
}

func (w *work) set(id string, raw json.RawMessage, m meta) {
	if _, ok := w.changed[id]; !ok {
		w.order = append(w.order, id)
	}
	w.changed[id], w.metas[id] = raw, m
}

func (w *work) add(el map[string]any) (meta, error) {
	raw, err := json.Marshal(el)
	if err != nil {
		return meta{}, err
	}
	m, err := parseElement(raw)
	if err != nil {
		return meta{}, err
	}
	if w.s.elements[m.ID] != nil {
		return meta{}, fmt.Errorf("%w: element %s exists", ErrInvalid, m.ID)
	}
	w.set(m.ID, raw, m)
	return m, nil
}

func (w *work) patch(id string, fields map[string]any) (meta, error) {
	raw, m, err := patch(w.raw(id), w.now, fields)
	if err != nil {
		return meta{}, err
	}
	w.set(id, raw, m)
	return m, nil
}

func (w *work) index() string {
	w.last = indexAfter(w.last)
	return w.last
}

// place finds where a new element goes: where asked, inside its frame, or
// below what the scene holds.
func (w *work) place(c Change, width, height float64) (float64, float64, error) {
	if c.X != nil && c.Y != nil {
		return *c.X, *c.Y, nil
	}
	if c.Frame != "" {
		f, ok := w.get(c.Frame)
		if !ok || f.IsDeleted || (f.Type != "frame" && f.Type != "magicframe") {
			return 0, 0, fmt.Errorf("%w: no frame %q", ErrInvalid, c.Frame)
		}
		n := 0
		for _, id := range w.ids() {
			if m, _ := w.get(id); !m.IsDeleted && str(m.FrameID) == f.ID && str(m.ContainerID) == "" {
				n++
			}
		}
		cols := max(1, int((f.Width-20)/(width+20)))
		return f.X + 20 + float64(n%cols)*(width+20), f.Y + 20 + float64(n/cols)*(height+20), nil
	}
	minX, maxY, any := math.Inf(1), math.Inf(-1), false
	for _, id := range w.ids() {
		if m, _ := w.get(id); !m.IsDeleted && !w.isNew(id) {
			minX, maxY, any = math.Min(minX, m.X), math.Max(maxY, m.Y+m.Height), true
		}
	}
	if !any {
		minX, maxY = 0, -60
	}
	x := minX + w.placed
	w.placed += width + 20
	return x, maxY + 60, nil
}

func (w *work) isNew(id string) bool { return w.s.elements[id] == nil }

func (w *work) ids() []string {
	out := append([]string{}, w.s.order...)
	for _, id := range w.order {
		if w.s.elements[id] == nil {
			out = append(out, id)
		}
	}
	return out
}

func newElementID() string { return "ai" + newID() + newID()[:3] }

func size2(v, def, low, high float64) float64 {
	if v == 0 {
		return def
	}
	return math.Min(math.Max(v, low), high)
}

// apply makes one change; it returns the element it made or changed.
func (w *work) apply(c Change) (string, error) {
	switch c.Op {
	case "note", "shape":
		kind, wDef, hDef, bg := "rectangle", 200.0, 200.0, "yellow"
		if c.Op == "shape" {
			kind, wDef, hDef, bg = c.Shape, 160, 100, "transparent"
			if kind == "" {
				kind = "rectangle"
			}
			if kind != "rectangle" && kind != "ellipse" && kind != "diamond" {
				return "", fmt.Errorf("%w: a shape is a rectangle, an ellipse or a diamond", ErrInvalid)
			}
		}
		fill, err := colour(c.Colour, noteColours[bg])
		if err != nil {
			return "", err
		}
		if fill == "" {
			fill = "transparent"
		}
		width, height := size2(c.Width, wDef, 20, 4000), size2(c.Height, hDef, 20, 4000)
		x, y, err := w.place(c, width, height)
		if err != nil {
			return "", err
		}
		el := base(kind, newElementID(), x, y, width, height, w.index(), w.now)
		el["backgroundColor"] = fill
		if c.Frame != "" {
			el["frameId"] = c.Frame
		}
		if kind != "ellipse" {
			el["roundness"] = map[string]int{"type": 3}
		}
		if c.Op == "note" {
			el["strokeWidth"], el["roughness"] = 1, 0
			el["customData"] = map[string]any{KEY: map[string]any{"kind": "note"}}
		}
		if strings.TrimSpace(c.Text) != "" {
			t := textElement(newElementID(), c.Text, el, 0, 0, "", w.now)
			el["boundElements"] = []bound{{ID: t["id"].(string), Type: "text"}}
			t["index"] = w.index()
			if _, err := w.add(el); err != nil {
				return "", err
			}
			_, err := w.add(t)
			return el["id"].(string), err
		}
		m, err := w.add(el)
		return m.ID, err
	case "text":
		if strings.TrimSpace(c.Text) == "" {
			return "", fmt.Errorf("%w: a text needs text", ErrInvalid)
		}
		lines := strings.Split(c.Text, "\n")
		x, y, err := w.place(c, float64(longest(lines))*charWidth, float64(len(lines))*fontSize*lineHeight)
		if err != nil {
			return "", err
		}
		el := textElement(newElementID(), c.Text, nil, x, y, w.index(), w.now)
		if c.Frame != "" {
			el["frameId"] = c.Frame
		}
		if c.Colour != "" {
			if el["strokeColor"], err = colour(c.Colour, ""); err != nil {
				return "", err
			}
		}
		m, err := w.add(el)
		return m.ID, err
	case "frame":
		width, height := size2(c.Width, 800, 100, 20000), size2(c.Height, 500, 100, 20000)
		c.Frame = ""
		x, y, err := w.place(c, width, height)
		if err != nil {
			return "", err
		}
		el := base("frame", newElementID(), x, y, width, height, w.index(), w.now)
		el["name"] = strings.TrimSpace(c.Text)
		if c.Text == "" {
			el["name"] = nil
		}
		m, err := w.add(el)
		return m.ID, err
	case "connect":
		from, err := w.live(c.From)
		if err != nil {
			return "", err
		}
		to, err := w.live(c.To)
		if err != nil {
			return "", err
		}
		if from.ID == to.ID || from.Type == "arrow" || to.Type == "arrow" || str(from.ContainerID) != "" || str(to.ContainerID) != "" {
			return "", fmt.Errorf("%w: an arrow connects two different elements that are no arrows", ErrInvalid)
		}
		id := newElementID()
		el := base("arrow", id, 0, 0, 0, 0, w.index(), w.now)
		for k, v := range arrowGeometry(box{from.X, from.Y, from.Width, from.Height}, box{to.X, to.Y, to.Width, to.Height}) {
			el[k] = v
		}
		el["roundness"] = map[string]int{"type": 2}
		el["startBinding"] = map[string]any{"elementId": from.ID, "focus": 0, "gap": 8}
		el["endBinding"] = map[string]any{"elementId": to.ID, "focus": 0, "gap": 8}
		el["startArrowhead"], el["endArrowhead"], el["lastCommittedPoint"], el["elbowed"] = nil, "arrow", nil, false
		if c.Colour != "" {
			if el["strokeColor"], err = colour(c.Colour, ""); err != nil {
				return "", err
			}
		}
		var label map[string]any
		if strings.TrimSpace(c.Text) != "" {
			pts := el["points"].([][2]float64)
			mx, my := el["x"].(float64)+pts[1][0]/2, el["y"].(float64)+pts[1][1]/2
			label = textElement(newElementID(), c.Text, map[string]any{"id": id, "x": mx - 100, "y": my - 20, "width": 200.0, "height": 40.0, "frameId": nil}, 0, 0, "", w.now)
			el["boundElements"] = []bound{{ID: label["id"].(string), Type: "text"}}
		}
		if _, err := w.add(el); err != nil {
			return "", err
		}
		if label != nil {
			label["index"] = w.index()
			if _, err := w.add(label); err != nil {
				return "", err
			}
		}
		for _, end := range []meta{from, to} {
			end, _ = w.get(end.ID)
			if _, err := w.patch(end.ID, map[string]any{"boundElements": append(end.BoundElements, bound{ID: id, Type: "arrow"})}); err != nil {
				return "", err
			}
		}
		return id, nil
	case "edit":
		m, err := w.live(c.ID)
		if err != nil {
			return "", err
		}
		return m.ID, w.edit(m, c.Text)
	case "move":
		m, err := w.live(c.ID)
		if err != nil {
			return "", err
		}
		if c.X == nil || c.Y == nil {
			return "", fmt.Errorf("%w: a move needs x and y", ErrInvalid)
		}
		return m.ID, w.move(m, *c.X-m.X, *c.Y-m.Y)
	case "colour":
		m, err := w.live(c.ID)
		if err != nil {
			return "", err
		}
		col, err := colour(c.Colour, "")
		if err != nil || col == "" {
			return "", fmt.Errorf("%w: colour %q", ErrInvalid, c.Colour)
		}
		field := "backgroundColor"
		switch m.Type {
		case "text", "arrow", "line", "freedraw":
			field = "strokeColor"
		case "frame", "magicframe", "image", "embeddable", "iframe":
			return "", fmt.Errorf("%w: a %s has no colour of its own", ErrInvalid, m.Type)
		}
		_, err = w.patch(m.ID, map[string]any{field: col})
		return m.ID, err
	case "delete":
		m, err := w.live(c.ID)
		if err != nil {
			return "", err
		}
		for _, b := range m.BoundElements {
			if t, ok := w.get(b.ID); ok && b.Type == "text" && !t.IsDeleted {
				if _, err := w.live(t.ID); err != nil {
					return "", err
				}
				if _, err := w.patch(t.ID, map[string]any{"isDeleted": true}); err != nil {
					return "", err
				}
			}
		}
		if m.Type == "frame" || m.Type == "magicframe" {
			for _, id := range w.ids() {
				if child, _ := w.get(id); !child.IsDeleted && str(child.FrameID) == m.ID {
					if _, err := w.live(id); err != nil {
						return "", err
					}
					if _, err := w.patch(id, map[string]any{"frameId": nil}); err != nil {
						return "", err
					}
				}
			}
		}
		_, err = w.patch(m.ID, map[string]any{"isDeleted": true})
		return m.ID, err
	}
	return "", fmt.Errorf("%w: unknown change %q", ErrInvalid, c.Op)
}

// edit sets the text an element shows: its own, its bound text's (made if
// it has none), a frame's name.
func (w *work) edit(m meta, text string) error {
	switch m.Type {
	case "frame", "magicframe":
		_, err := w.patch(m.ID, map[string]any{"name": strings.TrimSpace(text)})
		return err
	case "text":
		lines := strings.Split(text, "\n")
		if str(m.ContainerID) != "" {
			if parent, ok := w.get(str(m.ContainerID)); ok {
				return w.edit(parent, text)
			}
		}
		_, err := w.patch(m.ID, map[string]any{"text": text, "originalText": text, "width": math.Max(float64(longest(lines))*charWidth, charWidth),
			"height": float64(len(lines)) * fontSize * lineHeight, "customData": map[string]any{KEY: map[string]any{"measure": true}}})
		return err
	case "line", "freedraw", "image", "embeddable", "iframe":
		return fmt.Errorf("%w: a %s holds no text", ErrInvalid, m.Type)
	}
	container := map[string]any{"id": m.ID, "x": m.X, "y": m.Y, "width": m.Width, "height": m.Height, "frameId": m.FrameID}
	if m.Type == "arrow" {
		container["x"], container["y"], container["width"], container["height"] = m.X+m.Width/2-100, m.Y+m.Height/2-20, 200.0, 40.0
	}
	t := textElement("", text, container, 0, 0, "", w.now)
	for _, b := range m.BoundElements {
		if old, ok := w.get(b.ID); ok && b.Type == "text" && !old.IsDeleted {
			if _, err := w.live(old.ID); err != nil {
				return err
			}
			_, err := w.patch(old.ID, map[string]any{"text": t["text"], "originalText": text, "x": t["x"], "y": t["y"],
				"width": t["width"], "height": t["height"], "customData": t["customData"]})
			return err
		}
	}
	t["id"], t["index"] = newElementID(), w.index()
	if _, err := w.add(t); err != nil {
		return err
	}
	_, err := w.patch(m.ID, map[string]any{"boundElements": append(m.BoundElements, bound{ID: t["id"].(string), Type: "text"})})
	return err
}

// move shifts an element, the text bound to it and, for a frame, what lies
// in it; arrows bound to what moved are drawn between their ends again.
func (w *work) move(m meta, dx, dy float64) error {
	moved := map[string]bool{}
	var shift func(m meta) error
	shift = func(m meta) error {
		if moved[m.ID] {
			return nil
		}
		moved[m.ID] = true
		if _, err := w.live(m.ID); err != nil {
			return err
		}
		if _, err := w.patch(m.ID, map[string]any{"x": m.X + dx, "y": m.Y + dy}); err != nil {
			return err
		}
		for _, b := range m.BoundElements {
			if t, ok := w.get(b.ID); ok && b.Type == "text" && !t.IsDeleted {
				if err := shift(t); err != nil {
					return err
				}
			}
		}
		if m.Type == "frame" || m.Type == "magicframe" {
			for _, id := range w.ids() {
				if child, _ := w.get(id); !child.IsDeleted && str(child.FrameID) == m.ID {
					if err := shift(child); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := shift(m); err != nil {
		return err
	}
	for _, id := range w.ids() {
		a, _ := w.get(id)
		if a.IsDeleted || a.Type != "arrow" || moved[id] || a.StartBinding == nil || a.EndBinding == nil ||
			(!moved[a.StartBinding.ElementID] && !moved[a.EndBinding.ElementID]) {
			continue
		}
		from, ok1 := w.get(a.StartBinding.ElementID)
		to, ok2 := w.get(a.EndBinding.ElementID)
		if !ok1 || !ok2 {
			continue
		}
		if _, err := w.live(id); err != nil {
			return err
		}
		if _, err := w.patch(id, arrowGeometry(box{from.X, from.Y, from.Width, from.Height}, box{to.X, to.Y, to.Width, to.Height})); err != nil {
			return err
		}
	}
	return nil
}

// Change runs an agent's changes, all or none, and returns the element
// each made or changed. The agent shows among the participants for
// AgentTTL after.
func (h *Hub) Change(k Key, principal, name string, changes []Change) ([]string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	read, edit := h.b.Access(k, principal)
	switch {
	case !read:
		return nil, fmt.Errorf("%w: whiteboard %s/%s", item.ErrNotFound, k.Item, k.First)
	case len(changes) == 0:
		return nil, fmt.Errorf("%w: no changes", ErrInvalid)
	}
	for _, c := range changes {
		if !edit && c.Op != "vote" && c.Op != "unvote" {
			return nil, ErrForbidden
		}
	}
	s, err := h.open(k)
	if err != nil {
		return nil, err
	}
	c := s.agents[principal]
	joined := false
	if c == nil {
		if len(s.clients) >= budget.Get().WhiteboardClients {
			h.gc(s)
			return nil, ErrBusy
		}
		s.joins++
		c = &client{id: "agent-" + newID(), principal: principal, name: name, editable: edit, joined: s.joins}
		s.agents[principal] = c
		h.add(s, c)
		joined = true
	}
	c.last, c.editable = h.now(), edit
	defer func() {
		if joined {
			h.presenceAfterChange(s)
		}
		h.gc(s)
	}()

	w := &work{s: s, changed: map[string]json.RawMessage{}, metas: map[string]meta{}, now: h.now().UnixMilli()}
	for _, id := range s.order {
		if idx := str(s.elements[id].m.Index); idx > w.last && validIndex(idx) {
			w.last = idx
		}
	}
	out := make([]string, len(changes))
	var votes []Op
	for i, ch := range changes {
		if ch.Op == "vote" || ch.Op == "unvote" {
			action := "cast"
			if ch.Op == "unvote" {
				action = "retract"
			}
			votes = append(votes, Op{Op: "vote", Action: action, Target: ch.ID})
			out[i] = ch.ID
			continue
		}
		id, err := w.apply(ch)
		if err != nil {
			return nil, fmt.Errorf("change %d: %w", i+1, err)
		}
		out[i] = id
	}
	var added int64
	newCount := 0
	for _, id := range w.order {
		added += int64(len(w.changed[id])) - size(s.elements[id])
		if s.elements[id] == nil {
			newCount++
		}
	}
	if len(s.elements)+newCount > budget.Get().WhiteboardElements || s.bytes+added > budget.Get().WhiteboardSceneBytes {
		return nil, ErrBusy
	}
	// The votes are checked against the scene as it will be, before
	// anything is written.
	if err := h.checkVotes(s, c, votes, w.get); err != nil {
		return nil, err
	}
	var changed []json.RawMessage
	for _, id := range w.order {
		e := s.elements[id]
		if e == nil {
			e = &element{}
			s.elements[id] = e
			s.order = append(s.order, id)
		}
		e.raw, e.m = w.changed[id], w.metas[id]
		changed = append(changed, e.raw)
	}
	s.bytes += added
	if len(changed) > 0 {
		h.changed(s, c)
		h.broadcast(s, Event{Kind: Elements, Elements: changed, By: c.id}, nil)
	}
	for i, op := range votes {
		if err := h.voteOp(s, c, op); err != nil {
			return out, fmt.Errorf("vote %d: %w", i+1, err)
		}
	}
	return out, nil
}

// checkVotes tells whether an agent's votes would all be taken.
func (h *Hub) checkVotes(s *session, c *client, votes []Op, get func(string) (meta, bool)) error {
	if len(votes) == 0 {
		return nil
	}
	if !s.vote.open {
		return fmt.Errorf("%w: no vote is open", ErrInvalid)
	}
	mine := map[string]int{}
	used := 0
	for id, n := range s.vote.votes[c.principal] {
		mine[id], used = n, used+n
	}
	for i, op := range votes {
		m, ok := get(op.Target)
		if !ok || m.IsDeleted || !votable[m.Type] {
			return fmt.Errorf("vote %d: %w: one votes on a note or a shape", i+1, ErrInvalid)
		}
		switch {
		case op.Action == "cast" && used >= s.vote.per:
			return fmt.Errorf("vote %d: %w: no votes left", i+1, ErrInvalid)
		case op.Action == "cast":
			mine[op.Target]++
			used++
		case mine[op.Target] > 0:
			mine[op.Target]--
			used--
		}
	}
	return nil
}
