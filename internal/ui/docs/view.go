package docs

import (
	"bytes"
	"text/template"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/style"
)

func windowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.S.Subtle).
		Padding(0, 0)
}

func (e Model) View() tea.View {
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		windowStyle().Render(e.viewport.View()),
		e.renderStatusBar(),
	))
}

func (m *Model) renderMarkdown() {
	cfg := config.Global
	data := struct {
		Cfg *config.Config
	}{
		Cfg: cfg,
	}

	tmpl, err := template.New("info").Parse(contentMarkdown)
	if err != nil {
		return
	}

	var processed bytes.Buffer
	if err := tmpl.Execute(&processed, data); err != nil {
		return
	}

	width := m.viewport.Width() - 2
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStyles(style.GlamourStyleConfig(cfg)),
		glamour.WithWordWrap(width),
	)

	str, _ := renderer.Render(processed.String())
	m.viewport.SetContent(str)
}
