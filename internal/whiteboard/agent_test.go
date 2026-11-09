package whiteboard

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/item"
)

func elementOf(t *testing.T, h *Hub, id string) map[string]any {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.sessions[key].elements[id]
	if e == nil {
		t.Fatalf("no element %s", id)
	}
	var m map[string]any
	_ = json.Unmarshal(e.raw, &m)
	return m
}

func viewOf(v View, id string) AgentElement {
	for _, e := range v.Elements {
		if e.ID == id {
			return e
		}
	}
	return AgentElement{}
}

func TestAgentMakesAndChangesElements(t *testing.T) {
	h, _, _ := newTest(t, el("old", "rectangle", 1, `"index":"a5","x":-50,"y":0,"height":100`))
	ann, _ := h.Join(key, "ann", "Ann")
	drain(ann)
	ids, err := h.Change(key, "ai", "Assistant", []Change{
		{Op: "frame", Text: "Ideas", Width: 1000, Height: 600},
		{Op: "note", Text: "A rather long idea that wraps over several lines of the note", Colour: "blue"},
		{Op: "shape", Shape: "diamond", Text: "Decide", X: ptr(400), Y: ptr(300)},
		{Op: "text", Text: "Heading\nsecond line", Colour: "#ff0000"},
		{Op: "shape"},
	})
	if err != nil {
		t.Fatal(err)
	}
	frame, note, diamond, text := ids[0], ids[1], ids[2], ids[3]
	e := next(t, ann, Elements)
	if len(e.Elements) != 7 || !strings.HasPrefix(e.By, "agent-") {
		t.Fatalf("broadcast %d elements by %s", len(e.Elements), e.By)
	}
	// Indices follow the scene's last one, in order.
	last := "a5"
	for _, raw := range e.Elements {
		var m meta
		_ = json.Unmarshal(raw, &m)
		if !validIndex(*m.Index) || *m.Index <= last {
			t.Fatalf("index %s after %s", *m.Index, last)
		}
		last = *m.Index
	}
	n := elementOf(t, h, note)
	if n["backgroundColor"] != "#a5d8ff" || n["y"].(float64) != 160 {
		t.Fatalf("note = %v", n)
	}
	view, err := h.Read(key, "ann")
	if err != nil || !view.Live {
		t.Fatal(err)
	}
	if v := viewOf(view, note); v.Kind != "note" || !strings.HasPrefix(v.Text, "A rather long idea") || v.Colour != "#a5d8ff" {
		t.Fatalf("note in view = %+v", v)
	}
	if v := viewOf(view, frame); v.Kind != "frame" || v.Text != "Ideas" {
		t.Fatalf("frame in view = %+v", v)
	}
	if v := viewOf(view, diamond); v.X != 400 || v.Text != "Decide" {
		t.Fatalf("diamond = %+v", v)
	}
	if v := viewOf(view, text); v.Colour != "#ff0000" || v.Text != "Heading\nsecond line" {
		t.Fatalf("text = %+v", v)
	}
	// The note's text is wrapped to fit it, and marked for measuring.
	for _, b := range n["boundElements"].([]any) {
		tid := b.(map[string]any)["id"].(string)
		tx := elementOf(t, h, tid)
		if !strings.Contains(tx["text"].(string), "\n") || tx["containerId"] != note || tx["width"].(float64) > 190 {
			t.Fatalf("bound text = %v", tx)
		}
		if !strings.Contains(string(mustJSON(tx["customData"])), `"measure":true`) {
			t.Fatal("not marked for measuring")
		}
	}

	// Into the frame, an arrow between two notes, labelled.
	ids, err = h.Change(key, "ai", "Assistant", []Change{
		{Op: "note", Text: "One", Frame: frame},
		{Op: "note", Text: "Two", Frame: frame, Colour: "pink"},
	})
	if err != nil {
		t.Fatal(err)
	}
	one, two := ids[0], ids[1]
	if o, w := elementOf(t, h, one), elementOf(t, h, two); o["frameId"] != frame || w["x"].(float64) <= o["x"].(float64) {
		t.Fatalf("placed in the frame: %v %v", o["x"], w["x"])
	}
	ids, err = h.Change(key, "ai", "Assistant", []Change{{Op: "connect", From: one, To: two, Text: "leads to"}})
	if err != nil {
		t.Fatal(err)
	}
	arrow := ids[0]
	view, _ = h.Read(key, "ann")
	if a := viewOf(view, arrow); a.From != one || a.To != two || a.Text != "leads to" {
		t.Fatalf("arrow = %+v", a)
	}
	if got := mustJSON(elementOf(t, h, one)["boundElements"]); !strings.Contains(string(got), arrow) {
		t.Fatalf("the note does not know its arrow: %s", got)
	}
	// Editing, moving (with its text and arrows), colouring.
	if _, err := h.Change(key, "ai", "Assistant", []Change{
		{Op: "edit", ID: one, Text: "One, edited"},
		{Op: "edit", ID: frame, Text: "Ideas, sorted"},
		{Op: "edit", ID: text, Text: "New heading"},
		{Op: "edit", ID: "old", Text: "Now with text"},
		{Op: "edit", ID: arrow, Text: "then"},
		{Op: "move", ID: two, X: ptr(700), Y: ptr(400)},
		{Op: "colour", ID: diamond, Colour: "green"},
		{Op: "colour", ID: text, Colour: "#00ff00"},
	}); err != nil {
		t.Fatal(err)
	}
	view, _ = h.Read(key, "ann")
	if viewOf(view, one).Text != "One, edited" || viewOf(view, frame).Text != "Ideas, sorted" || viewOf(view, text).Text != "New heading" ||
		viewOf(view, "old").Text != "Now with text" || viewOf(view, arrow).Text != "then" {
		t.Fatalf("edited = %+v", view.Elements)
	}
	if v := viewOf(view, two); v.X != 700 || v.Y != 400 {
		t.Fatalf("moved = %+v", v)
	}
	a := elementOf(t, h, arrow)
	pts := a["points"].([]any)
	if end := pts[1].([]any); a["x"].(float64)+end[0].(float64) < 690 {
		t.Fatalf("the arrow did not follow: %v", a)
	}
	if viewOf(view, diamond).Colour != "#b2f2bb" || viewOf(view, text).Colour != "#00ff00" {
		t.Fatal("colours not set")
	}
	// Moving the frame moves what is in it.
	before := viewOf(view, one)
	if _, err := h.Change(key, "ai", "Assistant", []Change{{Op: "move", ID: frame, X: ptr(100), Y: ptr(100)}}); err != nil {
		t.Fatal(err)
	}
	view, _ = h.Read(key, "ann")
	f, after := viewOf(view, frame), viewOf(view, one)
	if after.X-before.X != 100-(f.X-100+100) && after.X == before.X {
		t.Fatalf("frame contents did not move: %+v -> %+v", before, after)
	}
	// Deleting a note deletes its text; deleting the frame frees what is
	// in it.
	if _, err := h.Change(key, "ai", "Assistant", []Change{{Op: "delete", ID: one}, {Op: "delete", ID: frame}}); err != nil {
		t.Fatal(err)
	}
	view, _ = h.Read(key, "ann")
	if viewOf(view, one).ID != "" || viewOf(view, frame).ID != "" || viewOf(view, two).Frame != "" {
		t.Fatalf("after deleting: %+v", view.Elements)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestAgentIsRefused(t *testing.T) {
	h, _, _ := newTest(t, el("a", "rectangle", 1, `"index":"a0"`), el("arr", "arrow", 1, `"index":"a1"`), el("img", "image", 1, `"index":"a2"`),
		el("l", "line", 1, `"index":"a3"`))
	ann, _ := h.Join(key, "ann", "Ann")
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "lock", IDs: []string{"a"}}})
	cases := map[string]Change{
		"held":            {Op: "edit", ID: "a", Text: "x"},
		"held by move":    {Op: "move", ID: "a", X: ptr(1), Y: ptr(1)},
		"missing":         {Op: "delete", ID: "nope"},
		"bad shape":       {Op: "shape", Shape: "star"},
		"bad colour":      {Op: "note", Colour: "mauve"},
		"empty text":      {Op: "text", Text: " "},
		"no frame":        {Op: "note", Frame: "a"},
		"arrow to arrow":  {Op: "connect", From: "img", To: "arr"},
		"arrow to itself": {Op: "connect", From: "img", To: "img"},
		"move without x":  {Op: "move", ID: "img"},
		"frame colour":    {Op: "colour", ID: "img", Colour: "blue"},
		"colour missing":  {Op: "colour", ID: "img"},
		"line text":       {Op: "edit", ID: "l", Text: "x"},
		"unknown":         {Op: "teleport"},
		"connect missing": {Op: "connect", From: "nope", To: "img"},
		"connect held":    {Op: "connect", From: "img", To: "a"},
		"colour held":     {Op: "colour", ID: "a", Colour: "blue"},
		"delete held":     {Op: "delete", ID: "a"},
		"edit missing":    {Op: "edit", ID: "nope"},
	}
	for name, c := range cases {
		_, err := h.Change(key, "ai", "AI", []Change{c})
		if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrLocked) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "edit", ID: "a"}}); err == nil || !strings.Contains(err.Error(), "Ann") {
		t.Errorf("the holder is named: %v", err)
	}
	if _, err := h.Change(key, "ai", "AI", nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("no changes: %v", err)
	}
	if _, err := h.Change(key, "eve", "Eve", []Change{{Op: "note"}}); !errors.Is(err, item.ErrNotFound) {
		t.Errorf("a stranger: %v", err)
	}
	// A batch is all or none.
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "note", Text: "kept?"}, {Op: "delete", ID: "nope"}}); err == nil {
		t.Fatal("the batch succeeded")
	}
	view, _ := h.Read(key, "ann")
	for _, e := range view.Elements {
		if e.Text == "kept?" {
			t.Fatal("half a batch was written")
		}
	}
	if viewOf(view, "a").HeldBy != "Ann" {
		t.Fatalf("held by = %q", viewOf(view, "a").HeldBy)
	}
}

