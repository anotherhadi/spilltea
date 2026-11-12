// Package proxy is spilltea's proxy, built on go-mitmproxy. In-scope traffic
// is decrypted, run through the rules, intercepted and stored; everything
// else passes through untouched.
package proxy

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/cert"
	mproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/sirupsen/logrus"

	"github.com/anotherhadi/httpql"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/endpoints"
	"github.com/anotherhadi/spilltea/internal/findings"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/query"
	"github.com/anotherhadi/spilltea/internal/rules"
	"github.com/anotherhadi/spilltea/internal/scan"
	"github.com/anotherhadi/spilltea/internal/scope"
)

type Kind int

const (
	KindRequest Kind = iota
	KindResponse
)

// Item is a request or response held until Decide.
type Item struct {
	Seq    int
	Kind   Kind
	Method string
	URL    string
	Color  string
	Time   time.Time
	// Request is, for a response, the request as it was sent.
	Raw     []byte
	Request []byte

	scheme string
	decide chan Decision
	once   sync.Once
}

type Decision struct {
	Drop bool
	// Gone means the proxy dropped it because the client went away.
	Gone bool
	// Raw replaces the message; nil sends it unchanged.
	Raw []byte
	// HoldResponse also holds the response, even if responses are not intercepted.
	HoldResponse bool
}

type (
	HeldMsg    struct{ Item *Item }
	EntryMsg   struct{ Entry project.Entry }
	FindingMsg struct{}
	ErrorMsg   struct{ Err error }
	// ReleasedMsg: the proxy decided on its own (intercept off, client gone).
	ReleasedMsg struct{ Item *Item }
)

type flow struct {
	start, sent, got time.Time

	req     *message.Request // as sent
	color   string
	asset   bool
	edited  bool
	dropped bool
	hold    bool // hold the response too
	err     string

	resp     *message.Response // as delivered to the client
	streamed bool
}

type Proxy struct {
	mproxy.BaseAddon

	// Addr is the actual address after Start (useful with port 0).
	Addr string

	cfg       *config.Config
	scope     *scope.Scope
	rules     *rules.Rules
	project   *project.Project
	filter    *httpql.Query[*query.Record]
	scanner   *scan.Scanner
	full      atomic.Bool
	untrusted atomic.Bool

	p       *mproxy.Proxy
	events  chan any
	closing chan struct{}
	closed  sync.Once

	holdReq, holdResp atomic.Bool
	seq               atomic.Int64

	mu      sync.Mutex
	pending []*Item
	flows   sync.Map // flow ID to *flow
}

func New(cfg *config.Config, p *project.Project) (*Proxy, error) {
	sc, err := scope.New(cfg.Scope.Include, cfg.Scope.Exclude)
	if err != nil {
		return nil, err
	}
	ru, err := rules.New(cfg)
	if err != nil {
		return nil, err
	}
	filter, err := query.Schema.Parse(cfg.Intercept.Filter)
	if err != nil {
		return nil, fmt.Errorf("intercept.filter: %w", err)
	}
	x := &Proxy{
		cfg: cfg, scope: sc, rules: ru, project: p, filter: filter,
		events:  make(chan any, 4096),
		closing: make(chan struct{}),
	}
	x.holdReq.Store(cfg.Intercept.Requests)
	x.holdResp.Store(cfg.Intercept.Responses)
	var hs []scan.Handler
	ds, err := findings.Detectors(cfg.Findings)
	if err != nil {
		return nil, fmt.Errorf("findings: %w", err)
	}
	if len(ds) > 0 {
		hs = append(hs, findings.NewHandler(p, ds, func(err error) {
			if err != nil {
				x.send(ErrorMsg{fmt.Errorf("findings: %w", err)})
				return
			}
			x.send(FindingMsg{})
		}))
	}
	if cfg.History.JSEndpoints {
		h, err := endpoints.NewHandler(cfg, p, func(e project.Entry) { x.send(EntryMsg{e}) },
			func(err error) { x.send(ErrorMsg{fmt.Errorf("endpoints: %w", err)}) })
		if err != nil {
			return nil, err
		}
		hs = append(hs, h)
	}
	if len(hs) > 0 {
		x.scanner = scan.New(hs...)
	}
	return x, nil
}

// Quiet silences go-mitmproxy's logs, which would break the TUI.
func Quiet() {
	logrus.SetOutput(io.Discard)
	logrus.SetLevel(logrus.PanicLevel)
	// go-mitmproxy's http2 server logs there
	log.SetOutput(io.Discard)
}

