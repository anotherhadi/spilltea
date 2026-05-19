package home

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/ui/components/teapot"
)

type itemKind int

const (
	kindNew itemKind = iota
	kindTemp
	kindExisting
)

type listItem struct {
	kind    itemKind
	name    string
	path    string
	count   int
	modTime time.Time
}

func (i listItem) icon() string {
	ic := icons.I
	switch i.kind {
	case kindNew:
		return ic.New
	case kindTemp:
		return ic.Temp
	default:
		return ic.Project
	}
}

func (i listItem) title() string {
	switch i.kind {
	case kindNew:
		return "New Project"
	case kindTemp:
		return "Temporary Session"
	default:
		return i.name
	}
}

func (i listItem) description() string {
	switch i.kind {
	case kindNew:
		return "create and name a new project"
	case kindTemp:
		return "isolated session, deleted on exit"
	default:
		date := i.modTime.Format("Jan 2, 2006")
		if i.count == 1 {
			return fmt.Sprintf("1 request · %s", date)
		}
		return fmt.Sprintf("%d requests · %s", i.count, date)
	}
}

// FilterValue contains only the text (no icon) so fuzzy match indices map
// directly onto title() and don't need an offset to account for icon width.
func (i listItem) FilterValue() string { return i.title() }

type homeDelegate struct {
	normalTitle   lipgloss.Style
	normalDesc    lipgloss.Style
	selectedTitle lipgloss.Style
	selectedDesc  lipgloss.Style
	filterMatch   lipgloss.Style
}

func newHomeDelegate() homeDelegate {
	s := style.S
	leftBorder := lipgloss.Border{Left: "│"}
	return homeDelegate{
		normalTitle: lipgloss.NewStyle().Foreground(s.Text).PaddingLeft(4),
		normalDesc:  lipgloss.NewStyle().Foreground(s.Subtle).Faint(true).PaddingLeft(4),
		selectedTitle: lipgloss.NewStyle().
			Border(leftBorder, false, false, false, true).
			BorderForeground(s.Primary).
			Foreground(s.Primary).Bold(true).PaddingLeft(3),
		selectedDesc: lipgloss.NewStyle().
			Border(leftBorder, false, false, false, true).
			BorderForeground(s.Primary).
			Foreground(s.MutedFg).PaddingLeft(3),
		filterMatch: lipgloss.NewStyle().Underline(true),
	}
}

func (d homeDelegate) Height() int                             { return 2 }
func (d homeDelegate) Spacing() int                            { return 1 }
func (d homeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d homeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li := item.(listItem)
	selected := index == m.Index()

	// Apply match highlighting only to the title text
	// separately so its width never shifts the highlight indices.
	titleText := li.title()
	if m.IsFiltered() {
		if matches := m.MatchesForItem(index); len(matches) > 0 {
			base := lipgloss.NewStyle()
			titleText = lipgloss.StyleRunes(titleText, matches, d.filterMatch.Inherit(base), base)
		}
	}

	full := li.icon() + titleText
	var titleLine, descLine string
	if selected {
		titleLine = d.selectedTitle.Render(full)
		descLine = d.selectedDesc.Render(li.description())
	} else {
		titleLine = d.normalTitle.Render(full)
		descLine = d.normalDesc.Render(li.description())
	}
	fmt.Fprintf(w, "%s\n%s", titleLine, descLine)
}

type Project struct {
	Name    string
	Path    string
	Count   int
	ModTime time.Time
}

type inputMode int

const (
	modeSelect inputMode = iota
	modeNaming
)

const (
	baseHeaderLines = 1 + 1 + 1 + 2
	teapotMinH      = 28 // minimum terminal height to show the teapot
	maxInnerW       = 80 // max content width inside the padding box
	maxInnerH       = 50 // max content height inside the padding box
)

type Model struct {
	mode        inputMode
	list        list.Model
	projectDir  string
	nameInput   textinput.Model
	selected    *Project
	width       int
	height      int
	teapotFrame int
}

