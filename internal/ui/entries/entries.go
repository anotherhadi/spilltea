// Package entries is the entry list, filtered with HTTPQL, with marks and
// notes. The history shows everything, the sitemap what is under a node.
package entries

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anotherhadi/foam/commands"
	"github.com/anotherhadi/foam/draw"
	"github.com/anotherhadi/foam/highlight"
	"github.com/anotherhadi/foam/input"
	"github.com/anotherhadi/foam/list"
	"github.com/anotherhadi/foam/notify"
	"github.com/anotherhadi/foam/overlay"
	"github.com/anotherhadi/foam/theme"
	"github.com/anotherhadi/httpql"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/query"
	"github.com/anotherhadi/spilltea/internal/ui/picker"
)

// Messages sent to the app, which does the actual work.
type (
	OpenMsg struct{}
	NoteMsg struct{ Entry project.Entry }
	DiffMsg struct {
		Entry project.Entry
		B     bool // right side
	}
	SteepMsg  struct{ Entry project.Entry }
	CopyMsg   struct{ Entry project.Entry }
	ForgetMsg struct {
		IDs      []int
		All      bool
		Filtered bool
	}
	// HistoryMsg carries the used filters, to be saved.
	HistoryMsg struct{ History []string }
	// MetaMsg: a mark changed. Lists share marks, so each one must
	// reapply its filter.
	MetaMsg struct{}
)

const historySize = 200

type filteredMsg struct {
	m       *Model
	gen     int
	matches map[int]bool
}

// Meta is shared between the app's lists.
type Meta = map[int]project.Meta

type Model struct {
	p    *project.Project
	meta Meta

	all    []project.Entry
	within func(project.Entry) bool
	assets bool  // shown
	shown  []int // indexes into all
	list   list.Model

	filter    input.Model
	filtering bool
	query     *httpql.Query[*query.Record]
	qerr      string
	matches   map[int]bool // by ID; nil when the filter needs no background pass
	pending   bool
	gen       int
	stale     *atomic.Bool

	filters []config.Filter
	presets map[string]string
	history []string
	histPos int
	draft   string

	Empty string
}

func New(p *project.Project, meta Meta, filters []config.Filter, history []string) *Model {
	m := &Model{p: p, meta: meta, filter: input.New("/ "), filters: filters, presets: map[string]string{},
		history: history, histPos: -1, Empty: "Nothing yet"}
	for _, f := range filters {
		m.presets[f.Name] = f.Query
	}
	m.filter.Placeholder = `resp.code.eq:200 AND req.path.cont:"admin"`
	return m
}

func (m *Model) SetHistory(h []string) { m.history = h }

func (m *Model) SetEntries(es []project.Entry) tea.Cmd {
	m.all = es
	return m.refilter()
}

// in ignores the HTTPQL filter.
func (m *Model) in(e project.Entry) bool {
	return (m.assets || !e.Asset) && (m.within == nil || m.within(e))
}

func (m *Model) SetAssets(show bool) tea.Cmd {
	m.assets = show
	return m.refilter()
}

// SetWithin keeps everything when within is nil.
func (m *Model) SetWithin(within func(project.Entry) bool) tea.Cmd {
	m.within = within
	m.list.Select(1 << 30) // the newest, at the bottom
	return m.refilter()
}

// Rewithin keeps the selection, unlike SetWithin.
func (m *Model) Rewithin(within func(project.Entry) bool) tea.Cmd {
	m.within = within
	return m.refilter()
}

// RefreshCheap reapplies a filter that does not read the messages.
func (m *Model) RefreshCheap() tea.Cmd {
	if m.query == nil || m.query.NeedsRaw() {
		return nil
	}
	return m.refilter()
}

func (m *Model) Filter() string { return m.filter.Value() }

// Add: if the selection is on the last entry, it follows the new one.
func (m *Model) Add(e project.Entry) {
	follow := len(m.shown) == 0 || m.list.Selected() == len(m.shown)-1
	m.all = append(m.all, e)
	if !m.in(e) {
		return
	}
	if m.query != nil {
		// a single entry to read from disk, no need for the background
		if !m.query.Match(m.record(e)) {
			return
		}
		if m.matches != nil {
			m.matches[e.ID] = true
		}
	}
	m.shown = append(m.shown, len(m.all)-1)
	m.list.SetLen(len(m.shown))
	if follow {
		m.list.Select(len(m.shown) - 1)
	}
}

