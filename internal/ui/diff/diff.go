// Package diff compares two messages side by side like git: removed lines on
// the left, added lines on the right, and what changed within a line.
package diff

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/anotherhadi/foam/commands"
	"github.com/anotherhadi/foam/draw"
	"github.com/anotherhadi/foam/theme"
)

type kind int

const (
	same kind = iota
	changed
	removed // left only
	added   // right only
)

// cell.num is 0 when that side has no line.
type cell struct {
	num  int
	runs []run
}

type run struct {
	text string
	hot  bool // the changed part of a changed line
}

type row struct {
	kind        kind
	left, right cell
}

type Model struct {
	rows  []row
	hunks []int // first row of each change
	y, x  int
	h     int
	has   bool
}

func New() *Model { return &Model{} }

func (m *Model) Has() bool { return m.has }

func (m *Model) Changes() int { return len(m.hunks) }

func lines(s string) []string {
	s = strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Set scrolls to the first change.
func (m *Model) Set(a, b string) {
	a, b = strings.ReplaceAll(a, "\r\n", "\n"), strings.ReplaceAll(b, "\r\n", "\n")
	dmp := diffmatchpatch.New()
	ca, cb, table := dmp.DiffLinesToChars(a, b)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(ca, cb, false), table)
	m.rows, m.hunks, m.has = nil, nil, true
	ln, rn := 0, 0
	var dels, ins []string
	flush := func() {
		if len(dels) == 0 && len(ins) == 0 {
			return
		}
		m.hunks = append(m.hunks, len(m.rows))
		for i := 0; i < max(len(dels), len(ins)); i++ {
			r := row{kind: changed}
			switch {
			case i >= len(dels):
				rn++
				r.kind, r.right = added, cell{rn, []run{{text: ins[i]}}}
			case i >= len(ins):
				ln++
				r.kind, r.left = removed, cell{ln, []run{{text: dels[i]}}}
			default:
				ln++
				rn++
				l, rr := inline(dmp, dels[i], ins[i])
				r.left, r.right = cell{ln, l}, cell{rn, rr}
			}
			m.rows = append(m.rows, r)
		}
		dels, ins = nil, nil
	}
	for _, d := range diffs {
		ls := lines(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			dels = append(dels, ls...)
		case diffmatchpatch.DiffInsert:
			ins = append(ins, ls...)
		default:
			flush()
			for _, l := range ls {
				ln++
				rn++
				m.rows = append(m.rows, row{kind: same, left: cell{ln, []run{{text: l}}}, right: cell{rn, []run{{text: l}}}})
			}
		}
	}
	flush()
	m.y, m.x = 0, 0
	if len(m.hunks) > 0 {
		m.y = max(m.hunks[0]-3, 0)
	}
}

func inline(dmp *diffmatchpatch.DiffMatchPatch, a, b string) (left, right []run) {
	ds := dmp.DiffCleanupSemantic(dmp.DiffMain(a, b, false))
	for _, d := range ds {
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			left = append(left, run{d.Text, true})
		case diffmatchpatch.DiffInsert:
			right = append(right, run{d.Text, true})
		default:
			left = append(left, run{d.Text, false})
			right = append(right, run{d.Text, false})
		}
	}
	return left, right
}

func (m *Model) Clear() { m.rows, m.hunks, m.has = nil, nil, false }

func (m *Model) clamp() {
	m.y = max(min(m.y, len(m.rows)-m.h), 0)
	m.x = max(m.x, 0)
}

func (m *Model) scroll(dy, dx int) tea.Cmd {
	m.y += dy
	m.x += dx
	m.clamp()
	return nil
}

func (m *Model) jump(d int) tea.Cmd {
	cur := m.y + 3
	if d > 0 {
		for _, h := range m.hunks {
			if h > cur {
				m.y = h - 3
				break
			}
		}
	} else {
		for i := len(m.hunks) - 1; i >= 0; i-- {
			if m.hunks[i] < cur {
				m.y = m.hunks[i] - 3
				break
			}
		}
	}
	m.clamp()
	return nil
}