type stdLog struct{ x *Proxy }

var untrustedAlerts = [][]byte{[]byte("tls: unknown certificate authority"), []byte("tls: bad certificate")}

func (l stdLog) Write(b []byte) (int, error) {
	untrusted := slices.ContainsFunc(untrustedAlerts, func(a []byte) bool { return bytes.Contains(b, a) })
	if untrusted && !l.x.untrusted.Swap(true) {
		go l.x.send(ErrorMsg{errors.New("a client does not trust spilltea's CA: import the output of `spilltea ca` as an authority")})
	}
	return len(b), nil
}

// Start listens, then serves in the background; later errors arrive as
// ErrorMsg.
func (x *Proxy) Start() error {
	Quiet()
	log.SetOutput(stdLog{x})
	// go-mitmproxy only listens inside Start, which blocks: try the address
	// first so an error (port in use) shows up right away.
	ln, err := net.Listen("tcp", x.cfg.Proxy.Listen)
	if err != nil {
		return err
	}
	x.Addr = ln.Addr().String()
	ln.Close()
	p, err := mproxy.NewProxy(&mproxy.Options{
		Addr:              x.Addr,
		StreamLargeBodies: max(x.cfg.History.MaxBodySize, 5<<20),
		SslInsecure:       x.cfg.Proxy.Insecure,
		NewCaFunc:         func() (cert.CA, error) { return LoadCA(x.cfg.Proxy.CA) },
		Upstream:          x.cfg.Proxy.Upstream,
	})
	if err != nil {
		return err
	}
	p.SetShouldInterceptRule(func(req *http.Request) bool {
		host, port := splitHost(req.Host)
		return x.scope.Host(host, port)
	})
	p.AddAddon(x)
	x.p = p
	go func() {
		if err := p.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			x.send(ErrorMsg{fmt.Errorf("proxy: %w", err)})
		}
	}()
	return nil
}

func splitHost(hostport string) (string, int) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, 443
	}
	n, _ := strconv.Atoi(port)
	return host, n
}

func (x *Proxy) Events() <-chan any { return x.events }

func (x *Proxy) send(ev any) {
	select {
	case x.events <- ev:
	case <-x.closing:
	}
}

func (x *Proxy) Close() error {
	x.closed.Do(func() { close(x.closing) })
	if x.scanner != nil {
		x.scanner.Close()
	}
	if x.p == nil {
		return nil
	}
	return x.p.Close()
}

func (x *Proxy) Holding(k Kind) bool {
	if k == KindRequest {
		return x.holdReq.Load()
	}
	return x.holdResp.Load()
}

// SetHolding: turning the intercept off releases what was held.
func (x *Proxy) SetHolding(k Kind, on bool) {
	if k == KindRequest {
		x.holdReq.Store(on)
	} else {
		x.holdResp.Store(on)
	}
	if on {
		return
	}
	for _, it := range x.Pending() {
		if it.Kind == k {
			x.release(it, Decision{})
		}
	}
}

func (x *Proxy) Pending() []*Item {
	x.mu.Lock()
	defer x.mu.Unlock()
	return slices.Clone(x.pending)
}

// Decide ignores a second decision on the same item.
func (x *Proxy) Decide(it *Item, d Decision) {
	it.once.Do(func() { it.decide <- d })
}

// release is a decision made by the proxy itself; the app is notified.
func (x *Proxy) release(it *Item, d Decision) {
	decided := false
	it.once.Do(func() { it.decide <- d; decided = true })
	if decided {
		x.send(ReleasedMsg{it})
	}
}

// hold waits for the decision; if the client leaves meanwhile, the item is dropped.
func (x *Proxy) hold(it *Item, done <-chan struct{}) Decision {
	it.Seq = int(x.seq.Add(1))
	it.decide = make(chan Decision, 1)
	x.mu.Lock()
	x.pending = append(x.pending, it)
	x.mu.Unlock()
	defer func() {
		x.mu.Lock()
		x.pending = slices.DeleteFunc(x.pending, func(p *Item) bool { return p == it })
		x.mu.Unlock()
	}()
	x.send(HeldMsg{it})
	select {
	case d := <-it.decide:
		return d
	case <-done:
		x.release(it, Decision{Drop: true, Gone: true})
		return <-it.decide // the app may have decided just before
	case <-x.closing:
		return Decision{Drop: true}
	}
}

func clientGone(f *mproxy.Flow) <-chan struct{} {
	if r := f.Request.Raw(); r != nil {
		return r.Context().Done()
	}
	return nil
}

