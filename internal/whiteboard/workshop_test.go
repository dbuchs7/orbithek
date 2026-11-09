package whiteboard

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTimer(t *testing.T) {
	h, _, clk := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	drain(bob)
	start := clk.now().UnixMilli()
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "timer", Action: "start", Seconds: 300}})
	tm := next(t, bob, Timer)
	if !tm.Timer.Running || tm.Timer.Ends != start+300_000 || tm.Timer.By != "Ann" || tm.Now != start {
		t.Fatalf("timer = %+v", tm.Timer)
	}
	clk.add(100 * time.Second)
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "timer", Action: "pause"}})
	if tm := next(t, bob, Timer); !tm.Timer.Paused || tm.Timer.Left != 200_000 {
		t.Fatalf("paused = %+v", tm.Timer)
	}
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "timer", Action: "add", Seconds: 60}})
	if tm := next(t, bob, Timer); tm.Timer.Left != 260_000 || tm.Timer.Total != 360_000 {
		t.Fatalf("lengthened = %+v", tm.Timer)
	}
	clk.add(time.Hour)
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "timer", Action: "resume"}})
	if tm := next(t, bob, Timer); tm.Timer.Paused || tm.Timer.Ends != clk.now().UnixMilli()+260_000 {
		t.Fatalf("resumed = %+v", tm.Timer)
	}
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "timer", Action: "add", Seconds: 1}})
	if tm := next(t, bob, Timer); tm.Timer.Ends != clk.now().UnixMilli()+270_000 {
		t.Fatalf("at least ten seconds are added: %+v", tm.Timer)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "timer", Action: "start", Seconds: 99999}})
	if tm := next(t, bob, Timer); tm.Timer.Total != timerLongest*1000 {
		t.Fatalf("bounded = %+v", tm.Timer)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "timer", Action: "stop"}})
	if tm := next(t, bob, Timer); tm.Timer.Running {
		t.Fatalf("stopped = %+v", tm.Timer)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "timer", Action: "dance"}})
	if evs := drain(bob); strings.Contains(kinds(evs), Timer) {
		t.Fatal("an unknown action was told")
	}
	// A late joiner sees the timer in its snapshot.
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "timer", Action: "start", Seconds: 60}})
	late, _ := h.Join(key, "rea", "Rea")
	if s := next(t, late, Snapshot); !s.Timer.Running {
		t.Fatalf("late snapshot = %+v", s.Timer)
	}
}

func TestVote(t *testing.T) {
	note := el("n1", "rectangle", 1, `"index":"a0","customData":{"wikiwikibangbang":{"kind":"note"}}`)
	h, _, _ := newTest(t, note, el("n2", "ellipse", 1, `"index":"a1"`), el("arr", "arrow", 1, `"index":"a2"`))
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	rea, _ := h.Join(key, "rea", "Rea")
	drain(ann)
	drain(bob)
	drain(rea)
	if _, err := h.Apply(key, rea.Client, "rea", []Op{{Op: "vote", Action: "start"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader started a vote: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "vote", Action: "cast", Target: "n1"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cast without a vote: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "vote", Action: "start", Votes: 21}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("too many votes: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "vote", Action: "start", Title: strings.Repeat("x", 201)}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("long title: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "vote", Action: "start", Title: " Next steps ", Votes: 2}}); err != nil {
		t.Fatal(err)
	}
	if v := next(t, bob, Vote); !v.Vote.Open || v.Vote.Title != "Next steps" || v.Vote.Left != 2 || v.Vote.By != "Ann" {
		t.Fatalf("vote = %+v", v.Vote)
	}
	// Bob votes twice on n1; a reader votes too; an arrow takes no vote.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "vote", Action: "cast", Target: "n1"}, {Op: "vote", Action: "cast", Target: "n1"}})
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "vote", Action: "cast", Target: "n2"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a third vote: %v", err)
	}
	if _, err := h.Apply(key, rea.Client, "rea", []Op{{Op: "vote", Action: "cast", Target: "arr"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("vote on an arrow: %v", err)
	}
	_, _ = h.Apply(key, rea.Client, "rea", []Op{{Op: "vote", Action: "cast", Target: "n2"}})
	// Each sees only their own votes; everyone sees how many voted.
	var mine Event
	for _, e := range drain(bob) {
		if e.Kind == Vote {
			mine = e
		}
	}
	if mine.Vote.Mine["n1"] != 2 || mine.Vote.Left != 0 || mine.Vote.Voters != 2 || mine.Vote.Results != nil {
		t.Fatalf("Bob's view = %+v", mine.Vote)
	}
	var annView Event
	for _, e := range drain(ann) {
		if e.Kind == Vote {
			annView = e
		}
	}
	if len(annView.Vote.Mine) != 0 || annView.Vote.Left != 2 {
		t.Fatalf("Ann's view = %+v", annView.Vote)
	}
	// Taking one back.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "vote", Action: "retract", Target: "n1"}, {Op: "vote", Action: "retract", Target: "n2"}})
	// An agent votes through its changes.
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "vote", ID: "n2"}, {Op: "vote", ID: "n2"}, {Op: "unvote", ID: "n2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "vote", ID: "arr"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("agent vote on an arrow: %v", err)
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "vote", ID: "n1"}, {Op: "vote", ID: "n1"}, {Op: "vote", ID: "n1"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("agent votes beyond its share: %v", err)
	}
	if _, err := h.Change(key, "rea", "Rea", []Change{{Op: "note"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader's agent made a note: %v", err)
	}
	// An agent may not end it; Ann does.
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "vote", Action: "end"}}); err != nil {
		t.Fatal(err)
	}
	var res Event
	for _, e := range drain(rea) {
		if e.Kind == Vote {
			res = e
		}
	}
	if !res.Vote.Closed || res.Vote.Results["n1"] != 1 || res.Vote.Results["n2"] != 2 || res.Vote.Voters != 3 {
		t.Fatalf("results = %+v", res.Vote)
	}
	view, _ := h.Read(key, "ai")
	for _, e := range view.Elements {
		if e.ID == "n2" && e.Votes != 2 {
			t.Fatalf("agent's view of the results: %+v", e)
		}
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "vote", Action: "end"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ending twice: %v", err)
	}
	if _, err := h.Apply(key, rea.Client, "rea", []Op{{Op: "vote", Action: "clear"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader cleared: %v", err)
	}
	drain(bob)
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "vote", Action: "clear"}})
	if v := next(t, bob, Vote); v.Vote.Closed || v.Vote.Open {
		t.Fatalf("cleared = %+v", v.Vote)
	}
}

