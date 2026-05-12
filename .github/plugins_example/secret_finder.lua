-- Scan response bodies for common API key / secret patterns.
-- Runs asynchronously so it never delays traffic.

Plugin = {
  name        = "Secret Finder",
  on_response = { sync = false },
}

local PATTERNS = {
  { pattern = "AIza[0-9A-Za-z%-_]{35}",                label = "Google API Key" },
  { pattern = "AKIA[0-9A-Z]{16}",                      label = "AWS Access Key" },
  { pattern = "sk%-[a-zA-Z0-9]{20,}",                  label = "OpenAI API Key" },
  { pattern = "ghp_[a-zA-Z0-9]{36}",                   label = "GitHub Personal Token" },
  { pattern = "Bearer%s+[a-zA-Z0-9%-_%.]+%.[a-zA-Z0-9%-_%.]+%.[a-zA-Z0-9%-_%.]+",
                                                         label = "JWT Bearer Token" },
}

function on_response(req, res)
  local body = res:get_body()
  if body == "" then return end

  for _, p in ipairs(PATTERNS) do
    if body:find(p.pattern) then
      local key = p.label .. ":" .. req.host
      create_finding({
        title       = p.label .. " in response",
        description = "**Host:** `" .. req.host .. "`\n\n" ..
                      "**Path:** `" .. req.path .. "`\n\n" ..
                      "Pattern `" .. p.pattern .. "` matched in the response body.",
        key         = key,
        severity    = "high",
      })
    end
  end
end
