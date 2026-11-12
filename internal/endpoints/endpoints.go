// Package endpoints adds the paths found in scripts to the history, without
// sending anything.
package endpoints

import (
	"crypto/sha256"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/importer"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
)

const cacheSize = 4096

type Endpoint struct {
	Method string // "" if unknown
	Path   string
}

type Handler struct {
	im     *importer.Importer
	failed func(error)
	cache  map[[sha256.Size]byte][]Endpoint
}

// added and failed are called from the scanner's goroutine.
func NewHandler(cfg *config.Config, p *project.Project, added func(project.Entry), failed func(error)) (*Handler, error) {
	im, err := importer.New(cfg, p, importer.Options{Source: "js", Added: added})
	if err != nil {
		return nil, err
	}
	return &Handler{im: im, failed: failed, cache: map[[sha256.Size]byte][]Endpoint{}}, nil
}

func (h *Handler) Handle(e project.Entry, req *message.Request, resp *message.Response) {
	h.im.Seen(e)
	if req == nil || resp == nil || resp.Status != 200 || len(resp.Body) == 0 || !script(req, resp) {
		return
	}
	sum := sha256.Sum256(resp.Body)
	eps, ok := h.cache[sum]
	if !ok {
		eps = Extract(resp.Body)
		if len(h.cache) >= cacheSize {
			clear(h.cache)
		}
		h.cache[sum] = eps
	}
	// relative paths are fetched from the page, not from the script
	base := req.URL
	if ref, err := url.Parse(req.Header.Get("Referer")); err == nil && ref.Host != "" && (ref.Scheme == "http" || ref.Scheme == "https") {
		base = ref
	}
	for _, ep := range eps {
		u, err := base.Parse(ep.Path)
		if err != nil || u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		u.Fragment = ""
		method := ep.Method
		if method == "" {
			method = "GET"
		}
		if err := h.im.Add(importer.Item{Source: "js", Method: method, URL: u}); err != nil {
			h.failed(err)
		}
	}
}

func script(req *message.Request, resp *message.Response) bool {
	mime := message.Mime(resp.Header.Get("Content-Type"))
	ext := path.Ext(req.URL.Path)
	return strings.Contains(mime, "javascript") || ext == ".js" || ext == ".mjs"
}

func Extract(src []byte) []Endpoint {
	ast, err := js.Parse(parse.NewInputBytes(src), js.Options{})
	if err != nil {
		return nil
	}
	c := &collector{methods: map[string]string{}, modules: map[*js.LiteralExpr]bool{}}
	js.Walk(c, ast)
	var out []Endpoint
	for _, s := range c.found {
		ep := Endpoint{Method: c.methods[s], Path: s}
		if !slices.Contains(out, ep) {
			out = append(out, ep)
		}
	}
	slices.SortFunc(out, func(a, b Endpoint) int { return strings.Compare(a.Path, b.Path) })
	return out
}

type collector struct {
	found   []string
	methods map[string]string
	modules map[*js.LiteralExpr]bool
}

func (c *collector) Enter(n js.INode) js.IVisitor {
	switch n := n.(type) {
	case *js.LiteralExpr:
		if n.TokenType == js.StringToken && !c.modules[n] {
			c.add(unquote(n.Data))
		}
	case *js.TemplateExpr:
		if n.Tag == nil {
			c.add(template(n))
		}
	case *js.CallExpr:
		c.call(n)
	}
	return c
}

func (c *collector) Exit(js.INode) {}

func (c *collector) add(s string) {
	if endpoint(s) {
		c.found = append(c.found, s)
	}
}

var httpMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

func (c *collector) call(n *js.CallExpr) {
	args := n.Args.List
	str := func(i int) string {
		if i >= len(args) {
			return ""
		}
		switch v := args[i].Value.(type) {
		case *js.LiteralExpr:
			if v.TokenType == js.StringToken {
				return unquote(v.Data)
			}
		case *js.TemplateExpr:
			return template(v)
		}
		return ""
	}
	name := ""
	switch x := n.X.(type) {
	case *js.Var:
		name = string(x.Data)
	case *js.DotExpr:
		name = x.Y.String()
	case *js.LiteralExpr:
		name = string(x.Data)
	}
	switch upper := strings.ToUpper(name); {
	case name == "require" || name == "define" || name == "import":
		for _, a := range args {
			c.module(a.Value)
		}
	case slices.Contains(httpMethods, upper):
		c.method(str(0), upper)
	case name == "open":
		if m := strings.ToUpper(str(0)); slices.Contains(httpMethods, m) {
			c.method(str(1), m)
		}
	case name == "fetch" && len(args) > 1:
		if obj, ok := args[1].Value.(*js.ObjectExpr); ok {
			for _, p := range obj.List {
				if p.Name != nil && !p.Name.IsComputed() && string(p.Name.Literal.Data) == "method" {
					if v, ok := p.Value.(*js.LiteralExpr); ok && v.TokenType == js.StringToken {
						c.method(str(0), strings.ToUpper(unquote(v.Data)))
					}
				}
			}
		}
	}
}

