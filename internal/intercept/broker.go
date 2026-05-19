package intercept

import (
	"log"
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
	Forward   Decision = iota // forward without showing in intercept
	Drop                      // drop the flow
	Intercept                 // pass to the TUI for user decision
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

	autoFwdMu      sync.RWMutex
	autoFwdRegexes []*regexp.Regexp

	onBeforeNewEntry func(db.Entry) bool
	onNewEntry       func(db.Entry)
}

func (b *Broker) SetOnBeforeNewEntry(cb func(db.Entry) bool) {
	b.onBeforeNewEntry = cb
}

func (b *Broker) SetOnNewEntry(cb func(db.Entry)) {
	b.onNewEntry = cb
}

func NewBroker() *Broker {
	b := &Broker{
		Incoming:         make(chan *PendingRequest, 64),
		IncomingResponse: make(chan *PendingResponse, 64),
	}
	b.SetAutoForwardRegex(config.Global.Intercept.AutoForwardRegex)
	return b
}

func (b *Broker) SetCaptureResponse(v bool) {
	b.captureResponse.Store(v)
}

// SetAutoForwardRegex compiles and stores patterns for requests that should
// be forwarded automatically without interception or history logging.
// Invalid patterns are silently skipped.
func (b *Broker) SetAutoForwardRegex(patterns []string) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			log.Printf("intercept: invalid auto_forward_regex %q: %v", p, err)
			continue
		}
		compiled = append(compiled, r)
	}
	b.autoFwdMu.Lock()
	b.autoFwdRegexes = compiled
	b.autoFwdMu.Unlock()
}

func (b *Broker) isAutoForwarded(target string) bool {
	b.autoFwdMu.RLock()
	regexes := b.autoFwdRegexes
	b.autoFwdMu.RUnlock()
	for _, r := range regexes {
		if r.MatchString(target) {
			return true
		}
	}
	return false
}

func (b *Broker) SetDB(d *db.DB) {
	b.dbMu.Lock()
	b.database = d
	b.dbMu.Unlock()
}

// Hold is called from the proxy addon: it blocks until a decision is made in the TUI.
func (b *Broker) Hold(f *proxy.Flow) Decision {
	target := f.Request.URL.Host + f.Request.URL.Path
	if b.isAutoForwarded(target) {
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
// Flows that were dropped or auto-forwarded are silently skipped.
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
	body := string(r.Body)
	pending := db.Entry{
		Timestamp:  time.Now(),
		Method:     r.Method,
		Host:       r.URL.Host,
		Path:       path,
		StatusCode: status,
		RequestRaw: FormatRawRequest(f),
		ResponseRaw: func() string {
			if config.Global.History.KeepResponses {
				return FormatRawResponse(f)
			}
			return ""
		}(),
	}
	if cb := b.onBeforeNewEntry; cb != nil {
		if !cb(pending) {
			return
		}
	}
	var (
		entry db.Entry
		err   error
	)
	if config.Global.History.SkipDuplicates {
		var dup bool
		entry, dup, err = d.InsertIfNotDuplicate(pending, body)
		if dup || err != nil {
			return
		}
	} else {
		entry, err = d.InsertEntry(pending, body)
	}
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
