package plugins

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/intercept"
	goproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
)

// GlobalDefault holds global defaults for a single plugin, read from config.yaml.
type GlobalDefault struct {
	Enable *bool
	Config interface{}
}

type Manager struct {
	mu      sync.RWMutex
	plugins []*Plugin

	db             *db.DB
	pluginsFile    *PluginsFile
	broker         *intercept.Broker
	globalDefaults map[string]GlobalDefault

	Notifs chan PluginNotifMsg
	Quit   chan string
}

func NewManager(broker *intercept.Broker) *Manager {
	mgr := &Manager{
		broker: broker,
		Notifs: make(chan PluginNotifMsg, 64),
		Quit:   make(chan string, 4),
	}
	if broker != nil {
		broker.SetOnBeforeNewEntry(mgr.RunSyncOnHistoryEntry)
		broker.SetOnNewEntry(mgr.RunAsyncOnHistoryEntry)
	}
	return mgr
}

func (m *Manager) SetDB(d *db.DB) {
	m.db = d
}

func (m *Manager) SetPluginsFile(pf *PluginsFile) {
	m.pluginsFile = pf
}

func (m *Manager) SetGlobalDefaults(defaults map[string]GlobalDefault) {
	m.globalDefaults = defaults
}

func (m *Manager) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
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
		if m.pluginsFile != nil {
			if enabled, configText, found := m.pluginsFile.get(p.ID); found {
				p.Enabled = enabled
				p.ConfigText = configText
			} else {
				if def, ok := m.globalDefaults[p.ID]; ok {
					if def.Enable != nil {
						p.Enabled = *def.Enable
					}
					if def.Config != nil {
						if raw, err := yaml.Marshal(def.Config); err == nil {
							p.ConfigText = string(raw)
						}
					}
				}
				m.pluginsFile.register(p.ID, p.Enabled, p.ConfigText)
			}
		}
		m.mu.Lock()
		m.plugins = append(m.plugins, p)
		m.mu.Unlock()
	}
	m.mu.Lock()
	sort.Slice(m.plugins, func(i, j int) bool { return m.plugins[i].Priority > m.plugins[j].Priority })
	m.mu.Unlock()
	return nil
}

