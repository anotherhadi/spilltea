package plugins

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/plugins"
	"github.com/anotherhadi/spilltea/internal/style"
)

type Model struct {
	manager  *plugins.Manager
	items    []plugins.Info
	cursor   int
	editing  bool
	filter   string
	filtered []plugins.Info

	listViewport viewport.Model
	textarea     textarea.Model
	filterInput  textinput.Model
	filtering    bool
	pager        paginator.Model
	help         help.Model

	width  int
	height int
}

func New(mgr *plugins.Manager) Model {
	ta := style.NewTextarea(false)
	ta.Placeholder = "plugin configuration..."
	ta.Blur()

	fi := textinput.New()
	fi.Prompt = ""

	return Model{
		manager:      mgr,
		listViewport: style.NewViewport(),
		textarea:     ta,
		filterInput:  fi,
		pager:        style.NewPaginator(),
		help:         style.NewHelp(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) IsEditing() bool { return m.editing || m.filtering }

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

	listH, detailH := style.SplitH(m.height, m.renderStatusBar(), 0.4)

	inner := m.width - 2
	if inner < 0 {
		inner = 0
	}

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

	m.filterInput.SetWidth(inner - 2)
	m.textarea.SetWidth(max(1, inner-2))
	m.textarea.SetHeight(max(3, detailH-6))

	m.refreshListViewport()
}

// Refresh reloads the plugin list from the manager.
func (m *Model) Refresh() {
	if m.manager == nil {
		return
	}
	pl := m.manager.GetPlugins()
	m.items = make([]plugins.Info, len(pl))
	for i, p := range pl {
		m.items[i] = p.Info()
	}
	m.applyFilter()
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.items
	} else {
		f := strings.ToLower(m.filter)
		filtered := make([]plugins.Info, 0, len(m.items))
		for _, p := range m.items {
			if strings.Contains(strings.ToLower(p.Name), f) {
				filtered = append(filtered, p)
			}
		}
		m.filtered = filtered
	}
	m.pager.SetTotalPages(len(m.filtered))
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.refreshListViewport()
	m.syncTextarea()
}

func (m *Model) selected() (plugins.Info, bool) {
	if len(m.filtered) == 0 {
		return plugins.Info{}, false
	}
	return m.filtered[m.cursor], true
}

func (m *Model) syncTextarea() {
	if m.editing {
		return
	}
	info, ok := m.selected()
	if !ok {
		m.textarea.SetValue("")
		return
	}
	m.textarea.SetValue(info.ConfigText)
}

func (m *Model) refreshListViewport() {
	if m.pager.PerPage > 0 {
		m.pager.Page = m.cursor / m.pager.PerPage
		m.pager.SetTotalPages(len(m.filtered))
	}
	m.listViewport.SetContent(m.renderList())
}

func shortenPath(p string) string {
	home := os.Getenv("HOME")
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

type pluginsKeyMap struct{ editing bool }

func (k pluginsKeyMap) ShortHelp() []key.Binding {
	pk := keys.Keys.Plugins
	g := keys.Keys.Global
	if k.editing {
		esc := key.NewBinding(key.WithKeys(g.Escape.Keys()...), key.WithHelp(g.Escape.Help().Key, "save & exit"))
		return []key.Binding{esc}
	}
	return []key.Binding{pk.Toggle, pk.EditConfig, pk.Filter}
}

func (k pluginsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
