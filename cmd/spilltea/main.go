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
	flag "github.com/spf13/pflag"
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
		flagConfig            = flag.StringP("config", "c", "", "path to config file")
		flagPluginsDir        = flag.String("plugins-dir", "", "path to plugins dir (overrides config)")
		flagHost              = flag.String("host", "", "proxy host (overrides config)")
		flagPort              = flag.IntP("port", "p", 0, "proxy port (overrides config)")
		flagUpstreamProxy     = flag.String("upstream-proxy", "", "upstream proxy URL, e.g. http://user:pass@host:8888 (overrides config)")
		flagVersion           = flag.BoolP("version", "v", false, "print version")
		flagProject           = flag.StringP("project", "P", "", `project name to open directly, or "tmp" for a temporary session`)
		flagAddDefaultPlugins = flag.Bool("add-default-plugins", false, "copy built-in example plugins into the plugins dir and exit")
		flagAddDefaultConfig  = flag.Bool("add-default-config", false, "copy the default config file to the config path and exit")
	)
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = func() {
		fmt.Println("Usage: spilltea [flags]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *flagVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *flagAddDefaultPlugins {
		home, _ := os.UserHomeDir()
		cfgPath := filepath.Join(home, ".config", "spilltea", "config.yaml")
		if *flagConfig != "" {
			cfgPath = *flagConfig
		}
		if err := config.Load(cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
		dir := config.ExpandPath(config.Global.App.PluginsDir)
		if *flagPluginsDir != "" {
			dir = *flagPluginsDir
		}
		n, err := spilltea.InstallDefaultPlugins(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "add-default-plugins: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("added %d plugin(s) to %s\n", n, dir)
		os.Exit(0)
	}

	if *flagAddDefaultConfig {
		home, _ := os.UserHomeDir()
		cfgPath := filepath.Join(home, ".config", "spilltea", "config.yaml")
		if *flagConfig != "" {
			cfgPath = *flagConfig
		}
		if err := config.WriteDefaultConfig(cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "add-default-config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("default config written to %s\n", cfgPath)
		os.Exit(0)
	}

	if *flagProject != "" && !homeUI.IsValidProjectName(*flagProject) {
		fmt.Fprintf(os.Stderr, "project: invalid name %q (only lowercase letters, digits, - and _ are allowed)\n", *flagProject)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "spilltea", "config.yaml")
	if *flagConfig != "" {
		cfgPath = *flagConfig
	}

	if err := config.Load(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	config.Global.Version = version

	if *flagPluginsDir != "" {
		config.Global.App.PluginsDir = *flagPluginsDir
	}
	if *flagHost != "" {
		config.Global.App.Host = *flagHost
	}
	if *flagPort != 0 {
		config.Global.App.Port = *flagPort
	}
	if *flagUpstreamProxy != "" {
		config.Global.App.UpstreamProxy = *flagUpstreamProxy
	}

	style.Init(config.Global)
	icons.Init(config.Global)
	keys.Init(config.Global)

	projectDir := config.ExpandPath(config.Global.App.ProjectDir)

	// Resolve project: either from --project flag or by running the home UI.
	var project *homeUI.Project
	if *flagProject != "" {
		p, err := homeUI.OpenProject(projectDir, *flagProject)
		if err != nil {
			fmt.Fprintf(os.Stderr, "project: %v\n", err)
			os.Exit(1)
		}
		project = p
	} else {
		finalModel, err := tea.NewProgram(homeUI.New(projectDir)).Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(1)
		}
		project = finalModel.(homeUI.Model).Selected()
	}

	// User quit the home screen without selecting a project.
	if project == nil {
		return
	}

	broker := intercept.NewBroker()
	m := appUI.New(broker, project.Name, project.Path)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui: %v\n", err)
		os.Exit(1)
	}
}
