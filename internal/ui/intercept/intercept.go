// Package intercept handles the intercept page: the queues of held requests
// and responses, and the editor. Edits are kept per item until it is sent.
package intercept

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anotherhadi/foam/draw"
	feditor "github.com/anotherhadi/foam/editor"
	"github.com/anotherhadi/foam/highlight"
	"github.com/anotherhadi/foam/list"
	"github.com/anotherhadi/foam/theme"
	"github.com/anotherhadi/foam/vim"

	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/proxy"
	"github.com/anotherhadi/spilltea/internal/ui/entries"
)

type Queue struct {
	Kind  proxy.Kind
	items []*proxy.Item
	list  list.Model
}

func (q *Queue) Len() int { return len(q.items) }

func (q *Queue) Selected() *proxy.Item {
	if len(q.items) == 0 {
		return nil
	}
	return q.items[q.list.Selected()]
}

func (q *Queue) List() *list.Model { return &q.list }

func (q *Queue) add(it *proxy.Item) {
	q.items = append(q.items, it)
	q.list.SetLen(len(q.items))
}

func (q *Queue) remove(it *proxy.Item) bool {
	i := slices.Index(q.items, it)
	if i < 0 {
		return false
	}
	q.items = slices.Delete(q.items, i, i+1)
	q.list.SetLen(len(q.items))
	return true
}

func (q *Queue) Items() []*proxy.Item { return slices.Clone(q.items) }

type Model struct {
	Requests, Responses Queue

	drafts map[*proxy.Item]string
	holds  map[*proxy.Item]bool // requests whose response will be held too
	shown  *proxy.Item
	ed     *feditor.Model
	cache  highlight.Cache
	w, h   int
}

func New() *Model {
	return &Model{
		Requests:  Queue{Kind: proxy.KindRequest},
		Responses: Queue{Kind: proxy.KindResponse},
		drafts:    map[*proxy.Item]string{},
		holds:     map[*proxy.Item]bool{},
	}
}

func (m *Model) ToggleHold(it *proxy.Item) bool {
	m.holds[it] = !m.holds[it]
	return m.holds[it]
}

func (m *Model) Holds(it *proxy.Item) bool { return m.holds[it] }

func (m *Model) Queue(k proxy.Kind) *Queue {
	if k == proxy.KindRequest {
		return &m.Requests
	}
	return &m.Responses
}

func (m *Model) Add(it *proxy.Item) { m.Queue(it.Kind).add(it) }

func (m *Model) Remove(it *proxy.Item) {
	m.Queue(it.Kind).remove(it)
	delete(m.drafts, it)
	delete(m.holds, it)
	if m.shown == it {
		m.shown, m.ed = nil, nil
	}
}

func (m *Model) Len() int { return m.Requests.Len() + m.Responses.Len() }

func (m *Model) Has(it *proxy.Item) bool {
	return slices.Contains(m.Queue(it.Kind).items, it)
}

// Show restores the edits already made to it; nil clears the editor.
func (m *Model) Show(it *proxy.Item) {
	if it == m.shown {
		return
	}
	m.shown = it
	if it == nil {
		m.ed = nil
		return
	}
	text, ok := m.drafts[it]
	if !ok {
		text = message.Text(it.Raw)
	}
	m.ed = feditor.New(text)
	m.ed.Style = func(lines []string) []string { return m.cache.Message(lines, highlight.Options{}) }
	m.ed.SetSize(m.w, m.h)
}

func (m *Model) Shown() *proxy.Item { return m.shown }

func (m *Model) Text() string {
	if m.ed == nil {
		return ""
	}
	return m.ed.Text()
}

// Draft returns nil if the text is unchanged.
func (m *Model) Draft(it *proxy.Item) []byte {
	if it == m.shown && m.ed != nil {
		m.sync()
	}
	text, ok := m.drafts[it]
	if !ok || text == message.Text(it.Raw) {
		return nil
	}
	return []byte(text)
}

func (m *Model) Check(it *proxy.Item) error {
	d := m.Draft(it)
	if d == nil {
		return nil
	}
	var err error
	if it.Kind == proxy.KindRequest {
		_, err = message.ParseRequest(d, scheme(it.URL))
	} else {
		_, err = message.ParseResponse(d)
	}
	return err
}

