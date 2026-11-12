package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestATableKeepsItsFormatsSizesAndColours: a table's marks, column
// widths, row heights and coloured cells are written and read back
// through the API; sizes that are all 0 and an empty list of cells read
// as none, and what the model refuses is refused (ADR-0200).
func TestATableKeepsItsFormatsSizesAndColours(t *testing.T) {
	w := newWorld(t)
	it := w.bert.expect("POST", "/api/items", `{"space":"PLAT","type":"task","title":"Hosts","properties":{"priority":"low"}}`, http.StatusCreated)
	key := field(it, "item", "key").(string)
	got := field(w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"table","text":"Host\tRole\ndb1\tprimary",
		"marks":[{"type":"bold","start":0,"end":4},{"type":"link","start":10,"end":13,"href":"https://db1.example.org"}],
		"columns":[160,0],"rows":[0,0],"cells":[{"row":1,"col":1,"fill":"yellow","border":"red"}]}]}`, http.StatusOK), "item", "body").([]any)
	b, _ := json.Marshal(got[0])
	want := `{"cells":[{"border":"red","col":1,"fill":"yellow","row":1}],"columns":[160,0],"kind":"table","marks":[{"end":4,"start":0,"type":"bold"},` +
		`{"end":13,"href":"https://db1.example.org","start":10,"type":"link"}],"text":"Host\tRole\ndb1\tprimary"}`
	if string(b) != want {
		t.Fatalf("the table reads\n%s, want\n%s", b, want)
	}
	got = field(w.bert.expect("PUT", "/api/items/"+key+"/body", `{"blocks":[{"kind":"table","text":"a\tb","columns":[0,0],"cells":[]}]}`, http.StatusOK), "item", "body").([]any)
	if tb := got[0].(map[string]any); tb["columns"] != nil || tb["cells"] != nil {
		t.Fatalf("sizes all 0 and no cells read as %v", tb)
	}
	for _, bad := range []string{
		`{"blocks":[{"kind":"table","text":"a\tb","marks":[{"type":"bold","start":0,"end":3}]}]}`,
		`{"blocks":[{"kind":"table","text":"a\tb","columns":[100]}]}`,
		`{"blocks":[{"kind":"table","text":"a\tb","cells":[{"row":0,"col":0,"fill":"#ff0000"}]}]}`,
		`{"blocks":[{"kind":"paragraph","text":"a","rows":[40]}]}`,
	} {
		w.bert.expect("PUT", "/api/items/"+key+"/body", bad, http.StatusBadRequest)
	}
}
