package importer

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/project"
)

func TestParse(t *testing.T) {
	for _, c := range []struct {
		line, source, method, url string
		status                    int
		rich                      bool
	}{
		{"https://a.test/x?y=1#frag", "import", "GET", "https://a.test/x?y=1", 0, false},
		{"https://a.test/x [200] [text/html]", "import", "GET", "https://a.test/x", 0, false},
		{"a.test/login", "import", "GET", "https://a.test/login", 0, false},
		{"localhost:8080/x", "import", "GET", "https://localhost:8080/x", 0, false},
		{`{"url":"https://a.test/old","input":"a.test","source":"waybackarchive"}`, "urlfinder", "GET", "https://a.test/old", 0, false},
		{`{"timestamp":"2026-11-07T10:00:00.5+01:00","url":"https://a.test/","input":"a.test","method":"GET","status_code":403,` +
			`"content_type":"text/html; charset=utf-8","content_length":12,"webserver":"nginx"}`, "httpx", "GET", "https://a.test/", 403, false},
		{`{"url":"https://a.test/","status_code":200,"request":"GET / HTTP/1.1\r\nHost: a.test\r\nUser-Agent: x\r\n\r\n",` +
			`"response":"HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<p>hi</p>"}`, "httpx", "GET", "https://a.test/", 200, true},
		{`{"timestamp":"2026-11-07T10:00:00Z","request":{"method":"POST","endpoint":"https://a.test/login","tag":"form","body":"u=a",` +
			`"headers":{"content_type":"application/x-www-form-urlencoded"}},"response":{"status_code":302,"headers":{"location":"/home"}}}`,
			"katana", "POST", "https://a.test/login", 302, true},
	} {
		it, err := Parse([]byte(c.line))
		if err != nil {
			t.Errorf("%s: %v", c.line, err)
			continue
		}
		if it.Source != c.source || it.Method != c.method || it.URL.String() != c.url || it.Status != c.status || it.rich() != c.rich {
			t.Errorf("%s: %s %s %s %d rich %v", c.line, it.Source, it.Method, it.URL, it.Status, it.rich())
		}
	}
	for _, bad := range []string{"ftp://a.test/", "not a url", "Found 3 URLs", "{}", `{"url":"mailto:x@a.test"}`, "{broken"} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("%s is read", bad)
		}
	}
}

func TestKatana(t *testing.T) {
	// chunked, as dumped
	raw := "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n"
	line, _ := json.Marshal(map[string]any{
		"request":  map[string]any{"method": "GET", "endpoint": "https://a.test/p", "raw": "GET /p HTTP/1.1\r\nHost: a.test\r\nCookie: s=1\r\n\r\n"},
		"response": map[string]any{"status_code": 200, "raw": raw},
	})
	it, err := Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if string(it.Response.Body) != "hello" || it.Response.Header.Get("Transfer-Encoding") != "" || it.Response.Header.Get("Content-Length") != "5" {
		t.Fatalf("body %q, headers %v", it.Response.Body, it.Response.Header)
	}
	if it.Request.Header.Get("Cookie") != "s=1" {
		t.Fatalf("request %v", it.Request.Header)
	}
}

func TestDecoded(t *testing.T) {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write([]byte("<html>hello</html>"))
	w.Close()
	r := response("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\n\r\n" + gz.String())
	if string(r.Body) != "<html>hello</html>" || r.Header.Get("Content-Encoding") != "" || r.Header.Get("Content-Length") != "18" {
		t.Fatalf("body %q, headers %v", r.Body, r.Header)
	}
}

func TestImport(t *testing.T) {
	cfg, err := config.Load([]string{"--project", t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Scope.Include = []string{"*.a.test"}
	p, _ := project.Open(cfg.App.Project)
	p.Add(project.Entry{Method: "GET", URL: "https://www.a.test/seen", Kept: project.KeptFull, Status: 200},
		[]byte("GET /seen HTTP/1.1\r\nHost: www.a.test\r\n\r\n"), []byte("HTTP/1.1 200 OK\r\n\r\n"))
	im, err := New(cfg, p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	in := strings.Join([]string{
		"https://www.a.test/seen",     // already there, with a response
		"https://www.a.test/new",      // added
		"https://www.a.test/new",      // twice
		"https://other.test/",         // out of scope
		"https://www.a.test/logo.png", // added, an asset
		"not a url at all",            // not read
		"",                            // skipped
		`{"url":"https://www.a.test/new","status_code":200,"response":"HTTP/1.1 200 OK\r\n\r\nbody"}`, // richer: added
		`{"url":"https://www.a.test/new","status_code":200}`,                                          // nothing more
	}, "\n")
	if err := im.Read(strings.NewReader(in)); err != nil {
		t.Fatal(err)
	}
	if im.Stats != (Stats{Added: 3, Duplicates: 3, OutOfScope: 1, Invalid: 1}) {
		t.Fatalf("%+v", im.Stats)
	}
	es, _, _ := p.Load()
	if len(es) != 4 || es[1].Source != "import" || es[1].Kept != "" || !es[2].Asset || es[3].Source != "httpx" || es[3].Kept != project.KeptFull {
		t.Fatalf("%+v", es)
	}
	if b, _ := p.ReadRequest(es[1].ID); string(b) != "GET /new HTTP/1.1\r\nHost: www.a.test\r\n\r\n" {
		t.Fatalf("%q", b)
	}
	if b, _ := p.ReadResponse(es[3].ID); string(b) != "HTTP/1.1 200 OK\r\n\r\nbody" {
		t.Fatalf("%q", b)
	}
}
