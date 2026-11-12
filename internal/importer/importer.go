// Package importer adds URLs found by other tools to the history, one per
// line: plain URLs, or JSON lines from httpx (-json), katana (-jsonl) and
// urlfinder (-j). Nothing is sent: we store whatever request and response the
// tool recorded, or a bare GET with no response.
package importer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/scope"
)

type Item struct {
	Source string
	Time   time.Time // zero if unknown
	Method string
	URL    *url.URL
	// A nil Request means a plain GET of URL.
	Request  *message.Request
	Response *message.Response
	// What the tool reports about the response when it doesn't include it.
	Status int
	Mime   string
	Len    int64
}

func (it Item) rich() bool { return it.Response != nil }

// Parse reads a URL (the first word, so "url [200]" works) or a JSON object
// from httpx, katana or urlfinder.
func Parse(line []byte) (Item, error) {
	line = bytes.TrimSpace(line)
	if len(line) > 0 && line[0] == '{' {
		return parseJSON(line)
	}
	f := strings.Fields(string(line))
	if len(f) == 0 {
		return Item{}, errors.New("empty line")
	}
	u, err := parseURL(f[0])
	return Item{Source: "import", Method: http.MethodGet, URL: u}, err
}

// A host without a scheme needs a dot or a port, otherwise any lone word in a
// text output would be taken for a host.
var bareHost = regexp.MustCompile(`^([A-Za-z0-9-]+\.[A-Za-z0-9.-]+(:[0-9]+)?|[A-Za-z0-9.-]+:[0-9]+|localhost)$`)

// parseURL assumes https when the scheme is missing.
func parseURL(s string) (*url.URL, error) {
	if !strings.Contains(s, "://") {
		host, _, _ := strings.Cut(s, "/")
		host, _, _ = strings.Cut(host, "?")
		if !bareHost.MatchString(host) {
			return nil, fmt.Errorf("%s: not a URL", s)
		}
		s = "https://" + s
	}
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return nil, err
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, fmt.Errorf("%s: not http or https", s)
	case u.Host == "":
		return nil, fmt.Errorf("%s: no host", s)
	}
	u.Fragment, u.RawFragment = "", ""
	return u, nil
}

// line merges the fields of all three tools.
type line struct {
	Timestamp string          `json:"timestamp"`
	URL       string          `json:"url"`
	Method    string          `json:"method"`
	Source    json.RawMessage `json:"source"` // urlfinder: where the URL was found
	// httpx
	StatusCode    int             `json:"status_code"`
	ContentType   string          `json:"content_type"`
	ContentLength int64           `json:"content_length"`
	Webserver     string          `json:"webserver"`
	Request       json.RawMessage `json:"request"`  // httpx -irr : texte brut ; katana : un objet
	Response      json.RawMessage `json:"response"` // idem
}

