package history

import (
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/db"
)

type searchKind int

const (
	searchKindOff searchKind = iota
	searchKindFulltext
	searchKindSQL
)

type SearchResultMsg struct {
	Entries []db.Entry
}

type SearchErrMsg struct {
	Err error
}

func SearchCmd(database *db.DB, term string) tea.Cmd {
	return func() tea.Msg {
		if database == nil {
			return SearchResultMsg{}
		}
		entries, err := database.SearchEntries(term)
		if err != nil {
			return SearchErrMsg{Err: err}
		}
		return SearchResultMsg{Entries: entries}
	}
}

func SQLCmd(database *db.DB, query string) tea.Cmd {
	return func() tea.Msg {
		if database == nil {
			return SearchResultMsg{}
		}
		entries, err := database.QueryEntries(query)
		if err != nil {
			return SearchErrMsg{Err: err}
		}
		return SearchResultMsg{Entries: entries}
	}
}
