package docs

import (
	"strings"

	spilltea "github.com/anotherhadi/spilltea"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func readDoc(name string) string {
	b, _ := spilltea.DocsFS.ReadFile(".github/docs/" + name)
	return string(b)
}

var contentMarkdown = strings.Join([]string{
	readDoc("main.md"),
	readDoc("proxy.md"),
	readDoc("certificate.md"),
	readDoc("history.md"),
}, "\n")

type Model struct {
	viewport viewport.Model
}

func New() Model {
	return Model{
		viewport: viewport.New(),
	}
}

func (e Model) Init() tea.Cmd {
	return nil
}