// Forget only touches the list, not the project.
func (m *Model) Forget(ids []int) {
	drop := map[int]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	sel, ok := m.Selected()
	m.all = slices.DeleteFunc(m.all, func(e project.Entry) bool { return drop[e.ID] })
	m.reorder(sel.ID, ok)
}

func (m *Model) Len() int { return len(m.all) }

// All ignores the filter. The slice belongs to the list: do not modify it.
func (m *Model) All() []project.Entry { return m.all }

func (m *Model) Selected() (project.Entry, bool) {
	if len(m.shown) == 0 {
		return project.Entry{}, false
	}
	return m.all[m.shown[m.list.Selected()]], true
}

func (m *Model) SelectID(id int) bool {
	for i, k := range m.shown {
		if m.all[k].ID == id {
			m.list.Select(i)
			return true
		}
	}
	return false
}

func (m *Model) Shown() []project.Entry {
	out := make([]project.Entry, len(m.shown))
	for i, k := range m.shown {
		out[i] = m.all[k]
	}
	return out
}

func (m *Model) record(e project.Entry) *query.Record { return query.New(m.p, e, m.meta[e.ID]) }

func (m *Model) reorder(keepID int, keep bool) {
	m.shown = m.shown[:0]
	for i, e := range m.all {
		if !m.in(e) {
			continue
		}
		if m.query != nil && (m.matches != nil && !m.matches[e.ID] || m.matches == nil && !m.query.Match(m.record(e))) {
			continue
		}
		m.shown = append(m.shown, i)
	}
	m.list.SetLen(len(m.shown))
	if keep {
		for i, k := range m.shown {
			if m.all[k].ID == keepID {
				m.list.Select(i)
				return
			}
		}
	}
	m.list.Select(m.list.Selected())
}

// refilter runs queries that read raw messages in the background.
func (m *Model) refilter() tea.Cmd {
	sel, ok := m.Selected()
	m.gen++
	m.qerr, m.pending = "", false
	if m.stale != nil {
		m.stale.Store(true)
		m.stale = nil
	}
	q, err := query.Schema.ParsePresets(m.filter.Value(), m.presets)
	switch {
	case err != nil:
		m.qerr = err.Error()
		return nil // keep the last valid filter
	case m.filter.Value() == "":
		m.query, m.matches = nil, nil
	case q.NeedsRaw() && len(m.all) > 0:
		m.query, m.matches = q, map[int]bool{}
		m.pending = true
		m.reorder(sel.ID, ok)
		gen, p, all, meta := m.gen, m.p, slices.Clone(m.all), maps.Clone(m.meta)
		stale := new(atomic.Bool)
		m.stale = stale
		assets, within := m.assets, m.within
		in := func(e project.Entry) bool { return (assets || !e.Asset) && (within == nil || within(e)) }
		return func() tea.Msg {
			matches := map[int]bool{}
			for _, e := range all {
				if stale.Load() {
					return nil
				}
				if !in(e) {
					continue
				}
				if q.Match(query.New(p, e, meta[e.ID])) {
					matches[e.ID] = true
				}
			}
			return filteredMsg{m, gen, matches}
		}
	default:
		m.query, m.matches = q, nil
	}
	m.reorder(sel.ID, ok)
	return nil
}

func (m *Model) Update(msg tea.Msg) bool {
	f, ok := msg.(filteredMsg)
	if !ok || f.m != m {
		return false
	}
	if f.gen != m.gen {
		return true // another filter was typed since
	}
	sel, keep := m.Selected()
	for id := range m.matches {
		f.matches[id] = true // added while computing
	}
	m.matches, m.pending, m.stale = f.matches, false, nil
	m.reorder(sel.ID, keep)
	return true
}

// Refresh is called after a mark or note changes.
func (m *Model) Refresh() tea.Cmd {
	if m.query == nil {
		return nil
	}
	return m.refilter()
}

func (m *Model) Capturing() bool { return m.filtering }

