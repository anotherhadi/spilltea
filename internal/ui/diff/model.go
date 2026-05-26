package diff

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/util"
)

// isWordChar reports whether c belongs to a "word" token (letter, digit, underscore).
func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// tokenize splits s into runs of word characters and individual non-word bytes.
func tokenize(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		if isWordChar(s[i]) {
			j := i
			for j < len(s) && isWordChar(s[j]) {
				j++
			}
			out = append(out, s[i:j])
			i = j
		} else {
			out = append(out, s[i:i+1])
			i++
		}
	}
	return out
}

// wordDiff computes a token-level diff between leftLine and rightLine and
// returns the two rendered strings with changed tokens highlighted.
func wordDiff(leftLine, rightLine string) (leftRendered, rightRendered string) {
	lToks := tokenize(leftLine)
	rToks := tokenize(rightLine)

	n, m := len(lToks), len(rToks)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if lToks[i-1] == rToks[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	type segment struct {
		kind int // 0=same, 1=left-only, 2=right-only
		tok  string
	}
	segs := make([]segment, 0, n+m)
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && lToks[i-1] == rToks[j-1]:
			segs = append(segs, segment{0, lToks[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			segs = append(segs, segment{2, rToks[j-1]})
			j--
		default:
			segs = append(segs, segment{1, lToks[i-1]})
			i--
		}
	}
	for lo, hi := 0, len(segs)-1; lo < hi; lo, hi = lo+1, hi-1 {
		segs[lo], segs[hi] = segs[hi], segs[lo]
	}

	boldErr := lipgloss.NewStyle().Foreground(ilovetui.S.Error).Bold(true)
	boldOk := lipgloss.NewStyle().Foreground(ilovetui.S.Success).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)

	var lb, rb strings.Builder
	for _, seg := range segs {
		switch seg.kind {
		case 0:
			lb.WriteString(dim.Render(seg.tok))
			rb.WriteString(dim.Render(seg.tok))
		case 1:
			lb.WriteString(boldErr.Render(seg.tok))
		case 2:
			rb.WriteString(boldOk.Render(seg.tok))
		}
	}
	return lb.String(), rb.String()
}

// pairAndHighlight collapses adjacent removed/added blocks onto the same rows
// (eliminating the interleaved padding lines) and applies word-level diff
// highlighting to each paired line. Unpaired excess removals/additions keep
// their original single-sided padding row.
func pairAndHighlight(left, right []diffLine) ([]diffLine, []diffLine) {
	newLeft := make([]diffLine, 0, len(left))
	newRight := make([]diffLine, 0, len(right))

	i := 0
	for i < len(left) {
		if left[i].kind != lineRemoved {
			newLeft = append(newLeft, left[i])
			newRight = append(newRight, right[i])
			i++
			continue
		}

		rStart := i
		for i < len(left) && left[i].kind == lineRemoved {
			i++
		}
		rEnd := i

		aStart := i
		for i < len(left) && left[i].kind == lineAdded {
			i++
		}
		aEnd := i

		nRemoved := rEnd - rStart
		nAdded := aEnd - aStart
		pairs := nRemoved
		if nAdded < pairs {
			pairs = nAdded
		}

		for k := 0; k < pairs; k++ {
			lLine := left[rStart+k]
			rLine := right[aStart+k]
			lLine.text, rLine.text = wordDiff(lLine.plainText, rLine.plainText)
			newLeft = append(newLeft, lLine)
			newRight = append(newRight, rLine)
		}

		for k := pairs; k < nRemoved; k++ {
			newLeft = append(newLeft, left[rStart+k])
			newRight = append(newRight, diffLine{kind: lineRemoved})
		}

		for k := pairs; k < nAdded; k++ {
			newLeft = append(newLeft, diffLine{kind: lineAdded})
			newRight = append(newRight, right[aStart+k])
		}
	}

	return newLeft, newRight
}

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
	text      string // displayed text (highlighted, possibly word-diff decorated)
	plainText string // plain text for word-diff pairing (empty for padding lines)
	kind      lineKind
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
		leftViewport:  ilovetui.NewViewport(),
		rightViewport: ilovetui.NewViewport(),
		help:          ilovetui.NewHelp(),
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

	viewportH := ilovetui.ContentHeight(panelH)

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
	m.leftLines, m.rightLines = pairAndHighlight(m.leftLines, m.rightLines)
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
	if m.left.raw == "" {
		placeholder := lipgloss.Place(
			m.leftViewport.Width(), m.leftViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			ilovetui.S.Faint.Render(util.CenterLines("<(^_^)>", "send two entries here to compare")),
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
			ilovetui.S.Faint.Render(util.CenterLines("(・3・)", "waiting for second entry…")),
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
			right = append(right, diffLine{text: hlB(j - 1), plainText: b[j-1], kind: lineAdded})
			j--
		default:
			left = append(left, diffLine{text: hlA(i - 1), plainText: a[i-1], kind: lineRemoved})
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
	pageGlobals := []key.Binding{g.Up, g.Down, g.CycleFocus, g.ScrollUp, g.ScrollDown, g.Left, g.Right}
	all := append(keys.Keys.Diff.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
