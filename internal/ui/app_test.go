package ui

import (
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anotherhadi/foam"
	fconfig "github.com/anotherhadi/foam/config"
	"github.com/anotherhadi/foam/theme"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/proxy"
)

func init() { proxy.Quiet() }

// split so that GitHub push protection does not take it for a real token
const slackToken = "xox" + "b-1234567890123-1234567890123-abcdefghijABCDEFGHIJklmn"

type env struct {
	m      *Model
	px     *proxy.Proxy
	client *http.Client
	base   string
	cfg    *config.Config
}

func newEnv(t *testing.T, edit func(*config.Config)) *env {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Add("Set-Cookie", "sid=abc; Path=/")
			io.WriteString(w, "<html><body><h1>"+strings.Join(slices.Collect(maps.Keys(r.URL.Query())), " ")+"</h1><p>"+r.URL.RawQuery+"</p>"+
				"<pre>java.lang.NullPointerException\n\tat com.acme.Foo.bar(Foo.java:42)</pre></body></html>")
			return
		}
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "text/javascript")
			io.WriteString(w, `const cfg={slack:"`+slackToken+`"};`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path": "`+r.URL.Path+`", "user": "`+r.URL.Query().Get("user")+`"}`)
	}))
	t.Cleanup(srv.Close)
	cfg, err := fconfig.LoadBytes[config.Config](foam.Defaults, config.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	cfg.App.Project = t.TempDir()
	cfg.Proxy.Listen, cfg.Proxy.CA = "127.0.0.1:0", t.TempDir()
	cfg.Steep.Collection = t.TempDir()
	if edit != nil {
		edit(cfg)
	}
	theme.Set(cfg.Theme)
	proj, err := project.Open(cfg.App.Project)
	if err != nil {
		t.Fatal(err)
	}
	px, err := proxy.New(cfg, proj)
	if err != nil {
		t.Fatal(err)
	}
	if err := px.Start(); err != nil {
		t.Fatal(err)
	}
	m, err := New(cfg, proj, px)
	if err != nil {
		t.Fatal(err)
	}
	m.events = nil // the tests take them
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 45})
	pu, _ := url.Parse("http://" + px.Addr)
	e := &env{m: m, px: px, base: srv.URL, cfg: cfg,
		client: &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}, Timeout: 5 * time.Second}}
	for range 50 {
		if resp, err := e.client.Get(srv.URL + "/warmup"); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	return e
}

// next feeds the model the proxy's events up to one of type want.
func (e *env) next(t *testing.T, want reflect.Type) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev := <-e.px.Events():
			do(e.m, eventMsg{ev})
			if reflect.TypeOf(ev) == want {
				return
			}
		case <-timeout:
			t.Fatalf("no %v", want)
		}
	}
}

func (e *env) get(path string) chan string {
	out := make(chan string, 1)
	go func() {
		resp, err := e.client.Get(e.base + path)
		if err != nil {
			out <- "error: " + err.Error()
			return
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		out <- resp.Status + " " + string(b)
	}()
	return out
}

func do(m *Model, msg tea.Msg) {
	_, cmd := m.Update(msg)
	drain(m, cmd, 0)
}

func drain(m *Model, cmd tea.Cmd, depth int) {
	if cmd == nil || depth > 20 {
		return
	}
	// a command still waiting after a while is a timer (a toast's): skipped
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(100 * time.Millisecond):
		return
	}
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drain(m, c, depth+1)
		}
	case nil:
	default:
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
			for i := range v.Len() {
				drain(m, v.Index(i).Interface().(tea.Cmd), depth+1)
			}
			return
		}
		_, next := m.Update(msg)
		drain(m, next, depth+1)
	}
}

