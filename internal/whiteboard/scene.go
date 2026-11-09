package whiteboard

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"strings"
	"unicode/utf8"
)

// A drawing is a PNG that carries its Excalidraw scene (ADR-0053): a text
// chunk named by Excalidraw's media type, whose text is a small JSON
// envelope around the scene, deflated and written as a string of Latin-1
// characters, one per byte. The server reads it to open a session and
// writes it into every checkpoint, so a saved version holds what the
// session held, not what a browser claims (ADR-0168).

// sceneKeyword names the text chunk that holds the scene.
const sceneKeyword = "application/vnd.excalidraw+json"

// staleChunk marks a picture whose pixels are older than its scene: the
// server wrote the scene without a browser to draw it. Ancillary, private
// and safe to copy (lower, lower, upper, lower), like the regions of
// ADR-0142.
const staleChunk = "wkSt"

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// Errors of reading a drawing.
var (
	ErrNotPNG   = errors.New("not a PNG picture")
	ErrNoScene  = errors.New("the picture carries no drawing")
	ErrTooLarge = errors.New("the drawing is too large")
)

// Scene is a drawing's content: its elements, in their order, each as
// Excalidraw wrote it; its image files by ID; and its background.
type Scene struct {
	Elements   []json.RawMessage
	Files      map[string]json.RawMessage
	Background string
}

type chunk struct {
	kind string
	data []byte
}

// chunks splits a PNG into its chunks, checking their frame (not their
// checksums: a viewer does that) and that it ends with IEND.
func chunks(p []byte) ([]chunk, error) {
	if !bytes.HasPrefix(p, pngSignature) {
		return nil, ErrNotPNG
	}
	var out []chunk
	for rest := p[len(pngSignature):]; len(rest) > 0; {
		if len(rest) < 12 {
			return nil, ErrNotPNG
		}
		n := binary.BigEndian.Uint32(rest)
		if uint64(n) > uint64(len(rest)-12) {
			return nil, ErrNotPNG
		}
		c := chunk{kind: string(rest[4:8]), data: rest[8 : 8+n]}
		out = append(out, c)
		rest = rest[12+n:]
		if c.kind == "IEND" {
			break
		}
	}
	if len(out) < 2 || out[0].kind != "IHDR" || out[len(out)-1].kind != "IEND" {
		return nil, ErrNotPNG
	}
	return out, nil
}

func writeChunk(w *bytes.Buffer, c chunk) {
	var head [8]byte
	binary.BigEndian.PutUint32(head[:4], uint32(len(c.data)))
	copy(head[4:], c.kind)
	w.Write(head[:])
	w.Write(c.data)
	crc := crc32.NewIEEE()
	crc.Write(head[4:])
	crc.Write(c.data)
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())
	w.Write(sum[:])
}

// envelope is Excalidraw's wrapper around the scene in the text chunk.
type envelope struct {
	Version    string `json:"version"`
	Encoding   string `json:"encoding"`
	Compressed bool   `json:"compressed"`
	Encoded    string `json:"encoded"`
}

// latin1 reads bytes as Latin-1 characters; toLatin1 writes such
// characters back as bytes.
func latin1(b []byte) string {
	var s strings.Builder
	for _, c := range b {
		s.WriteRune(rune(c))
	}
	return s.String()
}

func toLatin1(s string) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xff {
			return nil, fmt.Errorf("%w: a character beyond Latin-1", ErrNoScene)
		}
		out = append(out, byte(r))
	}
	return out, nil
}

// sceneFile is the scene as Excalidraw serialises it.
type sceneFile struct {
	Type     string                     `json:"type"`
	Version  int                        `json:"version"`
	Source   string                     `json:"source"`
	Elements []json.RawMessage          `json:"elements"`
	AppState map[string]json.RawMessage `json:"appState"`
	Files    map[string]json.RawMessage `json:"files"`
}