func TestPresentation(t *testing.T) {
	frames := []json.RawMessage{
		el("f2", "frame", 1, `"index":"a0","x":900,"y":0,"height":500`),
		el("f1", "frame", 1, `"index":"a1","x":0,"y":10,"height":500`),
		el("f3", "frame", 1, `"index":"a2","x":0,"y":800,"height":500`),
	}
	h, _, _ := newTest(t, frames...)
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	drain(bob)
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "frame", Action: "next"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stepping without presenting: %v", err)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "present", Action: "start"}})
	if p := next(t, bob, Presentation); !p.Presentation.Active || p.Presentation.Frame != "f1" || p.Presentation.Name != "Ann" {
		t.Fatalf("start = %+v", p.Presentation)
	}
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "present", Action: "start"}}); !errors.Is(err, ErrLocked) {
		t.Fatalf("a second presenter: %v", err)
	}
	for _, want := range []string{"f2", "f3", "f3"} {
		_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "frame", Action: "next"}})
		if p := next(t, bob, Presentation); p.Presentation.Frame != want {
			t.Fatalf("next = %s, want %s", p.Presentation.Frame, want)
		}
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "frame", Action: "previous"}})
	if p := next(t, bob, Presentation); p.Presentation.Frame != "f2" {
		t.Fatalf("previous = %+v", p.Presentation)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "frame", Target: "f1"}})
	if p := next(t, bob, Presentation); p.Presentation.Frame != "f1" {
		t.Fatalf("goto = %+v", p.Presentation)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "frame", Target: "nope"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown frame: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "view", X: 1, Y: 2, Zoom: 0}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zoom 0: %v", err)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "view", X: 1, Y: 2, Zoom: 1.5}})
	if p := next(t, bob, Presentation); p.Presentation.Frame != "" || p.Presentation.Zoom != 1.5 {
		t.Fatalf("view = %+v", p.Presentation)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "present", Action: "dance"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown action: %v", err)
	}
	// Anyone may end it; then another presents.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "present", Action: "stop"}})
	if p := next(t, bob, Presentation); p.Presentation.Active {
		t.Fatalf("stop = %+v", p.Presentation)
	}
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "present", Action: "stop"}}) // nothing to stop
	drain(ann)
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "present", Action: "start", Target: "f3"}})
	if p := next(t, ann, Presentation); p.Presentation.Frame != "f3" {
		t.Fatalf("start at a frame = %+v", p.Presentation)
	}
	// The presenter leaving ends it.
	bob.Leave()
	if p := next(t, ann, Presentation); p.Presentation.Active {
		t.Fatalf("leave = %+v", p.Presentation)
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "note"}}); err != nil {
		t.Fatal(err)
	}
}

func TestPresentationWithoutFrames(t *testing.T) {
	h, _, _ := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	drain(bob)
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "present", Action: "start", X: 10, Y: 20, Zoom: 2}})
	if p := next(t, bob, Presentation); p.Presentation.Frame != "" || p.Presentation.X != 10 || p.Presentation.Zoom != 2 {
		t.Fatalf("view = %+v", p.Presentation)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "frame", Action: "next"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("no frame to step to: %v", err)
	}
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "comments"}})
	if c := next(t, bob, Comments); c.By != "Ann" {
		t.Fatalf("comments = %+v", c)
	}
}
