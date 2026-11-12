package ui

import (
	"fmt"
	"net"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anotherhadi/foam/draw"
	"github.com/anotherhadi/foam/geom"
	"github.com/anotherhadi/foam/minsize"
	"github.com/anotherhadi/foam/theme"

	"github.com/anotherhadi/spilltea/internal/proxy"
)

var minSize = minsize.Size{W: 80, H: 20}

// The sitemap tree takes a quarter of the width, within these bounds.
const (
	treeMin = 24
	treeMax = 48
)

// showResponses: the responses pane only appears while responses are
// intercepted or some are still held.
func (m *Model) showResponses() bool {
	return m.px != nil && m.px.Holding(proxy.KindResponse) || m.inter.Responses.Len() > 0
}

func (m *Model) layout() {
	m.rects = [paneCount]geom.Rect{}
	if m.w == 0 || m.h == 0 {
		return
	}
	top, bottom := 1, 0
	if theme.HelpBar() {
		bottom = 1
	}
	if theme.Spacious() {
		top++
		if bottom > 0 {
			bottom++
		}
	}
	body := geom.Rect{X: 0, Y: top, W: m.w, H: max(m.h-top-bottom, 0)}
	switch m.page {
	case pageIntercept:
		rows := body.Rows(geom.Ratio(2), geom.Ratio(3))
		if m.showResponses() {
			cols := rows[0].Columns(geom.Ratio(1), geom.Ratio(1))
			m.rects[paneRequests], m.rects[paneResponses] = cols[0], cols[1]
		} else {
			m.rects[paneRequests] = rows[0]
			if m.focus[pageIntercept] == paneResponses {
				m.focus[pageIntercept] = paneRequests
			}
		}
		m.rects[paneDetail] = rows[1]
		r := rows[1]
		m.inter.SetSize(max(r.W-2, 0), max(r.H-2, 0))
	case pageHistory:
		rows := body.Rows(geom.Ratio(1), geom.Ratio(1))
		cols := rows[1].Columns(geom.Ratio(1), geom.Ratio(1))
		m.rects[paneHistory], m.rects[paneHistRequest], m.rects[paneHistResponse] = rows[0], cols[0], cols[1]
	case pageSitemap:
		tw := min(max(body.W/4, treeMin), treeMax)
		cols := body.Columns(geom.Fixed(tw), geom.Ratio(1))
		rows := cols[1].Rows(geom.Ratio(2), geom.Ratio(3))
		msgs := rows[1].Columns(geom.Ratio(1), geom.Ratio(1))
		m.rects[paneTree], m.rects[paneSite] = cols[0], rows[0]
		m.rects[paneSiteRequest], m.rects[paneSiteResponse] = msgs[0], msgs[1]
	case pageDiff:
		cols := body.Columns(geom.Ratio(1), geom.Ratio(1))
		m.rects[paneDiffA], m.rects[paneDiffB] = cols[0], cols[1]
	case pageFindings:
		rows := body.Rows(geom.Ratio(1), geom.Ratio(1))
		cols := rows[0].Columns(geom.Ratio(1), geom.Ratio(1))
		m.rects[paneFindings], m.rects[paneFinding], m.rects[paneSeen] = cols[0], cols[1], rows[1]
	}
	if m.zoomed {
		f := m.focus[m.page]
		for _, p := range pagePanes[m.page] {
			if p != f {
				m.rects[p] = geom.Rect{}
			}
		}
		m.rects[f] = body
		if f == paneDetail {
			m.inter.SetSize(max(body.W-2, 0), max(body.H-2, 0))
		}
	}
}

func (m *Model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.WindowTitle = "spilltea"
	if theme.Mouse() {
		v.MouseMode = tea.MouseModeCellMotion
	}
	if m.w == 0 || m.h == 0 {
		return v
	}
	if !minSize.Fits(m.w, m.h) {
		v.SetContent(minSize.View(m.w, m.h))
		return v
	}
	m.layout()

	c := lipgloss.NewCanvas(m.w, m.h)
	layers := []*lipgloss.Layer{lipgloss.NewLayer(m.topBar())}
	for _, p := range pagePanes[m.page] {
		r := m.rects[p]
		if r.Empty() {
			continue
		}
		layers = append(layers, lipgloss.NewLayer(m.paneView(p, r)).X(r.X).Y(r.Y))
	}
	if theme.HelpBar() {
		layers = append(layers, lipgloss.NewLayer(m.helpBar()).Y(m.h-1))
	}
	c.Compose(lipgloss.NewCompositor(layers...))
	draw.FillRow(c, 0, theme.Surface())
	if theme.HelpBar() {
		draw.FillRow(c, m.h-1, theme.Surface())
	}
	m.overlays.Draw(c)
	if t := m.toasts(); len(t) > 0 {
		c.Compose(lipgloss.NewCompositor(t...))
	}
	v.SetContent(c.Render())

	if m.overlays.Top() == nil && m.page == pageIntercept && m.focus[pageIntercept] == paneDetail {
		if x, y, ok := m.inter.CursorPosition(); ok {
			r := m.rects[paneDetail]
			cur := tea.NewCursor(r.X+1+x, r.Y+1+y)
			cur.Shape = tea.CursorBar
			v.Cursor = cur
		}
	}
	return v
}

