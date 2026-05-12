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
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

type Model struct {
	database *db.DB
	findings []db.Finding
	cursor   int

	listViewport viewport.Model
	bodyViewport viewport.Model
	pager        paginator.Model
	help         help.Model

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

	listVH := style.PanelContentH(listH) - 1 // -1 for the pager dots row
	if listVH < 0 {
		listVH = 0
	}
	m.listViewport.SetWidth(inner)
	m.listViewport.SetHeight(listVH)
	m.pager.PerPage = listVH
	if m.pager.PerPage < 1 {
		m.pager.PerPage = 1
	}

	bodyVH := style.PanelContentH(bodyH)
	m.bodyViewport.SetWidth(inner)
	m.bodyViewport.SetHeight(bodyVH)

	m.refreshListViewport()
	m.refreshBody()
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(findingsKeyMap{}))
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
	if len(m.findings) == 0 {
		m.bodyViewport.SetContent("")
		return
	}
	f := m.findings[m.cursor]
	rendered := renderMarkdown(f.Description, m.bodyViewport.Width())
	m.bodyViewport.SetContent(rendered)
	m.bodyViewport.GotoTop()
}

func renderMarkdown(src string, width int) string {
	if src == "" {
		return style.S.Faint.Render(" (ㆆ _ ㆆ)\nno description")
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
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style.GlamourStyleConfig(config.Global)),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return buf.String()
	}
	out, err := r.Render(buf.String())
	if err != nil {
		return buf.String()
	}
	return out
}

type findingsKeyMap struct{}

func (findingsKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	f := keys.Keys.Findings
	return []key.Binding{g.Up, g.Down, f.Dismiss}
}

func (findingsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{findingsKeyMap{}.ShortHelp()}
}
