package ui

import (
	"fmt"
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

// longModel is a model full of what does not fit: long hosts, long paths,
// long queries, long notes, long lines in the messages.
func longModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg, err := fconfig.LoadBytes[config.Config](foam.Defaults, config.Defaults)
	if err != nil {
		t.Fatal(err)
	}
	cfg.App.Project = t.TempDir()
	theme.Set(cfg.Theme)
	proj, _ := project.Open(cfg.App.Project)
	m, err := New(cfg, proj, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	host := "very-long-subdomain-name-for-testing." + strings.Repeat("deep.", 8) + "example.com:8443"
	path := "/" + strings.Repeat("segment-of-a-long-path/", 12) + "file.php"
	query := "?" + strings.Repeat("param=value&", 30) + "token=eyJhbGciOiJIUzI1NiJ9." + strings.Repeat("x", 300)
	body := `{"data": "` + strings.Repeat("é漢字 long line ", 80) + `"}`
	urls := []string{
		"https://" + host + path + query,
		"https://short.test/",
		"http://127.0.0.1:8080/" + strings.Repeat("a", 500),
		"https://" + host + "/x.png",
		"https://short.test/api/" + strings.Repeat("%2F", 80),
	}
	for i, u := range urls {
		req := []byte("POST " + strings.TrimPrefix(u, "https://"+host) + " HTTP/1.1\r\nHost: " + host + "\r\nCookie: " + strings.Repeat("c=1; ", 100) + "\r\n\r\n" + body)
		resp := []byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nX-Long: " + strings.Repeat("v", 400) + "\r\n\r\n" + body)
		e, err := proj.Add(project.Entry{Time: time.Now(), Method: []string{"POST", "GET", "OPTIONS", "PROPFIND", "VERYLONGMETHOD"}[i],
			URL: u, Status: 200 + i*100, Len: int64(len(body)) * 1000, Mime: "application/vnd.something-very-long+json", MS: 12345,
			Kept: project.KeptFull, Color: "red", Asset: strings.HasSuffix(u, ".png"), Edited: i == 0}, req, resp)
		if err != nil {
			t.Fatal(err)
		}
		do(m, eventMsg{proxy.EntryMsg{Entry: e}})
		if i == 0 {
			m.hist.SetMeta(e.ID, project.Meta{Marked: true, Note: strings.Repeat("a long note ", 30)})
		}
	}
	for i, u := range urls {
		it := &proxy.Item{Seq: i + 1, Kind: proxy.Kind(i % 2), Method: "POST", URL: u, Color: "blue", Time: time.Now(),
			Raw:     []byte("POST /x HTTP/1.1\r\nHost: " + host + "\r\n\r\n" + body),
			Request: []byte("POST /x HTTP/1.1\r\nHost: " + host + "\r\n\r\n")}
		if it.Kind == proxy.KindResponse {
			it.Raw = []byte("HTTP/1.1 500 Internal Server Error\r\nX: " + strings.Repeat("y", 300) + "\r\n\r\n" + body)
		}
		do(m, eventMsg{proxy.HeldMsg{Item: it}})
	}
	all := m.hist.All()
	m.diffA, m.diffB = side{all[0], true}, side{all[2], true}
	m.tree.SetEntries(nil)
	return m
}

var borders = "█│╭╮╰╯─┃┏┓┗┛━║╔╗╚╝═┌┐└┘"

// checkFrames tells what overflows: a line wider than the window, or a pane
// whose right border is not where it should be.
func checkFrames(t *testing.T, m *Model, label string) {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > m.w {
			t.Fatalf("%s: line %d is %d wide in %d:\n%s", label, i, w, m.w, ansi.Strip(l))
		}
	}
	if m.overlays.Top() != nil {
		return
	}
	for _, p := range pagePanes[m.page] {
		r := m.rects[p]
		if r.Empty() {
			continue
		}
		for y := r.Y; y < r.Y+r.H && y < len(lines); y++ {
			for _, x := range []int{r.X, r.X + r.W - 1} {
				c := ansi.Strip(ansi.Cut(lines[y], x, x+1))
				if !strings.Contains(borders, c) || c == "" {
					if y == 0 || toastAt(m, x, y) {
						continue
					}
					t.Fatalf("%s: pane %d, row %d, col %d is %q, not a border:\n%s", label, p, y, x, c, ansi.Strip(lines[y]))
				}
			}
		}
	}
}

func toastAt(m *Model, x, y int) bool {
	for _, r := range m.toastRects {
		if r.Contains(x, y) {
			return true
		}
	}
	return false
}

func TestResponsiveLong(t *testing.T) {
	m := longModel(t)
	m.notes.Dismiss()
	sizes := [][2]int{{80, 20}, {81, 21}, {97, 33}, {120, 30}, {133, 27}, {160, 45}, {220, 60}, {300, 80}}
	for _, size := range sizes {
		do(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for p := range pageCount {
			m.setPage(p)
			if p == pageSitemap {
				do(m, nil)
				typeKeys(m, "L") // every folder open: the deepest paths
			}
			for _, z := range []bool{false, true} {
				m.zoomed = z
				for _, pn := range pagePanes[p] {
					m.setFocus(pn)
					do(m, nil)
					checkFrames(t, m, fmt.Sprintf("%dx%d page %d pane %d zoom %v", size[0], size[1], p, pn, z))
				}
			}
			m.zoomed = false
		}
	}
}

// TestResponsiveScroll scrolls every pane to its end: the scrollbars and the
// last rows must still fit.
func TestResponsiveScroll(t *testing.T) {
	m := longModel(t)
	m.notes.Dismiss()
	do(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	for p := range pageCount {
		m.setPage(p)
		for _, pn := range pagePanes[p] {
			m.setFocus(pn)
			do(m, nil)
			typeKeys(m, "G")
			if pn == paneDetail {
				key(m, tea.KeyEscape, 0)
			}
			checkFrames(t, m, fmt.Sprintf("page %d pane %d at the end", p, pn))
		}
	}
	// the overlays too: help, palette, the note prompt
	m.setFocus(paneHistory)
	for _, k := range []string{"?", "n"} {
		typeKeys(m, k)
		checkFrames(t, m, "overlay "+k)
		key(m, tea.KeyEscape, 0)
	}
	key(m, 'p', tea.ModCtrl)
	checkFrames(t, m, "palette")
	key(m, tea.KeyEscape, 0)
}
