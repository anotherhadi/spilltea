package diff

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSet(t *testing.T) {
	m := New()
	m.Set("GET /a HTTP/1.1\nHost: h\nCookie: id=1\nX: same\n", "GET /a HTTP/1.1\nHost: h\nCookie: id=2\nNew: line\nX: same\n")
	kinds := []kind{}
	for _, r := range m.rows {
		kinds = append(kinds, r.kind)
	}
	want := []kind{same, same, changed, added, same}
	if len(kinds) != len(want) {
		t.Fatalf("rows %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("rows %v, want %v", kinds, want)
		}
	}
	if m.Changes() != 1 {
		t.Fatalf("changes %d", m.Changes())
	}
	hot := ""
	for _, r := range m.rows[2].right.runs {
		if r.hot {
			hot += r.text
		}
	}
	if hot != "2" {
		t.Fatalf("what changed in the line: %q", hot)
	}
	left := strings.Split(ansi.Strip(m.Left(30, 10)), "\n")
	right := strings.Split(ansi.Strip(m.Right(30, 10)), "\n")
	if !strings.Contains(left[2], "-Cookie: id=1") || !strings.Contains(right[2], "+Cookie: id=2") ||
		!strings.Contains(left[3], "╱") || !strings.Contains(right[3], "+New: line") {
		t.Fatalf("left:\n%s\nright:\n%s", strings.Join(left, "\n"), strings.Join(right, "\n"))
	}
}
