package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/keys"
	spilltea "github.com/anotherhadi/spilltea"
	"github.com/anotherhadi/spilltea/internal/style"
	appUI "github.com/anotherhadi/spilltea/internal/ui/app"
	homeUI "github.com/anotherhadi/spilltea/internal/ui/home"
	flag "github.com/spf13/pflag"
)

// Version is overwritten at build time by goreleaser/ldflag with the current version tag, or "dev" if not set.
var version = "dev"

func main() {
	var (
		flagConfig            = flag.StringP("config", "c", "", "path to config file")
		flagPluginsDir        = flag.String("plugins-dir", "", "path to plugins dir (overrides config)")
		flagHost              = flag.String("host", "", "proxy host (overrides config)")
		flagPort              = flag.IntP("port", "p", 0, "proxy port (overrides config)")
		flagVersion           = flag.BoolP("version", "v", false, "print version")
		flagProject           = flag.StringP("project", "P", "", `project name to open directly, or "tmp" for a temporary session`)
		flagAddDefaultPlugins = flag.Bool("add-default-plugins", false, "copy built-in example plugins into the plugins dir and exit")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *flagAddDefaultPlugins {
		cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "spilltea", "config.yaml")
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

	if *flagProject != "" && !homeUI.IsValidProjectName(*flagProject) {
		fmt.Fprintf(os.Stderr, "project: invalid name %q (only lowercase letters, digits, - and _ are allowed)\n", *flagProject)
		os.Exit(1)
	}

	cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "spilltea", "config.yaml")
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

	addr := fmt.Sprintf("%s:%d", config.Global.App.Host, config.Global.App.Port)
	// Check if the proxy port is available before starting the UI.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxy: cannot bind to %s: %v\n", addr, err)
		os.Exit(1)
	}
	ln.Close()

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
