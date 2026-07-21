Plugin = {
	name = "JWT Decoder",
	description = [[
Decodes JWTs found in request headers and exposes the decoded payload inline.

For every request header whose value looks like a JWT (three dot-separated
base64url segments), the base64url-encoded payload is decoded and added back to
the request as a new header:

```
Authorization: Bearer eyJ...   ->   X-JWT-Decoded-Authorization: {"sub":"123",...}
X-Auth-Token:  eyJ...          ->   X-JWT-Decoded-X-Auth-Token:  {"sub":"123",...}
```

A leading `Bearer ` prefix is stripped from any header value before decoding,
so custom authorization headers (`Authorization-Test`, `X-Auth-Token`, ...)
are handled too. The decoded header is added to the outbound request, so it is visible
in the intercept, history and replay views (and forwarded upstream).

Pure Lua, no external dependencies.
  ]],
	on_request = { sync = true },
}

-- Reverse lookup table: base64url character -> 6-bit value (built once at load time).
local B64_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
local B64_DEC = {}
for i = 1, #B64_ALPHABET do
	B64_DEC[B64_ALPHABET:sub(i, i)] = i - 1
end

-- Decode an unpadded base64url string. Returns the decoded string, or nil if
local function b64url_decode(s)
	if #s % 4 == 1 then
		return nil
	end
	local out, acc, bits = {}, 0, 0
	for c in s:gmatch(".") do
		local v = B64_DEC[c]
		if not v then
			return nil
		end
		acc, bits = acc * 64 + v, bits + 6
		if bits >= 8 then
			bits = bits - 8
			out[#out + 1] = string.char(math.floor(acc / 2 ^ bits))
			acc = acc % 2 ^ bits
		end
	end
	return table.concat(out)
end

-- If value is a JWT, return its decoded payload
local function decode_jwt_payload(value)
	-- Three dot-separated base64url segments; the signature may be empty (alg:none).
	local payload = value:match("^[A-Za-z0-9_-]+%.([A-Za-z0-9_-]+)%.[A-Za-z0-9_-]*$")
	if not payload then
		return nil
	end
	local decoded = b64url_decode(payload)
	if not decoded then
		return nil
	end
	-- Only accept plausible JSON payloads.
	if decoded:match("^%s*{") == nil then
		return nil
	end
	decoded = decoded:gsub("[\r\n]+", " ")
	return decoded
end

function on_request(req)
	for name, value in pairs(req.headers) do
		local token = value:gsub("^[Bb][Ee][Aa][Rr][Ee][Rr]%s+", "")
		local payload = decode_jwt_payload(token)
		if payload then
			req:set_header("X-JWT-Decoded-" .. name, payload)
		end
	end
end
