package style

import (
	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
)

type Styles struct {
	PanelEditing lipgloss.Style

	PagerDotActive   string
	PagerDotInactive string

	methodGet      lipgloss.Style
	methodPost     lipgloss.Style
	methodPutPatch lipgloss.Style
	methodDelete   lipgloss.Style
	methodDefault  lipgloss.Style
}

var S *Styles

func Init() {
	methodBase := lipgloss.NewStyle().Bold(true).Width(7)
	S = &Styles{
		PanelEditing: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ilovetui.S.Base0E),

		PagerDotActive:   lipgloss.NewStyle().Foreground(ilovetui.S.Primary).SetString("•").String(),
		PagerDotInactive: lipgloss.NewStyle().Foreground(ilovetui.S.Subtle).SetString("•").String(),

		methodGet:      methodBase.Foreground(ilovetui.S.Success),
		methodPost:     methodBase.Foreground(ilovetui.S.Warning),
		methodPutPatch: methodBase.Foreground(ilovetui.S.Primary),
		methodDelete:   methodBase.Foreground(ilovetui.S.Error),
		methodDefault:  methodBase.Foreground(ilovetui.S.Text),
	}
}

func NewHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(ilovetui.S.Primary)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(ilovetui.S.Muted)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(ilovetui.S.Primary)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(ilovetui.S.Muted)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	h.Styles.Ellipsis = lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	return h
}

func (s *Styles) Method(method string) lipgloss.Style {
	switch method {
	case "GET":
		return s.methodGet
	case "POST":
		return s.methodPost
	case "PUT", "PATCH":
		return s.methodPutPatch
	case "DELETE":
		return s.methodDelete
	default:
		return s.methodDefault
	}
}
