package copyas

import (
	"encoding/base64"
	"fmt"
	"os"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/style"
)

const (
	popupW = 61
	popupH = 20
)

// writeClipboard uses the OSC 52 terminal escape sequence to set the clipboard.
// Supported by most modern terminals (foot, kitty, wezterm, alacritty, xterm…).
func writeClipboard(text string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(os.Stderr, "\033]52;c;%s\a", encoded)
}

type OpenMsg struct {
	RawRequest string
	Scheme     string
}

type formatItem struct {
	id    string
	title string
	desc  string
}

func (f formatItem) Title() string       { return f.title }
func (f formatItem) Description() string { return f.desc }
func (f formatItem) FilterValue() string { return f.title }

var allFormats = []list.Item{
	formatItem{"raw", "Raw", "raw HTTP request"},
	formatItem{"curl", "cURL", "command line HTTP request"},
	formatItem{"python", "Python", "requests library"},
	formatItem{"go", "Go", "net/http package"},
	formatItem{"ffuf", "FFUF", "web fuzzer: FUZZ in query string"},
	formatItem{"markdown", "Markdown", "formatted for documentation"},
	formatItem{"har", "HAR", "HTTP Archive (JSON)"},
	formatItem{"httpie", "HTTPie", "HTTPie command line client"},
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

	l := list.New(allFormats, delegate, popupW, 8)
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

// listHeight = panel content area - hint line (1)
func (m Model) listHeight() int {
	return style.PanelContentH(m.popupHeight()) - 1
}