func (m *Model) Input(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filtering = false
		return true, m.remember()
	case "esc", "ctrl+c":
		m.filtering = false
		return true, nil
	case "up":
		return true, m.browse(-1)
	case "down":
		return true, m.browse(1)
	case "ctrl+k", "ctrl+p":
		m.list.Move(-1)
		return true, nil
	case "ctrl+j", "ctrl+n":
		m.list.Move(1)
		return true, nil
	}
	if !m.filter.Update(msg) {
		return false, nil
	}
	m.histPos = -1
	return true, m.refilter()
}

func (m *Model) browse(d int) tea.Cmd {
	if m.histPos == -1 {
		if d > 0 || len(m.history) == 0 {
			return nil
		}
		m.draft, m.histPos = m.filter.Value(), len(m.history)
	}
	m.histPos = max(m.histPos+d, 0)
	if m.histPos >= len(m.history) {
		m.histPos = -1
		m.filter.SetValue(m.draft)
	} else {
		m.filter.SetValue(m.history[m.histPos])
	}
	return m.refilter()
}

func (m *Model) remember() tea.Cmd {
	m.histPos = -1
	q := strings.TrimSpace(m.filter.Value())
	if q == "" || m.qerr != "" || len(m.history) > 0 && m.history[len(m.history)-1] == q {
		return nil
	}
	m.history = append(slices.DeleteFunc(m.history, func(h string) bool { return h == q }), q)
	if len(m.history) > historySize {
		m.history = m.history[len(m.history)-historySize:]
	}
	return send(HistoryMsg{slices.Clone(m.history)})
}

func (m *Model) pickFilter() tea.Cmd {
	var items []picker.Item
	for _, f := range m.filters {
		items = append(items, picker.Item{Name: f.Name, Value: f.Query})
	}
	for i := len(m.history) - 1; i >= 0; i-- {
		items = append(items, picker.Item{Name: "recent", Value: m.history[i]})
	}
	return overlay.Open(picker.New("Filters", "filters", items, func(it picker.Item) tea.Cmd {
		m.filtering = false
		m.filter.SetValue(it.Value)
		return tea.Batch(m.refilter(), m.remember())
	}))
}

func (m *Model) SetFilter(q string) tea.Cmd {
	m.filter.SetValue(q)
	return m.refilter()
}

func (m *Model) toggleMark() tea.Cmd {
	e, ok := m.Selected()
	if !ok {
		return nil
	}
	meta := m.meta[e.ID]
	meta.Marked = !meta.Marked
	if err := m.SetMeta(e.ID, meta); err != nil {
		return notify.Err(err)
	}
	m.list.Move(1)
	return send(MetaMsg{})
}

func (m *Model) Meta(id int) project.Meta { return m.meta[id] }

func (m *Model) SetMeta(id int, meta project.Meta) error {
	if meta == (project.Meta{}) {
		delete(m.meta, id)
	} else {
		m.meta[id] = meta
	}
	return m.p.SetMeta(id, meta)
}

func (m *Model) Paste(s string) tea.Cmd {
	if !m.filtering {
		return nil
	}
	m.filter.Update(tea.PasteMsg{Content: s})
	return m.refilter()
}

func (m *Model) note() string {
	switch {
	case m.qerr != "":
		return m.qerr
	case m.pending:
		return "filtering"
	}
	return ""
}

func send(v tea.Msg) tea.Cmd { return func() tea.Msg { return v } }

