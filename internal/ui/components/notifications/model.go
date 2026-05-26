package notifications

import (
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/charmbracelet/x/ansi"
)

type Kind string

const (
	KindInfo    Kind = "info"
	KindSuccess Kind = "success"
	KindWarning Kind = "warning"
	KindError   Kind = "error"
)

type NotificationMsg struct {
	Title string
	Body  string
	Kind  Kind
}

type DismissMsg struct{ ID int }

type notification struct {
	id    int
	title string
	body  string
	kind  Kind
}

type Model struct {
	queue  []notification
	nextID int
	width  int
	height int
}

func New() Model { return Model{} }

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m Model) HasNotifications() bool {
	return len(m.queue) > 0
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NotificationMsg:
		n := notification{id: m.nextID, title: msg.Title, body: msg.Body, kind: msg.Kind}
		m.nextID++
		m.queue = append(m.queue, n)
		return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg { return DismissMsg{ID: n.id} })
	case DismissMsg:
		for i, n := range m.queue {
			if n.id == msg.ID {
				m.queue = append(m.queue[:i], m.queue[i+1:]...)
				break
			}
		}
	}
	return m, nil
}

func (m Model) View(background string) string {
	if len(m.queue) == 0 {
		return background
	}

	const popupW = 34

	var popups []string
	start := len(m.queue) - 3
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.queue); i++ {
		n := m.queue[i]
		var accent color.Color
		switch n.kind {
		case KindSuccess:
			accent = ilovetui.S.Success
		case KindWarning:
			accent = ilovetui.S.Warning
		case KindError:
			accent = ilovetui.S.Error
		default:
			accent = ilovetui.S.Primary
		}

		titleStr := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(n.title)
		bodyStr := lipgloss.NewStyle().Foreground(ilovetui.S.Text).Width(popupW).Render(n.body)

		inner := lipgloss.JoinVertical(lipgloss.Left, titleStr, bodyStr)
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1).
			Render(inner)
		popups = append(popups, box)
	}

	popup := strings.Join(popups, "\n")
	return overlayTopRight(background, popup, m.width, m.height)
}

func overlayTopRight(bg, popup string, w, h int) string {
	bgLines := strings.Split(bg, "\n")

	popupLines := strings.Split(popup, "\n")
	popupH := len(popupLines)
	popupW := 0
	for _, l := range popupLines {
		if vw := lipgloss.Width(l); vw > popupW {
			popupW = vw
		}
	}

	const marginTop = 1
	const marginRight = 2
	startY := marginTop
	startX := w - popupW - marginRight
	if startX < 0 {
		startX = 0
	}

	result := make([]string, h)
	for y := 0; y < h; y++ {
		bgLine := ""
		if y < len(bgLines) {
			bgLine = bgLines[y]
		}

		popupY := y - startY
		if popupY >= 0 && popupY < popupH {
			prefix := ansi.Truncate(bgLine, startX, "")
			suffix := ansi.TruncateLeft(bgLine, startX+popupW, "")
			result[y] = prefix + popupLines[popupY] + suffix
		} else {
			result[y] = bgLine
		}
	}

	return strings.Join(result, "\n")
}
