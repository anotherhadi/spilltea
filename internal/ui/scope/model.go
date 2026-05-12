package scope

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

const (
	fieldNone      = -1
	fieldWhitelist = 0
	fieldBlacklist = 1
)

const (
	minTaH = 3
	maxTaH = 12
	fixedH = 8 // (blank + label + desc + blank) x2
)

type ScopeChangedMsg struct {
	Whitelist []string
	Blacklist []string
}

type Model struct {
	focusIdx int

	wlTextarea textarea.Model
	blTextarea textarea.Model

	innerH int
	width  int
	height int

	help help.Model
}

func New(name, path string) Model {
	wl := style.NewTextarea(true)
	wl.Placeholder = "one pattern per line..."

	bl := style.NewTextarea(true)
	bl.Placeholder = "one pattern per line..."
	bl.Blur()

	return Model{
		focusIdx:   fieldNone,
		wlTextarea: wl,
		blTextarea: bl,
		help:       style.NewHelp(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) SetScope(whitelist, blacklist []string) {
	m.wlTextarea.SetValue(strings.Join(whitelist, "\n"))
	m.blTextarea.SetValue(strings.Join(blacklist, "\n"))
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.syncLayout()
}

func (m *Model) syncLayout() {
	if m.width == 0 {
		return
	}
	m.help.SetWidth(m.width - 2)

	statusH := strings.Count(m.renderStatusBar(), "\n") + 1
	panelH := m.height - statusH
	m.innerH = max(1, style.PanelContentH(panelH))

	taH := (m.innerH - fixedH) / 2
	if taH < minTaH {
		taH = minTaH
	}
	if taH > maxTaH {
		taH = maxTaH
	}
	// width - 2 (panel border) - 1 (leading space in view) - 3 (right margin + cursor)
	taW := max(1, m.width-6)
	m.wlTextarea.SetWidth(taW)
	m.wlTextarea.SetHeight(taH)
	m.blTextarea.SetWidth(taW)
	m.blTextarea.SetHeight(taH)
}

func (m Model) IsEditing() bool {
	return m.focusIdx == fieldWhitelist || m.focusIdx == fieldBlacklist
}

func (m *Model) scopeChangedCmd() tea.Cmd {
	wl := parseLines(m.wlTextarea.Value())
	bl := parseLines(m.blTextarea.Value())
	return func() tea.Msg {
		return ScopeChangedMsg{Whitelist: wl, Blacklist: bl}
	}
}

func parseLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (m Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(
		m.help.View(formKeyMap{focusIdx: m.focusIdx}),
	)
}

type formKeyMap struct {
	focusIdx int
}

func (k formKeyMap) ShortHelp() []key.Binding {
	cycle := keys.Keys.Global.CycleFocus
	hlp := keys.Keys.Global.Help

	switch k.focusIdx {
	case fieldWhitelist, fieldBlacklist:
		esc := keys.Keys.Global.Escape
		escBinding := key.NewBinding(key.WithKeys(esc.Keys()...), key.WithHelp(esc.Help().Key, "unfocus"))
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "new line")),
			escBinding,
			cycle,
		}
	}
	return []key.Binding{cycle, hlp}
}

func (k formKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