func (m *Model) Commands() []commands.Command {
	selected := func() bool { _, ok := m.Selected(); return ok }
	with := func(f func(project.Entry) tea.Msg) func() tea.Cmd {
		return func() tea.Cmd {
			e, ok := m.Selected()
			if !ok {
				return nil
			}
			return send(f(e))
		}
	}
	return append(m.list.Commands(),
		commands.Command{ID: "entries.open", Help: "read", When: selected, Run: func() tea.Cmd { return send(OpenMsg{}) }},
		commands.Command{ID: "entries.filter", Help: "filter", Run: func() tea.Cmd { m.filtering, m.histPos = true, -1; return nil }},
		commands.Command{ID: "entries.filters", Title: "Pick a saved or recent filter", Run: m.pickFilter},
		commands.Command{ID: "entries.clear-filter", Title: "Clear the filter", When: func() bool { return m.filter.Value() != "" },
			Run: func() tea.Cmd { m.filter.Reset(); return m.refilter() }},
		commands.Command{ID: "entries.mark", Title: "Toggle the mark on the entry", Help: "mark", When: selected, Run: m.toggleMark},
		commands.Command{ID: "entries.note", Title: "Write a note on the entry", Help: "note", When: selected,
			Run: with(func(e project.Entry) tea.Msg { return NoteMsg{e} })},
		commands.Command{ID: "entries.diff-a", Title: "Diff: take the entry as the left side", When: selected,
			Run: with(func(e project.Entry) tea.Msg { return DiffMsg{Entry: e} })},
		commands.Command{ID: "entries.diff-b", Title: "Diff: take the entry as the right side, and compare", Help: "diff", When: selected,
			Run: with(func(e project.Entry) tea.Msg { return DiffMsg{Entry: e, B: true} })},
		commands.Command{ID: "entries.steep", Title: "Send the request to steep", Help: "steep", When: selected,
			Run: with(func(e project.Entry) tea.Msg { return SteepMsg{e} })},
		commands.Command{ID: "entries.copy", Title: "Copy the entry (request, curl, response or URL)", Help: "copy", When: selected,
			Run: with(func(e project.Entry) tea.Msg { return CopyMsg{e} })},
		commands.Command{ID: "entries.forget", Title: "Delete the selected entry", When: selected,
			Run: with(func(e project.Entry) tea.Msg { return ForgetMsg{IDs: []int{e.ID}} })},
		commands.Command{ID: "entries.forget-all", Title: "Delete the entries shown",
			When: func() bool { return len(m.shown) > 0 && !m.pending && m.qerr == "" },
			Run: func() tea.Cmd {
				ids := make([]int, len(m.shown))
				for i, k := range m.shown {
					ids[i] = m.all[k].ID
				}
				return send(ForgetMsg{IDs: ids, All: true, Filtered: m.filter.Value() != "" || m.within != nil || !m.assets})
			}},
	)
}

// Title is "History (18)", or "History (18/240)" with a filter.
func (m *Model) Title(name string) string {
	total := 0
	for _, e := range m.all {
		if m.in(e) {
			total++
		}
	}
	if len(m.shown) != total {
		return fmt.Sprintf("%s (%d/%d)", name, len(m.shown), total)
	}
	return fmt.Sprintf("%s (%d)", name, len(m.shown))
}

func (m *Model) Scroll() draw.Scroll { return m.list.Scroll() }

func (m *Model) header() int {
	n := 1
	if m.filtering || m.filter.Value() != "" {
		n += 2
		if m.note() != "" {
			n++
		}
	}
	return n
}

// Mouse: a click on the already-selected row opens it only if focused.
func (m *Model) Mouse(ev tea.MouseMsg, focused bool) tea.Cmd {
	mm := ev.Mouse()
	if _, click := ev.(tea.MouseClickMsg); click && mm.Y < m.header()-1 {
		m.filtering = true
		return nil
	}
	mm.Y -= m.header()
	switch ev.(type) {
	case tea.MouseClickMsg:
		if _, activate := m.list.Reselect(tea.MouseClickMsg(mm), focused); activate {
			return send(OpenMsg{})
		}
	case tea.MouseWheelMsg:
		m.list.Mouse(tea.MouseWheelMsg(mm))
	}
	return nil
}

// Size : 512, 12K, 1.5M.
func Size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 10*1024:
		return fmt.Sprintf("%.0fK", float64(n)/1024)
	}
	return strconv.FormatInt(n, 10)
}

func duration(ms int64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

func clock(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("15:04:05")
}

func Status(e project.Entry) string {
	switch {
	case e.Error == "dropped":
		return theme.Warning().Render("drop")
	case e.Error != "":
		return theme.Danger().Render(" err")
	case e.Status == 0:
		return theme.Subtle().Render("   -")
	}
	return highlight.StatusColor(e.Status).Style().Bold(true).Render(fmt.Sprintf("%4d", e.Status))
}

func Target(raw string) (host, target string) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", raw
	}
	t := u.EscapedPath()
	if t == "" {
		t = "/"
	}
	if u.RawQuery != "" {
		t += "?" + u.RawQuery
	}
	return u.Host, t
}

