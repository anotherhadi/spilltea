// Package findings is the list of the findings page.
package findings

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anotherhadi/foam/commands"
	"github.com/anotherhadi/foam/draw"
	"github.com/anotherhadi/foam/list"
	"github.com/anotherhadi/foam/notify"
	"github.com/anotherhadi/foam/theme"

	"github.com/anotherhadi/spilltea/internal/project"
)

type (
	OpenMsg    struct{}
	NoteMsg    struct{ Finding project.Finding }
	TriagedMsg struct{}
)

type Model struct {
	p       *project.Project
	all     []project.Finding
	shown   []int
	list    list.Model
	triaged bool
}

func New(p *project.Project) *Model { return &Model{p: p} }

func (m *Model) Reload() error {
	fs, err := m.p.Findings()
	m.set(fs, true)
	return err
}

func (m *Model) set(fs []project.Finding, keep bool) {
	sel, ok := m.Selected()
	slices.SortStableFunc(fs, func(a, b project.Finding) int {
		if c := cmp.Compare(rank(a.Status), rank(b.Status)); c != 0 {
			return c
		}
		return strings.Compare(a.Detector, b.Detector)
	})
	m.all = fs
	m.shown = m.shown[:0]
	for i, f := range fs {
		if m.triaged || f.Status == project.StatusNew {
			m.shown = append(m.shown, i)
		}
	}
	m.list.SetLen(len(m.shown))
	if ok && keep {
		for i, k := range m.shown {
			if m.all[k].Key == sel.Key {
				m.list.Select(i)
				return
			}
		}
	}
	m.list.Select(m.list.Selected())
}

func rank(s project.Status) int {
	switch s {
	case project.StatusNew:
		return 0
	case project.StatusRelevant:
		return 1
	}
	return 2
}

func (m *Model) Untriaged() int {
	n := 0
	for _, f := range m.all {
		if f.Status == project.StatusNew {
			n++
		}
	}
	return n
}

func (m *Model) Selected() (project.Finding, bool) {
	if len(m.shown) == 0 {
		return project.Finding{}, false
	}
	return m.all[m.shown[m.list.Selected()]], true
}

func (m *Model) SetTriage(key string, t project.Triage) tea.Cmd {
	if err := m.p.SetTriage(key, t); err != nil {
		return notify.Err(err)
	}
	for i := range m.all {
		if m.all[i].Key == key {
			m.all[i].Triage = t
		}
	}
	at := m.list.Selected()
	m.set(m.all, false)
	if f, ok := m.Selected(); ok && f.Key == key {
		m.list.Select(at + 1)
	}
	return send(TriagedMsg{})
}

func (m *Model) status(s project.Status) tea.Cmd {
	f, ok := m.Selected()
	if !ok {
		return nil
	}
	return m.SetTriage(f.Key, project.Triage{Status: s, Note: f.Note})
}

func send(v tea.Msg) tea.Cmd { return func() tea.Msg { return v } }

func (m *Model) Commands() []commands.Command {
	return append(m.list.Commands(), m.TriageCommands()...)
}

// TriageCommands leave out the list's moves.
func (m *Model) TriageCommands() []commands.Command {
	selected := func() bool { _, ok := m.Selected(); return ok }
	is := func(s project.Status) func() bool {
		return func() bool { f, ok := m.Selected(); return ok && f.Status == s }
	}
	isNot := func(s project.Status) func() bool {
		return func() bool { f, ok := m.Selected(); return ok && f.Status != s }
	}
	title := "Show the triaged findings"
	if m.triaged {
		title = "Hide the triaged findings"
	}
	return []commands.Command{
		{ID: "findings.open", Title: "Go to the requests where it was seen", Help: "requests", When: selected,
			Run: func() tea.Cmd { return send(OpenMsg{}) }},
		{ID: "findings.relevant", Title: "Triage the finding: relevant", Help: "relevant", When: isNot(project.StatusRelevant),
			Run: func() tea.Cmd { return m.status(project.StatusRelevant) }},
		{ID: "findings.noise", Title: "Triage the finding: noise", Help: "noise", When: isNot(project.StatusNoise),
			Run: func() tea.Cmd { return m.status(project.StatusNoise) }},
		{ID: "findings.untriage", Title: "Set the finding back to new", When: func() bool { return selected() && !is(project.StatusNew)() },
			Run: func() tea.Cmd { return m.status(project.StatusNew) }},
		{ID: "findings.note", Title: "Write a note on the finding", Help: "note", When: selected,
			Run: func() tea.Cmd {
				f, _ := m.Selected()
				return send(NoteMsg{f})
			}},
		{ID: "findings.triaged", Title: title, Run: func() tea.Cmd {
			m.triaged = !m.triaged
			m.set(m.all, true)
			return nil
		}},
	}
}

