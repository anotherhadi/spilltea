package style

import (
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
)

type Styles struct {
	PanelEditing lipgloss.Style

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

		methodGet:      methodBase.Foreground(ilovetui.S.Success),
		methodPost:     methodBase.Foreground(ilovetui.S.Warning),
		methodPutPatch: methodBase.Foreground(ilovetui.S.Primary),
		methodDelete:   methodBase.Foreground(ilovetui.S.Error),
		methodDefault:  methodBase.Foreground(ilovetui.S.Text),
	}
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