var paneIcons = [paneCount]string{
	paneRequests:     "\uf093 ",
	paneResponses:    "\uf019 ",
	paneDetail:       "\uf044 ",
	paneHistory:      "\uf1da ",
	paneHistRequest:  "\uf1c9 ",
	paneHistResponse: "\uf01c ",
	paneTree:         "\uf0e8 ",
	paneSite:         "\uf0ce ",
	paneSiteRequest:  "\uf1c9 ",
	paneSiteResponse: "\uf01c ",
	paneDiffA:        "\uf0db ",
	paneDiffB:        "\uf0db ",
	paneFindings:     "\uf188 ",
	paneFinding:      "\uf05a ",
	paneSeen:         "\uf1da ",
}

func (m *Model) paneView(p pane, r geom.Rect) string {
	w, h := max(r.W-2, 0), max(r.H-2, 0)
	focused := p == m.focus[m.page] && m.overlays.Top() == nil
	o := draw.FrameOpts{Active: focused}
	title := ""
	var content string
	switch p {
	case paneRequests:
		title = fmt.Sprintf("Requests (%d)", m.inter.Requests.Len())
		o.Right = m.holdState(proxy.KindRequest)
		content = m.inter.QueueView(&m.inter.Requests, w, h, focused, m.queueEmpty(proxy.KindRequest))
		o.Scroll = m.inter.Requests.List().Scroll()
	case paneResponses:
		title = fmt.Sprintf("Responses (%d)", m.inter.Responses.Len())
		o.Right = m.holdState(proxy.KindResponse)
		content = m.inter.QueueView(&m.inter.Responses, w, h, focused, m.queueEmpty(proxy.KindResponse))
		o.Scroll = m.inter.Responses.List().Scroll()
	case paneDetail:
		title = m.inter.Title()
		if it := m.inter.Shown(); it != nil {
			o.Right = theme.Subtle().Render(draw.Clean(ansi.Truncate(it.URL, max(w/2, 10), "…")))
		}
		content = m.inter.DetailView(w, h, focused, "Nothing held")
		o.Scroll = m.inter.Scroll()
	case paneHistory:
		title = m.hist.Title("History")
		content = m.hist.View(w, h, focused)
		o.Scroll = m.hist.Scroll()
	case paneSite:
		title = m.site.Title("Entries")
		if m.siteNode != "" {
			title += " · " + draw.Clean(m.siteNode)
		}
		content = m.site.View(w, h, focused)
		o.Scroll = m.site.Scroll()
	case paneTree:
		title = "Sitemap"
		content = m.tree.View(w, h, focused)
		o.Scroll = m.tree.Scroll()
	case paneHistRequest, paneSiteRequest:
		title = "Request"
		rd := m.reader(p)
		content = rd.View(w, h, focused, "")
		o.Right, o.Scroll = rd.Status(), rd.Scroll()
	case paneHistResponse, paneSiteResponse:
		title = "Response"
		rd := m.reader(p)
		l := m.hist
		if p == paneSiteResponse {
			l = m.site
		}
		content = rd.View(w, h, focused, draw.Clean(m.noResponse(l)))
		o.Right, o.Scroll = rd.Status(), rd.Scroll()
	case paneDiffA, paneDiffB:
		s, other := m.diffA, m.diffB
		if p == paneDiffB {
			s, other = m.diffB, m.diffA
		}
		what := "requests"
		if m.diffResp {
			what = "responses"
		}
		title = "Diff · " + what
		if s.ok {
			title = fmt.Sprintf("#%d · %s %s", s.entry.ID, s.entry.Method, draw.Clean(s.entry.URL))
		}
		switch {
		case !s.ok || !other.ok:
			msg := fmt.Sprintf("%s: left side · %s: right side", m.keyOf("entries.diff-a"), m.keyOf("entries.diff-b"))
			content = draw.Hatch(w, h, msg)
		case m.diffing:
			content = draw.Hatch(w, h, "Comparing…")
		case p == paneDiffA:
			content = m.diff.Left(w, h)
			n := fmt.Sprintf("%d changes", m.diff.Changes())
			if m.diff.Changes() == 1 {
				n = "1 change"
			}
			o.Right = theme.Subtle().Render(what + " · " + n)
		default:
			content = m.diff.Right(w, h)
		}
		o.Scroll = m.diff.Scroll()
	case paneFindings:
		title = m.finds.Title()
		content = m.finds.View(w, h, focused)
		o.Scroll = m.finds.Scroll()
	case paneFinding:
		title = "Finding"
		if _, ok := m.finds.Selected(); ok {
			m.findView.SetSize(w, h)
			content = m.findView.View()
		} else {
			content = draw.Hatch(w, h, "Nothing selected")
		}
		o.Scroll = m.findView.Scroll()
	case paneSeen:
		title = m.seen.Title("Seen in")
		content = m.seen.View(w, h, focused)
		o.Scroll = m.seen.Scroll()
	}
	o.Title = theme.Icon(paneIcons[p], "") + title
	return draw.Frame(content, r.W, r.H, o)
}

