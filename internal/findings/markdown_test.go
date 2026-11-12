package findings

import (
	"strings"
	"testing"
	"time"

	"github.com/anotherhadi/spilltea/internal/project"
)

func TestMarkdown(t *testing.T) {
	p, _ := project.Open(t.TempDir())
	gone, _ := p.Add(project.Entry{Method: "GET", URL: "https://a.test/old"}, []byte("GET /old HTTP/1.1\r\nHost: a.test\r\n\r\n"), nil)
	e, _ := p.Add(project.Entry{Method: "GET", URL: "https://a.test/app"}, []byte("GET /app HTTP/1.1\r\nHost: a.test\r\n\r\n"),
		[]byte("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<p>secret body</p>"))
	p.Forget([]int{gone.ID})
	f := project.Finding{Title: "q reflected in text", Detector: "reflection", Detail: "a.test/app\n```js\nx\n```",
		Entries: []int{gone.ID, e.ID}, First: time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local),
		Triage: project.Triage{Status: project.StatusRelevant, Note: "XSS\nconfirmed"}}
	got := Markdown(p, f)
	for _, want := range []string{
		"## q reflected in text\n\n- Detector: reflection\n- Seen in 2 requests, first on 2026-11-09 10:00\n- Note: XSS\n  confirmed\n",
		"\n````\na.test/app\n```js\nx\n```\n````\n",
		"### Request #2\n\n```http\nGET /app HTTP/1.1\nHost: a.test\n```\n",
		"### Response #2\n\n```http\nHTTP/1.1 200 OK\nContent-Type: text/html\n```\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret body") {
		t.Error("the response body is copied")
	}
	f.Title, f.Detail = "\x1b[31mevil", "a\x1b]52;c;x\x07b"
	if got := Markdown(p, f); strings.ContainsAny(got, "\x1b\x07") {
		t.Errorf("control characters are copied: %q", got)
	}
}
