package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// uploaded uploads content as a file of an item and answers its ID; source,
// if given, is the file it is a new version of.
func (c *client) uploaded(id, name string, content []byte, source string) string {
	c.t.Helper()
	path := "/api/items/" + id + "/attachments?name=" + name
	if source != "" {
		path += "&sourceItem=" + id + "&sourceAttachment=" + source
	}
	rec := c.upload(path, "application/octet-stream", content)
	if rec.Code != http.StatusCreated {
		c.t.Fatalf("upload %s: %d %s", name, rec.Code, rec.Body)
	}
	var created struct{ Attachment attachmentJSON }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return string(created.Attachment.ID)
}

func TestPDFHighlightsThroughTheAPI(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Annual report","properties":{"priority":"low"}}`, http.StatusCreated)
	key, id := field(it, "item", "key").(string), field(it, "item", "id").(string)
	v1 := w.bert.uploaded(id, "report.pdf", []byte("%PDF-1.4\n% first\n"), "")
	v2 := w.bert.uploaded(id, "report.pdf", []byte("%PDF-1.4\n% second\n"), v1)
	png := w.bert.uploaded(id, "chart.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 40)...), "")

	// The reader learns the PDF's versions from any of them.
	for _, f := range []string{v1, v2} {
		pdf := w.vera.expect("GET", "/api/items/"+key+"/pdfs/"+f, "", http.StatusOK)
		if pdf["first"] != v1 || field(pdf, "newest", "id") != v2 || len(pdf["versions"].([]any)) != 2 || field(pdf, "newest", "name") != "report.pdf" {
			t.Fatalf("pdf by %s: %v", f, pdf)
		}
	}
	w.vera.expect("GET", "/api/items/"+key+"/pdfs/"+png, "", http.StatusNotFound)
	w.vera.expect("GET", "/api/items/"+key+"/pdfs/0123456789abcdef0123456789abcdef", "", http.StatusNotFound)
	w.carla.expect("GET", "/api/items/"+key+"/pdfs/"+v1, "", http.StatusNotFound)

	path := "/api/items/" + key + "/comments"
	c := w.bert.expect("POST", path, fmt.Sprintf(`{"text":"Check Q4","anchor":{"file":%q,"page":3,"quote":"revenue grew","prefix":"In 2025 ",`+
		`"rects":[{"x":0.1,"y":0.2,"w":0.5,"h":0.02}],"color":"green"}}`, v2), http.StatusCreated)
	if field(c, "comment", "anchor", "file") != v1 || field(c, "comment", "anchor", "version") != v2 || field(c, "comment", "anchor", "page") != 3.0 ||
		field(c, "comment", "anchor", "color") != "green" || field(c, "comment", "anchor", "prefix") != "In 2025 " {
		t.Fatalf("created %v", c)
	}
	rects := field(c, "comment", "anchor", "rects").([]any)
	if len(rects) != 1 || rects[0].(map[string]any)["w"] != 0.5 {
		t.Fatalf("rects %v", rects)
	}
	// An agent names the page and the quote only.
	bare := w.bert.expect("POST", path, fmt.Sprintf(`{"text":"?","anchor":{"file":%q,"page":1,"quote":"outlook"}}`, v1), http.StatusCreated)
	if a := field(bare, "comment", "anchor").(map[string]any); a["version"] != v1 || a["rects"] != nil || a["color"] != nil {
		t.Fatalf("bare %v", a)
	}
	for name, anchor := range map[string]string{
		"no PDF":       fmt.Sprintf(`{"file":%q,"page":1,"quote":"x"}`, png),
		"no file":      `{"file":"0123456789abcdef0123456789abcdef","page":1,"quote":"x"}`,
		"no page":      fmt.Sprintf(`{"file":%q,"quote":"x"}`, v1),
		"off the page": fmt.Sprintf(`{"file":%q,"page":1,"quote":"x","rects":[{"x":0.9,"y":0,"w":0.5,"h":0.1}]}`, v1),
		"a colour":     fmt.Sprintf(`{"file":%q,"page":1,"quote":"x","color":"red"}`, v1),
		"page on text": `{"quote":"x","page":2}`,
	} {
		w.bert.expect("POST", path, `{"text":"?","anchor":`+anchor+`}`, http.StatusBadRequest)
		_ = name
	}
	list := w.vera.expect("GET", path, "", http.StatusOK)["comments"].([]any)
	if len(list) != 2 || list[0].(map[string]any)["anchor"].(map[string]any)["file"] != v1 {
		t.Fatalf("listed %v", list)
	}
	// A viewer reads the highlights and makes none.
	w.vera.expect("POST", path, fmt.Sprintf(`{"text":"?","anchor":{"file":%q,"page":1,"quote":"x"}}`, v1), http.StatusForbidden)
	// The PDF stays with its highlights.
	other := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Archive","properties":{"priority":"low"}}`, http.StatusCreated)
	w.bert.expect("POST", "/api/items/"+key+"/attachments/"+v2+"/move", fmt.Sprintf(`{"to":%q}`, field(other, "item", "key")), http.StatusConflict)
}
