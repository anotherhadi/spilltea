package plugins

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/intercept"
	goproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
	lua "github.com/yuin/gopher-lua"
)

type Manager struct {
	mu      sync.RWMutex
	plugins []*Plugin

	db     *db.DB
	broker *intercept.Broker

	Notifs chan PluginNotifMsg
	Quit   chan string
}

func NewManager(broker *intercept.Broker) *Manager {
	mgr := &Manager{
		broker:   broker,
		Notifs: make(chan PluginNotifMsg, 64),
		Quit:   make(chan string, 4),
	}
	if broker != nil {
		broker.SetOnNewEntry(mgr.RunOnHistoryEntry)
	}
	return mgr
}

func (m *Manager) SetDB(d *db.DB) {
	m.db = d
}

func (m *Manager) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var states map[string]db.PluginState
	if m.db != nil {
		list, err := m.db.LoadPluginStates()
		if err == nil {
			states = make(map[string]db.PluginState, len(list))
			for _, s := range list {
				states[s.Name] = s
			}
		}
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		p, err := m.loadPlugin(path)
		if err != nil {
			log.Printf("plugin load error %s: %v", path, err)
			continue
		}
		if s, ok := states[p.Name]; ok {
			p.Enabled = s.Enabled
			p.ConfigText = s.ConfigText
		}
		m.mu.Lock()
		m.plugins = append(m.plugins, p)
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) loadPlugin(path string) (*Plugin, error) {
	p := &Plugin{
		FilePath: path,
		Enabled:  true,
		hooks:    make(map[string]HookConfig),
	}
	p.L = newLuaState(m, p)
	if err := p.L.DoFile(path); err != nil {
		p.L.Close()
		return nil, err
	}

	pluginTable, ok := p.L.GetGlobal("Plugin").(*lua.LTable)
	if !ok {
		p.L.Close()
		return nil, fmt.Errorf("missing Plugin table")
	}

	if s, ok := pluginTable.RawGetString("name").(lua.LString); ok {
		p.Name = string(s)
	}
	if p.Name == "" {
		p.Name = strings.TrimSuffix(filepath.Base(path), ".lua")
	}

	// Defaults when not overridden by the Plugin table.
	hookDefaults := map[string]bool{
		"on_start":         true,  // always sync
		"on_request":       false, // async
		"on_response":      false, // async
		"on_quit":          true,  // always sync
		"on_history_entry": false, // always async
	}
	for hookName, defaultSync := range hookDefaults {
		// Plugin table entry overrides the default (except on_start/on_quit/on_history_entry which are fixed).
		if hookName != "on_start" && hookName != "on_quit" && hookName != "on_history_entry" {
			if tbl, ok := pluginTable.RawGetString(hookName).(*lua.LTable); ok {
				p.hooks[hookName] = HookConfig{Sync: tbl.RawGetString("sync") == lua.LTrue}
				continue
			}
		}
		// Auto-detect: register the hook if the function exists as a global.
		if p.L.GetGlobal(hookName) != lua.LNil {
			p.hooks[hookName] = HookConfig{Sync: defaultSync}
		}
	}

	return p, nil
}

func (m *Manager) GetPlugins() []*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Plugin, len(m.plugins))
	copy(out, m.plugins)
	return out
}

func (m *Manager) TogglePlugin(name string) {
	m.mu.RLock()
	var found *Plugin
	for _, p := range m.plugins {
		if p.Name == name {
			found = p
			break
		}
	}
	m.mu.RUnlock()
	if found == nil {
		return
	}
	found.mu.Lock()
	found.Enabled = !found.Enabled
	enabled := found.Enabled
	configText := found.ConfigText
	found.mu.Unlock()
	if m.db != nil {
		_ = m.db.SavePluginState(name, enabled, configText)
	}
}