func (x *Proxy) flowOf(f *mproxy.Flow) *flow {
	if v, ok := x.flows.Load(f.Id); ok {
		return v.(*flow)
	}
	return nil
}

func (x *Proxy) isAssetExt(r *message.Request) bool { return x.cfg.Assets.Ext(message.Ext(r.URL)) }

func (x *Proxy) isAssetType(contentType string) bool {
	return x.cfg.Assets.Type(message.Mime(contentType))
}

var dropped = func() *mproxy.Response {
	return &mproxy.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": {"text/plain; charset=utf-8"}, "X-Spilltea": {"dropped"}},
		Body:       []byte("Dropped by spilltea.\n"),
	}
}

func (x *Proxy) Requestheaders(f *mproxy.Flow) {
	color := f.Request.Header.Get(message.PwnFoxHeader)
	if x.cfg.HTTP.StripPwnFox {
		f.Request.Header.Del(message.PwnFoxHeader)
	}
	// CONNECT is only the HTTPS tunnel, not a real request; a WebSocket
	// upgrade is recorded whole by WebSocketEnd instead.
	if f.Request.Method == http.MethodConnect || isWSUpgrade(f.Request.Header) || !x.scope.URL(f.Request.URL) {
		return
	}
	fl := &flow{start: f.StartTime, color: strings.ToLower(strings.TrimSpace(color))}
	x.flows.Store(f.Id, fl)
	go func() {
		<-f.Done()
		x.flows.Delete(f.Id)
		x.record(f, fl)
	}()
}

func toRequest(r *mproxy.Request) *message.Request {
	return &message.Request{Method: r.Method, URL: r.URL, Proto: r.Proto, Header: r.Header, Body: r.Body}
}

func (x *Proxy) Request(f *mproxy.Flow) {
	fl := x.flowOf(f)
	if fl == nil {
		return
	}
	req := toRequest(f.Request)
	req.Header = req.Header.Clone()
	hadBody := len(req.Body) > 0
	changed, err := x.rules.Request(req)
	if err != nil {
		x.send(ErrorMsg{fmt.Errorf("rules on %s: %w", f.Request.URL, err)})
	}
	fl.asset = x.isAssetExt(req)
	if x.holdReq.Load() && !fl.asset && x.passes(req, nil, fl) {
		it := &Item{Kind: KindRequest, Method: req.Method, URL: req.URL.String(), Color: fl.color,
			Time: fl.start, Raw: req.Bytes(), scheme: req.URL.Scheme}
		d := x.hold(it, clientGone(f))
		fl.hold = d.HoldResponse
		switch {
		case d.Drop:
			fl.dropped, fl.req = true, req
			if d.Gone {
				fl.err = "client gone"
			}
			f.Response = dropped()
			return
		case d.Raw != nil && string(d.Raw) != string(it.Raw):
			edited, err := message.ParseRequest(d.Raw, it.scheme)
			if err != nil {
				// the editor already checks; never send a half-edited
				// request
				x.send(ErrorMsg{fmt.Errorf("%s: %w (sent unchanged)", it.URL, err)})
				break
			}
			req, changed, fl.edited = edited, true, true
		}
	}
	if changed {
		message.FixLength(req.Header, req.Body, hadBody)
		f.Request.Method, f.Request.URL, f.Request.Header, f.Request.Body = req.Method, req.URL, req.Header, req.Body
	}
	fl.req = req
	fl.sent = time.Now()
}

// passes applies intercept.filter; resp is nil for a request.
func (x *Proxy) passes(req *message.Request, resp *message.Response, fl *flow) bool {
	r := &query.Record{Color: fl.color, Asset: fl.asset, Edited: fl.edited}
	r.Method, r.Created = req.Method, fl.start
	r.SetURL(req.URL.String())
	raw := string(req.Bytes())
	r.ReqRaw = func() string { return raw }
	if resp != nil {
		r.Code, r.Len, r.Mime = resp.Status, int64(len(resp.Body)), message.Mime(resp.Header.Get("Content-Type"))
		r.Roundtrip = fl.got.Sub(fl.sent).Milliseconds()
		rraw := string(resp.Bytes())
		r.RespRaw = func() string { return rraw }
	}
	return x.filter.Match(r)
}

func (x *Proxy) Responseheaders(f *mproxy.Flow) {
	if fl := x.flowOf(f); fl != nil {
		fl.got = time.Now()
	}
}

