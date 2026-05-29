package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	spilltea "github.com/anotherhadi/spilltea"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	appUI "github.com/anotherhadi/spilltea/internal/ui/app"
	homeUI "github.com/anotherhadi/spilltea/internal/ui/home"
	"github.com/spf13/cobra"
)

// Version is overwritten at build time by goreleaser/ldflag with the current version tag, or "dev" if not set.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

func main() {
	var (
		flagConfig            string
		flagPluginsDir        string
		flagHost              string
		flagPort              int
		flagUpstreamProxy     string
		flagProject           string
		flagAddDefaultPlugins bool
		flagAddDefaultConfig  bool
	)

	rootCmd := &cobra.Command{
		Use:           "spilltea",
		Short:         "A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagAddDefaultPlugins {
				home, _ := os.UserHomeDir()
				cfgPath := filepath.Join(home, ".config", "spilltea", "config.yaml")
				if flagConfig != "" {
					cfgPath = flagConfig
				}
				if err := config.Load(cfgPath); err != nil {
					return fmt.Errorf("config: %w", err)
				}
				dir := config.ExpandPath(config.Global.App.PluginsDir)
				if flagPluginsDir != "" {
					dir = flagPluginsDir
				}
				n, err := spilltea.InstallDefaultPlugins(dir)
				if err != nil {
					return fmt.Errorf("add-default-plugins: %w", err)
				}
				fmt.Printf("added %d plugin(s) to %s\n", n, dir)
				return nil
			}

			if flagAddDefaultConfig {
				home, _ := os.UserHomeDir()
				cfgPath := filepath.Join(home, ".config", "spilltea", "config.yaml")
				if flagConfig != "" {
					cfgPath = flagConfig
				}
				if err := config.WriteDefaultConfig(cfgPath); err != nil {
					return fmt.Errorf("add-default-config: %w", err)
				}
				fmt.Printf("default config written to %s\n", cfgPath)
				return nil
			}

			if flagProject != "" && !homeUI.IsValidProjectName(flagProject) {
				return fmt.Errorf("project: invalid name %q (only lowercase letters, digits, - and _ are allowed)", flagProject)
			}

			home, _ := os.UserHomeDir()
			cfgPath := filepath.Join(home, ".config", "spilltea", "config.yaml")
			if flagConfig != "" {
				cfgPath = flagConfig
			}

			if err := config.Load(cfgPath); err != nil {
				return fmt.Errorf("config: %w", err)
			}
			config.Global.Version = version

			if flagPluginsDir != "" {
				config.Global.App.PluginsDir = flagPluginsDir
			}
			if flagHost != "" {
				config.Global.App.Host = flagHost
			}
			if flagPort != 0 {
				config.Global.App.Port = flagPort
			}
			if flagUpstreamProxy != "" {
				config.Global.App.UpstreamProxy = flagUpstreamProxy
			}

			style.Init()
			icons.Init(config.Global)
			keys.Init(config.Global)

			projectDir := config.ExpandPath(config.Global.App.ProjectDir)

			if flagProject != "" {
				project, err := homeUI.OpenProject(projectDir, flagProject)
				if err != nil {
					return fmt.Errorf("project: %w", err)
				}
				broker := intercept.NewBroker()
				m := appUI.New(broker, project.Name, project.Path)
				final, err := tea.NewProgram(m).Run()
				if err != nil {
					return fmt.Errorf("tui: %w", err)
				}
				if app, ok := final.(appUI.Model); ok {
					if ferr := app.FatalErr(); ferr != nil {
						return ferr
					}
				}
				return nil
			}

			root := rootModel{home: homeUI.New(projectDir)}
			final, err := tea.NewProgram(root).Run()
			if err != nil {
				return fmt.Errorf("tui: %w", err)
			}
			if r, ok := final.(rootModel); ok {
				if ferr := r.FatalErr(); ferr != nil {
					return ferr
				}
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&flagConfig, "config", "c", "", "path to config file")
	rootCmd.Flags().StringVar(&flagPluginsDir, "plugins-dir", "", "path to plugins dir (overrides config)")
	rootCmd.Flags().StringVar(&flagHost, "host", "", "proxy host (overrides config)")
	rootCmd.Flags().IntVarP(&flagPort, "port", "p", 0, "proxy port (overrides config)")
	rootCmd.Flags().StringVar(&flagUpstreamProxy, "upstream-proxy", "", "upstream proxy URL, e.g. http://user:pass@host:8888 (overrides config)")
	rootCmd.Flags().StringVarP(&flagProject, "project", "P", "", `project name to open directly, or "tmp" for a temporary session`)
	rootCmd.Flags().BoolVar(&flagAddDefaultPlugins, "add-default-plugins", false, "copy built-in example plugins into the plugins dir and exit")
	rootCmd.Flags().BoolVar(&flagAddDefaultConfig, "add-default-config", false, "copy the default config file to the config path and exit")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