type column struct {
	name  string
	width int
	value func(project.Entry) string
}

// Optional columns, dropped from the end when space runs out.
var optional = []column{
	{"time", 8, func(e project.Entry) string { return clock(e.Time) }},
	{"rtt", 6, func(e project.Entry) string {
		if e.MS == 0 {
			if e.Status > 0 && e.Source == "" && e.Kept != project.KeptNone {
				return "<1ms" // measured, under a millisecond
			}
			return ""
		}
		return duration(e.MS)
	}},
	{"mime", 16, func(e project.Entry) string { return draw.Clean(e.Mime) }},
}

func visibleColumns(w int) []column {
	room := w - 48 // fixed columns plus minimal room for the URL
	var out []column
	for _, c := range optional {
		if room < c.width+1 {
			break
		}
		room -= c.width + 1
		out = append(out, c)
	}
	return out
}

func (m *Model) View(w, h int, focused bool) string {
	var lines []string
	if m.filtering || m.filter.Value() != "" {
		f := m.filter
		if !m.filtering {
			f.Update(tea.KeyPressMsg{Code: tea.KeyHome})
		}
		lines = append(lines, f.View(w, m.filtering))
		if n := m.note(); n != "" {
			c := theme.Subtle()
			if m.qerr != "" {
				c = theme.Danger()
			}
			lines = append(lines, c.Render(ansi.Truncate(n, w, "…")))
		}
		lines = append(lines, draw.Rule(w))
	}
	m.list.SetHeight(max(h-len(lines)-1, 0))
	cols := visibleColumns(w)
	head := "   " + fmt.Sprintf("%5s %-7s %4s %6s", "#", "method", "code", "size")
	for _, c := range cols {
		head += fmt.Sprintf(" %-*s", c.width, c.name)
	}
	lines = append(lines, theme.Subtle().Render(ansi.Truncate(head+" url", w, "")))
	start, end := m.list.Visible()
	for i := start; i < end; i++ {
		e := m.all[m.shown[i]]
		meta := m.meta[e.ID]
		mark := " "
		switch {
		case meta.Marked:
			mark = theme.Accent().Render("●")
		case meta.Note != "":
			mark = theme.Info().Render("✎")
		}
		dot := " "
		if c, ok := highlight.PwnFoxColor(e.Color); ok {
			dot = c.Render(highlight.PwnFoxIcon())
		}
		meth := highlight.MethodColor(e.Method).Style().Bold(true).Render(fmt.Sprintf("%-7s", ansi.Truncate(draw.Clean(e.Method), 7, "")))
		row := dot + mark + fmt.Sprintf(" %5d ", e.ID) + meth + " " + Status(e) + fmt.Sprintf(" %6s", Size(e.Len))
		for _, c := range cols {
			row += fmt.Sprintf(" %-*s", c.width, ansi.Truncate(c.value(e), c.width, "…"))
		}
		host, target := Target(e.URL)
		u := theme.Fg().Render(draw.Clean(host)) + theme.Subtle().Render(draw.Clean(target))
		if e.Asset {
			u = theme.Subtle().Render(draw.Clean(host + target))
		}
		if e.Edited {
			u += theme.Warning().Render(" edited")
		}
		for _, f := range m.p.FindingsOf(e.ID) {
			if f.Status != project.StatusNoise {
				u += theme.Danger().Render(" " + draw.Clean(f.Title))
			}
		}
		row = ansi.Truncate(row+" "+u, w, "…")
		if i == m.list.Selected() {
			row = draw.Selected(row, w, focused)
		}
		lines = append(lines, row)
	}
	if len(m.all) == 0 {
		lines = append(lines, theme.Subtle().Render(" "+ansi.Truncate(m.Empty, max(w-1, 0), "…")))
	} else if len(m.shown) == 0 && !m.pending {
		lines = append(lines, theme.Subtle().Render(" No match"))
	}
	return draw.Fit(strings.Join(lines, "\n"), w, h)
}
