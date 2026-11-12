package noteimport

import (
	"path"
	"regexp"
	"strings"
	"unicode"
)

// wikiScheme marks a target written as a wiki link, [[Note]] or
// ![[picture.png]], once the link is CommonMark: the resolvers of the
// plan find the note or file it names.
const wikiScheme = "wiki:"

// pictureExts are the files a note shows as pictures; the attachment keeps
// only those a page shows inline (ADR-0045).
var pictureExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".bmp": true, ".avif": true}

// prepared is a note's Markdown with its wiki links written as CommonMark
// links, and what the note says beyond what CommonMark reads.
type prepared struct {
	text string
	// tags are the inline #tags, in the order they occur.
	tags []string
	// embeds are the targets of ![[…]] that are neither pictures nor
	// notes, such as a PDF or a recording; notes are the embedded notes,
	// which become mentions.
	embeds, notes []string
}

var (
	wikiRE = regexp.MustCompile(`(!?)\[\[([^\[\]\n]+?)\]\]`)
	// tagRE finds an inline tag: # and letters, digits, _, - and / after
	// a space or the line's start, not only digits.
	tagRE = regexp.MustCompile(`(^|[\s(])#([\p{L}\p{N}_][\p{L}\p{N}_/-]*)`)
	// blockIDRE is a block reference's anchor at a line's end, "^abc1".
	blockIDRE = regexp.MustCompile(`\s\^[A-Za-z0-9-]+\s*$`)
	// commentRE is a comment of the notes application, %% … %%.
	commentRE = regexp.MustCompile(`%%[^\n]*?%%`)
	fenceRE   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// prepare rewrites the wiki links of a note's Markdown as CommonMark
// links and collects its tags, outside code: [[Note]], [[Note|alias]] and
// [[Note#Heading]] become links to the target wiki:Note, ![[picture.png]]
// a picture of wiki:picture.png, ![[Note]] a link to the note.
func prepare(md string) prepared {
	var p prepared
	lines := strings.Split(md, "\n")
	fence := ""
	for i, line := range lines {
		if m := fenceRE.FindStringSubmatch(line); m != nil {
			switch {
			case fence == "":
				fence = m[1]
			case strings.HasPrefix(m[1], fence[:1]) && len(m[1]) >= len(fence):
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		lines[i] = p.line(line)
	}
	p.text = strings.Join(lines, "\n")
	return p
}

// line rewrites one line outside fenced code, leaving code spans as they
// are.
func (p *prepared) line(line string) string {
	line = blockIDRE.ReplaceAllString(line, "")
	var b strings.Builder
	for line != "" {
		i := strings.IndexByte(line, '`')
		if i < 0 {
			b.WriteString(p.prose(line))
			break
		}
		b.WriteString(p.prose(line[:i]))
		run := len(line[i:]) - len(strings.TrimLeft(line[i:], "`"))
		ticks := line[i : i+run]
		rest := line[i+run:]
		end := strings.Index(rest, ticks)
		if end < 0 {
			b.WriteString(ticks)
			line = rest
			continue
		}
		b.WriteString(ticks + rest[:end] + ticks)
		line = rest[end+run:]
	}
	return b.String()
}

// prose rewrites text that is not code.
func (p *prepared) prose(s string) string {
	s = commentRE.ReplaceAllString(s, "")
	for _, m := range tagRE.FindAllStringSubmatch(s, -1) {
		tag := strings.Trim(m[2], "/-")
		if strings.IndexFunc(tag, func(r rune) bool { return !unicode.IsDigit(r) }) >= 0 {
			p.tags = append(p.tags, tag)
		}
	}
	return wikiRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := wikiRE.FindStringSubmatch(m)
		embed, inner := sub[1] == "!", sub[2]
		target, alias, hasAlias := strings.Cut(inner, "|")
		target = strings.TrimSpace(target)
		name, heading, _ := strings.Cut(target, "#")
		if target == "" {
			return m
		}
		ext := strings.ToLower(path.Ext(name))
		switch {
		case embed && pictureExts[ext]:
			// A picture's alias is its size, e.g. ![[a.png|300]].
			return "![" + escapeText(path.Base(name)) + "](<" + escapeDest(wikiScheme+name) + ">)"
		case embed && ext != "" && ext != ".md":
			p.embeds = append(p.embeds, name)
			return escapeText(path.Base(name))
		case embed:
			p.notes = append(p.notes, name)
		}
		label := strings.TrimSpace(alias)
		if !hasAlias || label == "" {
			base := strings.TrimSuffix(path.Base(name), ".md")
			switch {
			case name == "":
				label = strings.TrimPrefix(heading, "^")
			case heading != "" && !strings.HasPrefix(heading, "^"):
				label = base + " › " + heading
			default:
				label = base
			}
		}
		return "[" + escapeText(label) + "](<" + escapeDest(wikiScheme+target) + ">)"
	})
}

// escapeText escapes the ASCII punctuation of a link's text, so an alias
// is shown as written.
func escapeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 && unicode.IsPunct(r) || r == '`' || r == '<' || r == '>' || r == '|' || r == '~' || r == '^' || r == '=' || r == '+' || r == '$' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escapeDest escapes what a destination in angle brackets may not hold.
func escapeDest(s string) string {
	return strings.NewReplacer(`\`, `\\`, "<", `\<`, ">", `\>`).Replace(s)
}
