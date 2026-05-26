package docs

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	spilltea "github.com/anotherhadi/spilltea"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/keys"
)

func readDoc(name string) string {
	b, _ := spilltea.DocsFS.ReadFile("docs/" + name)
	return string(b)
}

var contentMarkdown = strings.Join([]string{
	readDoc("main.md"),
	readDoc("legal-disclaimer.md"),
	readDoc("basics.md"),
	readDoc("proxy.md"),
	readDoc("certificate.md"),
	readDoc("history.md"),
}, "\n")

type matchEntry struct {
	line  int
	start int
	end   int
}

type Model struct {
	viewport    viewport.Model
	help        help.Model
	searchInput textinput.Model
	searching   bool

	matches    []matchEntry
	matchIndex int

	renderedLines []string
	strippedLines []string

	width  int
	height int
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = "/"
	s := ti.Styles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(ilovetui.S.Primary)
	ti.SetStyles(s)

	return Model{
		viewport:    viewport.New(),
		help:        ilovetui.NewHelp(),
		searchInput: ti,
	}
}

func (e Model) Init() tea.Cmd {
	return nil
}

func (m Model) IsEditing() bool { return m.searching }

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.help.SetWidth(w - 2)
	m.searchInput.SetWidth(w - 4)

	statusH := strings.Count(m.renderStatusBar(), "\n") + 1
	frameW := windowStyle().GetHorizontalFrameSize()
	frameH := windowStyle().GetVerticalFrameSize()

	m.viewport.SetWidth(w - frameW)
	m.viewport.SetHeight(h - frameH - statusH)
	m.renderMarkdown()
}

func (m *Model) applySearch() {
	query := m.searchInput.Value()
	m.matches = nil
	m.matchIndex = 0

	if query != "" {
		re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(query))
		if err == nil {
			for lineIdx, stripped := range m.strippedLines {
				for _, match := range re.FindAllStringIndex(stripped, -1) {
					m.matches = append(m.matches, matchEntry{
						line:  lineIdx,
						start: match[0],
						end:   match[1],
					})
				}
			}
		}
	}

	m.rebuildViewportContent()
	if len(m.matches) > 0 {
		m.viewport.SetYOffset(m.matches[0].line)
	}
}

func (m *Model) searchNext() {
	if len(m.matches) == 0 {
		return
	}
	m.matchIndex = (m.matchIndex + 1) % len(m.matches)
	m.rebuildViewportContent()
	m.viewport.SetYOffset(m.matches[m.matchIndex].line)
}

func (m *Model) searchPrev() {
	if len(m.matches) == 0 {
		return
	}
	m.matchIndex = (m.matchIndex - 1 + len(m.matches)) % len(m.matches)
	m.rebuildViewportContent()
	m.viewport.SetYOffset(m.matches[m.matchIndex].line)
}

func (m *Model) rebuildViewportContent() {
	if len(m.matches) == 0 || m.searchInput.Value() == "" {
		m.viewport.SetContent(strings.Join(m.renderedLines, "\n"))
		return
	}

	type lineInfo struct {
		intervals  [][]int
		currentIdx int
	}
	byLine := make(map[int]*lineInfo)
	for i, match := range m.matches {
		li := byLine[match.line]
		if li == nil {
			li = &lineInfo{currentIdx: -1}
			byLine[match.line] = li
		}
		li.intervals = append(li.intervals, []int{match.start, match.end})
		if i == m.matchIndex {
			li.currentIdx = len(li.intervals) - 1
		}
	}

	lines := make([]string, len(m.renderedLines))
	for i, ansiLine := range m.renderedLines {
		if li, ok := byLine[i]; ok {
			lines[i] = injectHighlightsInLine(ansiLine, li.intervals, li.currentIdx)
		} else {
			lines[i] = ansiLine
		}
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func lipglossAnsiCodes(s lipgloss.Style) (open, close string) {
	const sentinel = "X"
	rendered := s.Render(sentinel)
	idx := strings.Index(rendered, sentinel)
	if idx < 0 {
		return "", ""
	}
	return rendered[:idx], rendered[idx+len(sentinel):]
}

func injectHighlightsInLine(ansiLine string, intervals [][]int, currentIdx int) string {
	if len(intervals) == 0 {
		return ansiLine
	}

	normalOpen, normalClose := lipglossAnsiCodes(lipgloss.NewStyle().Background(ilovetui.S.SubtleBg))
	currentOpen, currentClose := lipglossAnsiCodes(lipgloss.NewStyle().Background(ilovetui.S.Primary).Foreground(ilovetui.S.Text))

	type injection struct {
		visPos   int
		code     string
		priority int // 0 = close (emit before opens at same pos), 1 = open
	}
	var injections []injection
	for i, iv := range intervals {
		open, close := normalOpen, normalClose
		if i == currentIdx {
			open, close = currentOpen, currentClose
		}
		injections = append(injections, injection{visPos: iv[0], code: open, priority: 1})
		injections = append(injections, injection{visPos: iv[1], code: close, priority: 0})
	}
	sort.SliceStable(injections, func(a, b int) bool {
		if injections[a].visPos != injections[b].visPos {
			return injections[a].visPos < injections[b].visPos
		}
		return injections[a].priority < injections[b].priority
	})

	var sb strings.Builder
	visPos := 0
	injIdx := 0
	i := 0
	for i < len(ansiLine) {
		for injIdx < len(injections) && injections[injIdx].visPos == visPos {
			sb.WriteString(injections[injIdx].code)
			injIdx++
		}
		if ansiLine[i] == '\x1b' {
			j := i + 1
			if j < len(ansiLine) {
				switch ansiLine[j] {
				case '[':
					j++
					for j < len(ansiLine) && (ansiLine[j] < '@' || ansiLine[j] > '~') {
						j++
					}
					if j < len(ansiLine) {
						j++
					}
				case ']':
					j++
					for j < len(ansiLine) {
						if ansiLine[j] == '\a' {
							j++
							break
						}
						if ansiLine[j] == '\x1b' && j+1 < len(ansiLine) && ansiLine[j+1] == '\\' {
							j += 2
							break
						}
						j++
					}
				default:
					j++
				}
			}
			sb.WriteString(ansiLine[i:j])
			i = j
		} else {
			_, size := utf8.DecodeRuneInString(ansiLine[i:])
			if size == 0 {
				size = 1
			}
			sb.WriteString(ansiLine[i : i+size])
			i += size
			visPos += size
		}
	}
	for injIdx < len(injections) {
		sb.WriteString(injections[injIdx].code)
		injIdx++
	}
	return sb.String()
}

func (m *Model) renderStatusBar() string {
	if m.searching {
		return lipgloss.NewStyle().Padding(0, 1).Render(m.searchInput.View())
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(docsKeyMap{width: m.width}))
}

type docsKeyMap struct{ width int }

func (docsKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	d := keys.Keys.Docs
	return []key.Binding{g.Up, g.Down, d.Search, g.Help}
}

func (m docsKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	d := keys.Keys.Docs
	pageGlobals := []key.Binding{g.Up, g.Down, g.ScrollUp, g.ScrollDown}
	all := append(d.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
