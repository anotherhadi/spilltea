package style

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/config"
)

type Styles struct {
	Primary   color.Color
	Success   color.Color
	Error     color.Color
	Warning   color.Color
	SubtleBg  color.Color
	Selection color.Color
	Text      color.Color
	MutedFg   color.Color
	Subtle    color.Color

	Bold  lipgloss.Style
	Faint lipgloss.Style

	Panel        lipgloss.Style
	PanelFocused lipgloss.Style
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

func Init(cfg *config.Config) {
	c := cfg.TUI.Colors

	subtleBg := lipgloss.Color("#" + c.Base01)  // Lighter Background (status bars)
	selection := lipgloss.Color("#" + c.Base02) // Selection Background
	subtle := lipgloss.Color("#" + c.Base03)    // Faint text, borders
	mutedFg := lipgloss.Color("#" + c.Base04)   // Muted foreground
	text := lipgloss.Color("#" + c.Base05)      // Default Foreground
	errCol := lipgloss.Color("#" + c.Base08)    // Red: errors
	warning := lipgloss.Color("#" + c.Base09)   // Orange: warnings
	success := lipgloss.Color("#" + c.Base0B)   // Green: success
	primary := lipgloss.Color("#" + c.Base0D)   // Accent: primary
	purple := lipgloss.Color("#" + c.Base0E)    // Purple: editing

	methodBase := lipgloss.NewStyle().Bold(true).Width(7)
	S = &Styles{
		Primary:   primary,
		Success:   success,
		Error:     errCol,
		Warning:   warning,
		SubtleBg:  subtleBg,
		Selection: selection,
		MutedFg:   mutedFg,
		Text:      text,
		Subtle:    subtle,

		Bold:  lipgloss.NewStyle().Bold(true),
		Faint: lipgloss.NewStyle().Foreground(subtle).Faint(true),

		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle),

		PanelFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary),

		PanelEditing: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple),

		PagerDotActive:   lipgloss.NewStyle().Foreground(primary).SetString("•").String(),
		PagerDotInactive: lipgloss.NewStyle().Foreground(subtle).SetString("•").String(),

		methodGet:      methodBase.Foreground(success),
		methodPost:     methodBase.Foreground(warning),
		methodPutPatch: methodBase.Foreground(primary),
		methodDelete:   methodBase.Foreground(errCol),
		methodDefault:  methodBase.Foreground(text),
	}
}

func NewHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(S.Primary)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(S.MutedFg)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(S.Subtle)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(S.Primary)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(S.MutedFg)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(S.Subtle)
	h.Styles.Ellipsis = lipgloss.NewStyle().Foreground(S.Subtle)
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