func typeKeys(m *Model, s string) {
	for _, r := range s {
		do(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func key(m *Model, code rune, mod tea.KeyMod) { do(m, tea.KeyPressMsg{Code: code, Mod: mod}) }

func screen(m *Model) string { return ansi.Strip(m.View().Content) }

func TestHistory(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	if m.page != pageHistory {
		t.Fatalf("starts on %v", m.page)
	}
	for _, p := range []string{"/a?user=alice", "/b?user=bob", "/admin"} {
		<-e.get(p)
		e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	}
	s := screen(m)
	if !strings.Contains(s, "History (4)") || !strings.Contains(s, "/admin") {
		t.Fatalf("history:\n%s", s)
	}
	// the selection follows the newest, its response is shown
	if !strings.Contains(s, `"path": "/admin"`) {
		t.Fatalf("response of the newest not shown:\n%s", s)
	}
	typeKeys(m, "/")
	typeKeys(m, `resp.raw.cont:"bob"`)
	key(m, tea.KeyEnter, 0)
	if es := m.hist.Shown(); len(es) != 1 || !strings.Contains(es[0].URL, "user=bob") {
		t.Fatalf("filtered: %+v", es)
	}
	typeKeys(m, "m")
	if e0, _ := m.hist.Selected(); !m.hist.Meta(e0.ID).Marked {
		t.Fatal("m marks")
	}
	_, meta, _ := m.proj.Load()
	if len(meta) != 1 {
		t.Fatalf("the mark is not kept: %v", meta)
	}
	key(m, 'g', tea.ModCtrl)
	if len(m.hist.Shown()) != 4 {
		t.Fatal("ctrl+g clears the filter")
	}
}

func TestNoteAndSteep(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	<-e.get("/login?user=x")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	typeKeys(m, "n")
	if m.overlays.Top() == nil {
		t.Fatal("n asks for a note")
	}
	typeKeys(m, "idor here")
	key(m, tea.KeyEnter, 0)
	sel, _ := m.hist.Selected()
	if m.hist.Meta(sel.ID).Note != "idor here" {
		t.Fatalf("note: %+v", m.hist.Meta(sel.ID))
	}
	if !strings.Contains(screen(m), "# note: idor here") {
		t.Fatalf("the note is not shown above the request:\n%s", screen(m))
	}
	key(m, tea.KeyEnter, 0) // from the request pane
	if m.focus[pageHistory] != paneHistRequest {
		t.Fatalf("focus %v", m.focus[pageHistory])
	}
	typeKeys(m, "s")
	host := strings.TrimPrefix(e.base, "http://")
	host = host[:strings.IndexByte(host, ':')]
	b, err := os.ReadFile(filepath.Join(e.cfg.Steep.Collection, "requests", "spilltea", host, "login.http"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "# steep proto: http\nGET /login?user=x HTTP/1.1\nHost: ") {
		t.Fatalf("sent to steep:\n%s", b)
	}
}

func TestIntercept(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	key(m, 'i', tea.ModAlt)
	if !e.px.Holding(proxy.KindRequest) {
		t.Fatal("alt+i intercepts the requests")
	}
	res := e.get("/orig?user=a")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	if m.inter.Requests.Len() != 1 {
		t.Fatal("the request is not in the queue")
	}
	key(m, '1', tea.ModAlt)
	if s := screen(m); !strings.Contains(s, "GET /orig?user=a HTTP/1.1") || !strings.Contains(s, "Intercept 1") {
		t.Fatalf("intercept page:\n%s", s)
	}
	// enter edits it, in the detail: the path, in vim
	key(m, tea.KeyEnter, 0)
	if m.focus[pageIntercept] != paneDetail {
		t.Fatalf("focus %v", m.focus[pageIntercept])
	}
	typeKeys(m, "f/lcwedited")
	key(m, tea.KeyEscape, 0)
	if !strings.HasPrefix(m.inter.Text(), "GET /edited?user=a") {
		t.Fatalf("edited text: %q", m.inter.Text())
	}
	// f is vim's own in the editor; ctrl+f forwards
	key(m, 'f', tea.ModCtrl)
	if r := <-res; !strings.Contains(r, `"path": "/edited"`) {
		t.Fatalf("client got %q", r)
	}
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	if m.inter.Len() != 0 {
		t.Fatal("the queue keeps a forwarded item")
	}
	last := m.hist.All()[len(m.hist.All())-1]
	if !last.Edited || !strings.Contains(last.URL, "/edited") {
		t.Fatalf("entry %+v", last)
	}

	// x drops from the queue
	res = e.get("/drop")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	key(m, 'k', tea.ModCtrl)
	typeKeys(m, "x")
	if r := <-res; !strings.HasPrefix(r, "502") {
		t.Fatalf("dropped: %q", r)
	}

	// hold the response of this one, then change it
	res = e.get("/resp")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	typeKeys(m, "r")
	typeKeys(m, "f")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	if m.inter.Responses.Len() != 1 || !m.showResponses() {
		t.Fatal("the response is not held")
	}
	if !strings.Contains(screen(m), "Responses (1)") {
		t.Fatalf("no responses pane:\n%s", screen(m))
	}
	key(m, 'l', tea.ModCtrl)
	if m.focus[pageIntercept] != paneResponses {
		t.Fatalf("focus %v", m.focus[pageIntercept])
	}
	typeKeys(m, "f")
	if r := <-res; !strings.Contains(r, `"path": "/resp"`) {
		t.Fatalf("response: %q", r)
	}
}

func TestQuitWithHeld(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	e.px.SetHolding(proxy.KindRequest, true)
	res := e.get("/x")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	typeKeys(m, "q")
	if m.overlays.Top() == nil {
		t.Fatal("quitting with something held asks first")
	}
	m.Close()
	if r := <-res; strings.HasPrefix(r, "200") {
		t.Fatalf("a held request went through: %q", r)
	}
}

func TestDropAllAsksFirst(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	key(m, '1', tea.ModAlt)
	e.px.SetHolding(proxy.KindRequest, true)
	a, b := e.get("/a"), e.get("/b")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	typeKeys(m, "X")
	if m.overlays.Top() == nil || m.inter.Requests.Len() != 2 {
		t.Fatalf("dropped before the answer: %d held", m.inter.Requests.Len())
	}
	key(m, tea.KeyEscape, 0)
	if m.inter.Requests.Len() != 2 {
		t.Fatal("dropped on no")
	}
	typeKeys(m, "X")
	key(m, tea.KeyLeft, 0)
	key(m, tea.KeyEnter, 0)
	for _, res := range []chan string{a, b} {
		if r := <-res; !strings.HasPrefix(r, "502") {
			t.Fatalf("dropped: %q", r)
		}
	}
	if m.inter.Len() != 0 {
		t.Fatal("the queue keeps them")
	}
}

func TestEmptyQueueActsOnShown(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	key(m, '1', tea.ModAlt)
	e.px.SetHolding(proxy.KindRequest, true)
	res := e.get("/resp")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	typeKeys(m, "rf")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	// the requests' queue is focused, and empty: f forwards the response shown
	if m.focus[pageIntercept] != paneRequests || m.inter.Shown() == nil || m.inter.Shown().Kind != proxy.KindResponse {
		t.Fatalf("focus %v, shown %+v", m.focus[pageIntercept], m.inter.Shown())
	}
	typeKeys(m, "f")
	if r := <-res; !strings.Contains(r, `"path": "/resp"`) {
		t.Fatalf("response: %q", r)
	}
}

func TestInterceptOffKeepsEdits(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	key(m, '1', tea.ModAlt)
	key(m, 'i', tea.ModAlt)
	res := e.get("/orig")
	e.next(t, reflect.TypeFor[proxy.HeldMsg]())
	key(m, tea.KeyEnter, 0)
	typeKeys(m, "f/lcwedited")
	key(m, tea.KeyEscape, 0)
	key(m, 'i', tea.ModAlt)
	if r := <-res; !strings.Contains(r, `"path": "/edited"`) {
		t.Fatalf("client got %q", r)
	}
}

func TestPipeEachReader(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	key(m, '2', tea.ModAlt)
	// the response's pipe goes to the response, not to the request before it
	key(m, tea.KeyEnter, 0)
	key(m, 'l', tea.ModCtrl)
	typeKeys(m, "|")
	typeKeys(m, "echo $((1000+337))")
	key(m, tea.KeyEnter, 0)
	// the first line is the output, the command is in the footer
	first := func(r interface {
		View(w, h int, focused bool, empty string) string
	}) string {
		l, _, _ := strings.Cut(ansi.Strip(r.View(60, 10, false, "")), "\n")
		return strings.TrimSpace(l)
	}
	if first(m.histRes) != "1337" || first(m.histReq) == "1337" {
		t.Fatalf("the pipe's output:\n%s", screen(m))
	}
}

func TestSitemapAndDiff(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	for _, p := range []string{"/api/users?user=a", "/api/users?user=b", "/login"} {
		<-e.get(p)
		e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	}
	key(m, '3', tea.ModAlt)
	s := screen(m)
	if !strings.Contains(s, "127.0.0.1") {
		t.Fatalf("sitemap:\n%s", s)
	}
	typeKeys(m, "l") // opens the host
	typeKeys(m, "j") // api
	if m.tree.SelectedName() == "" || !strings.HasSuffix(m.tree.SelectedName(), "/api") {
		t.Fatalf("selected %q", m.tree.SelectedName())
	}
	do(m, nil)
	if n := len(m.site.Shown()); n != 2 {
		t.Fatalf("the entries under api: %d", n)
	}
	key(m, 'l', tea.ModCtrl)
	typeKeys(m, "k")
	typeKeys(m, "a")
	typeKeys(m, "j")
	typeKeys(m, "b")
	if m.page != pageDiff {
		t.Fatalf("b compares: page %v", m.page)
	}
	typeKeys(m, "t") // the responses
	s = screen(m)
	if !strings.Contains(s, `-{"path": "/api/users", "user": "a"}`) || !strings.Contains(s, `+{"path": "/api/users", "user": "b"}`) {
		t.Fatalf("diff:\n%s", s)
	}
}

func TestMarkSharedByTheLists(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	key(m, '2', tea.ModAlt)
	typeKeys(m, "/row.marked.eq:true")
	key(m, tea.KeyEnter, 0)
	if len(m.hist.Shown()) != 0 {
		t.Fatal("nothing is marked")
	}
	// marked in the sitemap, it passes the history's filter
	key(m, '3', tea.ModAlt)
	typeKeys(m, "j")
	do(m, nil)
	key(m, 'l', tea.ModCtrl)
	typeKeys(m, "m")
	if len(m.hist.Shown()) != 1 {
		t.Fatalf("the history's filter keeps %d", len(m.hist.Shown()))
	}
}

func TestEveryPageEverySize(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	<-e.get("/x")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	for _, size := range [][2]int{{80, 20}, {100, 30}, {200, 60}, {40, 10}} {
		do(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for p := range pageCount {
			m.setPage(p)
			for _, z := range []bool{false, true} {
				m.zoomed = z
				for _, pn := range pagePanes[p] {
					m.setFocus(pn)
					v := m.View().Content
					for i, l := range strings.Split(v, "\n") {
						if w := ansi.StringWidth(l); w > size[0] {
							t.Fatalf("%v %v: line %d is %d wide", size, pn, i, w)
						}
					}
				}
			}
		}
	}
}

func TestAssetsHidden(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	<-e.get("/logo.png")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	<-e.get("/page")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	// the warmup and /page
	if n := len(m.hist.Shown()); n != 2 {
		t.Fatalf("%d shown, assets included", n)
	}
	typeKeys(m, "A")
	if n := len(m.hist.Shown()); n != 3 {
		t.Fatalf("A shows the assets: %d", n)
	}
	key(m, '3', tea.ModAlt)
	if !strings.Contains(screen(m), "logo.png") {
		t.Fatalf("the sitemap has the assets too:\n%s", screen(m))
	}
	typeKeys(m, "A")
	if strings.Contains(screen(m), "logo.png") {
		t.Fatalf("A hides them again:\n%s", screen(m))
	}
}

func TestLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8080": true, "[::1]:8080": true, "localhost:8080": true,
		":8080": false, "0.0.0.0:8080": false, "192.168.1.2:8080": false,
	} {
		if got := loopback(addr); got != want {
			t.Errorf("%s: %v, want %v", addr, got, want)
		}
	}
}

// openSteep fakes a steep running on root: its socket and its .root file.
func openSteep(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "steep")
	os.MkdirAll(dir, 0o700)
	ln, err := net.Listen("unix", filepath.Join(dir, name+".sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	os.WriteFile(filepath.Join(dir, name+".root"), []byte(root), 0o600)
}

func TestSteepPick(t *testing.T) {
	e := newEnv(t, nil)
	m := e.m
	<-e.get("/login")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	openSteep(t, a, "k.1")
	openSteep(t, b, "k.2")
	count := func(col string) int {
		n, _ := filepath.Glob(filepath.Join(col, "requests", "spilltea", "*", "login*.http"))
		return len(n)
	}
	sent := func(col string) bool { return count(col) > 0 }

	typeKeys(m, "s")
	if m.overlays.Top() == nil {
		t.Fatal("two steeps open: s asks where to send")
	}
	key(m, tea.KeyDown, 0)
	key(m, tea.KeyEnter, 0)
	if !sent(b) || sent(a) || sent(e.cfg.Steep.Collection) {
		t.Fatal("not sent to the picked collection")
	}
	// the last pick comes first
	typeKeys(m, "s")
	key(m, tea.KeyEnter, 0)
	if sent(a) || count(b) != 2 {
		t.Fatalf("enter did not send to the last pick: a %d, b %d", count(a), count(b))
	}

	// a steep open on the configured collection wins, without asking
	openSteep(t, e.cfg.Steep.Collection, "k.3")
	typeKeys(m, "s")
	if m.overlays.Top() != nil || !sent(e.cfg.Steep.Collection) {
		t.Fatal("not sent to the configured collection")
	}
	if _, err := os.Stat("requests"); err == nil {
		os.RemoveAll("requests")
		t.Fatal("a request went to an empty collection path")
	}
}

func TestFindings(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Findings.Secrets = true })
	m := e.m
	for _, p := range []string{"/app.js", "/vendor.js"} {
		<-e.get(p)
		e.next(t, reflect.TypeFor[proxy.EntryMsg]())
		e.next(t, reflect.TypeFor[proxy.FindingMsg]())
	}
	if !strings.Contains(screen(m), "Findings  1 ") {
		t.Fatalf("no badge:\n%s", screen(m))
	}
	key(m, '5', tea.ModAlt)
	s := screen(m)
	if os.Getenv("SHOW") != "" {
		t.Log("\n" + s)
	}
	if !strings.Contains(s, "slack-bot-token") || !strings.Contains(s, "seen in 2 requests: #2 #3") || !strings.Contains(s, "Seen in (2)") {
		t.Fatalf("findings page:\n%s", s)
	}
	typeKeys(m, "r")
	if m.finds.Untriaged() != 0 || !strings.Contains(screen(m), "Everything is triaged") || strings.Contains(screen(m), "Findings  1 ") {
		t.Fatalf("r triages it:\n%s", screen(m))
	}
	typeKeys(m, "t")
	if f, ok := m.finds.Selected(); !ok || f.Status != project.StatusRelevant {
		t.Fatalf("t shows the triaged: %+v", f)
	}
	typeKeys(m, "n")
	typeKeys(m, "prod key")
	key(m, tea.KeyEnter, 0)
	if !strings.Contains(screen(m), "note: prod key") {
		t.Fatalf("note:\n%s", screen(m))
	}
	typeKeys(m, "u")
	if m.finds.Untriaged() != 1 {
		t.Fatal("u sets it back to new")
	}

	key(m, tea.KeyEnter, 0)
	if m.focus[pageFindings] != paneSeen {
		t.Fatalf("focus %v", m.focus[pageFindings])
	}
	key(m, tea.KeyEnter, 0)
	if sel, ok := m.hist.Selected(); m.page != pageHistory || !ok || sel.ID != 2 || !m.assets {
		t.Fatalf("page %v, selected %+v", m.page, sel)
	}
	if !strings.Contains(screen(m), "app.js slack-bot-token") || !strings.Contains(screen(m), "# finding: slack-bot-token (secrets)") {
		t.Fatalf("history:\n%s", screen(m))
	}
	drain(m, m.hist.SetFilter(`preset:"findings"`), 0)
	if es := m.hist.Shown(); len(es) != 2 {
		t.Fatalf("findings preset: %+v", es)
	}
}

func TestRescan(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Findings.Secrets = false })
	m := e.m
	<-e.get("/me?user=" + slackToken)
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	key(m, '5', tea.ModAlt)
	typeKeys(m, "R")
	if !strings.Contains(screen(m), "No detector is on") {
		t.Fatalf("no detector:\n%s", screen(m))
	}
	m.cfg.Findings.Secrets = true
	typeKeys(m, "R")
	f, ok := m.finds.Selected()
	if !ok || f.Title != "slack-bot-token" || len(f.Entries) != 1 || f.Entries[0] != 2 || m.rescanning {
		t.Fatalf("%+v", f)
	}
	if !strings.Contains(screen(m), "Scan done, 1 request with new findings") {
		t.Fatalf("toast:\n%s", screen(m))
	}
	typeKeys(m, "R")
	if !strings.Contains(screen(m), "nothing new") {
		t.Fatalf("second scan:\n%s", screen(m))
	}
}

