Plugin = {
  name        = "Scopes",
  description = [[
Auto-forward requests and exclude them from history based on patterns.

**Config**:
- `pattern`    - whitelist: only intercept matching requests/responses and history entries
- `!pattern`   - blacklist: skip matching requests/responses and history entries
- `r:pattern`  - whitelist for requests/responses only (history unaffected)
- `r:!pattern` - blacklist for requests/responses only
- `h:pattern`  - whitelist for history entries only (requests unaffected)
- `h:!pattern` - blacklist for history entries only
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

Example (disable history: whitelist never matches any real URL):
```
h:^$
```
  ]],
  priority         = 100,
  on_request       = { sync = true },
  on_response       = { sync = true },
  on_history_entry = { sync = true },
}

local blacklist      = {}
local whitelist      = {}
local blacklist_req  = {}
local whitelist_req  = {}
local blacklist_hist = {}
local whitelist_hist = {}

function on_config(config_text)
  blacklist, whitelist = {}, {}
  blacklist_req, whitelist_req = {}, {}
  blacklist_hist, whitelist_hist = {}, {}
  for line in config_text:gmatch("[^\n]+") do
    local trimmed = line:match("^%s*(.-)%s*$")
    if trimmed ~= "" and trimmed:sub(1, 1) ~= "#" then
      local scope = trimmed:match("^([rh]):")
      local rest  = scope and trimmed:sub(3) or trimmed
      local is_bl = rest:sub(1, 1) == "!"
      local pattern = is_bl and rest:sub(2) or rest
      if scope == "r" then
        table.insert(is_bl and blacklist_req or whitelist_req, pattern)
      elseif scope == "h" then
        table.insert(is_bl and blacklist_hist or whitelist_hist, pattern)
      else
        table.insert(is_bl and blacklist or whitelist, pattern)
      end
    end
  end
end

local function check_skip(url, bl_extra, wl_extra)
  for _, p in ipairs(blacklist) do
    if url:match(p) then return true end
  end
  for _, p in ipairs(bl_extra) do
    if url:match(p) then return true end
  end
  local wl = {}
  for _, p in ipairs(whitelist)  do wl[#wl + 1] = p end
  for _, p in ipairs(wl_extra)   do wl[#wl + 1] = p end
  if #wl > 0 then
    for _, p in ipairs(wl) do
      if url:match(p) then return false end
    end
    return true
  end
  return false
end

function on_request(req)
  if check_skip(req.url, blacklist_req, whitelist_req) then return "forward" end
end

function on_response(req)
  if check_skip(req.url, blacklist_req, whitelist_req) then return "forward" end
end

function on_history_entry(entry)
  if check_skip(entry.host .. entry.path, blacklist_hist, whitelist_hist) then return "skip" end
end
