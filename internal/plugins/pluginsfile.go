package plugins

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type pluginFileEntry struct {
	Enable bool        `yaml:"enable"`
	Config interface{} `yaml:"config,omitempty"`
}

type pluginsFileData struct {
	Plugins map[string]pluginFileEntry `yaml:"plugins"`
}

type PluginsFile struct {
	path string
	data pluginsFileData
}

func OpenPluginsFile(dbPath string) (*PluginsFile, error) {
	path := filepath.Join(filepath.Dir(dbPath), "plugins.yaml")
	pf := &PluginsFile{
		path: path,
		data: pluginsFileData{Plugins: make(map[string]pluginFileEntry)},
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return pf, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, &pf.data); err != nil {
		return nil, err
	}
	if pf.data.Plugins == nil {
		pf.data.Plugins = make(map[string]pluginFileEntry)
	}
	return pf, nil
}

func (pf *PluginsFile) save() error {
	raw, err := yaml.Marshal(&pf.data)
	if err != nil {
		return err
	}
	return os.WriteFile(pf.path, raw, 0o600)
}

func (pf *PluginsFile) get(id string) (enabled bool, config string, found bool) {
	e, ok := pf.data.Plugins[id]
	if !ok {
		return false, "", false
	}
	if e.Config == nil {
		return e.Enable, "", true
	}
	raw, err := yaml.Marshal(e.Config)
	if err != nil {
		return e.Enable, "", true
	}
	return e.Enable, string(raw), true
}

func (pf *PluginsFile) register(id string, defaultEnabled bool) {
	if _, ok := pf.data.Plugins[id]; !ok {
		pf.data.Plugins[id] = pluginFileEntry{Enable: defaultEnabled}
		_ = pf.save()
	}
}

func (pf *PluginsFile) setEnabled(id string, enabled bool) error {
	e := pf.data.Plugins[id]
	e.Enable = enabled
	pf.data.Plugins[id] = e
	return pf.save()
}

func (pf *PluginsFile) setConfig(id string, configText string) error {
	e := pf.data.Plugins[id]
	if configText == "" {
		e.Config = nil
	} else {
		var parsed interface{}
		if err := yaml.Unmarshal([]byte(configText), &parsed); err != nil {
			return err
		}
		e.Config = parsed
	}
	pf.data.Plugins[id] = e
	return pf.save()
}
