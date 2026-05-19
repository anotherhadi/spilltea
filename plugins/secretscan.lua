Plugin = {
  name        = "Secret Scan",
  description = [[
Scans HTML, JavaScript and JSON content (requests and responses) for hardcoded
secrets by matching common secret key names followed by a non-trivial value.

Uses `grep -E` (available on all Unix systems, no extra dependencies).
  ]],
  on_request  = { sync = false },
  on_response = { sync = false },
  disable_by_default = true,
}

local CONTENT_TYPES = {
  "text/html",
  "text/javascript",
  "application/javascript",
  "application/json",
}

-- Key name alternation (case-insensitive via grep -i)
-- Suffixes are required (no bare generic keyword alone).
local KEYS = {
  "access(_key|_token)", "accessid_secret", "account(_key|_sid)",
  "admin_pass(word)?", "admin_user",
  "(algolia|aws|gcp|azure|heroku|firebase|github|gitlab|slack|datadog|stripe|twilio|vercel|supabase|sendgrid|cloudinary|cloudflare|bitbucket|npm|netlify|auth0|okta|sentry)(_?(api|secret|access)(_?(key|token|id|sid|secret))?|_?(key|token|id|sid|secret))",
  "ansible_vault_password", "aos_key",
  "api(_key|_secret|_token)",
  "app_(id|key|secret)", "application(_key|_id|_secret)",
  "auth(_token|_secret|orization)", "authkey", "authsecret",
  "bearer_?token",
  "bucket(_password|_key)",
  "cert_?pass(word)?", "certificate_password",
  "client(_id|_secret)",
  "codecov_token", "consumer_(key|secret)",
  "connection_?string", "credentials?", "crypt(_key|_secret)",
  "db_(password|passwd|user(name)?)",
  "deploy(_key|_password|_token)",
  "docker_?pass(word)?", "dockerhub_?password",
  "encryption_(key|password)",
  "jwt_secret", "json_web_token",
  "keycloak_secret", "kubernetes_token",
  "ldap_(password|bindpw)", "login(_password|_token)",
  "mail_?password", "mail_smtp_pass",
  "mysql_password", "mongo_password",
  "netlify_token", "npm(_token|_auth_token)",
  "oauth(_token|_secret)",
  "openai_(api_key|secret)",
  "pass(word)?", "passwd",
  "private(_key|_token)",
  "rds_password",
  "s3(_key|_secret|_access_key_id)",
  "secret(_key|_token|_id)", "security_token",
  "sendgrid_api_key",
  "ses_(smtp|access|secret)",
  "service(_account|_key|_token)",
  "smtp_pass(word)?", "smtp_secret",
  "sonar_token",
  "ssh(_key|_private_key|_rsa)",
  "supabase(_anon|_service)?_key",
  "symfony_secret",
  "telegram_bot_token",
  "token",
  "travis_token",
  "vault(_token|_secret)",
  "webhook(_secret|_token)",
  "zapier_webhook_token",
}

-- Built once at load time.
-- Pattern breakdown:
--   KEY[a-z0-9._-]{0,20}   key name + optional alphanumeric suffix (e.g. _ID in AWS_ACCESS_KEY_ID)
--   [^=:a-zA-Z0-9_]{0,3}  optional non-identifier chars before separator (e.g. closing " in JSON "key":)
--   [[:space:]]*[:=]        REQUIRED: actual = or : assignment operator
--   [[:space:]]*"?          optional whitespace + opening quote
--   [a-zA-Z0-9+/=_.-]{8,} the secret value, at least 8 chars
local KEY_PAT  = "(" .. table.concat(KEYS, "|") .. ")"
local FULL_PAT = KEY_PAT .. '[a-z0-9._-]{0,20}[^=:a-zA-Z0-9_]{0,3}[[:space:]]*[:=][[:space:]]*"?[a-zA-Z0-9+/=_.-]{8,}'
local GREP_CMD = "grep -Eoni '" .. FULL_PAT .. "'"

local function is_relevant(ct)
  if not ct or ct == "" then return false end
  ct = ct:lower()
  for _, t in ipairs(CONTENT_TYPES) do
    if ct:find(t, 1, true) then return true end
  end
  return false
end

local function build_context(lines, linenum)
  local lo = math.max(1, linenum - 6)
  local hi = math.min(#lines, linenum + 6)

  local before, after = {}, {}
  for i = lo, linenum - 1 do
    local l = lines[i] or ""
    if #l > 120 then l = l:sub(1, 120) .. "..." end
    table.insert(before, l)
  end
  for i = linenum + 1, hi do
    local l = lines[i] or ""
    if #l > 120 then l = l:sub(1, 120) .. "..." end
    table.insert(after, l)
  end

  local matched_line = lines[linenum] or ""
  if #matched_line > 200 then matched_line = matched_line:sub(1, 200) .. "..." end

  local parts = {}
  if #before > 0 then
    table.insert(parts, "```\n" .. table.concat(before, "\n") .. "\n```")
  end
  table.insert(parts, "> **`" .. matched_line .. "`**")
  if #after > 0 then
    table.insert(parts, "```\n" .. table.concat(after, "\n") .. "\n```")
  end
  return table.concat(parts, "\n\n")
end

local function scan(label, ct, body, host, path)
  if not is_relevant(ct) then return end
  if not body or body == "" then return end

  local out, err = shell_pipe(GREP_CMD, body)
  if err and err ~= "" then
    log("grep error on " .. label .. " for " .. host .. path .. ": " .. err)
    return
  end
  if not out or out == "" then return end

  local lines = {}
  for line in (body .. "\n"):gmatch("([^\n]*)\n") do
    table.insert(lines, line)
  end

  for entry in out:gmatch("[^\n]+") do
    local linenum_str, matched = entry:match("^(%d+):(.+)$")
    if linenum_str then
      local linenum = tonumber(linenum_str)
      matched = matched:match("^%s*(.-)%s*$")
      if matched ~= "" then
        local display = matched
        if #display > 200 then display = display:sub(1, 200) .. "..." end
        local ctx = build_context(lines, linenum)
        create_finding({
          title       = "Potential secret in " .. label .. " (" .. host .. ")",
          description = "**Host:** `" .. host .. "`  \n**Path:** `" .. path .. "`\n\n**Match:** `" .. display .. "`\n\n" .. ctx,
          key         = host .. "|" .. path .. "|" .. label .. "|" .. matched,
          severity    = "high",
        })
      end
    end
  end
end

function on_request(req)
  scan("request", req.headers["Content-Type"] or "", req:get_body(), req.host, req.path)
end

function on_response(req, res)
  local ct = ""
  if res.headers then
    ct = res.headers["Content-Type"] or ""
  end
  scan("response", ct, res:get_body(), req.host, req.path)
end
