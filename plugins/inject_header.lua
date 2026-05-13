Plugin = {
  name        = "Inject Header",
  description = [[
Inject custom headers into every intercepted request.

**Config**:
- one 'Header-Name: value' per line.
  ]],
  on_request  = { sync = true },
}

local headers = {}

function on_config(config_text)
  headers = {}
  for line in config_text:gmatch("[^\n]+") do
    local name, value = line:match("^([^:]+):%s*(.+)$")
    if name and value then
      table.insert(headers, { name = name, value = value })
    end
  end
end

function on_request(req)
  for _, h in ipairs(headers) do
    req:set_header(h.name, h.value)
  end
end
