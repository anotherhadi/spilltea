Plugin = {
	name = "Inject Header",
	description = [[
Inject custom headers into every intercepted request.

**Config**:
```yaml
headers:
  - "X-My-Header: myvalue"
```
  ]],
	on_request = { sync = true },
}

local headers = {}

function on_config()
	headers = {}
	local cfg = get_config()
	if cfg and cfg.headers then
		for _, line in ipairs(cfg.headers) do
			local name, value = line:match("^([^:]+):%s*(.+)$")
			if name and value then
				table.insert(headers, { name = name, value = value })
			end
		end
	end
end

function on_request(req)
	for _, h in ipairs(headers) do
		req:set_header(h.name, h.value)
	end
end
