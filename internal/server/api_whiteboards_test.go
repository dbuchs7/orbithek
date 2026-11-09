package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/whiteboard"
)

// boardStream reads a whiteboard's server-sent events.
func boardStream(t *testing.T, srv *httptest.Server, c *client, path string) *stream {
	t.Helper()
	req, _ := http.NewRequest("GET", srv.URL+path, nil)
	req.AddCookie(c.cookie)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream %s: status %d", path, res.StatusCode)
	}
	s := &stream{t: t, res: res, events: make(chan sse, 256)}
	go func() {
		defer close(s.events)
		r := bufio.NewReader(res.Body)
		var cur sse
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				cur.kind = line[len("event: "):]
			case strings.HasPrefix(line, "data: "):
				_ = json.Unmarshal([]byte(line[len("data: "):]), &cur.data)
			case line == "" && cur.kind != "":
				s.events <- cur
				cur = sse{}
			}
		}
	}()
	t.Cleanup(func() { res.Body.Close() })
	return s
}

func TestWhiteboardThroughTheAPI(t *testing.T) {
	var hub *whiteboard.Hub
	w := newWorld(t, func(d *Deps) {
		hub = NewWhiteboards(d.DB, nil, nil)
		d.Whiteboards = hub
	})
	srv := httptest.NewServer(w.bert.h)
	t.Cleanup(srv.Close) // after the streams' bodies are closed
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Retro","properties":{"priority":"low"}}`, http.StatusCreated)
	w.bert.expect("PUT", "/api/items/PLAT-1/body", `{"blocks":[{"kind":"paragraph","text":"Before the board"}]}`, http.StatusOK)

	// Making one: an empty drawing at the end of the body.
	w.vera.expect("POST", "/api/items/PLAT-1/whiteboards", `{"title":"Retro board"}`, http.StatusForbidden)
	made := w.bert.expect("POST", "/api/items/PLAT-1/whiteboards", `{"title":"Retro / board"}`, http.StatusCreated)
	file := made["file"].(string)
	if name := field(made, "attachment", "name"); name != "Retro board.drawing.png" {
		t.Fatalf("name = %v", name)
	}
	body := field(w.vera.expect("GET", "/api/items/PLAT-1", "", http.StatusOK), "item", "body").([]any)
	if last := body[len(body)-1].(map[string]any); last["kind"] != "image" || last["ref"] != file || last["text"] != "Retro / board" {
		t.Fatalf("body = %v", body)
	}
	base := "/api/items/PLAT-1/whiteboards/" + file

	// Reading without a session; strangers find nothing.
	v := w.vera.expect("GET", base, "", http.StatusOK)
	if v["live"] != false || len(v["elements"].([]any)) != 0 {
		t.Fatalf("view = %v", v)
	}
	w.carla.expect("GET", base, "", http.StatusNotFound)
	w.bert.expect("GET", "/api/items/PLAT-1/whiteboards/0123456789abcdef0123456789abcdef", "", http.StatusNotFound)

	// An agent writes notes; a reader's agent may not.
	changed := w.bert.expect("POST", base+"/changes", `{"changes":[{"op":"note","text":"Keep the daily"},{"op":"note","text":"Drop the weekly","colour":"pink"}]}`, http.StatusOK)
	ids := changed["elements"].([]any)
	if len(ids) != 2 {
		t.Fatalf("changed = %v", changed)
	}
	w.vera.expect("POST", base+"/changes", `{"changes":[{"op":"note"}]}`, http.StatusForbidden)
	w.bert.expect("POST", base+"/changes", `{"changes":[{"op":"teleport"}]}`, http.StatusBadRequest)
	w.bert.expect("POST", base+"/changes", `{"changes":[{"op":"note","colour":"x"}],"extra":1}`, http.StatusBadRequest)

	// Browsers join: a snapshot with the agent's notes.
	bert := boardStream(t, srv, w.bert, base+"/live")
	snap := bert.next("snapshot")
	bertClient := snap["client"].(string)
	if snap["keeper"] != true || snap["editable"] != true || len(snap["elements"].([]any)) != 4 {
		t.Fatalf("snapshot = %v", snap)
	}
	vera := boardStream(t, srv, w.vera, base+"/live")
	vsnap := vera.next("snapshot")
	veraClient := vsnap["client"].(string)
	if vsnap["editable"] == true || vsnap["keeper"] == true {
		t.Fatalf("a reader's snapshot = %v", vsnap)
	}
	if rec := w.carla.do("GET", base+"/live", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger joined: %d", rec.Code)
	}
	live := w.vera.expect("GET", "/api/whiteboards", "", http.StatusOK)["whiteboards"].([]any)
	if len(live) != 1 || field(live[0].(map[string]any), "key") != "PLAT-1" || field(live[0].(map[string]any), "participants") != 2.0 {
		t.Fatalf("live whiteboards = %v", live)
	}
	if got := w.carla.expect("GET", "/api/whiteboards", "", http.StatusOK)["whiteboards"].([]any); len(got) != 0 {
		t.Fatalf("a stranger sees %v", got)
	}

	ops := func(c *client, clientID, ops string, status int) map[string]any {
		t.Helper()
		return c.expect("POST", base+"/live", `{"client":"`+clientID+`","ops":`+ops+`}`, status)
	}
	note := ids[0].(string)
	ops(w.bert, bertClient, `[{"op":"lock","ids":["`+note+`"]}]`, http.StatusOK)
	if l := vera.next("locks")["locks"].([]any); field(l[0].(map[string]any), "client") != bertClient {
		t.Fatalf("locks = %v", l)
	}
	// The agent is refused what bert holds, with his name.
	if rec := w.bert.do("POST", base+"/changes", `{"changes":[{"op":"edit","id":"`+note+`","text":"x"}]}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "bert") {
		t.Fatalf("held: %d %s", rec.Code, rec.Body.String())
	}
	el := `{"id":"r1","type":"rectangle","version":1,"x":0,"y":0,"width":10,"height":10,"index":"b1"}`
	ops(w.bert, bertClient, `[{"op":"put","element":`+el+`}]`, http.StatusOK)
	if e := vera.next("elements"); len(e["elements"].([]any)) != 1 {
		t.Fatalf("elements = %v", e)
	}
	ops(w.vera, veraClient, `[{"op":"put","element":`+el+`}]`, http.StatusForbidden)
	ops(w.vera, veraClient, `[{"op":"pointer","x":1,"y":2}]`, http.StatusOK)
	ops(w.vera, "nope", `[{"op":"pointer"}]`, http.StatusConflict)
	ops(w.bert, bertClient, `[{"op":"fly"}]`, http.StatusBadRequest)
	if res := ops(w.bert, bertClient, `[{"op":"put","element":`+el+`}]`, http.StatusOK); len(res["refused"].([]any)) != 1 {
		t.Fatalf("a stale version was taken: %v", res)
	}

	// A comment on an element names the drawing by its first version.
	c := w.bert.expect("POST", "/api/items/PLAT-1/comments", `{"text":"Why?","anchor":{"drawing":"`+file+`","element":"`+note+`","quote":"Keep the daily"}}`, http.StatusCreated)
	if field(c, "comment", "anchor", "element") != note || field(c, "comment", "anchor", "drawing") != file {
		t.Fatalf("comment = %v", c)
	}
	w.bert.expect("POST", "/api/items/PLAT-1/comments", `{"text":"?","anchor":{"drawing":"0123456789abcdef0123456789abcdef","element":"x"}}`, http.StatusBadRequest)

	// A checkpoint: the keeper draws, the server writes its scene.
	ops(w.bert, bertClient, `[{"op":"save"}]`, http.StatusOK)
	hub.Tick()
	token := bert.next("checkpoint")["token"].(string)
	upload := func(c *client, query string, content []byte, ctype string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", base+"/checkpoints?"+query, bytes.NewReader(content))
		req.Header.Set("Content-Type", ctype)
		req.AddCookie(c.cookie)
		rec := httptest.NewRecorder()
		c.h.ServeHTTP(rec, req)
		return rec
	}
	if rec := upload(w.bert, "client="+bertClient+"&token="+token, whiteboard.Placeholder(), "image/png"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong type: %d", rec.Code)
	}
	if rec := upload(w.bert, "client="+bertClient+"&token=wrong", whiteboard.Placeholder(), "application/octet-stream"); rec.Code != http.StatusConflict {
		t.Fatalf("wrong token: %d", rec.Code)
	}
	if rec := upload(w.vera, "client="+veraClient+"&token="+token, whiteboard.Placeholder(), "application/octet-stream"); rec.Code != http.StatusForbidden {
		t.Fatalf("a reader's checkpoint: %d", rec.Code)
	}
	rec := upload(w.bert, "client="+bertClient+"&token="+token, whiteboard.Placeholder(), "application/octet-stream")
	if rec.Code != http.StatusOK {
		t.Fatalf("checkpoint: %d %s", rec.Code, rec.Body.String())
	}
	saved := vera.next("saved")["saved"].(map[string]any)
	newFile := saved["file"].(string)
	if newFile == file || saved["by"] != "bert" {
		t.Fatalf("saved = %v", saved)
	}
	// The body shows the new version; the old address still leads to the
	// whiteboard.
	body = field(w.vera.expect("GET", "/api/items/PLAT-1", "", http.StatusOK), "item", "body").([]any)
	if body[len(body)-1].(map[string]any)["ref"] != newFile {
		t.Fatalf("the body still shows %v", body[len(body)-1])
	}
	w.vera.expect("GET", "/api/items/PLAT-1/whiteboards/"+newFile, "", http.StatusOK)
	var stored []byte
	w.db.Read(func(s *item.Store) {})
	f, err := w.db.OpenAttachment(item.ID(field(w.vera.expect("GET", "/api/items/PLAT-1", "", http.StatusOK), "item", "id").(string)), item.ID(newFile))
	if err != nil {
		t.Fatal(err)
	}
	stored, _ = readAllClose(f)
	sc, stale, err := whiteboard.ReadScene(stored, 1<<26)
	if err != nil || stale || len(sc.Elements) != 5 {
		t.Fatalf("stored scene: %d elements, stale %v, %v", len(sc.Elements), stale, err)
	}
	// Losing access closes the stream.
	w.admin.expect("DELETE", "/api/spaces/PLAT/roles/"+w.ids["vera"], "", http.StatusNoContent)
	if e := vera.next("closed"); e["reason"] == "" {
		t.Fatal("no reason")
	}
	_ = time.Second
}

func readAllClose(f interface {
	Read([]byte) (int, error)
	Close() error
}) ([]byte, error) {
	defer f.Close()
	var b bytes.Buffer
	_, err := b.ReadFrom(f)
	return b.Bytes(), err
}

func TestWhiteboardNamesAndLimits(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "Whiteboard.drawing.png",
		"  ":                    "Whiteboard.drawing.png",
		"a/b\\c":                "a b c.drawing.png",
		strings.Repeat("é", 90): strings.Repeat("é", 80) + ".drawing.png",
	} {
		if got := whiteboardName(in); got != want {
			t.Errorf("whiteboardName(%q) = %q", in, got)
		}
	}
	w := newWorld(t)
	w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Retro","properties":{"priority":"low"}}`, http.StatusCreated)
	file := w.bert.expect("POST", "/api/items/PLAT-1/whiteboards", `{}`, http.StatusCreated)["file"].(string)
	base := "/api/items/PLAT-1/whiteboards/" + file
	w.bert.expect("POST", base+"/live", `{"client":"x","ops":[{"op":"pointer"}]}`, http.StatusConflict)
	w.bert.expect("POST", base+"/changes", `{"changes":[]}`, http.StatusBadRequest)
	w.carla.expect("POST", base+"/changes", `{"changes":[{"op":"note"}]}`, http.StatusNotFound)
	w.carla.expect("POST", base+"/live", `{"client":"x","ops":[]}`, http.StatusNotFound)
	if rec := w.carla.do("POST", base+"/checkpoints?client=x&token=y", ""); rec.Code != http.StatusUnsupportedMediaType && rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger's checkpoint: %d", rec.Code)
	}
	w.carla.expect("POST", "/api/items/PLAT-1/whiteboards", `{}`, http.StatusNotFound)
}