func (x *Proxy) Response(f *mproxy.Flow) {
	fl := x.flowOf(f)
	if fl == nil || fl.dropped || f.Response == nil {
		return
	}
	if fl.req == nil {
		// request too big, streamed: the Request hook never ran
		fl.req = toRequest(f.Request)
	}
	enc := f.Response.Header.Get("Content-Encoding")
	body, err := message.Decode(enc, f.Response.Body)
	decoded := err == nil && enc != "" && enc != "identity"
	if err != nil {
		body = f.Response.Body
	}
	resp := &message.Response{Proto: f.Request.Proto, Status: f.Response.StatusCode, Header: f.Response.Header.Clone(), Body: body}
	if decoded {
		// the decompressed body is what gets shown and edited
		resp.Header.Del("Content-Encoding")
		message.FixLength(resp.Header, resp.Body, true)
	}
	changed, err := x.rules.Response(resp)
	if err != nil {
		x.send(ErrorMsg{fmt.Errorf("rules on the response of %s: %w", f.Request.URL, err)})
	}
	fl.asset = fl.asset || x.isAssetType(resp.Header.Get("Content-Type"))
	if (x.holdResp.Load() || fl.hold) && !fl.asset && x.passes(fl.req, resp, fl) {
		it := &Item{Kind: KindResponse, Method: fl.req.Method, URL: fl.req.URL.String(), Color: fl.color,
			Time: fl.start, Raw: resp.Bytes(), Request: fl.req.Bytes()}
		d := x.hold(it, clientGone(f))
		switch {
		case d.Drop:
			fl.dropped = true
			if d.Gone {
				fl.err = "client gone"
			}
			fl.resp = resp
			*f.Response = *dropped()
			return
		case d.Raw != nil && string(d.Raw) != string(it.Raw):
			edited, err := message.ParseResponse(d.Raw)
			if err != nil {
				x.send(ErrorMsg{fmt.Errorf("the response of %s: %w (sent unchanged)", it.URL, err)})
				break
			}
			resp, changed, fl.edited = edited, true, true
		}
	}
	if changed {
		message.FixLength(resp.Header, resp.Body, true)
		f.Response.StatusCode, f.Response.Header, f.Response.Body = resp.Status, resp.Header, resp.Body
	}
	fl.resp = resp
}

func (x *Proxy) RequestError(f *mproxy.Flow, err error) {
	if fl := x.flowOf(f); fl != nil {
		fl.err = err.Error()
	}
}

func isWSUpgrade(h http.Header) bool {
	return strings.Contains(strings.ToLower(h.Get("Connection")), "upgrade") && strings.EqualFold(h.Get("Upgrade"), "websocket")
}

var wsFrameNames = map[int]string{
	websocket.TextMessage:   "text",
	websocket.BinaryMessage: "binary",
	websocket.CloseMessage:  "close",
	websocket.PingMessage:   "ping",
	websocket.PongMessage:   "pong",
}

func wsFrameName(t int) string {
	if n, ok := wsFrameNames[t]; ok {
		return n
	}
	return fmt.Sprintf("opcode %d", t)
}

// wsTranscript renders a WebSocket session as readable text: a metadata
// line per frame, then its content (hex for binary, so a NUL byte never
// makes the whole thing look binary to the findings detectors).
func wsTranscript(msgs []*mproxy.WebSocketMessage) []byte {
	var b bytes.Buffer
	for _, m := range msgs {
		arrow := "<--"
		if m.FromClient {
			arrow = "-->"
		}
		fmt.Fprintf(&b, "%s %s %s %dB\n", arrow, m.Timestamp.Local().Format("15:04:05.000"), wsFrameName(m.Type), len(m.Content))
		if m.Type == websocket.BinaryMessage {
			b.WriteString(hex.EncodeToString(m.Content))
		} else {
			b.Write(m.Content)
		}
		b.WriteString("\n")
	}
	return b.Bytes()
}