// ReadScene returns the scene a drawing's PNG carries, and whether its
// picture is stale. The scene may unpack to at most limit bytes.
func ReadScene(p []byte, limit int64) (Scene, bool, error) {
	cs, err := chunks(p)
	if err != nil {
		return Scene{}, false, err
	}
	var text []byte
	stale := false
	for _, c := range cs {
		switch {
		case c.kind == staleChunk:
			stale = true
		case c.kind == "tEXt" && text == nil && bytes.HasPrefix(c.data, []byte(sceneKeyword+"\x00")):
			text = c.data[len(sceneKeyword)+1:]
		}
	}
	if text == nil {
		return Scene{}, false, ErrNoScene
	}
	var env envelope
	if err := json.Unmarshal([]byte(latin1(text)), &env); err != nil || env.Encoding != "bstring" {
		return Scene{}, false, fmt.Errorf("%w: unknown encoding", ErrNoScene)
	}
	raw, err := toLatin1(env.Encoded)
	if err != nil {
		return Scene{}, false, err
	}
	if env.Compressed {
		z, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return Scene{}, false, fmt.Errorf("%w: %v", ErrNoScene, err)
		}
		raw, err = io.ReadAll(io.LimitReader(z, limit+1))
		if err != nil {
			return Scene{}, false, fmt.Errorf("%w: %v", ErrNoScene, err)
		}
	}
	if int64(len(raw)) > limit {
		return Scene{}, false, ErrTooLarge
	}
	if !utf8.Valid(raw) {
		return Scene{}, false, fmt.Errorf("%w: not UTF-8", ErrNoScene)
	}
	var f sceneFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return Scene{}, false, fmt.Errorf("%w: %v", ErrNoScene, err)
	}
	sc := Scene{Elements: f.Elements, Files: f.Files}
	if sc.Files == nil {
		sc.Files = map[string]json.RawMessage{}
	}
	if bg, ok := f.AppState["viewBackgroundColor"]; ok {
		_ = json.Unmarshal(bg, &sc.Background)
	}
	return sc, stale, nil
}

// encode serialises the scene as Excalidraw does for a file.
func (sc Scene) encode() ([]byte, error) {
	bg, _ := json.Marshal(sc.Background)
	if sc.Background == "" {
		bg = []byte(`"#ffffff"`)
	}
	elements := sc.Elements
	if elements == nil {
		elements = []json.RawMessage{}
	}
	files := sc.Files
	if files == nil {
		files = map[string]json.RawMessage{}
	}
	return json.Marshal(sceneFile{Type: "excalidraw", Version: 2, Source: "orbithek", Elements: elements,
		AppState: map[string]json.RawMessage{"viewBackgroundColor": bg}, Files: files})
}

// sceneChunk makes the text chunk that carries the scene.
func sceneChunk(sc Scene) (chunk, error) {
	raw, err := sc.encode()
	if err != nil {
		return chunk{}, err
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	var env bytes.Buffer
	enc := json.NewEncoder(&env)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(envelope{Version: "1", Encoding: "bstring", Compressed: true, Encoded: latin1(z.Bytes())}); err != nil {
		return chunk{}, err
	}
	text, err := toLatin1(strings.TrimSuffix(env.String(), "\n"))
	if err != nil {
		return chunk{}, err
	}
	return chunk{kind: "tEXt", data: append([]byte(sceneKeyword+"\x00"), text...)}, nil
}

// WriteScene returns the picture with sc as its scene, in place of any
// scene it carried, marked stale or not. Other chunks are kept, such as
// the regions of the processes in it (ADR-0142).
func WriteScene(p []byte, sc Scene, stale bool) ([]byte, error) {
	cs, err := chunks(p)
	if err != nil {
		return nil, err
	}
	text, err := sceneChunk(sc)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.Write(pngSignature)
	for _, c := range cs {
		switch {
		case c.kind == staleChunk, c.kind == "tEXt" && bytes.HasPrefix(c.data, []byte(sceneKeyword+"\x00")):
			continue
		case c.kind == "IEND":
			writeChunk(&out, text)
			if stale {
				writeChunk(&out, chunk{kind: staleChunk, data: []byte("1")})
			}
		}
		writeChunk(&out, c)
	}
	return out.Bytes(), nil
}

// Placeholder is the picture of a new whiteboard before anyone has drawn
// on it: white, the size of a small view.
func Placeholder() []byte {
	img := image.NewGray(image.Rect(0, 0, 640, 360))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
