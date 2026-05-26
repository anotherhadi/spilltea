package findings

import (
	"bytes"
	"text/template"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/util"
)

type Model struct {
	database   *db.DB
	findings   []db.Finding
	cursor     int
	hasUnread  bool
	knownCount int

	listViewport viewport.Model
	bodyViewport viewport.Model
	pager        paginator.Model
	help         help.Model

	renderer      *glamour.TermRenderer
	rendererWidth int

	width  int
	height int
}

func New() Model {
	return Model{
		listViewport: style.NewViewport(),
		bodyViewport: style.NewViewport(),
		pager:        style.NewPaginator(),
		help:         style.NewHelp(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) HasUnread() bool { return m.hasUnread }
func (m *Model) ClearUnread()   { m.hasUnread = false; m.knownCount = len(m.findings) }

func (m *Model) CurrentMarkdown() string {
	if len(m.findings) == 0 {
		return ""
	}
	f := m.findings[m.cursor]
	return "# " + f.Title + "\n\n" + f.Description
}

func (m *Model) SetDB(d *db.DB) {
	m.database = d
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.recalcSizes()
}

func (m *Model) recalcSizes() {
	if m.width == 0 {
		return
	}
	m.help.SetWidth(m.width - 2)
	inner := m.width - 2

	listH, bodyH := style.SplitH(m.height, m.renderStatusBar(), 0.35)

	listVH := ilovetui.ContentHeight(listH) - 1 // -1 for the pager dots row
	if listVH < 0 {
		listVH = 0
	}
	m.listViewport.SetWidth(inner)
	m.listViewport.SetHeight(listVH)
	m.pager.PerPage = listVH
	if m.pager.PerPage < 1 {
		m.pager.PerPage = 1
	}

	bodyVH := ilovetui.ContentHeight(bodyH)
	m.bodyViewport.SetWidth(inner)
	m.bodyViewport.SetHeight(bodyVH)

	if m.rendererWidth != inner {
		m.renderer = nil
		m.rendererWidth = 0
	}

	m.refreshListViewport()
	m.refreshBody()
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(findingsKeyMap{width: m.width}))
}

// RefreshCmd loads findings from the database.
func RefreshCmd(d *db.DB) tea.Cmd {
	return func() tea.Msg {
		if d == nil {
			return FindingsLoadedMsg{}
		}
		list, err := d.LoadFindings()
		if err != nil {
			return FindingsLoadedMsg{Err: err}
		}
		return FindingsLoadedMsg{Findings: list}
	}
}

type FindingsLoadedMsg struct {
	Findings []db.Finding
	Err      error
}

func (m *Model) refreshBody() {
	m.refreshBodyScroll(true)
}

func (m *Model) refreshBodyKeepScroll() {
	m.refreshBodyScroll(false)
}

func (m *Model) refreshBodyScroll(reset bool) {
	if len(m.findings) == 0 {
		m.bodyViewport.SetContent("")
		return
	}
	f := m.findings[m.cursor]
	rendered := m.renderMarkdownCached(f.Description, m.bodyViewport.Width())
	m.bodyViewport.SetContent(rendered)
	if reset {
		m.bodyViewport.GotoTop()
	}
}

func (m *Model) renderMarkdownCached(src string, width int) string {
	if src == "" {
		return ilovetui.S.Faint.Render(util.CenterLines("(ㆆ _ ㆆ)", "no description"))
	}
	tmpl, err := template.New("").Parse(src)
	if err != nil {
		return src
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return src
	}
	if width < 10 {
		width = 80
	}
	// Rebuild renderer if width changed or not yet built.
	if m.renderer == nil || m.rendererWidth != width {
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(ilovetui.GlamourStyleConfig()),
			glamour.WithWordWrap(width),
		)
		if err == nil {
			m.renderer = r
			m.rendererWidth = width
		}
	}
	if m.renderer == nil {
		return buf.String()
	}
	out, err := m.renderer.Render(buf.String())
	if err != nil {
		return buf.String()
	}
	return out
}

type findingsKeyMap struct{ width int }

func (findingsKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	f := keys.Keys.Findings
	return []key.Binding{g.Up, g.Down, f.Dismiss, g.Copy, g.Help}
}

func (m findingsKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.ScrollUp, g.ScrollDown, g.Copy}
	all := append(keys.Keys.Findings.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
