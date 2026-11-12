package rules

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/message"
)

func TestRequest(t *testing.T) {
	cfg := &config.Config{
		Rules: []config.Rule{
			{On: "request", Part: "head", Match: `(?m)^Accept-Encoding:.*$`, Replace: "Accept-Encoding: identity"},
			{On: "request", Part: "head", Replace: "X-Added: 1"},
			{On: "request", Part: "body", Match: `user=(\w+)`, Replace: "user=${1}2"},
			{On: "response", Part: "body", Match: `secret`, Replace: "hidden"},
		},
		HTTP: config.HTTP{Headers: []string{"X-Bug-Bounty: me"}, UserAgentSuffix: "hunter"},
	}
	r, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://api.test/login")
	req := &message.Request{Method: "POST", URL: u, Header: http.Header{
		"Accept-Encoding": {"gzip, br"}, "User-Agent": {"Firefox"},
	}, Body: []byte("user=bob")}
	if ok, err := r.Request(req); !ok || err != nil {
		t.Fatalf("Request: %v, %v", ok, err)
	}
	h := req.Header
	if h.Get("Accept-Encoding") != "identity" || h.Get("X-Added") != "1" || h.Get("X-Bug-Bounty") != "me" ||
		h.Get("User-Agent") != "Firefox hunter" || string(req.Body) != "user=bob2" || req.URL.String() != u.String() {
		t.Fatalf("request: %+v %s", h, req.Body)
	}
	// again: the suffix and the headers are not added twice
	if _, err := r.Request(req); err != nil || req.Header.Get("User-Agent") != "Firefox hunter" || len(req.Header["X-Bug-Bounty"]) != 1 {
		t.Fatalf("twice: %v", req.Header)
	}
	resp := &message.Response{Status: 200, Header: http.Header{}, Body: []byte("a secret")}
	if ok, _ := r.Response(resp); !ok || string(resp.Body) != "a hidden" || resp.Status != 200 {
		t.Fatalf("response: %+v", resp)
	}
}

func TestBrokenRule(t *testing.T) {
	r, _ := New(&config.Config{Rules: []config.Rule{{On: "request", Part: "head", Match: `^GET .*`, Replace: "nonsense"}}})
	u, _ := url.Parse("https://h/")
	req := &message.Request{Method: "GET", URL: u, Header: http.Header{}}
	if _, err := r.Request(req); err == nil || req.Method != "GET" {
		t.Fatalf("a rule breaking the request: %v, %+v", err, req)
	}
}