func TestReadWithoutSession(t *testing.T) {
	h, _, _ := newTest(t,
		el("n", "rectangle", 1, `"backgroundColor":"#ffec99","boundElements":[{"id":"t","type":"text"}],"customData":{"wikiwikibangbang":{"kind":"note"}}`),
		el("t", "text", 1, `"text":"wrapped","originalText":"Hello world","containerId":"n"`),
		el("gone", "rectangle", 2, `"isDeleted":true`),
		el("img", "image", 1, ""))
	v, err := h.Read(key, "rea")
	if err != nil || v.Live {
		t.Fatal(v, err)
	}
	if len(v.Elements) != 2 || v.Elements[0].Kind != "note" || v.Elements[0].Text != "Hello world" || v.Elements[0].Colour != "#ffec99" || v.Elements[1].Colour != "" {
		t.Fatalf("view = %+v", v.Elements)
	}
	if n, _ := h.Stats(); n != 0 {
		t.Fatal("reading opened a session")
	}
	if _, err := h.Read(key, "eve"); !errors.Is(err, item.ErrNotFound) {
		t.Fatalf("a stranger read: %v", err)
	}
}

func TestElementHelpers(t *testing.T) {
	idx, prev := "", ""
	for range 3000 {
		idx = indexAfter(idx)
		if !validIndex(idx) || idx <= prev {
			t.Fatalf("index %q after %q", idx, prev)
		}
		prev = idx
	}
	for _, s := range []string{"", "a", "!0", "a0!", "a00", "A00000000000000000000000000"} {
		if validIndex(s) {
			t.Errorf("%q is valid", s)
		}
	}
	if got := indexAfter("zzzzzzzzzzzzzzzzzzzzzzzzzzz"); got <= "zzzzzzzzzzzzzzzzzzzzzzzzzzz" {
		t.Errorf("after the largest integer: %q", got)
	}
	if got := indexAfter("Zz"); got != "a0" {
		t.Errorf("after Zz: %q", got)
	}
	if got := wrap("a bb ccccccccccccccccccc d\n\nend", 60); strings.Join(got, "|") != "a bb|ccccc|ccccc|ccccc|cccc|d||end" {
		t.Errorf("wrap = %q", got)
	}
	if c, err := colour("", "#000000"); c != "#000000" || err != nil {
		t.Error(c, err)
	}
	if c, _ := colour("transparent", ""); c != "transparent" {
		t.Error(c)
	}
	if ValidElementID("a b") || !ValidElementID("abc_D-1") {
		t.Error("ValidElementID")
	}
	if _, _, err := patch(json.RawMessage(`[1]`), 0, map[string]any{}); !errors.Is(err, ErrInvalid) {
		t.Errorf("patch of no object: %v", err)
	}
	if got := setIndex(json.RawMessage(`[1]`), "a0"); string(got) != "[1]" {
		t.Errorf("setIndex of no object: %s", got)
	}
	b := box{0, 0, 100, 100}
	if x, y := b.edge(50, 50, 8); x != 50 || y != 50 {
		t.Errorf("edge towards the centre: %v %v", x, y)
	}
	if x, _ := b.edge(500, 50, 8); x != 108 {
		t.Errorf("edge to the right: %v", x)
	}
	if x, y := b.edge(50, 500, 0); x != 50 || y != 100 {
		t.Errorf("edge below: %v %v", x, y)
	}
}
