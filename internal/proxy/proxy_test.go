package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/anotherhadi/foam"
	fconfig "github.com/anotherhadi/foam/config"
	"github.com/gorilla/websocket"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/project"
)

type seen struct {
	path, color, ua string
}

func setup(t *testing.T, edit func(*config.Config)) (*Proxy, *project.Project, *http.Client, string, chan seen) {
	t.Helper()
	got := make(chan seen, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- seen{r.URL.RequestURI(), r.Header.Get("X-PwnFox-Color"), r.Header.Get("User-Agent")}
		if strings.HasSuffix(r.URL.Path, ".png") {
			w.Header().Set("Content-Type", "image/png")
		}
		io.WriteString(w, "hello "+r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	cfg, err := fconfig.LoadBytes[config.Config](foam.Defaults, config.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Proxy.Listen = "127.0.0.1:0"
	cfg.Proxy.CA = t.TempDir()
	if edit != nil {
		edit(cfg)
	}
	p, _ := project.Open(t.TempDir())
	x, err := New(cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	pu, _ := url.Parse("http://" + x.Addr)
	c := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}, Timeout: 5 * time.Second}
	// the proxy serves in the background
	for i := 0; i < 50; i++ {
		if resp, err := c.Get(srv.URL + "/warmup.png"); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	<-got
	next[EntryMsg](t, x)
	return x, p, c, srv.URL, got
}

func next[T any](t *testing.T, x *Proxy) T {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-x.Events():
			if v, ok := ev.(T); ok {
				return v
			}
			if e, ok := ev.(ErrorMsg); ok {
				t.Fatalf("error: %v", e.Err)
			}
		case <-timeout:
			var zero T
			t.Fatalf("no %T", zero)
			return zero
		}
	}
}

func get(c *http.Client, u string, header ...string) chan string {
	out := make(chan string, 1)
	go func() {
		req, _ := http.NewRequest("GET", u, nil)
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		resp, err := c.Do(req)
		if err != nil {
			out <- "error: " + err.Error()
			return
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		out <- resp.Status + " " + resp.Header.Get("X-Spilltea") + " " + string(b)
	}()
	return out
}

func TestRecord(t *testing.T) {
	x, p, c, base, got := setup(t, func(c *config.Config) { c.HTTP.UserAgentSuffix = "hunter" })
	res := get(c, base+"/a?x=1", "X-PwnFox-Color", "Red", "User-Agent", "ua")
	s := <-got
	if s.color != "" || s.ua != "ua hunter" || s.path != "/a?x=1" {
		t.Fatalf("server saw %+v", s)
	}
	if r := <-res; !strings.Contains(r, "hello /a") {
		t.Fatalf("client got %q", r)
	}
	e := next[EntryMsg](t, x).Entry
	if e.Status != 200 || e.Color != "red" || e.Kept != project.KeptFull || e.Asset {
		t.Fatalf("entry %+v", e)
	}
	resp, _ := p.ReadResponse(e.ID)
	req, _ := p.ReadRequest(e.ID)
	if !strings.HasSuffix(string(resp), "hello /a") || !strings.Contains(string(req), "User-Agent: ua hunter") {
		t.Fatalf("stored:\n%s\n%s", req, resp)
	}
}

func TestInterceptRequest(t *testing.T) {
	x, p, c, base, got := setup(t, nil)
	x.SetHolding(KindRequest, true)
	res := get(c, base+"/orig")
	it := next[HeldMsg](t, x).Item
	if it.Kind != KindRequest || !strings.HasPrefix(string(it.Raw), "GET /orig HTTP/1.1") {
		t.Fatalf("held %q", it.Raw)
	}
	x.Decide(it, Decision{Raw: []byte(strings.Replace(string(it.Raw), "/orig", "/edited", 1))})
	if s := <-got; s.path != "/edited" {
		t.Fatalf("server saw %s", s.path)
	}
	if r := <-res; !strings.Contains(r, "hello /edited") {
		t.Fatalf("client %q", r)
	}
	if e := next[EntryMsg](t, x).Entry; !e.Edited || !strings.HasSuffix(e.URL, "/edited") {
		t.Fatalf("entry %+v", e)
	}

	res = get(c, base+"/drop")
	it = next[HeldMsg](t, x).Item
	x.Decide(it, Decision{Drop: true})
	if r := <-res; !strings.HasPrefix(r, "502 Bad Gateway dropped") {
		t.Fatalf("dropped: %q", r)
	}
	if e := next[EntryMsg](t, x).Entry; e.Error != "dropped" || e.Status != 0 {
		t.Fatalf("dropped entry %+v", e)
	}
	select {
	case s := <-got:
		t.Fatalf("the server got a dropped request: %+v", s)
	default:
	}

	// assets are not held
	res = get(c, base+"/logo.png")
	<-got
	<-res
	e := next[EntryMsg](t, x).Entry
	if !e.Asset || e.Kept != project.KeptStatus {
		t.Fatalf("asset %+v", e)
	}
	if b, _ := p.ReadResponse(e.ID); b != nil {
		t.Fatal("an asset's body is kept")
	}

	// turning the intercept off sends what is held
	res = get(c, base+"/later")
	it = next[HeldMsg](t, x).Item
	x.SetHolding(KindRequest, false)
	if r := <-res; !strings.Contains(r, "hello /later") {
		t.Fatalf("released: %q", r)
	}
	if rel := next[ReleasedMsg](t, x).Item; rel != it {
		t.Fatal("released another item")
	}
}

func TestInterceptResponse(t *testing.T) {
	x, _, c, base, got := setup(t, func(c *config.Config) { c.Intercept.Filter = `req.path.cont:"held"` })
	x.SetHolding(KindRequest, true)
	res := get(c, base+"/held")
	it := next[HeldMsg](t, x).Item
	x.Decide(it, Decision{HoldResponse: true})
	<-got
	rit := next[HeldMsg](t, x).Item
	if rit.Kind != KindResponse || !strings.HasSuffix(string(rit.Raw), "hello /held") {
		t.Fatalf("held response %q", rit.Raw)
	}
	x.Decide(rit, Decision{Raw: []byte(strings.Replace(string(rit.Raw), "hello", "bonjour", 1))})
	if r := <-res; !strings.Contains(r, "bonjour /held") {
		t.Fatalf("client %q", r)
	}
	next[EntryMsg](t, x)

	// the filter keeps the others going
	if r := <-get(c, base+"/free"); !strings.Contains(r, "hello /free") {
		t.Fatalf("not held: %q", r)
	}
	next[EntryMsg](t, x)
}

func TestOutOfScope(t *testing.T) {
	x, _, c, base, got := setup(t, func(c *config.Config) { c.Scope.Exclude = []string{"127.0.0.1/secret"} })
	x.SetHolding(KindRequest, true)
	if r := <-get(c, base+"/secret/x", "X-PwnFox-Color", "blue"); !strings.Contains(r, "hello /secret/x") {
		t.Fatalf("out of scope: %q", r)
	}
	if s := <-got; s.color != "" {
		t.Fatal("PwnFox's header goes out of scope too")
	}
	select {
	case ev := <-x.Events():
		t.Fatalf("out of scope, yet %T", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestClientGone(t *testing.T) {
	x, _, c, base, _ := setup(t, nil)
	x.SetHolding(KindRequest, true)
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/gone", nil)
	go func() {
		if resp, err := c.Do(req); err == nil {
			resp.Body.Close()
		}
	}()
	it := next[HeldMsg](t, x).Item
	cancel()
	if rel := next[ReleasedMsg](t, x).Item; rel != it {
		t.Fatal("released another item")
	}
	if e := next[EntryMsg](t, x).Entry; e.Error != "client gone" {
		t.Fatalf("entry %+v", e)
	}
}

func TestCloseDrops(t *testing.T) {
	x, _, c, base, _ := setup(t, nil)
	x.SetHolding(KindRequest, true)
	res := get(c, base+"/x")
	next[HeldMsg](t, x)
	x.Close()
	if r := <-res; strings.Contains(r, "hello") {
		t.Fatalf("a held request went through on close: %q", r)
	}
}

func TestHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "secure "+r.URL.Path)
	}))
	defer srv.Close()
	cfg, _ := fconfig.LoadBytes[config.Config](foam.Defaults, config.Defaults)
	cfg.Proxy.Listen, cfg.Proxy.CA, cfg.Proxy.Insecure = "127.0.0.1:0", t.TempDir(), true
	p, _ := project.Open(t.TempDir())
	x, _ := New(cfg, p)
	if err := x.Start(); err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	pool := x509.NewCertPool()
	ca := x.p.GetCertificate()
	pool.AddCert(&ca)
	pu, _ := url.Parse("http://" + x.Addr)
	c := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{RootCAs: pool}}}
	var resp *http.Response
	var err error
	for range 50 {
		if resp, err = c.Get(strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + "/tls"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "secure /tls" {
		t.Fatalf("%q", b)
	}
	if e := next[EntryMsg](t, x).Entry; !strings.HasPrefix(e.URL, "https://") || e.Status != 200 {
		t.Fatalf("entry %+v", e)
	}
}

func TestWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()
		mt, msg, err := c.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		c.WriteMessage(mt, msg)
	}))
	defer srv.Close()
	cfg, _ := fconfig.LoadBytes[config.Config](foam.Defaults, config.Defaults)
	cfg.Proxy.Listen, cfg.Proxy.CA, cfg.Proxy.Insecure = "127.0.0.1:0", t.TempDir(), true
	p, _ := project.Open(t.TempDir())
	x, _ := New(cfg, p)
	if err := x.Start(); err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	pool := x509.NewCertPool()
	ca := x.p.GetCertificate()
	pool.AddCert(&ca)
	pu, _ := url.Parse("http://" + x.Addr)
	d := &websocket.Dialer{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{RootCAs: pool}}
	wsURL := strings.Replace(strings.Replace(srv.URL, "127.0.0.1", "localhost", 1), "https://", "wss://", 1)
	var conn *websocket.Conn
	var err error
	for range 50 {
		if conn, _, err = d.Dial(wsURL+"/chat", nil); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "ping" {
		t.Fatalf("echo: %q, %v", msg, err)
	}
	conn.Close()
	e := next[EntryMsg](t, x).Entry
	if e.Status != http.StatusSwitchingProtocols || e.Mime != "websocket" || !strings.HasPrefix(e.URL, "wss://") || e.Kept != project.KeptFull {
		t.Fatalf("entry %+v", e)
	}
	resp, _ := p.ReadResponse(e.ID)
	if n := strings.Count(string(resp), "ping"); n != 2 {
		t.Fatalf("transcript:\n%s", resp)
	}
	if !strings.Contains(string(resp), "--> ") || !strings.Contains(string(resp), "<-- ") {
		t.Fatalf("transcript missing direction markers:\n%s", resp)
	}
}

func TestUntrustedCA(t *testing.T) {
	x, _, _, _, _ := setup(t, nil)
	if _, ok := log.Writer().(stdLog); !ok {
		t.Fatalf("the standard log goes to %T", log.Writer())
	}
	for range 3 {
		log.Printf("http2: server: error reading preface from client 127.0.0.1:41000: remote error: tls: unknown certificate authority")
	}
	log.Printf("http2: server: error reading preface from client 127.0.0.1:41002: read: connection reset by peer")
	e := next[ErrorMsg](t, x)
	if !strings.Contains(e.Err.Error(), "spilltea ca") {
		t.Fatalf("error %v", e.Err)
	}
	select {
	case ev := <-x.Events():
		t.Fatalf("then %T %v", ev, ev)
	case <-time.After(200 * time.Millisecond):
	}
}
