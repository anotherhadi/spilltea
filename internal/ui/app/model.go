package app

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/plugins"
	proxyPkg "github.com/anotherhadi/spilltea/internal/proxy"
	copyUI "github.com/anotherhadi/spilltea/internal/ui/components/copy"
	copyasUI "github.com/anotherhadi/spilltea/internal/ui/components/copyas"
	notificationsUI "github.com/anotherhadi/spilltea/internal/ui/components/notifications"
	diffUI "github.com/anotherhadi/spilltea/internal/ui/diff"
	docsUI "github.com/anotherhadi/spilltea/internal/ui/docs"
	findingsUI "github.com/anotherhadi/spilltea/internal/ui/findings"
	historyUI "github.com/anotherhadi/spilltea/internal/ui/history"
	interceptUI "github.com/anotherhadi/spilltea/internal/ui/intercept"
	pluginsUI "github.com/anotherhadi/spilltea/internal/ui/plugins"
	replayUI "github.com/anotherhadi/spilltea/internal/ui/replay"
	"github.com/sirupsen/logrus"
)

const tickInterval = 2 * time.Second

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

var pageShortcuts = func() map[string]page {
	m := make(map[string]page, len(pageRegistry))
	for i, e := range pageRegistry {
		m[strconv.Itoa(i+1)] = e.id
	}
	return m
}()

type Model struct {
	broker        *intercept.Broker
	page          page
	projectName   string
	projectPath   string
	database      *db.DB
	logFile       *os.File
	pluginManager *plugins.Manager
	fatalErr      error
	logFileErr    error

	width         int
	height        int
	sidebarState  sidebarState
	intercept     interceptUI.Model
	history       historyUI.Model
	replay        replayUI.Model
	diff          diffUI.Model
	docs          docsUI.Model
	pluginsPage   pluginsUI.Model
	findingsPage  findingsUI.Model
	copyAs        copyasUI.Model
	copy          copyUI.Model
	notifications notificationsUI.Model
}

func New(broker *intercept.Broker, name, path string) Model {
	cfg := config.Global
	mgr := plugins.NewManager(broker)

	m := Model{
		broker:        broker,
		page:          pageIntercept,
		projectName:   name,
		projectPath:   path,
		pluginManager: mgr,
		intercept:     interceptUI.New(broker),
		history:       historyUI.New(),
		replay:        replayUI.New(),
		diff:          diffUI.New(),
		docs:          docsUI.New(),
		pluginsPage:   pluginsUI.New(mgr),
		findingsPage:  findingsUI.New(),
		copyAs:        copyasUI.New(),
		copy:          copyUI.New(),
		notifications: notificationsUI.New(),
		sidebarState:  sidebarState(cfg.TUI.DefaultSidebarState),
	}

	d, err := db.Open(path)
	if err != nil {
		m.fatalErr = err
		return m
	}
	m.database = d
	broker.SetDB(d)
	m.history.SetDB(d)
	m.replay.SetDB(d)
	m.findingsPage.SetDB(d)
	mgr.SetDB(d)

	pf, err := plugins.OpenPluginsFile(path)
	if err != nil {
		log.Printf("plugins file: %v", err)
	}
	mgr.SetPluginsFile(pf)

	if len(cfg.Plugins) > 0 {
		defaults := make(map[string]plugins.GlobalDefault, len(cfg.Plugins))
		for id, gp := range cfg.Plugins {
			defaults[id] = plugins.GlobalDefault{Enable: gp.Enable, Config: gp.Config}
		}
		mgr.SetGlobalDefaults(defaults)
	}

	pluginsDir := config.ExpandPath(cfg.App.PluginsDir)
	if err := mgr.LoadFromDir(pluginsDir); err != nil {
		log.Printf("plugins: %v", err)
	}
	m.pluginsPage.Refresh()

	if lf, err := os.OpenFile(filepath.Join(filepath.Dir(path), "logs.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		m.logFile = lf
		log.SetOutput(lf)
		logrus.SetOutput(lf)
	} else {
		m.logFileErr = err
	}

	return m
}

func (m Model) FatalErr() error { return m.fatalErr }

func (m Model) Init() tea.Cmd {
	mgr := m.pluginManager
	cmds := []tea.Cmd{
		intercept.WaitForRequest(m.broker),
		intercept.WaitForResponse(m.broker),
		tickCmd(),
		proxyPkg.StartCmd(m.broker, mgr),
		plugins.WaitForNotif(mgr),
		plugins.WaitForQuit(mgr),
		findingsUI.RefreshCmd(m.database),
		func() tea.Msg { mgr.RunOnStart(); return nil },
	}
	if m.logFileErr != nil {
		err := m.logFileErr
		cmds = append(cmds, func() tea.Msg {
			return notificationsUI.NotificationMsg{
				Title: "Warning",
				Body:  "Could not open log file: " + err.Error(),
				Kind:  notificationsUI.KindWarning,
			}
		})
	}
	return tea.Batch(cmds...)
}