func scheme(u string) string {
	if strings.HasPrefix(u, "http://") {
		return "http"
	}
	return "https"
}

func (m *Model) sync() {
	if m.ed != nil && m.shown != nil && m.ed.Changed() {
		m.drafts[m.shown] = m.ed.Text()
	}
}

func (m *Model) SetSize(w, h int) {
	m.w, m.h = w, h
	if m.ed != nil {
		m.ed.SetSize(w, h)
	}
}

// Busy: vim keeps the keys in insert or visual mode, or mid-command.
func (m *Model) Busy() bool {
	return m.ed != nil && (m.ed.Mode() != vim.Normal || m.ed.Pending() != "")
}

func (m *Model) Key(msg tea.KeyPressMsg) bool {
	if m.ed == nil {
		return false
	}
	ok := m.ed.Key(msg)
	m.sync()
	return ok
}

func (m *Model) Paste(s string) {
	if m.ed != nil {
		m.ed.Paste(s)
		m.sync()
	}
}

func (m *Model) Mouse(ev tea.MouseMsg) {
	if m.ed != nil {
		m.ed.Mouse(ev)
	}
}

func (m *Model) Yanked() (string, bool) {
	if m.ed == nil {
		return "", false
	}
	return m.ed.TakeYank()
}

func (m *Model) CursorPosition() (x, y int, ok bool) {
	if m.ed == nil {
		return 0, 0, false
	}
	return m.ed.CursorPosition()
}

func (m *Model) Scroll() draw.Scroll {
	if m.ed == nil {
		return draw.Scroll{}
	}
	return m.ed.Scroll()
}

func (m *Model) Title() string {
	if m.shown == nil {
		return "Detail"
	}
	what := "Request"
	if m.shown.Kind == proxy.KindResponse {
		what = "Response"
	}
	t := fmt.Sprintf("%s #%d · %s", what, m.shown.Seq, m.ed.Mode())
	if p := m.ed.Pending(); p != "" {
		t += " " + p
	}
	if _, edited := m.drafts[m.shown]; edited && m.Draft(m.shown) != nil {
		t += " · edited"
	}
	return t
}

func (m *Model) DetailView(w, h int, focused bool, empty string) string {
	if m.ed == nil {
		return draw.Hatch(w, h, empty)
	}
	return m.ed.View(focused)
}

func (m *Model) QueueView(q *Queue, w, h int, focused bool, empty string) string {
	q.list.SetHeight(h)
	if len(q.items) == 0 {
		return draw.Hatch(w, h, empty)
	}
	var lines []string
	start, end := q.list.Visible()
	for i := start; i < end; i++ {
		it := q.items[i]
		dot := " "
		if c, ok := highlight.PwnFoxColor(it.Color); ok {
			dot = c.Render(highlight.PwnFoxIcon())
		}
		edited := " "
		if _, ok := m.drafts[it]; ok && m.Draft(it) != nil {
			edited = theme.Warning().Render("✎")
		}
		meth := highlight.MethodColor(it.Method).Style().Bold(true).Render(fmt.Sprintf("%-7s", ansi.Truncate(draw.Clean(it.Method), 7, "")))
		if m.holds[it] {
			edited = theme.Accent().Render("↩")
		}
		row := dot + edited + theme.Subtle().Render(fmt.Sprintf("%4d ", it.Seq))
		if it.Kind == proxy.KindResponse {
			code := "   ?"
			if r, err := message.ParseResponse(it.Raw); err == nil {
				code = highlight.StatusColor(r.Status).Style().Bold(true).Render(fmt.Sprintf("%4d", r.Status))
			}
			row += code + " "
		}
		host, target := entries.Target(it.URL)
		row = ansi.Truncate(row+meth+" "+theme.Fg().Render(draw.Clean(host))+theme.Subtle().Render(draw.Clean(target)), w, "…")
		if i == q.list.Selected() {
			row = draw.Selected(row, w, focused)
		}
		lines = append(lines, row)
	}
	return draw.Fit(strings.Join(lines, "\n"), w, h)
}
