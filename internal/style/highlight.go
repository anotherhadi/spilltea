package style

import (
	"bytes"
	"encoding/json"
	"strings"

	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/config"
	"golang.org/x/net/html"
	"image/color"
)

func Paint(c color.Color, s string) string {
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

func HighlightHTTP(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = strings.ReplaceAll(raw, "\t", "    ")
	idx := strings.Index(raw, "\n\n")
	if idx == -1 {
		return highlightHeaders(raw)
	}
	headers := raw[:idx+2]
	body := raw[idx+2:]
	result := highlightHeaders(headers)
	if body == "" {
		return result
	}
	pretty := config.Global != nil && config.Global.TUI.PrettyPrintBody
	switch detectBodyType(headers) {
	case "json":
		if pretty {
			body = prettyJSON(body)
		}
		result += highlightJSON(body)
	case "html":
		if pretty {
			body = prettyHTML(body)
		}
		result += highlightHTML(body)
	default:
		result += body
	}
	return result
}

func detectBodyType(headers string) string {
	for _, line := range strings.Split(headers, "\n") {
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "content-type:") {
			continue
		}
		ct := strings.ToLower(strings.TrimSpace(line[len("content-type:"):]))
		switch {
		case strings.Contains(ct, "json"):
			return "json"
		case strings.Contains(ct, "html"):
			return "html"
		}
		break
	}
	return ""
}

func highlightHeaders(raw string) string {
	var out strings.Builder
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if i == 0 {
			out.WriteString(highlightStatusLine(trimmed))
		} else if trimmed == "" {
			out.WriteString(line)
		} else if idx := strings.Index(trimmed, ": "); idx != -1 {
			out.WriteString(Paint(ilovetui.S.Subtle, trimmed[:idx+2]))
			out.WriteString(Paint(ilovetui.S.Text, trimmed[idx+2:]))
		} else {
			out.WriteString(line)
		}
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func highlightStatusLine(line string) string {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return line
	}
	switch parts[0] {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		result := S.Method(parts[0]).Width(0).Render(parts[0]) + " "
		result += Paint(ilovetui.S.Primary, parts[1])
		if len(parts) == 3 {
			result += " " + Paint(ilovetui.S.Subtle, parts[2])
		}
		return result
	}
	result := Paint(ilovetui.S.Subtle, parts[0]) + " "
	result += Paint(ilovetui.S.Warning, parts[1])
	if len(parts) == 3 {
		result += " " + Paint(ilovetui.S.Muted, parts[2])
	}
	return result
}

func highlightJSON(s string) string {
	var out strings.Builder
	i, n := 0, len(s)
	for i < n {
		ch := s[i]
		switch {
		case ch == '"':
			j := i + 1
			for j < n {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if s[j] == '"' {
					j++
					break
				}
				j++
			}
			str := s[i:j]
			k := j
			for k < n && (s[k] == ' ' || s[k] == '\t') {
				k++
			}
			if k < n && s[k] == ':' {
				out.WriteString(Paint(ilovetui.S.Primary, str))
			} else {
				out.WriteString(Paint(ilovetui.S.Success, str))
			}
			i = j
		case (ch >= '0' && ch <= '9') || (ch == '-' && i+1 < n && s[i+1] >= '0' && s[i+1] <= '9'):
			j := i
			if s[j] == '-' {
				j++
			}
			for j < n && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.' || s[j] == 'e' || s[j] == 'E' || s[j] == '+' || s[j] == '-') {
				j++
			}
			out.WriteString(Paint(ilovetui.S.Warning, s[i:j]))
			i = j
		case i+4 <= n && s[i:i+4] == "true":
			out.WriteString(Paint(ilovetui.S.Error, "true"))
			i += 4
		case i+5 <= n && s[i:i+5] == "false":
			out.WriteString(Paint(ilovetui.S.Error, "false"))
			i += 5
		case i+4 <= n && s[i:i+4] == "null":
			out.WriteString(Paint(ilovetui.S.Error, "null"))
			i += 4
		case ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ':' || ch == ',':
			out.WriteString(Paint(ilovetui.S.Subtle, string(ch)))
			i++
		default:
			out.WriteByte(ch)
			i++
		}
	}
	return out.String()
}

func prettyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(strings.TrimSpace(s)), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

var voidHTMLElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

func prettyHTML(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	var buf strings.Builder
	walkHTMLNode(&buf, doc, 0)
	return strings.TrimRight(buf.String(), "\n")
}

func walkHTMLNode(w *strings.Builder, n *html.Node, depth int) {
	indent := strings.Repeat("  ", depth)
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkHTMLNode(w, c, depth)
		}
	case html.DoctypeNode:
		w.WriteString("<!DOCTYPE " + n.Data + ">\n")
	case html.CommentNode:
		w.WriteString(indent + "<!--" + n.Data + "-->\n")
	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text != "" {
			w.WriteString(indent + text + "\n")
		}
	case html.ElementNode:
		tag := buildHTMLOpenTag(n)
		if voidHTMLElements[n.Data] {
			w.WriteString(indent + tag + "\n")
			return
		}
		w.WriteString(indent + tag + "\n")
		if n.Data == "script" || n.Data == "style" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					text := strings.TrimSpace(c.Data)
					if text != "" {
						for _, line := range strings.Split(text, "\n") {
							w.WriteString(indent + "  " + line + "\n")
						}
					}
				}
			}
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walkHTMLNode(w, c, depth+1)
			}
		}
		w.WriteString(indent + "</" + n.Data + ">\n")
	}
}

func buildHTMLOpenTag(n *html.Node) string {
	var sb strings.Builder
	sb.WriteString("<" + n.Data)
	for _, attr := range n.Attr {
		sb.WriteString(" ")
		if attr.Namespace != "" {
			sb.WriteString(attr.Namespace + ":")
		}
		sb.WriteString(attr.Key + `="` + escapeHTMLAttr(attr.Val) + `"`)
	}
	sb.WriteString(">")
	return sb.String()
}

func escapeHTMLAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func highlightHTML(s string) string {
	var out strings.Builder
	i, n := 0, len(s)
	for i < n {
		if i+4 <= n && s[i:i+4] == "<!--" {
			end := strings.Index(s[i:], "-->")
			if end == -1 {
				out.WriteString(Paint(ilovetui.S.Subtle, s[i:]))
				break
			}
			end = i + end + 3
			out.WriteString(Paint(ilovetui.S.Subtle, s[i:end]))
			i = end
			continue
		}
		if s[i] != '<' {
			out.WriteByte(s[i])
			i++
			continue
		}
		out.WriteString(Paint(ilovetui.S.Subtle, "<"))
		i++
		if i < n && (s[i] == '/' || s[i] == '!') {
			out.WriteString(Paint(ilovetui.S.Subtle, string(s[i])))
			i++
		}
		j := i
		for j < n && s[j] != ' ' && s[j] != '>' && s[j] != '/' && s[j] != '\t' && s[j] != '\n' && s[j] != '\r' {
			j++
		}
		if j > i {
			out.WriteString(Paint(ilovetui.S.Primary, s[i:j]))
			i = j
		}
		for i < n && s[i] != '>' {
			ch := s[i]
			switch {
			case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
				out.WriteByte(ch)
				i++
			case ch == '/':
				out.WriteString(Paint(ilovetui.S.Subtle, "/"))
				i++
			case ch == '=':
				out.WriteString(Paint(ilovetui.S.Subtle, "="))
				i++
			case ch == '"' || ch == '\'':
				q := ch
				j = i + 1
				for j < n && s[j] != q {
					j++
				}
				if j < n {
					j++
				}
				out.WriteString(Paint(ilovetui.S.Success, s[i:j]))
				i = j
			default:
				j = i
				for j < n && s[j] != '=' && s[j] != ' ' && s[j] != '>' && s[j] != '/' && s[j] != '\t' && s[j] != '\n' {
					j++
				}
				out.WriteString(Paint(ilovetui.S.Warning, s[i:j]))
				i = j
			}
		}
		if i < n && s[i] == '>' {
			out.WriteString(Paint(ilovetui.S.Subtle, ">"))
			i++
		}
	}
	return out.String()
}
