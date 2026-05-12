-- Inject a custom header into every request.
-- Config format (one per line): Header-Name: value

Plugin = {
  name       = "Inject Header",
  on_request = { sync = true },
}

local headers = {}

function on_start(config_text)
  for line in config_text:gmatch("[^\n]+") do
    local name, value = line:match("^([^:]+):%s*(.+)$")
    if name and value then
      table.insert(headers, { name = name, value = value })
    end
  end
  log("loaded " .. #headers .. " header(s)")
end

function on_request(req)
  for _, h in ipairs(headers) do
    req:set_header(h.name, h.value)
  end
  return "forward"
end
