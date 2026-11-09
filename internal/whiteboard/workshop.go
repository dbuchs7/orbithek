package whiteboard

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode/utf8"
)

// The tools of a workshop (ADR-0170): a countdown, a vote and a
// presentation. They are state of the session, never part of the scene.

// The bounds of the tools.
const (
	timerShortest = 10
	timerLongest  = 2 * 60 * 60
	votesMost     = 20
	votesDefault  = 3
	titleLongest  = 200
)

// timerOp starts, pauses, resumes, lengthens or stops the countdown.
func (h *Hub) timerOp(s *session, c *client, op Op) {
	now := h.now().UnixMilli()
	t := &s.timer
	switch op.Action {
	case "start":
		secs := min(max(op.Seconds, timerShortest), timerLongest)
		*t = TimerState{Running: true, Ends: now + int64(secs)*1000, Total: int64(secs) * 1000, By: c.name}
	case "pause":
		if t.Running && !t.Paused {
			t.Paused, t.Left, t.Ends = true, max(t.Ends-now, 0), 0
		}
	case "resume":
		if t.Running && t.Paused {
			t.Paused, t.Ends, t.Left = false, now+t.Left, 0
		}
	case "add":
		secs := int64(min(max(op.Seconds, timerShortest), timerLongest)) * 1000
		switch {
		case t.Running && t.Paused:
			t.Left += secs
		case t.Running:
			t.Ends = max(t.Ends, now) + secs
		}
		t.Total += secs
	case "stop":
		*t = TimerState{}
	default:
		return
	}
	timer := *t
	h.broadcast(s, Event{Kind: Timer, Now: now, Timer: &timer}, nil)
}

// voteFor is the vote as principal sees it.
func (h *Hub) voteFor(s *session, principal string) VoteState {
	v := s.vote
	st := VoteState{Open: v.open, Closed: v.closed, Title: v.title, Per: v.per, By: v.by}
	if v.open {
		st.Mine = maps.Clone(v.votes[principal])
		used := 0
		for _, n := range st.Mine {
			used += n
		}
		st.Left = v.per - used
		for _, mine := range v.votes {
			if len(mine) > 0 {
				st.Voters++
			}
		}
	}
	if v.closed {
		st.Results = maps.Clone(v.results)
		st.Voters = len(v.votes)
	}
	return st
}

// sendVotes tells every browser the vote as it sees it.
func (h *Hub) sendVotes(s *session) {
	for _, c := range s.clients {
		v := h.voteFor(s, c.principal)
		h.send(s, c, Event{Kind: Vote, Vote: &v})
	}
}

// voteOp starts, ends, cancels or clears a vote, or casts or takes back a
// vote. Starting and ending need the right to edit; casting does not, so
// whoever follows the workshop may vote.
func (h *Hub) voteOp(s *session, c *client, op Op) error {
	v := &s.vote
	switch op.Action {
	case "start":
		if !c.editable || c.agent() {
			return ErrForbidden
		}
		title := strings.TrimSpace(op.Title)
		if utf8.RuneCountInString(title) > titleLongest {
			return fmt.Errorf("%w: a vote's title has at most %d characters", ErrInvalid, titleLongest)
		}
		per := op.Votes
		if per == 0 {
			per = votesDefault
		}
		if per < 1 || per > votesMost {
			return fmt.Errorf("%w: 1 to %d votes per person", ErrInvalid, votesMost)
		}
		*v = vote{open: true, title: title, per: per, by: c.name, votes: map[string]map[string]int{}}
	case "end":
		if !c.editable || c.agent() || !v.open {
			return fmt.Errorf("%w: no vote is open", ErrForbidden)
		}
		results := map[string]int{}
		for p, mine := range v.votes {
			if len(mine) == 0 {
				delete(v.votes, p)
				continue
			}
			for id, n := range mine {
				if e := s.elements[id]; e != nil && !e.m.IsDeleted {
					results[id] += n
				}
			}
		}
		v.open, v.closed, v.results = false, true, results
	case "cancel", "clear":
		if !c.editable || c.agent() {
			return ErrForbidden
		}
		*v = vote{}
	case "cast", "retract":
		if !v.open {
			return fmt.Errorf("%w: no vote is open", ErrInvalid)
		}
		e := s.elements[op.Target]
		if e == nil || e.m.IsDeleted || !votable[e.m.Type] {
			return fmt.Errorf("%w: one votes on a note or a shape", ErrInvalid)
		}
		mine := v.votes[c.principal]
		if mine == nil {
			mine = map[string]int{}
			v.votes[c.principal] = mine
		}
		used := 0
		for _, n := range mine {
			used += n
		}
		if op.Action == "cast" {
			if used >= v.per {
				return fmt.Errorf("%w: no votes left", ErrInvalid)
			}
			mine[op.Target]++
		} else if mine[op.Target] > 0 {
			mine[op.Target]--
			if mine[op.Target] == 0 {
				delete(mine, op.Target)
			}
		}
	default:
		return fmt.Errorf("%w: vote action %q", ErrInvalid, op.Action)
	}
	h.sendVotes(s)
	return nil
}