// Commands reuse the viewer's bindings but scroll both sides together.
func (m *Model) Commands() []commands.Command {
	page := func() int { return max(m.h-1, 1) }
	return []commands.Command{
		{ID: "viewer.down", Help: "scroll", Run: func() tea.Cmd { return m.scroll(1, 0) }},
		{ID: "viewer.up", Run: func() tea.Cmd { return m.scroll(-1, 0) }},
		{ID: "viewer.left", Run: func() tea.Cmd { return m.scroll(0, -4) }},
		{ID: "viewer.right", Run: func() tea.Cmd { return m.scroll(0, 4) }},
		{ID: "viewer.page-down", Run: func() tea.Cmd { return m.scroll(page(), 0) }},
		{ID: "viewer.page-up", Run: func() tea.Cmd { return m.scroll(-page(), 0) }},
		{ID: "viewer.half-down", Run: func() tea.Cmd { return m.scroll(m.h/2, 0) }},
		{ID: "viewer.half-up", Run: func() tea.Cmd { return m.scroll(-m.h/2, 0) }},
		{ID: "viewer.top", Run: func() tea.Cmd { m.y = 0; return nil }},
		{ID: "viewer.bottom", Run: func() tea.Cmd { m.y = len(m.rows); m.clamp(); return nil }},
		{ID: "diff.next", Title: "Go to the next change", Help: "next change", When: func() bool { return len(m.hunks) > 0 }, Run: func() tea.Cmd { return m.jump(1) }},
		{ID: "diff.prev", Title: "Go to the previous change", When: func() bool { return len(m.hunks) > 0 }, Run: func() tea.Cmd { return m.jump(-1) }},
	}
}

func (m *Model) Mouse(msg tea.MouseMsg) {
	if w, ok := msg.(tea.MouseWheelMsg); ok {
		switch w.Button {
		case tea.MouseWheelUp:
			m.scroll(-3, 0)
		case tea.MouseWheelDown:
			m.scroll(3, 0)
		}
	}
}

func (m *Model) SetHeight(h int) {
	m.h = max(h, 0)
	m.clamp()
}

func (m *Model) Scroll() draw.Scroll {
	return draw.Scroll{Total: len(m.rows), Visible: m.h, Offset: m.y}
}

func (m *Model) Left(w, h int) string  { return m.side(w, h, false) }
func (m *Model) Right(w, h int) string { return m.side(w, h, true) }

func (m *Model) side(w, h int, right bool) string {
	m.SetHeight(h)
	numW := len(strconv.Itoa(len(m.rows))) + 1
	textW := max(w-numW-1, 0)
	var out []string
	for i := m.y; i < len(m.rows) && i < m.y+h; i++ {
		r := m.rows[i]
		c, mine := r.left, removed
		base, hot := theme.Danger().Style(), lipgloss.NewStyle().Foreground(theme.Bg().Value()).Background(theme.Danger().Value())
		if right {
			c, mine = r.right, added
			base, hot = theme.Success().Style(), lipgloss.NewStyle().Foreground(theme.Bg().Value()).Background(theme.Success().Value())
		}
		gutter := strings.Repeat(" ", numW)
		if c.num > 0 {
			gutter = theme.Subtle().Render(draw.Pad(strconv.Itoa(c.num), numW-1)) + " "
		}
		switch {
		case r.kind == same:
			base = theme.Fg().Style()
		case r.kind != changed && r.kind != mine:
			// the line belongs to the other side
			out = append(out, gutter+" "+theme.Muted().Render(strings.Repeat("╱", textW)))
			continue
		}
		mark := " "
		if r.kind != same {
			mark = base.Render(map[bool]string{false: "-", true: "+"}[right])
		}
		out = append(out, gutter+mark+render(c.runs, m.x, textW, base, hot))
	}
	return draw.Fit(strings.Join(out, "\n"), w, h)
}

// render draws from column x, over w columns.
func render(runs []run, x, w int, base, hot lipgloss.Style) string {
	var b strings.Builder
	pos, used := 0, 0
	for _, r := range runs {
		t := draw.Clean(strings.ReplaceAll(r.text, "\t", "    "))
		tw := ansi.StringWidth(t)
		if pos+tw <= x {
			pos += tw
			continue
		}
		if pos < x {
			t = ansi.Cut(t, x-pos, tw)
			tw = ansi.StringWidth(t)
			pos = x
		}
		if used+tw > w {
			t = ansi.Truncate(t, w-used, "")
			tw = ansi.StringWidth(t)
		}
		st := base
		if r.hot {
			st = hot
		}
		b.WriteString(st.Render(t))
		used += tw
		pos += tw
		if used >= w {
			break
		}
	}
	return b.String()
}
