package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

//go:embed default_config.yaml
var defaultConfig []byte

type Config struct {
	Version string `mapstructure:"-"`

	App struct {
		Host           string `mapstructure:"host"`
		Port           int    `mapstructure:"port"`
		CertDir        string `mapstructure:"cert_dir"`
		ProjectDir     string `mapstructure:"project_dir"`
		PluginsDir     string `mapstructure:"plugins_dir"`
		UpstreamProxy  string `mapstructure:"upstream_proxy"`
		ProxyAuth      string `mapstructure:"proxy_auth"`
		MaxBodySizeMB  int    `mapstructure:"max_body_size_mb"`
		ExternalEditor string `mapstructure:"external_editor"`
		SslInsecure    bool   `mapstructure:"ssl_insecure"`
	} `mapstructure:"app"`

	TUI struct {
		UseNerdfontIcons    bool   `mapstructure:"use_nerdfont_icons"`
		DefaultSidebarState string `mapstructure:"default_sidebar_state"`
		PrettyPrintBody     bool   `mapstructure:"pretty_print_body"`
	} `mapstructure:"tui"`

	Intercept struct {
		DefaultInterceptEnabled bool     `mapstructure:"default_intercept_enabled"`
		DefaultCaptureResponse  bool     `mapstructure:"default_capture_response"`
		AutoForwardRegex        []string `mapstructure:"auto_forward_regex"`
		QueueSize               int      `mapstructure:"queue_size"`
	} `mapstructure:"intercept"`

	Replay struct {
		SwitchToPageOnSend bool `mapstructure:"switch_to_page_on_send"`
	} `mapstructure:"replay"`

	History struct {
		SkipDuplicates bool `mapstructure:"skip_duplicates"`
		KeepResponses  bool `mapstructure:"keep_responses"`
	} `mapstructure:"history"`

	Keybindings Keybindings `mapstructure:"keybindings"`

	Plugins map[string]GlobalPlugin `mapstructure:"plugins"`
}

type GlobalPlugin struct {
	Enable *bool       `mapstructure:"enable"`
	Config interface{} `mapstructure:"config"`
}

var Global *Config

var projectCLIOverrides func(*Config)

// SetCLIOverrides stores a function that re-applies CLI flag overrides.
// It is called automatically after MergeProjectConfig so that CLI flags
// always win over both global and per-project config.
func SetCLIOverrides(fn func(*Config)) {
	projectCLIOverrides = fn
}

// MergeProjectConfig merges <projectDir>/config.yaml on top of the current
// Global config, then re-applies any registered CLI overrides.
// If no config.yaml exists in the project directory, this is a no-op.
func MergeProjectConfig(projectDir string) error {
	projectConfigPath := filepath.Join(projectDir, "config.yaml")
	if _, err := os.Stat(projectConfigPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	pv := viper.New()
	pv.SetConfigFile(projectConfigPath)
	if err := pv.ReadInConfig(); err != nil {
		return fmt.Errorf("project config: %w", err)
	}
	if err := viper.MergeConfigMap(pv.AllSettings()); err != nil {
		return fmt.Errorf("merge project config: %w", err)
	}
	if err := viper.Unmarshal(Global); err != nil {
		return err
	}
	Global.App.ProxyAuth = expandEnvValue(Global.App.ProxyAuth)
	Global.App.UpstreamProxy = expandEnvValue(Global.App.UpstreamProxy)
	if projectCLIOverrides != nil {
		projectCLIOverrides(Global)
	}
	return nil
}

func Load(path string) error {
	var defaults map[string]any
	if err := yaml.Unmarshal(defaultConfig, &defaults); err != nil {
		return fmt.Errorf("default config: %w", err)
	}
	for k, v := range flatten("", defaults) {
		viper.SetDefault(k, v)
	}

	viper.SetConfigType("yaml")
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	Global = &Config{}
	if err := viper.Unmarshal(Global); err != nil {
		return err
	}
	Global.App.ProxyAuth = expandEnvValue(Global.App.ProxyAuth)
	Global.App.UpstreamProxy = expandEnvValue(Global.App.UpstreamProxy)
	return nil
}

func WriteDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, defaultConfig, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// expandEnvValue replaces a value starting with "$" with the corresponding
// environment variable, enabling secrets to be kept out of config files.
func expandEnvValue(s string) string {
	if len(s) > 1 && s[0] == '$' {
		if val := os.Getenv(s[1:]); val != "" {
			return val
		}
	}
	return s
}

func flatten(prefix string, m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]any); ok {
			for nk, nv := range flatten(key, nested) {
				out[nk] = nv
			}
		} else {
			out[key] = v
		}
	}
	return out
}
