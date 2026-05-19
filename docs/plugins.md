# Plugins

Spilltea supports Lua plugins that can intercept, modify, and analyze HTTP traffic.
You can found some pre-built plugins [here](../../plugins/).

## Where to place plugins

Put `.lua` files in the directory configured by `plugins_dir` in your config file (default: `~/.config/spilltea/plugins`).

Each file is loaded as a separate plugin at startup. The plugin list is shown on the **Plugins** page.

## Plugin structure

Every plugin must declare a `Plugin` table and implement the hooks it wants to use.

```lua
Plugin = {
  name        = "My Plugin",
  description = "What this plugin does.",
  priority    = 0,  -- higher = runs before other plugins (default: 0)

  -- Declare which hooks you use and whether they are synchronous (default: false).
  -- on_config and on_quit are always sync and do not need to be declared here.
  on_start          = { sync = true  },
  on_request        = { sync = true  },
  on_response       = { sync = false },
  on_history_entry  = { sync = true  },
}
```

### Hook reference

| Hook                      | When called                           | Sync/async   | Return value (sync only)                        |
| ------------------------- | ------------------------------------- | ------------ | ----------------------------------------------- |
| `on_config(config_text)`  | At startup and on config save         | always sync  | ignored                                         |
| `on_start()`              | Once at startup, after `on_config`    | configurable | ignored                                         |
| `on_quit()`               | When the app exits                    | always sync  | ignored                                         |
| `on_request(req)`         | Every request, before auto-forward    | configurable | `"drop"`, `"forward"`, or `nil`                 |
| `on_response(req, res)`   | Every response                        | configurable | `"drop"`, `"forward"`, or `nil`                 |
| `on_history_entry(entry)` | Sync: before DB insert / Async: after | configurable | `"skip"` (don't save), `"keep"` or `nil` (save) |

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
```

### Finding deduplication

A finding is identified by `(plugin_name, key)`. If a finding with that pair already exists in the database it will **not** be re-created, even across restarts. If the user **dismisses** a finding it is permanently hidden and will never reappear, even if the plugin generates it again.

## Configuration

Each plugin gets a **config textarea** on the Plugins page. The raw text is passed as-is to `on_config(config_text)`. Parse it however you like (line by line, key=value, JSON, etc.).

`on_config` is called once at startup (before `on_start`) and again every time the user saves the config in the UI.

## Sync vs async

- **`sync = true`**: spilltea waits for the hook to return before continuing. The hook can return a decision value (see below).
- **`sync = false`** (default for all configurable hooks): the hook runs in a background goroutine. Return values are ignored.

`on_config` and `on_quit` are always synchronous regardless of the Plugin table declaration.

### Return values for sync hooks

**`on_request` and `on_response`:**

| Return value | Effect                                                                            |
| ------------ | --------------------------------------------------------------------------------- |
| `"drop"`     | The flow is dropped immediately and never shown in the intercept panel.           |
| `"forward"`  | The flow is forwarded immediately without going through the intercept panel.      |
| `nil`        | Normal behaviour: the flow appears in the intercept panel for the user to decide. |

**`on_history_entry` (sync only):**

| Return value      | Effect                            |
| ----------------- | --------------------------------- |
| `"skip"`          | The entry is not saved to the DB. |
| `"keep"` or `nil` | The entry is saved normally.      |

Sync `on_history_entry` runs **before** the DB insert, so it can prevent an entry from ever appearing in history. Async `on_history_entry` runs **after** the insert and cannot affect it.

## Priority

Plugins with a higher `priority` value run before plugins with a lower value (default `0`). This matters for sync hooks that return a decision: the first plugin to return a non-nil value short-circuits the remaining plugins.

```lua
Plugin = {
  name     = "Scopes",
  priority = 100,  -- runs before all default-priority plugins
  ...
}
```

> A sync hook that hangs will block traffic for that flow. There is no automatic timeout.
