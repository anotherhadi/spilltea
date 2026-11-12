package message

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	u, _ := url.Parse("https://api.test/login?next=%2F")
	r := &Request{Method: "POST", URL: u, Proto: "HTTP/2.0", Header: http.Header{
		"Content-Type": {"application/json"}, "Accept": {"*/*"},
	}, Body: []byte(`{"a":1}`)}
	raw := r.Bytes()
	want := "POST /login?next=%2F HTTP/2.0\r\nHost: api.test\r\nAccept: */*\r\nContent-Type: application/json\r\n\r\n{\"a\":1}"
	if string(raw) != want {
		t.Fatalf("raw:\n%q\nwant\n%q", raw, want)
	}
	// as the editor gives it back, "\n" line ends
	back, err := ParseRequest([]byte(Text(raw)), "https")
	if err != nil {
		t.Fatal(err)
	}
	if back.URL.String() != u.String() || back.Method != "POST" || string(back.Body) != `{"a":1}` || back.Header.Get("Accept") != "*/*" {
		t.Fatalf("back: %+v %s", back, back.URL)
	}
	for _, bad := range []string{"", "GET /\nnope", "GET / HTTP/1.1\n\n", "hello"} {
		if _, err := ParseRequest([]byte(bad), "https"); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestResponse(t *testing.T) {
	r := &Response{Proto: "HTTP/1.1", Status: 404, Header: http.Header{"Server": {"x"}}, Body: []byte("nope")}
	raw := r.Bytes()
	if !strings.HasPrefix(string(raw), "HTTP/1.1 404 Not Found\r\nServer: x\r\n\r\nnope") {
		t.Fatalf("%q", raw)
	}
	back, err := ParseResponse([]byte(Text(raw)))
	if err != nil || back.Status != 404 || string(back.Body) != "nope" {
		t.Fatalf("%+v %v", back, err)
	}
	h := http.Header{"Transfer-Encoding": {"chunked"}}
	FixLength(h, []byte("abc"), false)
	if h.Get("Content-Length") != "3" || h.Get("Transfer-Encoding") != "" {
		t.Fatalf("FixLength: %v", h)
	}
}

func TestSplitAndExt(t *testing.T) {
	head, body := Split([]byte("a\r\nb\r\n\r\nc\n\nd"))
	if string(head) != "a\r\nb" || string(body) != "c\n\nd" {
		t.Fatalf("%q %q", head, body)
	}
	if _, body := Split([]byte("a\nb")); body != nil {
		t.Fatal("no blank line, no body")
	}
	u, _ := url.Parse("https://h/a.b/Logo.PNG?x=1")
	if Ext(u) != "png" || Mime("Text/HTML; charset=utf-8") != "text/html" {
		t.Fatal("Ext, Mime")
	}
}

func TestParseRequestTarget(t *testing.T) {
	for _, target := range []string{"api/x", "@evil.test/", "evil.test"} {
		raw := "GET " + target + " HTTP/1.1\nHost: a.test\n\n"
		if r, err := ParseRequest([]byte(raw), "https"); err == nil {
			t.Errorf("%q: accepted, URL %s", target, r.URL)
		}
	}
	r, err := ParseRequest([]byte("OPTIONS * HTTP/1.1\nHost: a.test\n\n"), "https")
	if err != nil || r.URL.Host != "a.test" || !strings.HasPrefix(string(r.Bytes()), "OPTIONS * HTTP/1.1") {
		t.Errorf("OPTIONS *: %v %+v", err, r)
	}
	for _, host := range []string{"evil.test/x", "a@evil.test", "a.test?x", "/"} {
		if r, err := ParseRequest([]byte("GET / HTTP/1.1\nHost: "+host+"\n\n"), "https"); err == nil {
			t.Errorf("Host %q: accepted, URL %s", host, r.URL)
		}
	}
	if _, err := ParseRequest([]byte("GET http:// HTTP/1.1\n\n"), "https"); err == nil {
		t.Error("an absolute target without a host is accepted")
	}
	r, err = ParseRequest([]byte("GET http://a.test/x HTTP/1.1\nHost: b.test\n\n"), "https")
	if err != nil || r.Header.Get("Host") != "a.test" {
		t.Errorf("the target's host wins: %v %v", err, r.Header)
	}
}
