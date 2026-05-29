package main

import (
	"log"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/keys"
	appUI "github.com/anotherhadi/spilltea/internal/ui/app"
	homeUI "github.com/anotherhadi/spilltea/internal/ui/home"
)

type rootState int

const (
	rootStateHome rootState = iota
	rootStateApp
)

type rootModel struct {
	state  rootState
	home   homeUI.Model
	app    tea.Model
	width  int
	height int
}

func (m rootModel) Init() tea.Cmd {
	return m.home.Init()
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
	}

	if m.state == rootStateHome {
		if sel, ok := msg.(homeUI.ProjectSelectedMsg); ok {
			if err := config.MergeProjectConfig(filepath.Dir(sel.Project.Path)); err != nil {
				log.Printf("project config: %v", err)
			}
			icons.Init(config.Global)
			keys.Init(config.Global)
			broker := intercept.NewBroker()
			app := appUI.New(broker, sel.Project.Name, sel.Project.Path)
			m.app = app
			m.state = rootStateApp
			return m, tea.Batch(app.Init(), func() tea.Msg {
				return tea.WindowSizeMsg{Width: m.width, Height: m.height}
			})
		}
		updated, cmd := m.home.Update(msg)
		m.home = updated.(homeUI.Model)
		return m, cmd
	}

	updated, cmd := m.app.Update(msg)
	m.app = updated
	return m, cmd
}

func (m rootModel) View() tea.View {
	if m.state == rootStateApp {
		return m.app.(interface{ View() tea.View }).View()
	}
	return m.home.View()
}

func (m rootModel) FatalErr() error {
	if m.state == rootStateApp {
		if app, ok := m.app.(appUI.Model); ok {
			return app.FatalErr()
		}
	}
	return nil
}
