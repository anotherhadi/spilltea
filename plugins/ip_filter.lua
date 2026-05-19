Plugin = {
  name        = "IP Filter (Whitelist/Blacklist)",
  description = [[
Checks that the proxy's outbound IP is in an allowed list on startup.

**Config**:
- one IP per line
- prefix with `!` for a blacklist entry (blocked)
- prefix with `#` to comment it out (ignored)
- if no IPs are configured, the check is skipped
  ]],
  on_start = { sync = false },
  disable_by_default = true,
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
        local ip = trimmed:sub(2):match("^%s*(.-)%s*$")
        if ip ~= "" then
          table.insert(blacklist, ip)
        end
      else
        table.insert(whitelist, trimmed)
      end
    end
  end
end

function on_start()
  if #whitelist == 0 and #blacklist == 0 then
    return
  end

  -- Fetch the current outbound IP via a public API.
  local ok, result = pcall(function()
    local handle = io.popen("curl -sf https://api.ipify.org 2>/dev/null")
    if not handle then return nil end
    local ip = handle:read("*a")
    handle:close()
    return ip and ip:match("^%s*(.-)%s*$") or nil
  end)

  if not ok or not result or result == "" then
    log("could not determine outbound IP, skipping check")
    notif("IP Filter", "Could not determine outbound IP, skipping check", "warning")
    return
  end

  for _, ip in ipairs(blacklist) do
    if result == ip then
      notif("IP Filter", "Outbound IP " .. result .. " is blacklisted!", "error")
      return
    end
  end

  if #whitelist == 0 then
    return
  end

  for _, ip in ipairs(whitelist) do
    if result == ip then
      return
    end
  end

  notif("IP Filter", "Outbound IP " .. result .. " is not in the whitelist!", "error")
end