// WebSocketEnd records a finished WebSocket connection as a single entry:
// the handshake request, and the session as a transcript in place of a
// response body. go-mitmproxy forwards frames live and has no hold hook for
// them, so unlike requests and responses, a WS connection can only be
// recorded once it closes, not intercepted frame by frame.
func (x *Proxy) WebSocketEnd(f *mproxy.Flow) {
	if f.Request == nil || f.WebSocket == nil || len(f.WebSocket.Messages) == 0 {
		return
	}
	host := f.Request.URL.Host
	if host == "" {
		host = f.Request.Header.Get("Host")
	}
	if host == "" {
		return
	}
	scheme := "ws"
	if f.ConnContext != nil && f.ConnContext.ClientConn.Tls {
		scheme = "wss"
	}
	u := *f.Request.URL
	u.Scheme, u.Host = scheme, host
	if !x.scope.URL(&u) {
		return
	}
	color := strings.ToLower(strings.TrimSpace(f.Request.Header.Get(message.PwnFoxHeader)))
	req := &message.Request{Method: http.MethodGet, URL: &u, Proto: f.Request.Proto, Header: f.Request.Header.Clone()}
	resp := &message.Response{Proto: "HTTP/1.1", Status: http.StatusSwitchingProtocols,
		Header: http.Header{"Upgrade": {"websocket"}, "Connection": {"Upgrade"}},
		Body:   wsTranscript(f.WebSocket.Messages)}
	x.recordWS(f.StartTime, color, req, resp)
}

func (x *Proxy) recordWS(start time.Time, color string, req *message.Request, resp *message.Response) {
	e := project.Entry{Time: start, Method: req.Method, URL: req.URL.String(), Color: color, Mime: "websocket",
		Status: resp.Status, Len: int64(len(resp.Body)), MS: time.Since(start).Milliseconds()}
	kept := project.Kept(x.cfg.History.Responses)
	if kept == project.KeptFull && int64(len(resp.Body)) > x.cfg.History.MaxBodySize {
		kept = project.KeptHeaders
	}
	var raw []byte
	switch kept {
	case project.KeptNone:
		e.Status, e.Len, e.Mime, e.MS = 0, 0, "", 0
	case project.KeptHeaders:
		raw = resp.Head()
	case project.KeptFull:
		raw = resp.Bytes()
	}
	e.Kept = kept
	e, err := x.project.Add(e, req.Bytes(), raw)
	if err != nil {
		x.send(ErrorMsg{fmt.Errorf("history: %w", err)})
		return
	}
	x.send(EntryMsg{e})
	if x.scanner != nil && !x.scanner.Scan(e, req, resp) && !x.full.Swap(true) {
		x.send(ErrorMsg{errors.New("too much traffic, some responses were not scanned")})
	}
}

func (x *Proxy) record(f *mproxy.Flow, fl *flow) {
	req := fl.req
	if req == nil {
		// streamed: the Request hook never ran
		req = toRequest(f.Request)
	}
	e := project.Entry{Time: fl.start, Method: req.Method, URL: req.URL.String(), Color: fl.color,
		Asset: fl.asset, Edited: fl.edited, Error: fl.err}
	if fl.dropped && e.Error == "" {
		e.Error = "dropped"
	}
	resp := fl.resp
	if resp == nil && f.Response != nil && !fl.dropped {
		// streamed or body not read: only the head is known
		resp = &message.Response{Proto: f.Request.Proto, Status: f.Response.StatusCode, Header: f.Response.Header}
		fl.streamed = true
		fl.asset = fl.asset || x.isAssetType(resp.Header.Get("Content-Type"))
		e.Asset = fl.asset
	}
	var raw []byte
	if resp != nil {
		e.Status, e.Len, e.Mime = resp.Status, int64(len(resp.Body)), message.Mime(resp.Header.Get("Content-Type"))
		if !fl.got.IsZero() && !fl.sent.IsZero() {
			e.MS = fl.got.Sub(fl.sent).Milliseconds()
		}
		kept := project.Kept(x.cfg.History.Responses)
		if fl.asset && kept != project.KeptNone {
			kept = project.KeptStatus
		}
		if kept == project.KeptFull && (fl.streamed || int64(len(resp.Body)) > x.cfg.History.MaxBodySize) {
			kept = project.KeptHeaders
		}
		switch kept {
		case project.KeptNone:
			e.Status, e.Len, e.Mime, e.MS = 0, 0, "", 0
		case project.KeptHeaders:
			raw = resp.Head()
		case project.KeptFull:
			raw = resp.Bytes()
		}
		e.Kept = kept
	}
	reqRaw := req.Bytes()
	if int64(len(req.Body)) > x.cfg.History.MaxBodySize {
		head, _ := message.Split(reqRaw)
		reqRaw = append(head, "\r\n\r\n"...)
	}
	e, err := x.project.Add(e, reqRaw, raw)
	if err != nil {
		x.send(ErrorMsg{fmt.Errorf("history: %w", err)})
		return
	}
	x.send(EntryMsg{e})
	if x.scanner != nil && !x.scanner.Scan(e, req, resp) && !x.full.Swap(true) {
		x.send(ErrorMsg{errors.New("too much traffic, some responses were not scanned")})
	}
}
