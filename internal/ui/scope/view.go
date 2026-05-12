package scope

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/style"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}

	s := style.S

	statusBar := m.renderStatusBar()
	statusH := strings.Count(statusBar, "\n") + 1
	panelH := m.height - statusH
	innerH := max(1, style.PanelContentH(panelH))

	taH := (innerH - fixedH) / 2
	if taH < minTaH {
		taH = minTaH
	}
	if taH > maxTaH {
		taH = maxTaH
	}

	var lines []string
	add := func(l string) { lines = append(lines, l) }

	add("")
	add(fieldLabel("Whitelist", m.focusIdx == fieldWhitelist))
	add(" " + s.Faint.Render("If non-empty, only matching requests are intercepted."))
	add("")
	wlContentLines := strings.Count(m.wlTextarea.Value(), "\n") + 1
	for _, l := range taLines(m.wlTextarea.View(), taH, wlContentLines) {
		add(" " + l)
	}

	add("")
	add(fieldLabel("Blacklist", m.focusIdx == fieldBlacklist))
	add(" " + s.Faint.Render("Matching requests are always excluded from history."))
	add("")
	blContentLines := strings.Count(m.blTextarea.Value(), "\n") + 1
	for _, l := range taLines(m.blTextarea.View(), taH, blContentLines) {
		add(" " + l)
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}
	content := strings.Join(lines[:innerH], "\n")

	panel := style.RenderWithTitle(s.PanelFocused, icons.I.Scope+"Scopes", content, m.width, panelH)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, panel, statusBar))
}

func fieldLabel(name string, focused bool) string {
	s := style.S
	c := s.MutedFg
	if focused {
		c = s.Primary
	}
	return "  " + lipgloss.NewStyle().Foreground(c).Bold(focused).Render(name)
}

func taLines(view string, h int, contentLines int) []string {
	raw := strings.Split(strings.TrimRight(view, "\n"), "\n")
	tilde := style.S.Faint.Render("~")
	for len(raw) < h {
		raw = append(raw, tilde)
	}
	if len(raw) > h {
		raw = raw[:h]
	}
	for i := contentLines; i < len(raw); i++ {
		raw[i] = tilde
	}
	return raw
}
