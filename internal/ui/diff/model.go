package diff

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

type slot struct {
	label string
	raw   string
}

type focusedSlot int

const (
	bothSlots focusedSlot = iota
	leftSlot
	rightSlot
)

func (f focusedSlot) next() focusedSlot {
	return (f + 1) % 3
}

type lineKind int

const (
	lineUnchanged lineKind = iota
	lineAdded
	lineRemoved
)

type diffLine struct {
	text string
	kind lineKind
}

type Model struct {
	left  slot
	right slot
	focus focusedSlot

	leftLines  []diffLine
	rightLines []diffLine

	leftViewport  viewport.Model
	rightViewport viewport.Model
	help          help.Model

	width  int
	height int
}

func New() Model {
	return Model{
		leftViewport:  style.NewViewport(),
		rightViewport: style.NewViewport(),
		help:          style.NewHelp(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

// CurrentRaw returns the raw content of the focused slot (left when both are focused).
func (m Model) CurrentRaw() string {
	if m.focus == rightSlot {
		return m.right.raw
	}
	return m.left.raw
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.recalcSizes()
}

func (m *Model) recalcSizes() {
	m.help.SetWidth(m.width - 2)

	statusH := strings.Count(m.renderStatusBar(), "\n") + 1
	panelH := m.height - statusH
	if panelH < 0 {
		panelH = 0
	}

	leftW := m.width / 2
	rightW := m.width - leftW

	leftInner := leftW - 2
	rightInner := rightW - 2
	if leftInner < 0 {
		leftInner = 0
	}
	if rightInner < 0 {
		rightInner = 0
	}

	viewportH := style.PanelContentH(panelH)

	m.leftViewport.SetWidth(leftInner)
	m.leftViewport.SetHeight(viewportH)
	m.rightViewport.SetWidth(rightInner)
	m.rightViewport.SetHeight(viewportH)

	m.refreshViewports()
}

func (m *Model) computeDiff() {
	if m.left.raw == "" || m.right.raw == "" {
		m.leftLines = nil
		m.rightLines = nil
		return
	}
	leftNorm := normRaw(m.left.raw)
	rightNorm := normRaw(m.right.raw)
	leftPlain := strings.Split(leftNorm, "\n")
	rightPlain := strings.Split(rightNorm, "\n")
	leftHL := hlLines(leftNorm)
	rightHL := hlLines(rightNorm)
	m.leftLines, m.rightLines = lcsAlignedDiff(leftPlain, rightPlain, leftHL, rightHL)
}

func normRaw(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

func hlLines(raw string) []string {
	s := strings.TrimRight(style.HighlightHTTP(raw), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (m *Model) refreshViewports() {
	s := style.S

	if m.left.raw == "" {
		placeholder := lipgloss.Place(
			m.leftViewport.Width(), m.leftViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			s.Faint.Render("             <(^_^)>\nsend two entries here to compare"),
		)
		m.leftViewport.SetContent(placeholder)
		m.rightViewport.SetContent("")
		return
	}

	if m.right.raw == "" {
		m.leftViewport.SetContent(style.HighlightHTTP(normRaw(m.left.raw)))
		placeholder := lipgloss.Place(
			m.rightViewport.Width(), m.rightViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			s.Faint.Render("          (・3・)\nwaiting for second entry…"),
		)
		m.rightViewport.SetContent(placeholder)
		return
	}

	m.leftViewport.SetContent(renderLeftLines(m.leftLines))
	m.rightViewport.SetContent(renderRightLines(m.rightLines))
}

func (m *Model) scroll(delta int) {
	offset := m.leftViewport.YOffset() + delta
	m.leftViewport.SetYOffset(offset)
	m.rightViewport.SetYOffset(offset)
}

func (m *Model) scrollH(delta int) {
	offset := m.leftViewport.XOffset() + delta
	m.leftViewport.SetXOffset(offset)
	m.rightViewport.SetXOffset(offset)
}

func lcsAlignedDiff(a, b, aHL, bHL []string) (left, right []diffLine) {
	hlA := func(i int) string {
		if i < len(aHL) {
			return aHL[i]
		}
		return a[i]
	}
	hlB := func(j int) string {
		if j < len(bHL) {
			return bHL[j]
		}
		return b[j]
	}

	n, m := len(a), len(b)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	left = make([]diffLine, 0, n+m)
	right = make([]diffLine, 0, n+m)
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			left = append(left, diffLine{text: hlA(i - 1), kind: lineUnchanged})
			right = append(right, diffLine{text: hlB(j - 1), kind: lineUnchanged})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			left = append(left, diffLine{kind: lineAdded})
			right = append(right, diffLine{text: hlB(j - 1), kind: lineAdded})
			j--
		default:
			left = append(left, diffLine{text: hlA(i - 1), kind: lineRemoved})
			right = append(right, diffLine{kind: lineRemoved})
			i--
		}
	}

	for lo, hi := 0, len(left)-1; lo < hi; lo, hi = lo+1, hi-1 {
		left[lo], left[hi] = left[hi], left[lo]
		right[lo], right[hi] = right[hi], right[lo]
	}
	return left, right
}

type diffKeyMap struct{ width int }

func (diffKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	return []key.Binding{g.Up, g.Down, g.CycleFocus, keys.Keys.Diff.Clear, g.Help}
}

func (m diffKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.CycleFocus, g.ScrollUp, g.ScrollDown, g.Left, g.Right, g.Copy, g.CopyAs}
	all := append(keys.Keys.Diff.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