func (m *Manager) SaveConfig(name, configText string) {
	m.mu.RLock()
	var found *Plugin
	for _, p := range m.plugins {
		if p.Name == name {
			found = p
			break
		}
	}
	m.mu.RUnlock()
	if found == nil {
		return
	}
	found.mu.Lock()
	found.ConfigText = configText
	enabled := found.Enabled
	hc, hasOnStart := found.hooks["on_start"]
	found.mu.Unlock()
	if m.db != nil {
		_ = m.db.SavePluginState(name, enabled, configText)
	}
	if !hasOnStart {
		return
	}
	// Re-run on_start so the plugin can re-parse the new config.
	if hc.Sync {
		found.mu.Lock()
		if _, err := callHook(found, "on_start", lua.LString(configText)); err != nil {
			log.Printf("plugin %s on_start (config reload): %v", name, err)
		}
		found.mu.Unlock()
	} else {
		go func() {
			found.mu.Lock()
			if _, err := callHook(found, "on_start", lua.LString(configText)); err != nil {
				log.Printf("plugin %s on_start (config reload): %v", name, err)
			}
			found.mu.Unlock()
		}()
	}
}

func (m *Manager) RunOnStart() {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		if _, ok := p.hooks["on_start"]; !ok {
			continue
		}
		p.mu.Lock()
		if _, err := callHook(p, "on_start", lua.LString(p.ConfigText)); err != nil {
			log.Printf("plugin %s on_start: %v", p.Name, err)
		}
		p.mu.Unlock()
	}
}

func (m *Manager) RunOnQuit() {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		if _, ok := p.hooks["on_quit"]; !ok {
			continue
		}
		p.mu.Lock()
		if _, err := callHook(p, "on_quit"); err != nil {
			log.Printf("plugin %s on_quit: %v", p.Name, err)
		}
		p.mu.Unlock()
	}
}

func (m *Manager) RunSyncOnRequest(f *goproxy.Flow) intercept.Decision {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks["on_request"]
		if !ok || !hc.Sync {
			continue
		}
		p.mu.Lock()
		result, err := callHook(p, "on_request", pushRequest(p.L, f))
		p.mu.Unlock()
		if err != nil {
			log.Printf("plugin %s on_request: %v", p.Name, err)
			continue
		}
		switch result {
		case "drop":
			return intercept.Drop
		case "forward":
			return intercept.Forward
		}
	}
	return intercept.Intercept
}

func (m *Manager) RunAsyncOnRequest(f *goproxy.Flow) {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks["on_request"]
		if !ok || hc.Sync {
			continue
		}
		go func(p *Plugin) {
			p.mu.Lock()
			if _, err := callHook(p, "on_request", pushRequest(p.L, f)); err != nil {
				log.Printf("plugin %s on_request: %v", p.Name, err)
			}
			p.mu.Unlock()
		}(p)
	}
}

func (m *Manager) RunSyncOnResponse(f *goproxy.Flow) intercept.Decision {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks["on_response"]
		if !ok || !hc.Sync {
			continue
		}
		p.mu.Lock()
		result, err := callHook(p, "on_response", pushRequest(p.L, f), pushResponse(p.L, f))
		p.mu.Unlock()
		if err != nil {
			log.Printf("plugin %s on_response: %v", p.Name, err)
			continue
		}
		switch result {
		case "drop":
			return intercept.Drop
		case "forward":
			return intercept.Forward
		}
	}
	return intercept.Intercept
}

func (m *Manager) RunAsyncOnResponse(f *goproxy.Flow) {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks["on_response"]
		if !ok || hc.Sync {
			continue
		}
		go func(p *Plugin) {
			p.mu.Lock()
			if _, err := callHook(p, "on_response", pushRequest(p.L, f), pushResponse(p.L, f)); err != nil {
				log.Printf("plugin %s on_response: %v", p.Name, err)
			}
			p.mu.Unlock()
		}(p)
	}
}

func (m *Manager) RunOnHistoryEntry(e db.Entry) {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		if _, ok := p.hooks["on_history_entry"]; !ok {
			continue
		}
		go func(p *Plugin) {
			p.mu.Lock()
			if _, err := callHook(p, "on_history_entry", pushEntry(p.L, e)); err != nil {
				log.Printf("plugin %s on_history_entry: %v", p.Name, err)
			}
			p.mu.Unlock()
		}(p)
	}
}
