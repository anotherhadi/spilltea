Plugin = {
  name        = "Scopes",
  description = [[
Auto-forward requests and exclude them from history based on patterns.

**Config**:
- `pattern`  - whitelist: only intercept matching requests
- `!pattern` - blacklist: don't intercept matching requests and exclude from history
- lines starting with `#` are comments

Example (ignore static assets):
```
!%.css$
!%.js$
!%.png$
```

Example (focus on mytarget.com, skip everything else):
```
mytarget%.com/
```

Example (intercept mytarget.com except its static assets):
```
mytarget%.com/
!%.css$
!%.js$
!%.png$
```
  ]],
  priority         = 100,
  on_request       = { sync = true },
  on_response       = { sync = true },
  on_history_entry = { sync = true },
}

local whitelist = {}
local blacklist = {}

function on_config(config_text)
  whitelist = {}
  blacklist = {}
  for line in config_text:gmatch("[^\n]+") do
    local trimmed = line:match("^%s*(.-)%s*$")
    if trimmed ~= "" and trimmed:sub(1, 1) ~= "#" then
      if trimmed:sub(1, 1) == "!" then
        table.insert(blacklist, trimmed:sub(2))
      else
        table.insert(whitelist, trimmed)
      end
    end
  end
end

local function should_skip(url)
  for _, pattern in ipairs(blacklist) do
    if url:match(pattern) then return true end
  end
  if #whitelist > 0 then
    for _, pattern in ipairs(whitelist) do
      if url:match(pattern) then return false end
    end
    return true
  end
  return false
end

function on_request(req)
  if should_skip(req.url) then return "forward" end
end

function on_response(req)
  if should_skip(req.url) then return "forward" end
end

function on_history_entry(entry)
  if should_skip(entry.host .. entry.path) then return "skip" end
end
