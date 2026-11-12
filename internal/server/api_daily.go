package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dbuchs7/orbithek/internal/auth"
	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// dailyRoutes are a person's daily notes (ADR-0203): the room they go to,
// kept with the account, the days one has a note on, and one note by its
// day, read, opened (made when missing) or added to. Every route acts on
// the caller's own notes only.
func dailyRoutes(h itemHandlers) []Route {
	get, post, put := http.MethodGet, http.MethodPost, http.MethodPut
	return []Route{
		{Method: get, Pattern: "/api/daily", Summary: "The room one's daily notes go to, whether one may still create there, and the days from from to to (YYYY-MM-DD) one has a daily note on (ADR-0203)", Handler: http.HandlerFunc(h.dailyDays)},
		{Method: put, Pattern: "/api/daily/room", Summary: "Choose the room one's daily notes go to, one where one may create items; empty clears the choice", Handler: http.HandlerFunc(h.setDailyRoom)},
		{Method: get, Pattern: "/api/daily/{date}", Summary: "One's daily note of a day (YYYY-MM-DD), if there is one", Handler: http.HandlerFunc(h.getDailyNote)},
		{Method: post, Pattern: "/api/daily/{date}", Summary: "Open one's daily note of a day: the note, made in one's daily room when there is none (created), restricted to oneself in the same step; title names a new note", Handler: http.HandlerFunc(h.openDailyNote)},
		{Method: post, Pattern: "/api/daily/{date}/paragraphs", Summary: "Add text at the end of one's daily note of a day, a paragraph per line, making the note first when there is none", Handler: http.HandlerFunc(h.appendDailyNote)},
	}
}

// errNoDailyRoom answers a note to be made before a room is chosen.
var errNoDailyRoom = errors.New("choose the room your daily notes go to first")

// dailyDate reads the day of the path, or answers the error.
func dailyDate(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	d, err := time.Parse(time.DateOnly, r.PathValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "the date is a day, YYYY-MM-DD")
		return time.Time{}, false
	}
	return d, true
}

type dailyNoteJSON struct {
	Item     itemJSON `json:"item"`
	Created  bool     `json:"created"`
	Editable bool     `json:"editable"`
}

func (h itemHandlers) dailyDays(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err1 := time.Parse(time.DateOnly, q.Get("from"))
	to, err2 := time.Parse(time.DateOnly, q.Get("to"))
	if err1 != nil || err2 != nil || to.Before(from) {
		writeError(w, http.StatusBadRequest, "from and to are days, YYYY-MM-DD, from not after to")
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > budget.Get().DailyNoteDays {
		writeError(w, http.StatusBadRequest, "at most "+strconv.Itoa(budget.Get().DailyNoteDays)+" days at once")
		return
	}
	room := h.d.DB.Accounts.DailyRoom(auth.UserID(who(r)))
	h.read(w, r, func(x *item.Session) (any, error) {
		out := []string{}
		for _, d := range x.DailyNoteDays(from, to) {
			out = append(out, d.Format(time.DateOnly))
		}
		return map[string]any{"space": room, "mayCreate": room != "" && x.Role(room) >= item.Editor, "days": out}, nil
	})
}

func (h itemHandlers) setDailyRoom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Space string `json:"space"`
	}
	if !decode(w, r, &req) {
		return
	}
	space := strings.TrimSpace(req.Space)
	if space != "" {
		var role item.Role
		h.d.DB.Read(func(s *item.Store) {
			if s.HasSpace(space) {
				role = s.As(who(r)).Role(space)
			}
		})
		switch {
		case role == 0:
			// A room one holds no role in does not exist for one (D10).
			writeError(w, http.StatusNotFound, "space not found")
			return
		case role < item.Editor:
			writeError(w, http.StatusForbidden, "daily notes go to a room where you may create pages")
			return
		}
	}
	switch err := h.d.DB.Accounts.SetDailyRoom(auth.UserID(who(r)), space); {
	case errors.Is(err, auth.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		h.fail(w, err)
	default:
		writeJSON(w, http.StatusOK, map[string]string{"space": space})
	}
}

func (h itemHandlers) getDailyNote(w http.ResponseWriter, r *http.Request) {
	d, ok := dailyDate(w, r)
	if !ok {
		return
	}
	h.read(w, r, func(x *item.Session) (any, error) {
		it, ok := x.DailyNote(d)
		if !ok {
			return nil, item.ErrNotFound
		}
		return dailyNoteJSON{Item: withSecrets(x, toItemJSON(it)), Editable: x.CanEdit(it.ID)}, nil
	})
}

func (h itemHandlers) openDailyNote(w http.ResponseWriter, r *http.Request) {
	d, ok := dailyDate(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	h.writeDaily(w, r, d, func(x *item.Session, room string) (item.Item, bool, error) {
		return x.OpenDailyNote(room, d, req.Title)
	})
}

func (h itemHandlers) appendDailyNote(w http.ResponseWriter, r *http.Request) {
	d, ok := dailyDate(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	h.writeDaily(w, r, d, func(x *item.Session, room string) (item.Item, bool, error) {
		return x.AppendToDailyNote(room, d, req.Title, req.Text)
	})
}

// writeDaily runs a command on the caller's daily note of day d in their
// daily room and answers the note. Without a room chosen, a note that does
// not exist yet is not made: the answer says a room is needed (needsRoom).
func (h itemHandlers) writeDaily(w http.ResponseWriter, r *http.Request, d time.Time, cmd func(x *item.Session, room string) (item.Item, bool, error)) {
	room := h.d.DB.Accounts.DailyRoom(auth.UserID(who(r)))
	if room == "" {
		var exists bool
		h.d.DB.Read(func(s *item.Store) { _, exists = s.As(who(r)).DailyNote(d) })
		if !exists {
			writeJSON(w, http.StatusConflict, map[string]any{"error": errNoDailyRoom.Error(), "needsRoom": true})
			return
		}
	}
	h.write(w, r, http.StatusOK, func(x *item.Session, _ *item.Store) (any, error) {
		it, created, err := cmd(x, room)
		if err != nil {
			return nil, err
		}
		return dailyNoteJSON{Item: withSecrets(x, toItemJSON(it)), Created: created, Editable: x.CanEdit(it.ID)}, nil
	})
}