// Selected returns the project chosen by the user, or nil if the program was
// quit without making a selection.
func (m Model) Selected() *Project { return m.selected }

func New(projectDir string) Model {
	projects := loadProjects(projectDir)

	l := list.New(buildItems(projects), newHomeDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)

	ti := textinput.New()
	ti.Placeholder = "my-project"
	ti.CharLimit = 64
	ti.SetWidth(inputPanelMaxW - 2 - 4)

	return Model{
		projectDir: projectDir,
		list:       l,
		nameInput:  ti,
	}
}

func (m Model) Init() tea.Cmd { return teapotTick() }

func (m Model) innerW() int {
	w := m.width - 2
	if w > maxInnerW {
		w = maxInnerW
	}
	if w < 0 {
		return 0
	}
	return w
}

func (m Model) innerH() int {
	h := m.height - 2
	if h > maxInnerH {
		h = maxInnerH
	}
	if h < 0 {
		return 0
	}
	return h
}

func (m Model) headerHeight() int {
	if m.height > teapotMinH {
		// teapot block replaces 1 \n (else branch) with frame \n's + \n\n
		// net addition = FrameLines() (= frame_internal_\n + \n\n - else_\n)
		return baseHeaderLines + teapot.FrameLines()
	}
	return baseHeaderLines
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	lw := m.listWidth()
	lh := m.innerH() - m.headerHeight() - 1
	if lh < 0 {
		lh = 0
	}
	m.list.SetSize(lw, lh)
	m.nameInput.SetWidth(inputPanelInnerW(m.innerW()))
}

func (m Model) IsEditing() bool { return m.mode == modeNaming }

func (m Model) listWidth() int {
	return m.innerW()
}

func inputPanelInnerW(termW int) int {
	panelW := inputPanelMaxW
	if termW < panelW+4 {
		panelW = termW - 4
	}
	if panelW < 10 {
		panelW = 10
	}
	return panelW - 2 - 4 // border (2) + padding (2×2)
}

func loadProjects(projectDir string) []Project {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil
	}
	var projects []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dbPath := filepath.Join(projectDir, e.Name(), "data.db")
		info, err := os.Stat(dbPath)
		if err != nil {
			continue
		}
		projects = append(projects, Project{
			Name:    e.Name(),
			Path:    dbPath,
			Count:   db.CountEntriesAt(dbPath),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ModTime.After(projects[j].ModTime)
	})
	return projects
}

func buildItems(projects []Project) []list.Item {
	items := []list.Item{
		listItem{kind: kindNew},
		listItem{kind: kindTemp},
	}
	for _, p := range projects {
		items = append(items, listItem{
			kind:    kindExisting,
			name:    p.Name,
			path:    p.Path,
			count:   p.Count,
			modTime: p.ModTime,
		})
	}
	return items
}

func (m Model) renderHelpLine() string {
	s := style.S
	k := keys.Keys.Home
	fs := m.list.FilterState()

	kStyle := lipgloss.NewStyle().Foreground(s.MutedFg).Inline(true)
	dStyle := s.Faint.Inline(true)

	sep := s.Faint.Inline(true).Render(" • ")
	item := func(keyStr, desc string) string {
		return kStyle.Render(keyStr) + " " + dStyle.Render(desc)
	}
	binding := func(b key.Binding) string {
		return item(b.Help().Key, b.Help().Desc)
	}

	var parts []string
	if fs == list.Filtering {
		parts = append(parts, item("enter", "apply filter"))
		parts = append(parts, item("esc", "cancel"))
	} else {
		parts = append(parts, item("↑/↓", "navigate"))
		if fs == list.FilterApplied {
			parts = append(parts, item("esc", "clear filter"))
		} else {
			parts = append(parts, binding(k.Filter))
		}
		parts = append(parts, binding(k.Open))
		parts = append(parts, binding(k.Delete))
		parts = append(parts, item(keys.Keys.Global.Quit.Help().Key, "quit"))
	}

	return strings.Join(parts, sep)
}