func (m *Model) Title() string {
	if len(m.shown) != len(m.all) {
		return fmt.Sprintf("Findings (%d/%d)", len(m.shown), len(m.all))
	}
	return fmt.Sprintf("Findings (%d)", len(m.shown))
}

func (m *Model) Scroll() draw.Scroll { return m.list.Scroll() }

// Mouse: a click on the already-selected row opens it only if focused.
func (m *Model) Mouse(ev tea.MouseMsg, focused bool) tea.Cmd {
	mm := ev.Mouse()
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

func Icon(s project.Status) string {
	switch s {
	case project.StatusRelevant:
		return theme.Danger().Style().Bold(true).Render("!")
	case project.StatusNoise:
		return theme.Subtle().Render("·")
	}
	return theme.Warning().Render("●")
}

func Seen(f project.Finding) string {
	if f.Full() {
		return fmt.Sprintf("%d+", len(f.Entries))
	}
	return fmt.Sprint(len(f.Entries))
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

func (m *Model) View(w, h int, focused bool) string {
	m.list.SetHeight(h)
	var lines []string
	start, end := m.list.Visible()
	for i := start; i < end; i++ {
		f := m.all[m.shown[i]]
		dim := f.Status == project.StatusNoise
		title := theme.Fg().Style().Bold(!dim).Render(fmt.Sprintf("%-24s", ansi.Truncate(draw.Clean(f.Title), 24, "…")))
		if dim {
			title = theme.Subtle().Render(fmt.Sprintf("%-24s", ansi.Truncate(draw.Clean(f.Title), 24, "…")))
		}
		row := " " + Icon(f.Status) + " " + theme.Accent().Render(fmt.Sprintf("%-8s", ansi.Truncate(f.Detector, 8, ""))) + " " +
			title + theme.Subtle().Render(fmt.Sprintf(" %4s ", Seen(f))) + theme.Subtle().Render(draw.Clean(firstLine(f.Detail)))
		row = ansi.Truncate(row, w, "…")
		if i == m.list.Selected() {
			row = draw.Selected(row, w, focused)
		}
		lines = append(lines, row)
	}
	switch {
	case len(m.all) == 0:
		lines = append(lines, theme.Subtle().Render(" No findings yet"))
	case len(m.shown) == 0:
		lines = append(lines, theme.Subtle().Render(" Everything is triaged"))
	}
	return draw.Fit(strings.Join(lines, "\n"), w, h)
}

func Detail(f project.Finding) []string {
	status := map[project.Status]string{project.StatusNew: "new", project.StatusRelevant: "relevant", project.StatusNoise: "noise"}[f.Status]
	first, last := f.First.Local(), f.Last.Local()
	lastFmt := "15:04:05"
	if first.Format(time.DateOnly) != last.Format(time.DateOnly) {
		lastFmt = time.DateTime
	}
	ids := make([]string, len(f.Entries))
	for i, id := range f.Entries {
		ids[i] = fmt.Sprintf("#%d", id)
	}
	seen := fmt.Sprintf("seen in %s: %s", plural(len(f.Entries), "request"), strings.Join(ids, " "))
	if f.Full() {
		seen = fmt.Sprintf("seen in %d+ requests, the first ones: %s", len(f.Entries), strings.Join(ids, " "))
	}
	out := []string{
		f.Title,
		fmt.Sprintf("%s · %s · first seen %s, last %s", f.Detector, status, first.Format(time.DateTime), last.Format(lastFmt)),
		seen,
	}
	if f.Note != "" {
		for _, l := range strings.Split(f.Note, "\n") {
			out = append(out, "note: "+l)
		}
	}
	out = append(out, "")
	return append(out, strings.Split(f.Detail, "\n")...)
}

func plural(n int, one string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %ss", n, one)
}
