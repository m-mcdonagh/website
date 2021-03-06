package markup

import (
	"bufio"
	"bytes"
	"io"
	"unicode"
)

type Attributes map[string][]byte

func (a Attributes) Write(w io.Writer) {
	for key, value := range a {
		w.Write([]byte{' '})
		io.WriteString(w, key)
		w.Write([]byte(`="`))
		w.Write(value)
		w.Write([]byte{'"'})
	}
}

type markupType uint8

const (
	blank markupType = iota
	heading
	subheading
	subsubheading
	link
	list
	blockquote
	preformatted
	text
)

var tag = [...][]byte{
	nil,
	[]byte("h1"),
	[]byte("h2"),
	[]byte("h3"),
	[]byte("a"),
	[]byte("li"),
	nil,
	nil,
	[]byte("p"),
}

var surroundTag = [...][]byte{
	nil,
	nil,
	nil,
	nil,
	nil,
	[]byte("ul"),
	[]byte("blockquote"),
	[]byte("pre"),
	nil,
}

type Markup struct {
	Raw []byte
	Attributes
	markupType
	start, end int
}

func (m Markup) Tag() []byte {
	return tag[m.markupType]
}

func (m Markup) SurroundTag() []byte {
	return surroundTag[m.markupType]
}

func (m Markup) Content() []byte {
	if m.markupType == preformatted {
		if bytes.HasPrefix(m.Raw, []byte("```")) {
			return nil
		}
		return m.Raw[m.start:m.end]
	}
	return bytes.TrimSpace(m.Raw[m.start:m.end])
}

func (m Markup) Gemtext(w io.Writer) {
	w.Write(m.Raw[0:m.end])
}

func openTag(w io.Writer, tag []byte, attr Attributes) {
	if tag != nil {
		w.Write([]byte{'<'})
		w.Write(tag)
		attr.Write(w)
		w.Write([]byte{'>'})
	}
}

func closeTag(w io.Writer, tag []byte) {
	if tag != nil {
		w.Write([]byte("</"))
		w.Write(tag)
		w.Write([]byte{'>'})
	}
}

func (m Markup) HTML(w io.Writer) {
	if m.markupType == blank {
		w.Write([]byte("<br />\n"))
		return
	}
	openTag(w, m.Tag(), m.Attributes)
	content := m.Content()
	w.Write(content)
	closeTag(w, m.Tag())
	if content != nil {
		w.Write([]byte{'\n'})
	}
}

type Markups []Markup

func (m Markups) Gemtext(w io.Writer) {
	for _, markup := range m {
		markup.Gemtext(w)
		w.Write([]byte{'\n'})
	}
}

func (m Markups) HTML(w io.Writer) {
	var lastMarkup Markup
	for _, markup := range m {
		if markup.markupType != lastMarkup.markupType {
			if tag := lastMarkup.SurroundTag(); tag != nil {
				closeTag(w, tag)
				w.Write([]byte{'\n'})
			}
			if tag := markup.SurroundTag(); tag != nil {
				openTag(w, tag, nil)
				w.Write([]byte{'\n'})
			}
		}
		markup.HTML(w)
		lastMarkup = markup
	}
	if tag := lastMarkup.SurroundTag(); tag != nil {
		closeTag(w, tag)
		w.Write([]byte{'\n'})
	}
}

func ParseFromGemtext(r io.Reader) (m Markups) {
	scanner := bufio.NewScanner(r)
	pre := false
	for scanner.Scan() {
		markup := ParseGemtextLine(scanner.Bytes())
		if markup.markupType == preformatted {
			pre = !pre
		}
		if pre {
			markup.markupType = preformatted
			markup.start = 0
			markup.end = len(markup.Raw)
		}
		m = append(m, markup)
	}
	return
}

var selectors = []byte{'#', '.'}
var attrKeys = map[byte]string{
	'#': "id",
	'.': "class",
}

func ParseGemtextLine(raw []byte) Markup {
	if len(raw) == 0 {
		return Markup{}
	}
	if bytes.HasPrefix(raw, []byte("```")) {
		return Markup{raw, nil, preformatted, 0, len(raw)}
	}

	var attr Attributes
	end := len(raw)
	for words := bytes.Split(raw, []byte{' '}); len(words) > 0; words = words[:len(words)-1] {
		word := words[len(words)-1]
		if len(word) == 0 {
			end -= 1
			continue
		}
		if len(word) == 1 || !bytes.Contains(selectors, word[0:1]) {
			break
		}
		if attr == nil {
			attr = make(map[string][]byte)
		}
		attr[attrKeys[word[0]]] = word[1:]
		end -= len(word) + 1
	}

	if bytes.HasPrefix(raw, []byte("###")) {
		return Markup{raw, attr, subsubheading, 4, end}
	}
	if bytes.HasPrefix(raw, []byte("##")) {
		return Markup{raw, attr, subheading, 3, end}
	}
	if bytes.HasPrefix(raw, []byte("#")) {
		return Markup{raw, attr, heading, 2, end}
	}
	if bytes.HasPrefix(raw, []byte("=>")) {
		if attr == nil {
			attr = make(Attributes)
		}
		hrefStart, hrefEnd := 0, 0
		for i, char := range string(raw[2:]) {
			if hrefStart == 0 {
				if unicode.IsSpace(char) {
					continue
				}
				hrefStart = i + 2
			} else if unicode.IsSpace(char) {
				hrefEnd = i + 2
			}
		}
		attr["href"] = raw[hrefStart:hrefEnd]
		return Markup{raw, attr, link, hrefEnd, end}
	}
	if bytes.HasPrefix(raw, []byte("* ")) {
		return Markup{raw, attr, list, 2, end}
	}
	if bytes.HasPrefix(raw, []byte(">")) {
		return Markup{raw, attr, blockquote, 1, end}
	}
	return Markup{raw, attr, text, 0, end}
}