func TestCopyFinding(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Findings.Secrets = true })
	m := e.m
	<-e.get("/me?user=" + slackToken)
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	e.next(t, reflect.TypeFor[proxy.FindingMsg]())
	key(m, '5', tea.ModAlt)
	typeKeys(m, "y")
	if !strings.Contains(screen(m), "Copied the finding") {
		t.Fatalf("y copies:\n%s", screen(m))
	}
}

func TestFindingsLive(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Findings.Secrets = true })
	m := e.m
	drain(m, m.hist.SetFilter(`preset:"findings"`), 0)
	typeKeys(m, "A")
	for _, p := range []string{"/app.js", "/vendor.js"} {
		<-e.get(p)
		e.next(t, reflect.TypeFor[proxy.EntryMsg]())
		e.next(t, reflect.TypeFor[proxy.FindingMsg]())
	}
	if n := len(m.hist.Shown()); n != 2 {
		t.Fatalf("the filter is not refreshed as findings come: %d shown", n)
	}

	key(m, '5', tea.ModAlt)
	key(m, tea.KeyEnter, 0)
	typeKeys(m, "j")
	if sel, _ := m.seen.Selected(); sel.ID != 3 {
		t.Fatalf("selected #%d", sel.ID)
	}
	<-e.get("/more.js")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	e.next(t, reflect.TypeFor[proxy.FindingMsg]())
	if sel, _ := m.seen.Selected(); sel.ID != 3 || len(m.seen.Shown()) != 3 {
		t.Fatalf("selection lost: #%d of %d", sel.ID, len(m.seen.Shown()))
	}
}

func TestTriageFromDetail(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Findings.Secrets = true })
	m := e.m
	<-e.get("/app.js")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	e.next(t, reflect.TypeFor[proxy.FindingMsg]())
	key(m, '5', tea.ModAlt)
	m.setFocus(paneFinding)
	typeKeys(m, "x")
	if m.finds.Untriaged() != 0 {
		t.Fatal("x triages from the detail")
	}
}

func TestFindingDetailSanitized(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Findings = config.Findings{Disclosure: true} })
	m := e.m
	<-e.get("/p.html?%1b%5b31mevil=zzzz9999")
	e.next(t, reflect.TypeFor[proxy.EntryMsg]())
	e.next(t, reflect.TypeFor[proxy.FindingMsg]())
	key(m, '5', tea.ModAlt)
	if v := m.View().Content; strings.Contains(v, "\x1b[31mevil") {
		t.Fatal("an escape sequence from the server reaches the terminal")
	}
}
