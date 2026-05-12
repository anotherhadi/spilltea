## Scopes

Scopes let you control which requests Spilltea intercepts. Patterns are Go regular expressions matched against `host/path` (e.g. `api.example.com/v1/users`).

- **Whitelist**: if non-empty, only matching requests are intercepted.
- **Blacklist**: matching requests are always ignored, even if whitelisted.

When both lists are set, a request must pass the whitelist _and_ not be in the blacklist.

### Examples

| Pattern                    | Matches                             |
| -------------------------- | ----------------------------------- |
| `example\.com`             | any request to `example.com`        |
| `^api\.example\.com`       | only the `api` subdomain            |
| `example\.com/api/v2`      | a specific path prefix              |
| `\.(js\|css\|png\|woff2?)` | static assets (useful in blacklist) |
| `googleapis\.com`          | all Google API traffic              |
| `/graphql$`                | any host with a `/graphql` endpoint |
