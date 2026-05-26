package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	normal := m.renderNormal()

	if m.copyAs.IsOpen() {
		v := tea.NewView(m.copyAs.View(normal))
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	if m.copy.IsOpen() {
		v := tea.NewView(m.copy.View(normal))
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	rendered := normal
	if m.notifications.HasNotifications() {
		rendered = m.notifications.View(normal)
	}

	v := tea.NewView(rendered)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderNormal() string {
	sidebar := m.renderSidebar()
	content := m.renderActivePage()
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
}

func (m *Model) renderActivePage() string {
	for _, e := range pageRegistry {
		if e.id == m.page && e.render != nil {
			return e.render(m)
		}
	}
	return ilovetui.S.Faint.Render("Work in progress")
}
