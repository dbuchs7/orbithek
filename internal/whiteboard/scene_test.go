package whiteboard

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"image/png"
	"strings"
	"testing"
)

func testScene() Scene {
	return Scene{
		Elements:   []json.RawMessage{json.RawMessage(`{"id":"a","type":"rectangle","version":1,"text":"Grüezi ü € 𝄞"}`)},
		Files:      map[string]json.RawMessage{"f1": json.RawMessage(`{"id":"f1","mimeType":"image/png","dataURL":"data:image/png;base64,AA=="}`)},
		Background: "#fafafa",
	}
}

func TestSceneRoundTrip(t *testing.T) {
	p, err := WriteScene(Placeholder(), testScene(), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(p)); err != nil {
		t.Fatalf("the picture no longer decodes: %v", err)
	}
	sc, stale, err := ReadScene(p, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if stale || sc.Background != "#fafafa" || len(sc.Elements) != 1 || len(sc.Files) != 1 {
		t.Fatalf("scene = %+v, stale %v", sc, stale)
	}
	var m map[string]any
	_ = json.Unmarshal(sc.Elements[0], &m)
	if m["text"] != "Grüezi ü € 𝄞" {
		t.Fatalf("text = %q", m["text"])
	}
	// Writing again replaces the scene and marks the picture stale; the
	// text chunk is Latin-1 throughout, as PNG requires.
	sc2 := Scene{Elements: []json.RawMessage{json.RawMessage(`{"id":"b","type":"text","version":3}`)}}
	p2, err := WriteScene(p, sc2, true)
	if err != nil {
		t.Fatal(err)
	}
	got, stale, err := ReadScene(p2, 1<<20)
	if err != nil || !stale || len(got.Elements) != 1 || !strings.Contains(string(got.Elements[0]), `"b"`) || got.Background != "#ffffff" {
		t.Fatalf("rewritten scene = %+v, stale %v, err %v", got, stale, err)
	}
	if n := bytes.Count(p2, []byte(sceneKeyword)); n != 1 {
		t.Fatalf("the picture holds %d scenes", n)
	}
	// Clearing the mark.
	p3, _ := WriteScene(p2, sc2, false)
	if _, stale, _ := ReadScene(p3, 1<<20); stale {
		t.Fatal("the stale mark stayed")
	}
	cs, _ := chunks(p2)
	for _, c := range cs {
		if c.kind == "tEXt" {
			for _, b := range c.data {
				if b == 0 && !bytes.HasPrefix(c.data, []byte(sceneKeyword)) {
					t.Fatal("a NUL in the text")
				}
			}
		}
	}
}

func TestSceneEncodesEmpty(t *testing.T) {
	p, err := WriteScene(Placeholder(), Scene{}, true)
	if err != nil {
		t.Fatal(err)
	}
	sc, stale, err := ReadScene(p, 1<<20)
	if err != nil || !stale || len(sc.Elements) != 0 || sc.Files == nil {
		t.Fatalf("empty scene = %+v %v %v", sc, stale, err)
	}
	raw, _ := Scene{}.encode()
	if !strings.Contains(string(raw), `"viewBackgroundColor":"#ffffff"`) || !strings.Contains(string(raw), `"elements":[]`) {
		t.Fatalf("encoded = %s", raw)
	}
}

// excalidrawChunk writes a scene chunk the way Excalidraw does, with an
// envelope chosen by the test.
func excalidrawChunk(t *testing.T, env string) []byte {
	t.Helper()
	cs, _ := chunks(Placeholder())
	var out bytes.Buffer
	out.Write(pngSignature)
	for _, c := range cs {
		if c.kind == "IEND" {
			writeChunk(&out, chunk{kind: "tEXt", data: append([]byte(sceneKeyword+"\x00"), env...)})
		}
		writeChunk(&out, c)
	}
	return out.Bytes()
}

func TestReadSceneRefuses(t *testing.T) {
	if _, _, err := ReadScene([]byte("GIF89a"), 1<<20); !errors.Is(err, ErrNotPNG) {
		t.Errorf("not a PNG: %v", err)
	}
	if _, _, err := ReadScene(Placeholder(), 1<<20); !errors.Is(err, ErrNoScene) {
		t.Errorf("no scene: %v", err)
	}
	if _, _, err := ReadScene(Placeholder()[:40], 1<<20); !errors.Is(err, ErrNotPNG) {
		t.Errorf("cut off: %v", err)
	}
	bad := append([]byte{}, Placeholder()...)
	bad[8+3] = 0xff // IHDR's length beyond the file
	if _, _, err := ReadScene(bad, 1<<20); !errors.Is(err, ErrNotPNG) {
		t.Errorf("bad length: %v", err)
	}
	if _, err := WriteScene([]byte("x"), Scene{}, false); !errors.Is(err, ErrNotPNG) {
		t.Errorf("write into no PNG: %v", err)
	}
	for name, env := range map[string]string{
		"not JSON":       `{`,
		"unknown coding": `{"version":"1","encoding":"base64","compressed":false,"encoded":"x"}`,
		"not deflated":   `{"version":"1","encoding":"bstring","compressed":true,"encoded":"xyz"}`,
		"not a scene":    `{"version":"1","encoding":"bstring","compressed":false,"encoded":"[1,2]"}`,
		"beyond latin1":  `{"version":"1","encoding":"bstring","compressed":false,"encoded":"€"}`,
		"not utf-8":      `{"version":"1","encoding":"bstring","compressed":false,"encoded":"ÿ"}`,
	} {
		if _, _, err := ReadScene(excalidrawChunk(t, env), 1<<20); !errors.Is(err, ErrNoScene) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Uncompressed scenes are read too.
	plain := `{"version":"1","encoding":"bstring","compressed":false,"encoded":"{\"elements\":[],\"appState\":{\"viewBackgroundColor\":\"#000000\"}}"}`
	if sc, _, err := ReadScene(excalidrawChunk(t, plain), 1<<20); err != nil || sc.Background != "#000000" {
		t.Errorf("plain scene: %+v %v", sc, err)
	}
	// A scene that unpacks beyond the limit is refused.
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write([]byte(`{"elements":[],"pad":"` + strings.Repeat("x", 5000) + `"}`))
	_ = zw.Close()
	env, _ := json.Marshal(envelope{Version: "1", Encoding: "bstring", Compressed: true, Encoded: latin1(z.Bytes())})
	lat, _ := toLatin1(string(env))
	if _, _, err := ReadScene(excalidrawChunk(t, string(lat)), 1000); !errors.Is(err, ErrTooLarge) {
		t.Errorf("too large: %v", err)
	}
}