func (m *Model) queueEmpty(k proxy.Kind) string {
	if m.px != nil && m.px.Holding(k) {
		return "Waiting"
	}
	id := "intercept.requests"
	if k == proxy.KindResponse {
		id = "intercept.responses"
	}
	return "Intercept off · " + m.keyOf(id)
}

func (m *Model) holdState(k proxy.Kind) string {
	if m.px != nil && m.px.Holding(k) {
		return theme.Warning().Style().Bold(true).Render("intercepting")
	}
	return theme.Subtle().Render("off")
}

// tabs also returns each tab's start column, for mouse clicks.
func (m *Model) tabs() (string, []int) {
	var b strings.Builder
	var starts []int
	x := ansi.StringWidth(m.brand())
	for p := range pageCount {
		label := pageNames[p]
		if p == pageIntercept && m.inter.Len() > 0 {
			label += fmt.Sprintf(" %d", m.inter.Len())
		}
		s := " " + label + " "
		st := theme.Subtle().Style()
		switch {
		case p == m.page:
			st = lipgloss.NewStyle().Foreground(theme.Bg().Value()).Background(theme.Primary().Value()).Bold(true)
		case p == pageIntercept && m.inter.Len() > 0:
			st = theme.Warning().Style().Bold(true)
		}
		tab := st.Render(s)
		if p == pageFindings {
			if n := m.finds.Untriaged(); n > 0 {
				tab += badge(n)
			}
		}
		starts = append(starts, x)
		b.WriteString(tab + " ")
		x += ansi.StringWidth(tab) + 1
	}
	return b.String(), append(starts, x)
}

func badge(n int) string {
	s := fmt.Sprint(n)
	if n > 9 {
		s = "9+"
	}
	return lipgloss.NewStyle().Foreground(theme.Bg().Value()).Background(theme.Warning().Value()).Bold(true).Render(" " + s + " ")
}

func (m *Model) tabAt(x int) (page, bool) {
	_, starts := m.tabs()
	for p := range pageCount {
		if x >= starts[p] && x < starts[p+1] {
			return p, true
		}
	}
	return 0, false
}

func (m *Model) brand() string {
	return theme.Accent().Style().Bold(true).Render(" "+theme.Icon("\U000f0d9f ", "")+"spilltea ") + " "
}

func (m *Model) topBar() string {
	tabs, _ := m.tabs()
	left := m.brand() + tabs
	var right []string
	for _, k := range []proxy.Kind{proxy.KindRequest, proxy.KindResponse} {
		if m.px != nil && m.px.Holding(k) {
			name := "req"
			if k == proxy.KindResponse {
				name = "resp"
			}
			right = append(right, theme.Warning().Style().Bold(true).Render("● "+name))
		}
	}
	right = append(right, theme.Subtle().Render(shortAddr(m.cfg.Proxy.Listen)+" · "+shortPath(m.proj.Root)+" "))
	r := strings.Join(right, "  ")
	pad := max(m.w-ansi.StringWidth(left)-ansi.StringWidth(r), 1)
	return ansi.Truncate(left+strings.Repeat(" ", pad)+r, m.w, "")
}

// helpBar drops hints until it fits, keeping the last one (help).
func (m *Model) helpBar() string {
	ks, ls := theme.Primary().Style(), theme.Subtle().Style()
	var parts []string
	for _, h := range m.reg.Hints(m.scopes()) {
		first, _, _ := strings.Cut(h.Keys, " / ")
		parts = append(parts, ks.Render(first)+" "+ls.Render(h.Label))
	}
	sep := ls.Render("  ·  ")
	line := func() string { return " " + strings.Join(parts, sep) }
	for len(parts) > 1 && ansi.StringWidth(line()) > m.w {
		parts = append(parts[:len(parts)-2], parts[len(parts)-1])
	}
	return draw.Pad(ansi.Truncate(line(), m.w, "…"), m.w)
}

func (m *Model) toasts() []*lipgloss.Layer {
	var out []*lipgloss.Layer
	w := min(48, max(m.w/3, 20), m.w)
	y := 0
	m.toastRects = m.toastRects[:0]
	live := m.notes.Live()
	for i := len(live) - 1; i >= 0 && i >= len(live)-5; i-- {
		s := live[i].Toast(w)
		r := geom.Rect{X: max(m.w-w, 0), Y: y, W: w, H: lipgloss.Height(s)}
		if len(out) > 0 && y+r.H > m.h {
			break
		}
		m.toastRects = append(m.toastRects, toastRect{r, live[i].ID})
		out = append(out, lipgloss.NewLayer(s).X(r.X).Y(r.Y).Z(20))
		y += r.H
	}
	return out
}

// shortAddr shortens a local address to its port: ":8080".
func shortAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return ":" + port
	}
	return addr
}

func shortPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(path, home); ok {
			return "~" + rest
		}
	}
	return path
}
