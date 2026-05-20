package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/icons"
	diffUI "github.com/anotherhadi/spilltea/internal/ui/diff"
	docsUI "github.com/anotherhadi/spilltea/internal/ui/docs"
	findingsUI "github.com/anotherhadi/spilltea/internal/ui/findings"
	historyUI "github.com/anotherhadi/spilltea/internal/ui/history"
	interceptUI "github.com/anotherhadi/spilltea/internal/ui/intercept"
	pluginsUI "github.com/anotherhadi/spilltea/internal/ui/plugins"
	replayUI "github.com/anotherhadi/spilltea/internal/ui/replay"
)

type page string

const (
	pageIntercept page = "Intercept"
	pageHistory   page = "History"
	pageReplay    page = "Replay"
	pageDiff      page = "Diff"
	pagePlugins   page = "Plugins"
	pageFindings  page = "Findings"
	pageDocs      page = "Docs"
)

// pageEntry describes a page and all its integration hooks.
type pageEntry struct {
	id   page
	icon func() string

	// render returns the page's view content. nil = show "empty".
	render func(m *Model) string
	// update is called when this page is active. nil = no-op.
	update func(m *Model, msg tea.Msg) tea.Cmd
	// isEditing reports whether the page is in text-editing mode.
	isEditing func(m *Model) bool
	// resize propagates a new (w, h) to the page model.
	resize func(m *Model, w, h int)
	// hasUpdate reports whether the page has unseen updates.
	hasUpdate func(m *Model) bool
}

var pageRegistry = []pageEntry{
	{
		id:   pageIntercept,
		icon: func() string { return icons.I.Intercept },

		render: func(m *Model) string { return m.intercept.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.intercept.Update(msg)
			m.intercept = updated.(interceptUI.Model)
			return cmd
		},
		isEditing: func(m *Model) bool { return m.intercept.IsEditing() },
		resize:    func(m *Model, w, h int) { m.intercept.SetSize(w, h) },
		hasUpdate: func(m *Model) bool { return m.intercept.HasUnread() },
	},
	{
		id:   pageHistory,
		icon: func() string { return icons.I.History },

		render: func(m *Model) string { return m.history.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.history.Update(msg)
			m.history = updated.(historyUI.Model)
			return cmd
		},
		isEditing: func(m *Model) bool { return m.history.IsEditing() },
		resize:    func(m *Model, w, h int) { m.history.SetSize(w, h) },
	},
	{
		id:   pageReplay,
		icon: func() string { return icons.I.Replay },

		render: func(m *Model) string { return m.replay.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.replay.Update(msg)
			m.replay = updated.(replayUI.Model)
			return cmd
		},
		isEditing: func(m *Model) bool { return m.replay.IsEditing() },
		resize:    func(m *Model, w, h int) { m.replay.SetSize(w, h) },
	},
	{
		id:   pageDiff,
		icon: func() string { return icons.I.Diff },

		render: func(m *Model) string { return m.diff.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.diff.Update(msg)
			m.diff = updated.(diffUI.Model)
			return cmd
		},
		resize: func(m *Model, w, h int) { m.diff.SetSize(w, h) },
	},
	{
		id:   pagePlugins,
		icon: func() string { return icons.I.Plugin },

		render: func(m *Model) string { return m.pluginsPage.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.pluginsPage.Update(msg)
			m.pluginsPage = updated.(pluginsUI.Model)
			return cmd
		},
		isEditing: func(m *Model) bool { return m.pluginsPage.IsEditing() },
		resize:    func(m *Model, w, h int) { m.pluginsPage.SetSize(w, h) },
	},
	{
		id:   pageFindings,
		icon: func() string { return icons.I.Findings },

		render: func(m *Model) string { return m.findingsPage.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.findingsPage.Update(msg)
			m.findingsPage = updated.(findingsUI.Model)
			return cmd
		},
		resize:    func(m *Model, w, h int) { m.findingsPage.SetSize(w, h) },
		hasUpdate: func(m *Model) bool { return m.findingsPage.HasUnread() },
	},
	{
		id:   pageDocs,
		icon: func() string { return icons.I.Docs },

		render: func(m *Model) string { return m.docs.View().Content },
		update: func(m *Model, msg tea.Msg) tea.Cmd {
			updated, cmd := m.docs.Update(msg)
			m.docs = updated.(docsUI.Model)
			return cmd
		},
		isEditing: func(m *Model) bool { return m.docs.IsEditing() },
		resize:    func(m *Model, w, h int) { m.docs.SetSize(w, h) },
	},
}
