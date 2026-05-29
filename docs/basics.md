## Project Management

Spilltea organizes work into **projects**. Each project maps to a SQLite database file that stores all intercepted traffic for that session & a log files.

On startup, you choose:

- **New project**: enter a name, stored in `~/.local/share/spilltea/projects/` by default
- **Existing project**: pick from a list of previous projects
- **Temporary**: no name needed, stored in `/tmp/spilltea/projects/` and will be deleted on your next reboot!

## Configuration

Spilltea is fully configured via a YAML file at `~/.config/spilltea/config.yaml`.
Check the default configuration with all the options [here](./internal/config/default_config.yaml)

Colors and styles can be customized using [ilovetui](https://github.com/anotherhadi/ilovetui), which applies theme changes across all compatible TUI applications at once.

## CLI Flags

<!-- exec: echo '```' && go run ./cmd/spilltea -h && echo '```' -->
```
A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.

Usage:
  spilltea [flags]

Flags:
      --add-default-config      copy the default config file to the config path and exit
      --add-default-plugins     copy built-in example plugins into the plugins dir and exit
  -c, --config string           path to config file
  -h, --help                    help for spilltea
      --host string             proxy host (overrides config)
      --plugins-dir string      path to plugins dir (overrides config)
  -p, --port int                proxy port (overrides config)
  -P, --project string          project name to open directly, or "tmp" for a temporary session
      --ssl-insecure            skip TLS certificate verification (overrides config)
      --upstream-proxy string   upstream proxy URL, e.g. http://user:pass@host:8888 (overrides config)
  -v, --version                 version for spilltea
```
<!-- endexec -->
