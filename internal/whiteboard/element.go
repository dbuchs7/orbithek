package whiteboard

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"strings"
	"unicode/utf8"
)

// KEY names this installation's data in an element's customData, as for
// the processes in a drawing (ADR-0142): stored data, so it keeps the
// product's former name (ADR-0153).
const KEY = "wikiwikibangbang"

// elementIDRE is the form of an element's or a file's ID. Excalidraw makes
// IDs of letters, digits, "_" and "-"; older scenes and libraries may hold
// a few more harmless characters.
var elementIDRE = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

// ValidElementID reports whether s can name an element of a drawing, as a
// comment's anchor does (ADR-0170).
func ValidElementID(s string) bool { return elementIDRE.MatchString(s) }

// elementTypes are the kinds of element a scene may hold.
var elementTypes = map[string]bool{
	"rectangle": true, "diamond": true, "ellipse": true, "arrow": true, "line": true, "freedraw": true,
	"text": true, "image": true, "frame": true, "magicframe": true, "embeddable": true, "iframe": true,
}

// votable are the kinds one may vote on (ADR-0169): notes and other shapes.
var votable = map[string]bool{"rectangle": true, "diamond": true, "ellipse": true, "image": true}

type binding struct {
	ElementID string `json:"elementId"`
}

type bound struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// meta is what the server reads of an element; the rest it keeps as the
// browser wrote it.
type meta struct {
	ID              string                     `json:"id"`
	Type            string                     `json:"type"`
	Version         float64                    `json:"version"`
	IsDeleted       bool                       `json:"isDeleted"`
	Index           *string                    `json:"index"`
	X               float64                    `json:"x"`
	Y               float64                    `json:"y"`
	Width           float64                    `json:"width"`
	Height          float64                    `json:"height"`
	Text            string                     `json:"text"`
	OriginalText    string                     `json:"originalText"`
	ContainerID     *string                    `json:"containerId"`
	FrameID         *string                    `json:"frameId"`
	Name            *string                    `json:"name"`
	BackgroundColor string                     `json:"backgroundColor"`
	StrokeColor     string                     `json:"strokeColor"`
	FileID          *string                    `json:"fileId"`
	BoundElements   []bound                    `json:"boundElements"`
	StartBinding    *binding                   `json:"startBinding"`
	EndBinding      *binding                   `json:"endBinding"`
	CustomData      map[string]json.RawMessage `json:"customData"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// parseElement reads and checks an element.
func parseElement(raw json.RawMessage) (meta, error) {
	var m meta
	if err := json.Unmarshal(raw, &m); err != nil {
		return meta{}, fmt.Errorf("%w: an element is no object: %v", ErrInvalid, err)
	}
	switch {
	case !elementIDRE.MatchString(m.ID):
		return meta{}, fmt.Errorf("%w: element ID %q", ErrInvalid, m.ID)
	case !elementTypes[m.Type]:
		return meta{}, fmt.Errorf("%w: element %s: kind %q", ErrInvalid, m.ID, m.Type)
	case m.Version < 1 || m.Version != math.Trunc(m.Version) || m.Version > 1<<53:
		return meta{}, fmt.Errorf("%w: element %s: version", ErrInvalid, m.ID)
	}
	return m, nil
}

// isNote reports whether an element is a note (ADR-0169).
func (m meta) isNote() bool {
	var d struct {
		Kind string `json:"kind"`
	}
	return m.CustomData != nil && json.Unmarshal(m.CustomData[KEY], &d) == nil && d.Kind == "note"
}

// textOf returns the text an element shows: its own, its bound text's, or a
// frame's name.
func textOf(m meta, byID func(string) (meta, bool)) string {
	switch {
	case m.Type == "text":
		if m.OriginalText != "" {
			return m.OriginalText
		}
		return m.Text
	case m.Type == "frame" || m.Type == "magicframe":
		return str(m.Name)
	}
	for _, b := range m.BoundElements {
		if b.Type == "text" {
			if t, ok := byID(b.ID); ok && !t.IsDeleted {
				return textOf(t, byID)
			}
		}
	}
	return ""
}

// The fractional indices that order a scene (Excalidraw 0.18): base-62
// keys whose integer part's length is told by its first character. Only
// keys after the last one are needed here.
const digits62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func intLen(head byte) int {
	switch {
	case head >= 'a' && head <= 'z':
		return int(head-'a') + 2
	case head >= 'A' && head <= 'Z':
		return int('Z'-head) + 2
	}
	return 0
}

// validIndex reports whether s is a fractional index.
func validIndex(s string) bool {
	if s == "" || intLen(s[0]) == 0 || len(s) < intLen(s[0]) || s == "A"+strings.Repeat("0", 26) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if strings.IndexByte(digits62, s[i]) < 0 {
			return false
		}
	}
	return len(s) == intLen(s[0]) || s[len(s)-1] != '0'
}

func incrementInt(x string) string {
	head, digs := x[0], []byte(x[1:])
	carry := true
	for i := len(digs) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(digits62, digs[i]) + 1
		if d == len(digits62) {
			digs[i] = '0'
		} else {
			digs[i], carry = digits62[d], false
		}
	}
	if !carry {
		return string(head) + string(digs)
	}
	switch head {
	case 'Z':
		return "a0"
	case 'z':
		return ""
	}
	h := head + 1
	if h > 'a' {
		digs = append(digs, '0')
	} else {
		digs = digs[:len(digs)-1]
	}
	return string(h) + string(digs)
}

func midpointAfter(a string) string {
	da := 0
	if a != "" {
		da = strings.IndexByte(digits62, a[0])
	}
	if len(digits62)-da > 1 {
		return string(digits62[int(math.Round(0.5*float64(da+len(digits62))))])
	}
	return string(digits62[da]) + midpointAfter(a[1:])
}

// indexAfter returns a fractional index after a ("" for the first).
func indexAfter(a string) string {
	if a == "" {
		return "a0"
	}
	ia := a[:intLen(a[0])]
	if i := incrementInt(ia); i != "" {
		return i
	}
	return ia + midpointAfter(a[len(ia):])
}

// patch sets fields of an element and counts it as changed: a new version,
// nonce and time, as Excalidraw does for its own changes.
func patch(raw json.RawMessage, now int64, fields map[string]any) (json.RawMessage, meta, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, meta{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var version float64
	_ = json.Unmarshal(obj["version"], &version)
	fields["version"] = version + 1
	fields["versionNonce"] = rand.Int32()
	fields["updated"] = now
	for k, v := range fields {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, meta{}, err
		}
		obj[k] = b
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, meta{}, err
	}
	m, err := parseElement(out)
	return out, m, err
}

// setIndex writes an element's index without counting it as a change: the
// server orders a scene it loaded, before anyone sees it.
func setIndex(raw json.RawMessage, index string) json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return raw
	}
	obj["index"], _ = json.Marshal(index)
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

// Text is measured by estimate on the server, which has no fonts; a
// browser measures it again (customData's measure, ADR-0168).
const (
	fontSize    = 20
	lineHeight  = 1.25
	charWidth   = 0.6 * fontSize
	textPadding = 5
	// fontFamily is Excalidraw's hand-drawn font, Excalifont.
	fontFamily = 5
)

// wrap breaks text into lines of at most width pixels, by estimate.
func wrap(text string, width float64) []string {
	perLine := max(1, int(width/charWidth))
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, w := range strings.Fields(para) {
			for utf8.RuneCountInString(w) > perLine {
				r := []rune(w)
				if line != "" {
					out, line = append(out, line), ""
				}
				out, w = append(out, string(r[:perLine])), string(r[perLine:])
			}
			switch {
			case line == "":
				line = w
			case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) <= perLine:
				line += " " + w
			default:
				out, line = append(out, line), w
			}
		}
		out = append(out, line)
	}
	return out
}

func longest(lines []string) int {
	n := 0
	for _, l := range lines {
		n = max(n, utf8.RuneCountInString(l))
	}
	return n
}

// base is what every element Excalidraw makes holds.
func base(kind, id string, x, y, w, h float64, index string, now int64) map[string]any {
	return map[string]any{
		"id": id, "type": kind, "x": x, "y": y, "width": w, "height": h, "angle": 0,
		"strokeColor": "#1e1e1e", "backgroundColor": "transparent", "fillStyle": "solid", "strokeWidth": 2,
		"strokeStyle": "solid", "roughness": 1, "opacity": 100, "groupIds": []string{}, "frameId": nil,
		"index": index, "roundness": nil, "seed": rand.Int32N(1 << 30), "version": 1, "versionNonce": rand.Int32(),
		"isDeleted": false, "boundElements": nil, "updated": now, "link": nil, "locked": false,
	}
}

// textElement makes a text, bound to container when one is given, which
// centres it.
func textElement(id, text string, container map[string]any, x, y float64, index string, now int64) map[string]any {
	var lines []string
	var w, h float64
	if container != nil {
		cw := container["width"].(float64) - 2*textPadding
		lines = wrap(text, cw)
		w = math.Min(cw, float64(longest(lines))*charWidth)
		h = float64(len(lines)) * fontSize * lineHeight
		x = container["x"].(float64) + (container["width"].(float64)-w)/2
		y = container["y"].(float64) + (container["height"].(float64)-h)/2
	} else {
		lines = strings.Split(text, "\n")
		w, h = float64(longest(lines))*charWidth, float64(len(lines))*fontSize*lineHeight
	}
	el := base("text", id, x, y, math.Max(w, charWidth), h, index, now)
	el["strokeWidth"] = 1
	el["text"] = strings.Join(lines, "\n")
	el["originalText"] = text
	el["fontSize"] = fontSize
	el["fontFamily"] = fontFamily
	el["textAlign"] = "left"
	el["verticalAlign"] = "top"
	el["autoResize"] = true
	el["lineHeight"] = lineHeight
	el["containerId"] = nil
	el["customData"] = map[string]any{KEY: map[string]any{"measure": true}}
	if container != nil {
		el["textAlign"], el["verticalAlign"], el["containerId"] = "center", "middle", container["id"]
		el["frameId"] = container["frameId"]
	}
	return el
}

// noteColours are the colours of notes, by name (ADR-0169).
var noteColours = map[string]string{
	"yellow": "#ffec99", "green": "#b2f2bb", "blue": "#a5d8ff", "pink": "#fcc2d7", "orange": "#ffd8a8", "violet": "#d0bfff",
}

var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// colour reads a colour by name or as #rrggbb.
func colour(c, def string) (string, error) {
	switch {
	case c == "":
		return def, nil
	case noteColours[c] != "":
		return noteColours[c], nil
	case hexColour.MatchString(c), c == "transparent":
		return c, nil
	}
	return "", fmt.Errorf("%w: colour %q: a name (yellow, green, blue, pink, orange, violet) or #rrggbb", ErrInvalid, c)
}

// box is an element's bounds, for placing arrows.
type box struct{ x, y, w, h float64 }

func (b box) centre() (float64, float64) { return b.x + b.w/2, b.y + b.h/2 }

// edge returns where the line from b's centre towards (tx, ty) leaves b,
// gap pixels outside it.
func (b box) edge(tx, ty, gap float64) (float64, float64) {
	cx, cy := b.centre()
	dx, dy := tx-cx, ty-cy
	if dx == 0 && dy == 0 {
		return cx, cy
	}
	hw, hh := b.w/2+gap, b.h/2+gap
	s := math.Inf(1)
	if dx != 0 {
		s = math.Min(s, hw/math.Abs(dx))
	}
	if dy != 0 {
		s = math.Min(s, hh/math.Abs(dy))
	}
	return cx + dx*s, cy + dy*s
}

// arrowGeometry places an arrow from one box to another.
func arrowGeometry(from, to box) map[string]any {
	tx, ty := to.centre()
	fx, fy := from.centre()
	sx, sy := from.edge(tx, ty, 8)
	ex, ey := to.edge(fx, fy, 8)
	return map[string]any{
		"x": sx, "y": sy, "width": math.Abs(ex - sx), "height": math.Abs(ey - sy),
		"points": [][2]float64{{0, 0}, {ex - sx, ey - sy}},
	}
}
