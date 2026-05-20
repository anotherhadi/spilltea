package plugins

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

type HookConfig struct {
	Sync bool
}

type Plugin struct {
	ID          string
	Name        string
	Description string
	FilePath    string
	Enabled     bool
	ConfigText  string
	Priority    int

	L     *lua.LState
	mu    sync.Mutex
	hooks map[string]HookConfig
}

func (p *Plugin) HookNames() []string {
	out := make([]string, 0, len(p.hooks))
	for name := range p.hooks {
		out = append(out, name)
	}
	return out
}

func (p *Plugin) HookConfig(name string) (HookConfig, bool) {
	hc, ok := p.hooks[name]
	return hc, ok
}

type Info struct {
	ID          string
	Name        string
	Description string
	FilePath    string
	Enabled     bool
	ConfigText  string
	Priority    int
	Hooks       map[string]HookConfig
}

func (p *Plugin) Info() Info {
	p.mu.Lock()
	enabled := p.Enabled
	configText := p.ConfigText
	p.mu.Unlock()

	hooks := make(map[string]HookConfig, len(p.hooks))
	for k, v := range p.hooks {
		hooks[k] = v
	}
	return Info{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		FilePath:    p.FilePath,
		Enabled:     enabled,
		ConfigText:  configText,
		Priority:    p.Priority,
		Hooks:       hooks,
	}
}

type PluginNotifMsg struct {
	Title string
	Body  string
	Kind  string // "info", "success", "warning", "error"
}

type PluginQuitMsg struct {
	Reason string
}
