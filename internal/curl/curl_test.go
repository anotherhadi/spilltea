package curl

import (
	"strings"
	"testing"

	"github.com/anotherhadi/spilltea/internal/message"
)

func TestFormat(t *testing.T) {
	req, err := message.ParseRequest([]byte("POST /a?b=1 HTTP/1.1\nHost: h.test\nAccept-Encoding: gzip\nContent-Length: 9\nX-A: it's\n\n{\"a\": 1}"), "https")
	if err != nil {
		t.Fatal(err)
	}
	want := "curl \\\n  -X 'POST' \\\n  'https://h.test/a?b=1' \\\n  -H 'X-A: it'\\''s' \\\n  --compressed \\\n  --data-raw '{\"a\": 1}'"
	if got := Format(req); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestFormatUnsafe(t *testing.T) {
	req, err := message.ParseRequest([]byte("POST /api/users/{}/x?a=[1] HTTP/1.1\nHost: h.test\n\n@/etc/passwd"), "https")
	if err != nil {
		t.Fatal(err)
	}
	got := Format(req)
	if !strings.Contains(got, "--globoff") || !strings.Contains(got, "--data-raw '@/etc/passwd'") || strings.Contains(got, "--data-binary") {
		t.Fatalf("got\n%s", got)
	}
}
