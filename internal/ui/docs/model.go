package docs

import (
	"strings"

	spilltea "github.com/anotherhadi/spilltea"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

func readDoc(name string) string {
	b, _ := spilltea.DocsFS.ReadFile(".github/docs/" + name)
	return string(b)
}

var contentMarkdown = strings.Join([]string{
	readDoc("main.md"),
	readDoc("proxy.md"),
	readDoc("certificate.md"),
	readDoc("history.md"),
}, "\n")

type Model struct {
	viewport viewport.Model
	help     help.Model

	width  int
	height int
}

func New() Model {
	return Model{
		viewport: viewport.New(),
		help:     style.NewHelp(),
	}
}

func (e Model) Init() tea.Cmd {
	return nil
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.help.SetWidth(w - 2)

	statusH := strings.Count(m.renderStatusBar(), "\n") + 1
	frameW := windowStyle().GetHorizontalFrameSize()
	frameH := windowStyle().GetVerticalFrameSize()

	m.viewport.SetWidth(w - frameW)
	m.viewport.SetHeight(h - frameH - statusH)
	m.renderMarkdown()
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(docsKeyMap{width: m.width}))
}

type docsKeyMap struct{ width int }

func (docsKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	return []key.Binding{g.Up, g.Down, g.Help}
}

func (m docsKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.ScrollUp, g.ScrollDown}
	all := append(pageGlobals, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