func (h *Hub) presentation(s *session) PresentationState {
	if s.present == nil {
		return PresentationState{}
	}
	return *s.present
}

// frames are the scene's frames in reading order: top to bottom, then left
// to right, rows told apart by half a frame's height.
func (s *session) frames() []string {
	type f struct {
		id   string
		x, y float64
		h    float64
	}
	var fs []f
	for _, id := range s.order {
		if e := s.elements[id]; !e.m.IsDeleted && (e.m.Type == "frame" || e.m.Type == "magicframe") {
			fs = append(fs, f{id, e.m.X, e.m.Y, e.m.Height})
		}
	}
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if d := a.y - b.y; d < -min(a.h, b.h)/2 || d > min(a.h, b.h)/2 {
			return a.y < b.y
		}
		return a.x < b.x
	})
	out := make([]string, len(fs))
	for i, x := range fs {
		out[i] = x.id
	}
	return out
}

// presentOp starts, steps or ends a presentation. One presents at a time;
// another takes over only once it ended.
func (h *Hub) presentOp(s *session, c *client, op Op) error {
	if c.agent() {
		return ErrForbidden
	}
	if s.presenter != nil && s.presenter != c && !(op.Op == "present" && op.Action == "stop") {
		return fmt.Errorf("%w: %s is presenting", ErrLocked, s.presenter.name)
	}
	frames := s.frames()
	frameAt := func(id string) bool {
		for _, f := range frames {
			if f == id {
				return true
			}
		}
		return false
	}
	switch {
	case op.Op == "present" && op.Action == "start":
		p := &PresentationState{Active: true, Client: c.id, Name: c.name}
		switch {
		case op.Target != "" && frameAt(op.Target):
			p.Frame = op.Target
		case len(frames) > 0:
			p.Frame = frames[0]
		default:
			p.X, p.Y, p.Zoom = op.X, op.Y, op.Zoom
		}
		s.present, s.presenter = p, c
	case op.Op == "present" && op.Action == "stop":
		if s.presenter == nil {
			return nil
		}
		s.present, s.presenter = nil, nil
	case s.presenter != c:
		return fmt.Errorf("%w: no presentation of yours", ErrInvalid)
	case op.Op == "frame":
		target := op.Target
		if op.Action == "next" || op.Action == "previous" {
			at := -1
			for i, f := range frames {
				if f == s.present.Frame {
					at = i
				}
			}
			step := 1
			if op.Action == "previous" {
				step = -1
			}
			if n := at + step; n >= 0 && n < len(frames) {
				target = frames[n]
			} else {
				target = s.present.Frame
			}
		}
		if !frameAt(target) {
			return fmt.Errorf("%w: no frame %q", ErrInvalid, target)
		}
		s.present.Frame = target
	case op.Op == "view":
		if op.Zoom <= 0 || op.Zoom > 30 {
			return fmt.Errorf("%w: zoom", ErrInvalid)
		}
		s.present.Frame, s.present.X, s.present.Y, s.present.Zoom = "", op.X, op.Y, op.Zoom
	default:
		return fmt.Errorf("%w: present action %q", ErrInvalid, op.Action)
	}
	p := h.presentation(s)
	h.broadcast(s, Event{Kind: Presentation, Now: h.now().UnixMilli(), Presentation: &p}, nil)
	return nil
}
