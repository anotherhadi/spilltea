# Plugins

> **Warning:** Plugins can execute arbitrary shell commands, read and write files via `shell_pipe`, and access all intercepted traffic. Only load plugins you trust and have reviewed. You are solely responsible for the plugins you run.

Spilltea supports Lua plugins that can intercept, modify, and analyze HTTP traffic.
You can found some pre-built plugins [here](../../plugins/).

## Where to place plugins

Put `.lua` files in the directory configured by `plugins_dir` in your config file (default: `~/.config/spilltea/plugins`).

Each file is loaded as a separate plugin at startup. The plugin list is shown on the **Plugins** page.

## Plugin structure

Every plugin must declare a `Plugin` table and implement the hooks it wants to use.

```lua
Plugin = {
  name               = "My Plugin",
  description        = "What this plugin does.",
  priority           = 0,     -- higher = runs before other plugins (default: 0)
  disable_by_default = true,  -- if true, plugin starts disabled on first load (default: false)

  -- Declare which hooks you use and whether they are synchronous (default: false).
  -- on_config and on_quit are always sync and do not need to be declared here.
  on_start          = { sync = true  },
  on_request        = { sync = true  },
  on_response       = { sync = false },
  on_history_entry  = { sync = true  },
}
```

### Hook reference

| Hook                      | When called                           | Sync/async   | Return value                                                                              |
| ------------------------- | ------------------------------------- | ------------ | ----------------------------------------------------------------------------------------- |
| `on_config()`             | At startup and on config save         | always sync  | ignored                                                                                   |
| `on_start()`              | Once at startup, after `on_config`    | configurable | `false` to self-disable the plugin, otherwise ignored                                     |
| `on_quit()`               | When the app exits                    | always sync  | ignored                                                                                   |
| `on_request(req)`         | Every request, before auto-forward    | configurable | `"drop"`, `"forward"`, or `nil` (nil does nothing and le the user/TUI choose) (sync only) |
| `on_response(req, res)`   | Every response                        | configurable | `"drop"`, `"forward"`, or `nil` (nil does nothing and le the user/TUI choose) (sync only) |
| `on_history_entry(entry)` | Sync: before DB insert / Async: after | configurable | `"skip"` (don't save), `"keep"` or `nil` (save) (sync only)                               |

## Request and response objects

### `req` (request)

| Field / method                | Type   | Description                         |
| ----------------------------- | ------ | ----------------------------------- |
| `req.method`                  | string | HTTP method                         |
| `req.url`                     | string | Full URL                            |
| `req.host`                    | string | Host                                |
| `req.path`                    | string | Path                                |
| `req.headers["Name"]`         | string | Request header value                |
| `req:get_body()`              | string | Raw request body (loaded on demand) |
| `req:set_header(name, value)` | -      | Set a request header                |
| `req:set_body(body)`          | -      | Replace the request body            |

### `res` (response)

| Field / method                | Type   | Description               |
| ----------------------------- | ------ | ------------------------- |
| `res.status_code`             | number | HTTP status code          |
| `res.headers["Name"]`         | string | Response header value     |
| `res:get_body()`              | string | Raw response body         |
| `res:set_header(name, value)` | -      | Set a response header     |
| `res:set_body(body)`          | -      | Replace the response body |

### `entry` (history entry)

| Field                | Type                         |
| -------------------- | ---------------------------- |
| `entry.id`           | number                       |
| `entry.method`       | string                       |
| `entry.host`         | string                       |
| `entry.path`         | string                       |
| `entry.status_code`  | number                       |
| `entry.timestamp`    | string (YYYY-MM-DD HH:MM:SS) |
| `entry.request_raw`  | string                       |
| `entry.response_raw` | string                       |

## Utility functions

```lua
-- Log a message to logs.log (prefixed with the plugin name)
log("message")

-- Send a notification bubble in the TUI
-- kind is optional: "info" (default), "success", "warning", "error"
notif("Title", "Body text", "warning")

-- Create a finding (shown on the Findings page, persisted in DB)
create_finding({
  title       = "API Key Found",
  description = "Markdown description of the finding...",
  key         = "stable-unique-id",  -- used for deduplication; defaults to title
  severity    = "high",              -- info | low | medium | high | critical
})

-- Run a raw SQL query against the project DB (entries, findings, replay_entries, …)
-- Returns a table of rows; each row is a table indexed by column name.
-- Returns nil + error string on failure.
local rows, err = db_query("SELECT id, host FROM entries WHERE host = ?", "example.com")
if err then
  log("query failed: " .. err)
else
  for i = 1, #rows do
    log(rows[i].host)
  end
end

-- Quit the app (useful for startup checks that fail)
quit("reason message")

-- Run a shell command, optionally piping a string to its stdin.
-- Returns: output string, error string (nil on success).
-- The command runs via "sh -c" with a 30-second timeout.
local out, err = shell_pipe("trufflehog filesystem --no-update --json /dev/stdin", body)
if err then
  log("command failed: " .. err)
else
  log("output: " .. out)
end

-- Return the plugin's config section as a Lua table (parsed from YAML).
-- Returns an empty table if no config is set.
local cfg = get_config()
```

### Finding deduplication

A finding is identified by `(plugin_name, key)`. If a finding with that pair already exists in the database it will **not** be re-created, even across restarts.
If the user **dismisses** a finding it is permanently hidden and will never reappear, even if the plugin generates it again.

## Configuration

Plugin configuration is stored in a `plugins.yaml` file alongside the project database.
Each plugin is keyed by its filename (without the `.lua` extension) and has an `enable` toggle and an optional `config` block (arbitrary YAML).

```yaml
plugins:
  my_plugin:
    enable: true
    config:
      some_key: some_value
      list:
        - item1
        - item2
  other_plugin:
    enable: false
```

The config block is edited from the **Plugins** page in the TUI.
Inside a plugin, call `get_config()` to retrieve the config as a Lua table.

Global defaults for any plugin can be set in `~/.config/spilltea/config.yaml` under a `plugins` key with the same structure as `plugins.yaml`.
These defaults are applied the first time a plugin is loaded in a project; once the plugin has an entry in the project's `plugins.yaml`, the project config takes full precedence and the global defaults are ignored.

`on_config()` is called once at startup (before `on_start`) and again every time the user saves the config in the TUI.
It is the right place to read `get_config()` and populate local variables.

```lua
local items = {}

function on_config()
  items = {}
  local cfg = get_config()
  if cfg and cfg.list then
    for _, v in ipairs(cfg.list) do
      table.insert(items, v)
    end
  end
end
```

## Sync vs async

- **`sync = true`**: spilltea waits for the hook to return before continuing. The hook can return a decision value (see below).
- **`sync = false`** (default for all configurable hooks): the hook runs in a background goroutine. Return values are ignored.

`on_config` and `on_quit` are always synchronous regardless of the Plugin table declaration.

Sync `on_history_entry` runs **before** the DB insert, so it can prevent an entry from ever appearing in history.
Async `on_history_entry` runs **after** the insert and cannot affect it.

## Priority

Plugins with a higher `priority` value run before plugins with a lower value (default `0`).
This matters for sync hooks that return a decision: the first plugin to return a non-nil value short-circuits the remaining plugins.

```lua
Plugin = {
  name     = "Scopes",
  priority = 100,  -- runs before all default-priority plugins
  ...
}
```

> A sync hook that hangs will block traffic for that flow. There is no automatic timeout.
