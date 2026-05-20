Plugin = {
	name = "TruffleHog",
	description = [[
Scans request and response bodies for secrets using [TruffleHog](https://github.com/trufflesecurity/trufflehog).

Requires `trufflehog` v3+ to be installed and available in PATH.

Each finding is stored on the **Findings** page with the matched detector output.
Findings are deduplicated per host+path+body content so repeated requests do not create duplicates.
  ]],
	on_start = { sync = false },
	on_request = { sync = false },
	on_response = { sync = false },
	disable_by_default = true,
}

function on_start()
	local result, _ = shell_pipe("command -v trufflehog 2>/dev/null")
	if not result or result:match("^%s*$") then
		log("trufflehog is not installed or not in PATH")
		notif("TruffleHog", "trufflehog is not installed or not in PATH, plugin disabled", "error")
		return false
	end
end

local function scan(label, content, host, path)
	if not content or content == "" then
		return
	end
	local out, err = shell_pipe(
		'f=$(mktemp) && cat > "$f" && trufflehog filesystem --no-color "$f"; rc=$?; rm -f "$f"; exit $rc',
		content
	)
	if err and err ~= "" then
		log("trufflehog error on " .. label .. ": " .. err)
		return
	end
	if not out or out == "" then
		return
	end
	local blocks = {}
	local current = nil
	for line in out:gmatch("[^\n]+") do
		if line:match("^Found ") then
			if current then
				table.insert(blocks, current)
			end
			current = line
		elseif current then
			current = current .. "\n" .. line
		end
	end
	if current then
		table.insert(blocks, current)
	end
	for _, block in ipairs(blocks) do
		create_finding({
			title = "Secret detected in " .. label .. " (" .. host .. ")",
			description = "**Host:** `" .. host .. "`  \n**Path:** `" .. path .. "`\n\n```\n" .. block .. "\n```",
			key = host .. "|" .. path .. "|" .. label .. "|" .. block,
			severity = "high",
		})
	end
end

function on_request(req)
	scan("request", req:get_body(), req.host, req.path)
end

function on_response(req, res)
	scan("response", res:get_body(), req.host, req.path)
end
