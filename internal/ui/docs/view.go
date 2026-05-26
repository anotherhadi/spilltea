package docs

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/charmbracelet/x/ansi"
)

func windowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ilovetui.S.Subtle).
		Padding(0, 0)
}

func (e Model) View() tea.View {
	statusBar := e.renderStatusBar()
	if len(e.matches) > 0 {
		var countText string
		if e.searching {
			countText = fmt.Sprintf("%d matches", len(e.matches))
		} else {
			countText = fmt.Sprintf("%d/%d", e.matchIndex+1, len(e.matches))
		}
		count := lipgloss.NewStyle().Padding(0, 1).
			Foreground(ilovetui.S.Muted).
			Render(countText)
		statusBar = lipgloss.JoinHorizontal(lipgloss.Top, statusBar, count)
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		windowStyle().Render(e.viewport.View()),
		statusBar,
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
		glamour.WithStyles(ilovetui.GlamourStyleConfig()),
		glamour.WithWordWrap(width),
	)

	str, _ := renderer.Render(processed.String())
	m.renderedLines = strings.Split(str, "\n")
	m.strippedLines = make([]string, len(m.renderedLines))
	for i, l := range m.renderedLines {
		m.strippedLines[i] = ansi.Strip(l)
	}
	m.applySearch()
}