func (m *Manager) loadPlugin(path string) (*Plugin, error) {
	p := &Plugin{
		ID:       strings.TrimSuffix(filepath.Base(path), ".lua"),
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
		p.Name = p.ID
	}

	if s, ok := pluginTable.RawGetString("description").(lua.LString); ok {
		p.Description = string(s)
	}

	if n, ok := pluginTable.RawGetString("priority").(lua.LNumber); ok {
		p.Priority = int(n)
	}

	if pluginTable.RawGetString("disable_by_default") == lua.LTrue {
		p.Enabled = false
	}

	// Hooks configurable via the Plugin table (sync field).
	configurableHooks := map[string]bool{
		"on_start":         false, // async by default
		"on_request":       false,
		"on_response":      false,
		"on_history_entry": false,
	}
	for hookName, defaultSync := range configurableHooks {
		if tbl, ok := pluginTable.RawGetString(hookName).(*lua.LTable); ok {
			p.hooks[hookName] = HookConfig{Sync: tbl.RawGetString("sync") == lua.LTrue}
			continue
		}
		if p.L.GetGlobal(hookName) != lua.LNil {
			p.hooks[hookName] = HookConfig{Sync: defaultSync}
		}
	}
	for _, fixedSync := range []string{"on_config", "on_quit"} {
		if p.L.GetGlobal(fixedSync) != lua.LNil {
			p.hooks[fixedSync] = HookConfig{Sync: true}
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

func (m *Manager) TogglePlugin(id string) {
	m.mu.RLock()
	var found *Plugin
	for _, p := range m.plugins {
		if p.ID == id {
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
	found.mu.Unlock()
	if m.pluginsFile != nil {
		if err := m.pluginsFile.setEnabled(id, enabled); err != nil {
			log.Printf("plugin %s: save state: %v", id, err)
		}
	}
	if !enabled {
		return
	}
	hc, ok := found.hooks["on_start"]
	if !ok {
		return
	}
	disableIfFalse := func(p *Plugin, ret lua.LValue) {
		if ret == lua.LFalse {
			p.Enabled = false
			if m.pluginsFile != nil {
				if err := m.pluginsFile.setEnabled(p.ID, false); err != nil {
					log.Printf("plugin %s: save state: %v", p.ID, err)
				}
			}
		}
	}
	if hc.Sync {
		found.mu.Lock()
		ret, err := callHook(found, "on_start")
		if err != nil {
			log.Printf("plugin %s on_start: %v", found.Name, err)
		} else {
			disableIfFalse(found, ret)
		}
		found.mu.Unlock()
	} else {
		go func() {
			found.mu.Lock()
			ret, err := callHook(found, "on_start")
			if err != nil {
				log.Printf("plugin %s on_start: %v", found.Name, err)
			} else {
				disableIfFalse(found, ret)
			}
			found.mu.Unlock()
		}()
	}
}

func (m *Manager) SaveConfig(id, configText string) {
	m.mu.RLock()
	var found *Plugin
	for _, p := range m.plugins {
		if p.ID == id {
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
	found.mu.Unlock()
	if m.pluginsFile != nil {
		if err := m.pluginsFile.setConfig(id, configText); err != nil {
			log.Printf("plugin %s: save config: %v", id, err)
		}
	}
	if _, ok := found.hooks["on_config"]; !ok {
		return
	}
	found.mu.Lock()
	if _, err := callHook(found, "on_config"); err != nil {
		log.Printf("plugin %s on_config: %v", id, err)
	}
	found.mu.Unlock()
}

func (m *Manager) RunOnStart() {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		if _, ok := p.hooks["on_config"]; ok {
			p.mu.Lock()
			if _, err := callHook(p, "on_config"); err != nil {
				log.Printf("plugin %s on_config: %v", p.Name, err)
			}
			p.mu.Unlock()
		}
	}
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks["on_start"]
		if !ok {
			continue
		}
		disableIfFalse := func(p *Plugin, ret lua.LValue) {
			if ret == lua.LFalse {
				p.Enabled = false
				if m.pluginsFile != nil {
					if err := m.pluginsFile.setEnabled(p.ID, false); err != nil {
						log.Printf("plugin %s: save state: %v", p.ID, err)
					}
				}
			}
		}
		if hc.Sync {
			p.mu.Lock()
			ret, err := callHook(p, "on_start")
			if err != nil {
				log.Printf("plugin %s on_start: %v", p.Name, err)
			} else {
				disableIfFalse(p, ret)
			}
			p.mu.Unlock()
		} else {
			go func(p *Plugin) {
				p.mu.Lock()
				ret, err := callHook(p, "on_start")
				if err != nil {
					log.Printf("plugin %s on_start: %v", p.Name, err)
				} else {
					disableIfFalse(p, ret)
				}
				p.mu.Unlock()
			}(p)
		}
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

// runSyncDecisionForPlugins runs hookName synchronously for all enabled plugins
// that registered it as sync, and returns the first non-Intercept decision.
func (m *Manager) runSyncDecisionForPlugins(hookName string, argsFor func(*Plugin) []lua.LValue) intercept.Decision {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks[hookName]
		if !ok || !hc.Sync {
			continue
		}
		p.mu.Lock()
		result, err := callHook(p, hookName, argsFor(p)...)
		p.mu.Unlock()
		if err != nil {
			log.Printf("plugin %s %s: %v", p.Name, hookName, err)
			continue
		}
		if s, ok := result.(lua.LString); ok {
			switch string(s) {
			case "drop":
				return intercept.Drop
			case "forward":
				return intercept.Forward
			}
		}
	}
	return intercept.Intercept
}

// runAsyncForPlugins fires hookName asynchronously for all enabled plugins
// that registered it as async.
func (m *Manager) runAsyncForPlugins(hookName string, argsFor func(*Plugin) []lua.LValue) {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks[hookName]
		if !ok || hc.Sync {
			continue
		}
		go func(p *Plugin) {
			p.mu.Lock()
			if _, err := callHook(p, hookName, argsFor(p)...); err != nil {
				log.Printf("plugin %s %s: %v", p.Name, hookName, err)
			}
			p.mu.Unlock()
		}(p)
	}
}

func (m *Manager) RunSyncOnRequest(f *goproxy.Flow) intercept.Decision {
	return m.runSyncDecisionForPlugins("on_request", func(p *Plugin) []lua.LValue {
		return []lua.LValue{pushRequest(p.L, p, f)}
	})
}

func (m *Manager) RunAsyncOnRequest(f *goproxy.Flow) {
	m.runAsyncForPlugins("on_request", func(p *Plugin) []lua.LValue {
		return []lua.LValue{pushRequest(p.L, p, f)}
	})
}

func (m *Manager) RunSyncOnResponse(f *goproxy.Flow) intercept.Decision {
	return m.runSyncDecisionForPlugins("on_response", func(p *Plugin) []lua.LValue {
		return []lua.LValue{pushRequest(p.L, p, f), pushResponse(p.L, f)}
	})
}

func (m *Manager) RunAsyncOnResponse(f *goproxy.Flow) {
	m.runAsyncForPlugins("on_response", func(p *Plugin) []lua.LValue {
		return []lua.LValue{pushRequest(p.L, p, f), pushResponse(p.L, f)}
	})
}

// RunSyncOnHistoryEntry is called before DB insert; returns false to skip saving.
func (m *Manager) RunSyncOnHistoryEntry(e db.Entry) bool {
	for _, p := range m.GetPlugins() {
		if !p.Enabled {
			continue
		}
		hc, ok := p.hooks["on_history_entry"]
		if !ok || !hc.Sync {
			continue
		}
		p.mu.Lock()
		result, err := callHook(p, "on_history_entry", pushEntry(p.L, e))
		p.mu.Unlock()
		if err != nil {
			log.Printf("plugin %s on_history_entry: %v", p.Name, err)
			continue
		}
		if s, ok := result.(lua.LString); ok && string(s) == "skip" {
			return false
		}
	}
	return true
}

func (m *Manager) RunAsyncOnHistoryEntry(e db.Entry) {
	m.runAsyncForPlugins("on_history_entry", func(p *Plugin) []lua.LValue {
		return []lua.LValue{pushEntry(p.L, e)}
	})
}
