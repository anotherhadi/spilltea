Plugin = {
	name = "IP Filter (Whitelist/Blacklist)",
	description = [[
Checks that the proxy's outbound IP is in an allowed list on startup.

**Config**:
```yaml
ips:
  - "1.2.3.4"     # whitelist entry
  - "!5.6.7.8"    # blacklist entry (blocked)
```

- If no IPs are configured, the check is skipped.
  ]],
	on_start = { sync = false },
	disable_by_default = true,
}

local whitelist = {}
local blacklist = {}

function on_config()
	whitelist, blacklist = {}, {}
	local cfg = get_config()
	if cfg and cfg.ips then
		for _, entry in ipairs(cfg.ips) do
			local trimmed = entry:match("^%s*(.-)%s*$")
			if trimmed ~= "" then
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
end

function on_start()
	if #whitelist == 0 and #blacklist == 0 then
		return
	end

	local result, err = shell_pipe("curl -sf https://api.ipify.org 2>/dev/null")
	result = result and result:match("^%s*(.-)%s*$") or nil

	if err or not result or result == "" then
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
