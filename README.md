```
█▀▀ █▀▀ █▀▀ ▀█▀ █ █ █   ▄▀ █▀▀ █▀█ █ █   █   ▀█▀ █▀▀ ▄▀█
▄▄█ ██▄ █▄▄  █  █▄█ █ ▄▀   ▄▄█ █▀▀ █ █▄▄ █▄▄  █  ██▄ █▀█
```

# Spilltea

An intercepting HTTP proxy for the terminal. Hold, edit and drop requests and
responses, keep the history as plain files, search it with
[HTTPQL](https://docs.caido.io/reference/httpql.html), and hand requests over to
[steep](https://github.com/anotherhadi/steep) to replay and fuzz them.

If you know Burp Suite or Caido: spilltea covers the proxy, history and sitemap,
steep covers the repeater and intruder. It is built for pentesters, bug bounty
hunters and CTF players.

## Installation

With go:

```sh
go install github.com/anotherhadi/spilltea/cmd/spilltea@latest
```

With Nix, from the [anotherhadi NUR](https://github.com/anotherhadi/nur):

```sh
nix run github:anotherhadi/nur#spilltea
```

```nix
# flake: inputs.nur-anotherhadi.url = "github:anotherhadi/nur";
environment.systemPackages = [ nur-anotherhadi.packages.${system}.spilltea ];

# or through NUR
environment.systemPackages = [ pkgs.nur.repos.anotherhadi.spilltea ];
```

Each [GitHub release](https://github.com/anotherhadi/spilltea/releases) has an
archive per platform (Linux, macOS and FreeBSD, amd64 and arm64) with the
`spilltea` binary:

```sh
tar xzf spilltea_1.2.3_linux_amd64.tar.gz
sudo install spilltea /usr/local/bin/
spilltea --version   # 1.2.3
```

## Quick start

```sh
spilltea ca > spilltea-ca.pem   # export the CA certificate (created on first run)
spilltea                        # start the proxy on 127.0.0.1:8080
```

Set your browser's proxy to `127.0.0.1:8080`
([PwnFox](https://github.com/yeswehack/PwnFox) can do it per container) and
import `spilltea-ca.pem` as a trusted authority for websites. The CA lives in
`~/.local/share/spilltea/ca`.

> [!WARNING]
> Anyone holding the CA's private key (`mitmproxy-ca.pem`) can intercept the TLS
> traffic of a browser that trusts it. Keep it private, and prefer a dedicated
> browser profile for testing.

## Pages

- **Intercept**: hold requests and responses, edit them in a vim-like editor,
  forward or drop them.
- **History**: everything in scope, filtered with HTTPQL, with marks and notes.
  Search a message, or pipe it through a shell command.
- **Sitemap**: the history as a tree of hosts and paths.
- **Diff**: two entries side by side, requests or responses.
- **Findings**: what the detectors found in the traffic, to triage one by one.

Press `?` to see the keys of the current pane, or `ctrl+p` for the command
palette.

## Scope

Out-of-scope traffic goes through untouched: it is not decrypted, intercepted or
stored. Put a program's scope in the project's own config:

```yaml
# <project>/config.yaml
scope:
    include: ["*.example.com", "api.example.org:8443", "example.net/api"]
    exclude: ["logout.example.com"]
```

Or from the environment, as comma-separated lists:

```sh
SPILLTEA_SCOPE__INCLUDE='*.example.com,api.example.org:8443' \
SPILLTEA_SCOPE__EXCLUDE='logout.example.com' \
spilltea
```

A pattern is `host[:port][/path]`, where `*` matches any run of characters and
the path is a prefix. An empty `include` list means everything.

`intercept.filter` limits what gets held, using HTTPQL. For example
`req.method.ne:"GET"` only holds GET requests.

## PwnFox

The `X-PwnFox-Color` header sent by
[PwnFox](https://github.com/yeswehack/PwnFox) containers colors the entries and
is stripped before the request leaves (`http.stripPwnFox`). Filter one container
with `row.color.eq:"red"`. Requests sent to steep keep the header, so steep
shows the color too.

## Findings

Detectors look at the traffic as it goes through, scripts and other assets
included, even when their body is not stored:

```yaml
findings:
    secrets: true # API keys, tokens...
    cookies: true # no Secure, no HttpOnly, SameSite=None
    cors: true # credentials allowed for a specific origin
    disclosure: true # stack traces, SQL errors, source maps, versions
    reflection: true # parameters found as is in an HTML response
```

In the history, `row.finding.eq:"secrets"` or `row.finding.cont:"aws"` filter
the requests with findings, and `preset:"findings"` all of them.

## Steep

`s` writes the request into a steep collection under `spilltea/<host>/<path>`
(`steep.folder`), and tells every steep open on that collection to open it.

## Import

Feed URLs found by recon tools into the history and the sitemap, without sending
any request:

```sh
urlfinder -d example.com -j | spilltea import
katana -u https://example.com -jsonl -o katana.jsonl && spilltea import katana.jsonl
httpx -l hosts.txt -json -irr | spilltea import --project ~/bb/example
```

Each line is either a URL (`url [200]` works too) or a JSON line from httpx
(`-json`, plus `-irr` for the full messages), katana (`-jsonl`) or urlfinder
(`-j`).

Filter by tool with `row.source.eq:"katana"` (`--source` sets the name).

With `history.jsEndpoints: true`, the paths found in the scripts going through
the proxy are added the same way, as `row.source.eq:"js"`.

## The project

The history lives in a project folder (`project` setting, `--project` or
`SPILLTEA_PROJECT`; `~/.local/share/spilltea/project` by default), as plain
files:

```
<project>/
├── config.yaml     # optional, written by you: scope, rules
├── index.jsonl     # one line per entry, appended as traffic comes in
├── meta.json       # marks and notes
├── findings.jsonl  # one line per finding seen in a request
├── findings.json   # their triage and notes
├── requests/<id>   # raw request, as sent
└── responses/<id>  # raw response, as stored
```

`history.responses` controls how much of a response is kept: `full`, `headers`,
`status` or `none`. Bodies larger than `history.maxBodySize` are dropped but
their headers are kept. Bodies are stored decompressed.

## Configuration

`spilltea --print-config` prints every setting, and
[`internal/config/default.yaml`](internal/config/default.yaml) lists the
defaults. Spilltea reads, in order: `~/.config/foam/config.yaml`,
`~/.config/spilltea/config.yaml`, the project's `config.yaml`, `SPILLTEA_*`
environment variables, then command-line flags.