type katanaRequest struct {
	Method   string            `json:"method"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
	Raw      string            `json:"raw"`
}

type katanaResponse struct {
	StatusCode    int                        `json:"status_code"`
	Headers       map[string]json.RawMessage `json:"headers"`
	Body          string                     `json:"body"`
	ContentLength int64                      `json:"content_length"`
	Raw           string                     `json:"raw"`
}

func parseJSON(b []byte) (Item, error) {
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return Item{}, err
	}
	it := Item{Source: "import", Method: http.MethodGet}
	if t, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
		it.Time = t
	}
	rawURL := l.URL
	var source string
	_ = json.Unmarshal(l.Source, &source)
	isObject := func(m json.RawMessage) bool { return len(m) > 0 && m[0] == '{' }
	switch {
	case isObject(l.Request):
		it.Source = "katana"
		var kr katanaRequest
		var kp katanaResponse
		if err := json.Unmarshal(l.Request, &kr); err != nil {
			return Item{}, fmt.Errorf("katana request: %w", err)
		}
		if isObject(l.Response) {
			if err := json.Unmarshal(l.Response, &kp); err != nil {
				return Item{}, fmt.Errorf("katana response: %w", err)
			}
		}
		rawURL = kr.Endpoint
		if kr.Method != "" {
			it.Method = strings.ToUpper(kr.Method)
		}
		u, err := parseURL(rawURL)
		if err != nil {
			return Item{}, err
		}
		it.URL = u
		it.Request = request(kr.Raw, u)
		if it.Request == nil && (len(kr.Headers) > 0 || kr.Body != "") {
			h := http.Header{}
			for k, v := range kr.Headers {
				h.Set(headerName(k), v)
			}
			it.Request = &message.Request{Method: it.Method, URL: u, Header: h, Body: []byte(kr.Body)}
		}
		it.Response = response(kp.Raw)
		if it.Response == nil && kp.StatusCode > 0 && (len(kp.Headers) > 0 || kp.Body != "") {
			it.Response = &message.Response{Status: kp.StatusCode, Header: katanaHeaders(kp.Headers), Body: []byte(kp.Body)}
		}
		it.Status, it.Len = kp.StatusCode, kp.ContentLength
		return it, nil
	case source != "" && l.StatusCode == 0:
		it.Source = "urlfinder"
	case l.StatusCode != 0 || l.ContentType != "" || l.Webserver != "" || len(l.Request) > 0:
		it.Source = "httpx"
	}
	if rawURL == "" {
		return Item{}, errors.New("no url")
	}
	if l.Method != "" {
		it.Method = strings.ToUpper(l.Method)
	}
	u, err := parseURL(rawURL)
	if err != nil {
		return Item{}, err
	}
	it.URL = u
	var raw string
	if json.Unmarshal(l.Request, &raw) == nil {
		it.Request = request(raw, u)
	}
	if raw = ""; json.Unmarshal(l.Response, &raw) == nil {
		it.Response = response(raw)
	}
	it.Status, it.Mime, it.Len = l.StatusCode, message.Mime(l.ContentType), l.ContentLength
	return it, nil
}

// request returns nil if the request is missing or unreadable.
func request(raw string, u *url.URL) *message.Request {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	r, err := message.ParseRequest([]byte(raw), u.Scheme)
	if err != nil {
		return nil
	}
	return r
}

// response normalizes the response the way the proxy stores it: unchunked
// and decompressed.
func response(raw string) *message.Response {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	r, err := message.ParseResponse([]byte(raw))
	if err != nil {
		return nil
	}
	fix := false
	if strings.EqualFold(r.Header.Get("Transfer-Encoding"), "chunked") {
		if b, err := io.ReadAll(httputil.NewChunkedReader(bytes.NewReader(r.Body))); err == nil {
			r.Body, fix = b, true
		}
	}
	if enc := r.Header.Get("Content-Encoding"); enc != "" && enc != "identity" {
		if b, err := message.Decode(enc, r.Body); err == nil {
			r.Body, fix = b, true
			r.Header.Del("Content-Encoding")
		}
	}
	if fix {
		message.FixLength(r.Header, r.Body, true)
	}
	return r
}

// headerName undoes the snake_case some tools use: content_type becomes
// Content-Type.
func headerName(k string) string {
	if !strings.Contains(k, "-") {
		k = strings.ReplaceAll(k, "_", "-")
	}
	return textproto.CanonicalMIMEHeaderKey(k)
}

// katanaHeaders: katana gives either a string or a list depending on the header.
func katanaHeaders(in map[string]json.RawMessage) http.Header {
	h := http.Header{}
	for k, raw := range in {
		var one string
		var many []string
		switch {
		case json.Unmarshal(raw, &one) == nil:
			h.Add(headerName(k), one)
		case json.Unmarshal(raw, &many) == nil:
			for _, v := range many {
				h.Add(headerName(k), v)
			}
		}
	}
	return h
}

type Options struct {
	// Source overrides the tool guessed from the lines.
	Source string
	// All also imports out-of-scope URLs.
	All bool
	// Added is called after each entry is stored.
	Added func(project.Entry)
}

type Stats struct {
	Added      int
	Duplicates int // already in the history, nothing new
	OutOfScope int
	Invalid    int // lines without a URL
}

// Importer only adds what is in scope and either new or more complete than
// what the history has (a response where there was none).
type Importer struct {
	cfg   *config.Config
	proj  *project.Project
	scope *scope.Scope
	opt   Options
	// keyed by "METHOD URL"; true if the history already has a response
	seen  map[string]bool
	Stats Stats
}

func New(cfg *config.Config, proj *project.Project, opt Options) (*Importer, error) {
	sc, err := scope.New(cfg.Scope.Include, cfg.Scope.Exclude)
	if err != nil {
		return nil, err
	}
	es, _, err := proj.Load()
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	im := &Importer{cfg: cfg, proj: proj, scope: sc, opt: opt, seen: map[string]bool{}}
	for _, e := range es {
		im.Seen(e)
	}
	return im, nil
}

// Seen tells about an entry stored by someone else.
func (im *Importer) Seen(e project.Entry) {
	k := e.Method + " " + e.URL
	im.seen[k] = im.seen[k] || e.Kept == project.KeptFull || e.Kept == project.KeptHeaders
}

// Read counts unreadable lines instead of failing.
func (im *Importer) Read(r io.Reader) error {
	br := bufio.NewReader(r)
	for {
		// katana lines hold whole bodies, so no length limit: ReadBytes rather
		// than a Scanner
		l, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(l)) > 0 {
			it, perr := Parse(l)
			if perr != nil {
				im.Stats.Invalid++
			} else if aerr := im.Add(it); aerr != nil {
				return aerr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (im *Importer) Add(it Item) error {
	if !im.opt.All && !im.scope.URL(it.URL) {
		im.Stats.OutOfScope++
		return nil
	}
	e, req, resp := im.entry(it)
	k := e.Method + " " + e.URL
	rich := resp != nil && (e.Kept == project.KeptFull || e.Kept == project.KeptHeaders)
	if had, ok := im.seen[k]; ok && (had || !rich) {
		im.Stats.Duplicates++
		return nil
	}
	e, err := im.proj.Add(e, req, resp)
	if err != nil {
		return err
	}
	im.seen[k] = rich
	im.Stats.Added++
	if im.opt.Added != nil {
		im.opt.Added(e)
	}
	return nil
}

// entry follows the same rules as the proxy: history.responses, and bodies
// over history.maxBodySize are dropped.
func (im *Importer) entry(it Item) (project.Entry, []byte, []byte) {
	req := it.Request
	if req == nil {
		req = &message.Request{Method: it.Method, URL: it.URL, Header: http.Header{}}
	}
	e := project.Entry{Time: it.Time, Method: req.Method, URL: it.URL.String(), Source: it.Source,
		Status: it.Status, Mime: it.Mime, Len: it.Len}
	if im.opt.Source != "" {
		e.Source = im.opt.Source
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	maxBody := im.cfg.History.MaxBodySize
	reqRaw := req.Bytes()
	if int64(len(req.Body)) > maxBody {
		head, _ := message.Split(reqRaw)
		reqRaw = append(head, "\r\n\r\n"...)
	}
	resp := it.Response
	if resp != nil {
		e.Status, e.Len, e.Mime = resp.Status, int64(len(resp.Body)), message.Mime(resp.Header.Get("Content-Type"))
	}
	e.Asset = im.cfg.Assets.Ext(message.Ext(it.URL)) || im.cfg.Assets.Type(e.Mime)
	if e.Status == 0 {
		return e, reqRaw, nil
	}
	kept := project.Kept(im.cfg.History.Responses)
	if kept != project.KeptNone && (e.Asset || resp == nil) {
		kept = project.KeptStatus
	}
	if kept == project.KeptFull && int64(len(resp.Body)) > maxBody {
		kept = project.KeptHeaders
	}
	var raw []byte
	switch kept {
	case project.KeptNone:
		e.Status, e.Len, e.Mime = 0, 0, ""
	case project.KeptHeaders:
		raw = resp.Head()
	case project.KeptFull:
		raw = resp.Bytes()
	}
	e.Kept = kept
	return e, reqRaw, raw
}
