package copy

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/style"
)

const (
	popupW = 55
	popupH = 20
)

type OpenMsg struct {
	RawRequest string
	Scheme     string
	ShowURL    bool
}

type copyItem struct {
	id    string
	title string
	desc  string
}

func (c copyItem) Title() string       { return c.title }
func (c copyItem) Description() string { return c.desc }
func (c copyItem) FilterValue() string { return c.title }

var allItems = []list.Item{
	copyItem{"raw", "Raw", "full HTTP request"},
	copyItem{"headers", "Headers", "request headers only"},
	copyItem{"body", "Body", "request body only"},
	copyItem{"url", "URL", "request URL"},
}

type Model struct {
	open       bool
	list       list.Model
	rawRequest string
	scheme     string
	width      int
	height     int
}

func New() Model {
	s := style.S

	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Foreground(s.Text).PaddingLeft(2)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Foreground(s.Subtle).PaddingLeft(2)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(s.Primary).
		Foreground(s.Primary).Bold(true).PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(s.Primary).
		Foreground(s.MutedFg).PaddingLeft(1)

	l := list.New(allItems, delegate, popupW, 8)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)

	return Model{list: l}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) IsOpen() bool { return m.open }

func (m *Model) Open(msg OpenMsg) {
	m.rawRequest = msg.RawRequest
	m.scheme = msg.Scheme
	m.open = true
	items := allItems
	if !msg.ShowURL {
		filtered := make([]list.Item, 0, len(allItems))
		for _, it := range allItems {
			if it.(copyItem).id != "url" {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	m.list.SetItems(items)
	m.list.ResetFilter()
	m.list.Select(0)
	m.list.SetSize(m.popupInnerWidth(), m.listHeight())
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(m.popupInnerWidth(), m.listHeight())
}

func (m Model) popupInnerWidth() int {
	w := popupW
	if m.width > 0 && m.width-4 < w {
		w = m.width - 4
	}
	return w
}

func (m Model) popupHeight() int {
	h := popupH
	if m.height > 0 && m.height-4 < h {
		h = m.height - 4
	}
	if h < 6 {
		h = 6
	}
	return h
}

func (m Model) listHeight() int {
	return style.PanelContentH(m.popupHeight()) - 1
}

func (m Model) extract(id string) string {
	raw := m.rawRequest
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	switch id {
	case "raw":
		return raw

	case "headers":
		var sb strings.Builder
		for _, l := range lines[1:] {
			if l == "" {
				break
			}
			sb.WriteString(l + "\n")
		}
		return strings.TrimRight(sb.String(), "\n")

	case "body":
		for i, l := range lines {
			if l == "" && i > 0 {
				return strings.TrimRight(strings.Join(lines[i+1:], "\n"), "\n")
			}
		}
		return ""

	case "url":
		scheme := m.scheme
		if scheme == "" {
			scheme = "https"
		}
		var host, path string
		if len(lines) > 0 {
			parts := strings.SplitN(lines[0], " ", 3)
			if len(parts) >= 2 {
				path = parts[1]
			}
		}
		for _, l := range lines[1:] {
			if l == "" {
				break
			}
			if kv := strings.SplitN(l, ": ", 2); len(kv) == 2 && strings.EqualFold(kv[0], "host") {
				host = strings.TrimSpace(kv[1])
			}
		}
		return scheme + "://" + host + path
	}
	return raw
}
