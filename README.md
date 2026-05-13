<div align="center">
    <img alt="logo" src="./.github/assets/logo.png" width="120px" />
</div>

<br>

# Spilltea

> A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.  
> Think Burp Suite or Caido, but entirely in your terminal.

[![Go Version](https://img.shields.io/github/go-mod/go-version/anotherhadi/spilltea)](go.mod)
[![Release](https://img.shields.io/github/v/release/anotherhadi/spilltea)](https://github.com/anotherhadi/spilltea/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/anotherhadi/spilltea)](https://goreportcard.com/report/github.com/anotherhadi/spilltea)

## What is Spilltea?

Spilltea is a **terminal-native HTTP(S) interception proxy**. It sits between your browser and the internet, letting you inspect, modify, and replay traffic without ever leaving your terminal.

It is intentionally minimal. No Electron, no browser, no bloat. Just a fast, keyboard-driven tool that gets out of your way.

## Features

- **Intercept**: Pause requests and responses in-flight. Inspect and modify them (even with your favorite editor) before forwarding.
- **HTTP History**: Every request that passes through the proxy is stored. Browse, search and filter your full session history.
- **Replay**: Pick any request from the history, modify it if needed, and send it again. Useful for manual testing and quick iteration
- **HTTPS Support** (using go-mitmproxy under the hood)
- Built-in Integrations:
  - **FFuf Export**: Generate a ffuf command or configuration directly from a request to start fuzzing instantly.
  - **cURL / HTTPie**: Copy any request as a curl or httpie command to your clipboard.
  - **Markdown Export**: Export any request and its response as a clean Markdown snippet, ready to drop into a report.

## Installation

<details>
<summary>Go install</summary>

```sh
go install github.com/anotherhadi/spilltea/cmd/spilltea@latest
```

Requires Go 1.22+. The binary will be placed in `$GOPATH/bin` (or `~/go/bin`).

</details>

<details>
<summary>Nix (temporary run, no install)</summary>

```sh
nix run github:anotherhadi/spilltea
```

</details>

<details>
<summary>NixOS (flake)</summary>

Add spilltea to your flake inputs:

```nix
inputs.spilltea.url = "github:anotherhadi/spilltea";
```

Then add the package to your system or home-manager packages:

```nix
environment.systemPackages = [ inputs.spilltea.packages.${pkgs.system}.default ];
```

</details>

## Project Management

Spilltea organizes work into **projects**. Each project maps to a SQLite database file that stores all intercepted traffic for that session & a log files.

On startup, you choose:

- **New project**: enter a name, stored in `~/.local/share/spilltea/projects/` by default
- **Existing project**: pick from a list of previous projects
- **Temporary**: no name needed, stored in `/tmp/spilltea/projects/` and will be deleted on your next reboot!

## Plugin System

Spilltea supports plugins written in **Lua**. Plugins are loaded from `~/.config/spilltea/plugins/` by default and do not require recompilation or access to the source code.
For a full reference and examples, see the [plugin documentation](./.github/docs/plugins.md) or [plugin examples](./plugins/).

## Configuration

Spilltea is fully configured via a YAML file at `~/.config/spilltea/config.yaml`.
Check the default configuration with all the options [here](./internal/config/default_config.yaml)

## CLI Flags

| Flag                    | Short | Description                                                                    |
| ----------------------- | ----- | ------------------------------------------------------------------------------ |
| `--config`              | `-c`  | Path to config file (default: `~/.config/spilltea/config.yaml`)                |
| `--plugin-dir`          |       | Path to plugins dir, overrides config (default: `~/.config/spilltea/plugins/`) |
| `--host`                |       | Proxy host, overrides config                                                   |
| `--port`                | `-p`  | Proxy port, overrides config                                                   |
| `--project`             | `-P`  | Project name to open directly, or `tmp` for a temporary session                |
| `--upstream-proxy`      |       | Upstream proxy URL, overrides config (e.g. `http://user:pass@host:8888`)       |
| `--version`             | `-v`  | Print version and exit                                                         |
| `--add-default-plugins` |       | Add the default plugins to your plugins dir and exit                           |

## Deployment

spilltea runs **locally** on the machine used for pentesting or CTF. There is no separate server component.

If you need to run spilltea on a remote machine (e.g., a VPS or pivot host), use SSH port forwarding:

```sh
ssh -L 8080:127.0.0.1:8080 user@remote-host
```

Then point your browser at `127.0.0.1:8080` as usual.

## Tech Stack

| Component          | Library                                                   |
| ------------------ | --------------------------------------------------------- |
| TUI                | [bubbletea](https://github.com/charmbracelet/bubbletea)   |
| Styles             | [lipgloss](https://github.com/charmbracelet/lipgloss)     |
| Proxy / MITM / TLS | [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy) |
| Storage            | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite)     |
| Config             | [viper](https://github.com/spf13/viper)                   |
| Plugins            | [gopher-lua](https://github.com/yuin/gopher-lua)          |

---

<div align="center">
  <a href="https://github.com/anotherhadi/spilltea">github</a> |
  <a href="https://gitlab.com/anotherhadi_mirror/spilltea">gitlab (mirror)</a> |
  <a href="https://git.hadi.icu/anotherhadi/spilltea">gitea (mirror)</a>
</div
