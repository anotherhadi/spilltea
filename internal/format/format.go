// Package format pretty-prints bodies: JSON like jq, HTML one element per line.
package format

import (
	"bytes"
	"encoding/json"
	"strings"

	"golang.org/x/net/html"
)

// Larger HTML is shown as is, the tokenizer would be too slow.
const maxHTML = 4 << 20

// Body detects HTML from contentType or from the start of the body.
func Body(body []byte, contentType string) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body, false
	}
	if (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid(trimmed) {
		var b bytes.Buffer
		if json.Indent(&b, trimmed, "", "  ") == nil {
			return b.Bytes(), true
		}
	}
	if len(trimmed) <= maxHTML && isHTML(trimmed, contentType) {
		return HTML(trimmed), true
	}
	return body, false
}

func isHTML(body []byte, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "html") {
		return true
	}
	if ct != "" && !strings.HasPrefix(ct, "text/plain") {
		return false
	}
	start := strings.ToLower(string(body[:min(len(body), 64)]))
	return strings.HasPrefix(start, "<!doctype html") || strings.HasPrefix(start, "<html")
}

// Elements without an end tag.
var void = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "param": true,
	"source": true, "track": true, "wbr": true,
}

// Elements whose content is kept verbatim.
var verbatim = map[string]bool{"pre": true, "textarea": true}

type token struct {
	kind html.TokenType
	name string
	raw  string
}

// HTML puts each tag on its own line; an element holding only a short text
// stays on one line. Tags keep their original bytes.
func HTML(body []byte) []byte {
	z := html.NewTokenizer(bytes.NewReader(body))
	var toks []token
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		t := token{kind: tt, raw: string(z.Raw())}
		if tt == html.StartTagToken || tt == html.EndTagToken || tt == html.SelfClosingTagToken {
			name, _ := z.TagName()
			t.name = string(name)
		}
		toks = append(toks, t)
	}

	var b strings.Builder
	depth := 0
	line := func(s string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat("  ", depth))
		b.WriteString(s)
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.kind {
		case html.StartTagToken:
			if verbatim[t.name] {
				s := t.raw
				for i+1 < len(toks) {
					i++
					s += toks[i].raw
					if toks[i].kind == html.EndTagToken && toks[i].name == t.name {
						break
					}
				}
				line(s)
				continue
			}
			if void[t.name] {
				line(t.raw)
				continue
			}
			// <title>text</title> on a single line
			if i+2 < len(toks) && toks[i+1].kind == html.TextToken && toks[i+2].kind == html.EndTagToken &&
				toks[i+2].name == t.name {
				if text := strings.TrimSpace(toks[i+1].raw); len(text) <= 80 && !strings.Contains(text, "\n") {
					line(t.raw + text + toks[i+2].raw)
					i += 2
					continue
				}
			}
			if i+1 < len(toks) && toks[i+1].kind == html.EndTagToken && toks[i+1].name == t.name {
				line(t.raw + toks[i+1].raw)
				i++
				continue
			}
			line(t.raw)
			depth++
		case html.EndTagToken:
			depth = max(depth-1, 0)
			line(t.raw)
		case html.TextToken:
			for _, l := range strings.Split(t.raw, "\n") {
				if l = strings.TrimSpace(l); l != "" {
					line(l)
				}
			}
		default: // self-closing tags, comments, doctype
			line(strings.TrimSpace(t.raw))
		}
	}
	return []byte(b.String())
}
