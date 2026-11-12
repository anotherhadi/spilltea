package endpoints

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/anotherhadi/foam"
	fconfig "github.com/anotherhadi/foam/config"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
)

const src = "const a='\\/api\\/v1\\/users', b=`/api/users/${id}/posts`;" +
	"fetch(\"/api/login\",{method:'post'}); http.delete('/api/session'); r.open(\"PUT\", 'api/v2/profile');" +
	"const re=/a\\/b\\/c/g; import('./chunk.js'); require('../utils/api'); define('lib/compiler/ast', ['lib/helpers/each/x'], function(){});" +
	"load('/static/logo.png'); const f='file:///etc/x', p='/definitions/asset~1inline', m='/node_modules/x/y';" +
	"const t='application/json', d='MM/DD/YYYY', o='and/or', n='/_next/data/x.json', s='http://www.w3.org/2000/svg';" +
	"const u='https://api.example.com/v1/graphql', q='/search?q=1&page={}', w='hello world', x='/api/v1/users';"

func TestExtract(t *testing.T) {
	var got []string
	for _, ep := range Extract([]byte(src)) {
		got = append(got, strings.TrimSpace(ep.Method+" "+ep.Path))
	}
	want := []string{
		"POST /api/login",
		"DELETE /api/session",
		"/api/users/{}/posts",
		"/api/v1/users",
		"/search?q=1&page={}",
		"PUT api/v2/profile",
		"https://api.example.com/v1/graphql",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s", strings.Join(got, "\n"))
	}
	if Extract([]byte("const = ;")) != nil {
		t.Fatal("invalid JS")
	}
	got = nil
	for _, ep := range Extract([]byte("x(`${base}/api/me`); x(`/api/${a}${b}/c`); x('//cdn.test/v1/x'); x('//nodot/a')")) {
		got = append(got, ep.Path)
	}
	if strings.Join(got, " ") != "//cdn.test/v1/x /api/me /api/{}/c" {
		t.Fatalf("templates and protocol-relative: %q", got)
	}
}

func TestHandler(t *testing.T) {
	cfg, err := fconfig.LoadBytes[config.Config](foam.Defaults, config.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Scope.Include = []string{"app.test"}
	p, _ := project.Open(t.TempDir())
	known, _ := p.Add(project.Entry{Method: "GET", URL: "https://app.test/api/v1/users"}, []byte("GET /api/v1/users HTTP/1.1\r\n\r\n"), nil)
	var added []string
	h, err := NewHandler(cfg, p, func(e project.Entry) { added = append(added, e.Method+" "+e.URL+" "+e.Source) },
		func(err error) { t.Fatal(err) })
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://cdn.test/static/main.js")
	req := &message.Request{Method: "GET", URL: u, Header: http.Header{"Referer": {"https://app.test/account/settings"}}}
	resp := &message.Response{Status: 200, Header: http.Header{"Content-Type": {"application/javascript"}}, Body: []byte(src)}
	h.Handle(project.Entry{ID: 2, Method: "GET", URL: u.String()}, req, resp)
	h.Handle(project.Entry{ID: 3, Method: "GET", URL: u.String()}, req, resp)
	own, _ := url.Parse("https://app.test/static/own.js")
	req = &message.Request{Method: "GET", URL: own, Header: http.Header{"Referer": {"ftp://x/y"}}}
	h.Handle(project.Entry{ID: 4, Method: "GET", URL: own.String()}, req, &message.Response{Status: 200,
		Header: http.Header{"Content-Type": {"application/javascript"}}, Body: []byte("x('/api/ftp/referer')")})
	want := []string{
		"POST https://app.test/api/login js",
		"DELETE https://app.test/api/session js",
		"GET https://app.test/api/users/%7B%7D/posts js",
		"GET https://app.test/search?q=1&page={} js",
		"PUT https://app.test/account/api/v2/profile js",
		"GET https://app.test/api/ftp/referer js",
	}
	if strings.Join(added, "\n") != strings.Join(want, "\n") {
		t.Fatalf("added:\n%s", strings.Join(added, "\n"))
	}
	es, _, _ := p.Load()
	if len(es) != len(want)+1 || es[0].ID != known.ID {
		t.Fatal(fmt.Sprint(len(es), " entries"))
	}
}
