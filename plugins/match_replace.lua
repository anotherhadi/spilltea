Plugin = {
	name = "Match & Replace",
	description = [[
Automatically find and replace content in requests and responses.

**Config**:
```yaml
rules:
  - on: "request"         # "request", "response", or "both" (default: "both")
    url: "example%.com"   # optional: Lua pattern to filter by URL
    target: "body"        # "body", "path", "url", or "header:Name"
    find: "old_string"    # Lua pattern to search
    replace: "new_string" # replacement string (supports %1, %2 for captures)
```

Example (inject a debug flag in JSON bodies):
```yaml
rules:
  - on: "both"
    url: "api%.example%.com"
    target: "body"
    find: '"debug":false'
    replace: '"debug":true'
```

Example (replace an Authorization header):
```yaml
rules:
  - on: "request"
    target: "header:Authorization"
    find: "Bearer .*"
    replace: "Bearer MY_NEW_TOKEN"
```

Example (rewrite a path prefix):
```yaml
rules:
  - on: "request"
    url: "staging%.example%.com"
    target: "path"
    find: "^/v1/"
    replace: "/v2/"
```
  ]],
	priority = 50,
	on_request = { sync = true },
	on_response = { sync = true },
}

local rules = {}

function on_config()
	rules = {}
	local cfg = get_config()
	if not cfg or not cfg.rules then
		return
	end
	for _, r in ipairs(cfg.rules) do
		if r.find and r.find ~= "" then
			table.insert(rules, {
				on = r.on or "both",
				url = r.url,
				target = r.target or "body",
				find = r.find,
				replace = r.replace or "",
			})
		end
	end
end

local function should_apply(rule, req, context)
	if rule.on ~= "both" and rule.on ~= context then
		return false
	end
	if rule.url and not req.url:match(rule.url) then
		return false
	end
	return true
end

local function apply(rule, req, res, context)
	local target = rule.target
	if target == "body" then
		local obj = (context == "response") and res or req
		local body = obj:get_body()
		local new_body = body:gsub(rule.find, rule.replace)
		if new_body ~= body then
			obj:set_body(new_body)
		end
	elseif target == "path" then
		local new_path = req.path:gsub(rule.find, rule.replace)
		if new_path ~= req.path then
			req:set_path(new_path)
		end
	elseif target == "url" then
		local new_url = req.url:gsub(rule.find, rule.replace)
		if new_url ~= req.url then
			req:set_url(new_url)
		end
	elseif target:sub(1, 7) == "header:" then
		local name = target:sub(8)
		local obj = (context == "response") and res or req
		local val = obj.headers[name] or ""
		local new_val = val:gsub(rule.find, rule.replace)
		if new_val ~= val then
			obj:set_header(name, new_val)
		end
	end
end

function on_request(req)
	for _, rule in ipairs(rules) do
		if should_apply(rule, req, "request") then
			apply(rule, req, nil, "request")
		end
	end
end

function on_response(req, res)
	for _, rule in ipairs(rules) do
		if should_apply(rule, req, "response") then
			apply(rule, req, res, "response")
		end
	end
end
