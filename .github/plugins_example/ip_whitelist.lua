-- Check that the proxy's outbound IP is in the whitelist before starting.
-- Config: one allowed IP per line. Leave empty to disable the check.

Plugin = {
  name     = "IP Whitelist",
  on_start = {},
}

function on_start(config_text)
  local allowed = {}
  for line in config_text:gmatch("[^\n]+") do
    local ip = line:match("^%s*(.-)%s*$")
    if ip ~= "" then
      table.insert(allowed, ip)
    end
  end

  if #allowed == 0 then
    log("no IPs configured, skipping check")
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
    return
  end

  log("outbound IP: " .. result)

  for _, ip in ipairs(allowed) do
    if result == ip then
      log("IP " .. result .. " is whitelisted")
      return
    end
  end

  notif("IP Whitelist", "Outbound IP " .. result .. " is NOT in the whitelist!")
  quit("outbound IP " .. result .. " not whitelisted")
end
