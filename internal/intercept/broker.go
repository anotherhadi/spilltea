package intercept

import (
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

type Decision int

const (
	Forward  Decision = iota // forward without showing in intercept
	Drop                     // drop the flow
	Intercept                // pass to the TUI for user decision
)

type PendingRequest struct {
	Flow      *proxy.Flow
	decision  chan Decision
	ArrivedAt time.Time
}

type PendingResponse struct {
	Flow      *proxy.Flow
	decision  chan Decision
	ArrivedAt time.Time
}

type Broker struct {
	Incoming         chan *PendingRequest
	IncomingResponse chan *PendingResponse
	captureResponse  atomic.Bool

	dbMu         sync.RWMutex
	database     *db.DB
	droppedFlows sync.Map // *proxy.Flow → struct{}
	outOfScope   sync.Map // *proxy.Flow → struct{}

	scopeMu   sync.RWMutex
	whitelist []*regexp.Regexp
	blacklist []*regexp.Regexp

	onNewEntry func(db.Entry)
}

func (b *Broker) SetOnNewEntry(cb func(db.Entry)) {
	b.onNewEntry = cb
}

// IsInScope reports whether the given target string (host+path) matches the
// current scope rules. Used by the plugin API.
func (b *Broker) IsInScope(target string) bool {
	b.scopeMu.RLock()
	wl := b.whitelist
	bl := b.blacklist
	b.scopeMu.RUnlock()
	return scopeMatches(wl, bl, target)
}

func NewBroker() *Broker {
	return &Broker{
		Incoming:         make(chan *PendingRequest, 64),
		IncomingResponse: make(chan *PendingResponse, 64),
	}
}

func (b *Broker) SetCaptureResponse(v bool) {
	b.captureResponse.Store(v)
}

// SetScope compiles and stores whitelist/blacklist regex patterns.
// Invalid patterns are silently skipped.
func (b *Broker) SetScope(whitelist, blacklist []string) {
	wl := compilePatterns(whitelist)
	bl := compilePatterns(blacklist)
	b.scopeMu.Lock()
	b.whitelist = wl
	b.blacklist = bl
	b.scopeMu.Unlock()
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if r, err := regexp.Compile(p); err == nil {
			out = append(out, r)
		}
	}
	return out
}

func (b *Broker) matchesScope(f *proxy.Flow) bool {
	target := f.Request.URL.Host + f.Request.URL.Path
	b.scopeMu.RLock()
	wl := b.whitelist
	bl := b.blacklist
	b.scopeMu.RUnlock()
	return scopeMatches(wl, bl, target)
}

func scopeMatches(wl, bl []*regexp.Regexp, target string) bool {
	if len(wl) > 0 {
		matched := false
		for _, r := range wl {
			if r.MatchString(target) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, r := range bl {
		if r.MatchString(target) {
			return false
		}
	}
	return true
}

func (b *Broker) SetDB(d *db.DB) {
	b.dbMu.Lock()
	b.database = d
	b.dbMu.Unlock()
}

// Hold is called from the proxy addon: it blocks until a decision is made in the TUI.
func (b *Broker) Hold(f *proxy.Flow) Decision {
	if !b.matchesScope(f) {
		b.outOfScope.Store(f, struct{}{})
		return Forward
	}
	p := &PendingRequest{
		Flow:      f,
		decision:  make(chan Decision, 1),
		ArrivedAt: time.Now(),
	}
	b.Incoming <- p
	d := <-p.decision
	if d == Drop {
		b.droppedFlows.Store(f, struct{}{})
	}
	return d
}

// HoldResponse is called from the proxy addon after receiving the response headers, but before reading the body.
func (b *Broker) HoldResponse(f *proxy.Flow) Decision {
	if _, oos := b.outOfScope.Load(f); oos {
		return Forward
	}
	if !b.captureResponse.Load() {
		return Forward
	}
	p := &PendingResponse{
		Flow:      f,
		decision:  make(chan Decision, 1),
		ArrivedAt: time.Now(),
	}
	b.IncomingResponse <- p
	return <-p.decision
}

// SaveEntry persists the completed flow to the history DB.
// It must be called after HoldResponse and before modifying f.Response.
// Flows that were dropped at the request phase are silently skipped.
func (b *Broker) SaveEntry(f *proxy.Flow) {
	b.dbMu.RLock()
	d := b.database
	b.dbMu.RUnlock()
	if d == nil {
		return
	}
	if _, oos := b.outOfScope.LoadAndDelete(f); oos {
		return
	}
	if _, dropped := b.droppedFlows.LoadAndDelete(f); dropped {
		return
	}
	status := 0
	if f.Response != nil {
		status = f.Response.StatusCode
	}
	r := f.Request
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	if config.Global.History.SkipDuplicates {
		body := string(r.Body)
		if dup, _ := d.HasDuplicate(r.Method, r.URL.Host, path, body); dup {
			return
		}
	}
	entry, err := d.InsertEntry(db.Entry{
		Timestamp:   time.Now(),
		Method:      r.Method,
		Host:        r.URL.Host,
		Path:        path,
		StatusCode:  status,
		RequestRaw:  FormatRawRequest(f),
		ResponseRaw: FormatRawResponse(f),
	})
	if err == nil {
		if cb := b.onNewEntry; cb != nil {
			go cb(entry)
		}
	}
}

func (b *Broker) Decide(p *PendingRequest, d Decision) {
	p.decision <- d
}

func (b *Broker) DecideResponse(p *PendingResponse, d Decision) {
	p.decision <- d
}
