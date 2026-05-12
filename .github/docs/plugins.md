# Plugins

Spilltea supports Lua plugins that can intercept, modify, and analyze HTTP traffic.

## Where to place plugins

Put `.lua` files in the directory configured by `plugins_dir` in your config file (default: `~/.config/spilltea/plugins`).

Each file is loaded as a separate plugin at startup. The plugin list is shown on the **Plugins** page.

## Plugin structure

Every plugin must declare a `Plugin` table and implement the hooks it wants to use.

```lua
Plugin = {
  name = "My Plugin",

  -- Declare which hooks you use and whether they are synchronous.
  on_start         = { sync = true  },
  on_request       = { sync = true  },
  on_response      = { sync = false },
  on_history_entry = {},
  on_quit          = {},
}
```

### Hook reference

| Hook                      | When called                 | Sync/async   | Return value        |
| ------------------------- | --------------------------- | ------------ | ------------------- |
| `on_start(config_text)`   | Once at startup             | always sync  | ignored             |
| `on_quit()`               | When the app exits          | always sync  | ignored             |
| `on_request(req)`         | Every request               | declared     | `"drop"`, `"forward"`, or `nil` (sync only) |
| `on_response(req, res)`   | Every response              | declared     | `"drop"`, `"forward"`, or `nil` (sync only) |
| `on_history_entry(entry)` | After a flow is saved to DB | always async | ignored             |

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
notif("Title", "Body text")

-- Create a finding (shown on the Findings page, persisted in DB)
create_finding({
  title       = "API Key Found",
  description = "Markdown description of the finding...",
  key         = "stable-unique-id",  -- used for deduplication; defaults to title
  severity    = "high",              -- info | low | medium | high | critical
})

-- Check if a URL matches the current scope (whitelist/blacklist)
local ok = is_in_scope("https://example.com/api/v1")

-- Quit the app (useful for startup checks that fail)
quit("reason message")
```

### Finding deduplication

A finding is identified by `(plugin_name, key)`. If a finding with that pair already exists in the database it will **not** be re-created, even across restarts. If the user **dismisses** a finding it is permanently hidden and will never reappear, even if the plugin generates it again.

## Configuration

Each plugin gets a **config textarea** on the Plugins page. The raw text is passed as-is to `on_start(config_text)`. Parse it however you like (line by line, key=value, JSON, etc.).

## Sync vs async

- **`sync = true`**: spilltea waits for the hook to return before continuing. For `on_request`/`on_response` this blocks the proxy goroutine; the hook can return one of the values below.
- **`sync = false`** (or omitted for supported hooks): the hook runs in a background goroutine. Return values are ignored. Use this for analysis and findings.

### Return values for `on_request` and `on_response` (sync only)

| Return value | Effect |
| ------------ | ------ |
| `"drop"`    | The flow is dropped immediately and never shown in the intercept panel. |
| `"forward"` | The flow is forwarded immediately without going through the intercept panel. |
| `nil`        | Normal behaviour: the flow appears in the intercept panel for the user to decide. |

The `sync` declaration is only meaningful for `on_request` and `on_response`. The other hooks have fixed behaviour:

- `on_start` is **always synchronous**: plugins are initialised one by one before the first request is accepted.
- `on_quit` is **always synchronous**: the app waits for all `on_quit` hooks before exiting.
- `on_history_entry` is **always asynchronous**.

> A sync `on_request` or `on_response` hook that hangs will block traffic for that flow. There is no automatic timeout.