func (c *collector) module(e js.IExpr) {
	switch v := e.(type) {
	case *js.LiteralExpr:
		c.modules[v] = true
	case *js.ArrayExpr:
		for _, el := range v.List {
			c.module(el.Value)
		}
	}
}

func (c *collector) method(url, m string) {
	if endpoint(url) && slices.Contains(httpMethods, m) {
		c.methods[url] = m
	}
}

func template(n *js.TemplateExpr) string {
	var b strings.Builder
	for i, p := range n.List {
		v := strings.TrimSuffix(string(p.Value), "${")
		if i == 0 {
			v = strings.TrimPrefix(v, "`")
		} else {
			v = strings.TrimPrefix(v, "}")
		}
		b.WriteString(v + "{}")
	}
	tail := string(n.Tail)
	if len(n.List) > 0 {
		tail = strings.TrimPrefix(tail, "}")
	} else {
		tail = strings.TrimPrefix(tail, "`")
	}
	b.WriteString(strings.TrimSuffix(tail, "`"))
	s := b.String()
	for strings.Contains(s, "{}{}") {
		s = strings.ReplaceAll(s, "{}{}", "{}")
	}
	// `${apiBase}/users`
	return strings.TrimPrefix(s, "{}")
}

func unquote(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	s := string(b[1 : len(b)-1])
	if !strings.Contains(s, `\`) {
		return s
	}
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			out.WriteByte(s[i])
			continue
		}
		i++
		switch {
		case s[i] == 'x' && i+2 < len(s):
			if r, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				out.WriteRune(rune(r))
				i += 2
				continue
			}
		case s[i] == 'u' && i+4 < len(s) && s[i+1] != '{':
			if r, err := strconv.ParseUint(s[i+1:i+5], 16, 16); err == nil {
				out.WriteRune(rune(r))
				i += 4
				continue
			}
		case s[i] == 'n' || s[i] == 't' || s[i] == 'r':
			out.WriteByte(' ')
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

var (
	relative   = regexp.MustCompile(`^/?[\w\-.~%{}:@]+(/[\w\-.~%{}:@]*)*(\?[\w\-.~%{}=&\[\]]*)?$`)
	mimeTypes  = []string{"text/", "application/", "image/", "audio/", "video/", "font/", "multipart/", "model/", "message/"}
	assetExts  = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".avif", ".ico", ".css", ".woff", ".woff2", ".ttf", ".eot", ".otf", ".mp4", ".webm", ".mp3", ".js", ".mjs", ".map", ".ts", ".tsx", ".jsx"}
	noiseHosts = []string{"www.w3.org", "reactjs.org", "react.dev", "fb.me", "github.com", "mozilla.org", "developer.mozilla.org", "schema.org"}
)

func endpoint(s string) bool {
	if len(s) < 3 || len(s) > 300 || !strings.Contains(s, "/") || !strings.ContainsFunc(s, isLetter) {
		return false
	}
	if strings.HasPrefix(s, "//") {
		host, _, _ := strings.Cut(s[2:], "/")
		return strings.Contains(host, ".") && endpoint("https:"+s)
	}
	if rest, ok := strings.CutPrefix(s, "https://"); ok || strings.HasPrefix(s, "http://") {
		if !ok {
			rest = strings.TrimPrefix(s, "http://")
		}
		host, _, _ := strings.Cut(rest, "/")
		return host != "" && !strings.ContainsAny(rest, " \"'<>\\") && !slices.ContainsFunc(noiseHosts, func(h string) bool {
			return host == h || strings.HasSuffix(host, "."+h)
		}) && !asset(rest)
	}
	if !relative.MatchString(s) || strings.HasPrefix(s, ".") || strings.Contains(s, "://") ||
		strings.HasPrefix(s, "/_next/") || strings.Contains(s, "node_modules") || asset(s) {
		return false
	}
	// JSON pointers
	if strings.Contains(s, "~1") {
		return false
	}
	// and/or, MM/DD/YYYY
	if !strings.HasPrefix(s, "/") && (strings.Count(s, "/") < 2 || !strings.ContainsFunc(s, unicode.IsLower)) {
		return false
	}
	lower := strings.ToLower(s)
	return !slices.ContainsFunc(mimeTypes, func(m string) bool { return strings.HasPrefix(lower, m) })
}

func asset(s string) bool {
	p, _, _ := strings.Cut(s, "?")
	ext := strings.ToLower(path.Ext(p))
	return slices.Contains(assetExts, ext)
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
