## History Search

The History page has a built-in search bar with two modes:

**Fulltext search**: press `/` to open it. Results filter in real time as you type across all fields: method, host, path, and the raw request/response bodies.

**SQL mode**: press `:` to open it, then `Enter` to run. You can write either a WHERE expression or a full SELECT query against the `entries` table.

WHERE expression (the `SELECT` is added automatically):

```sql
status_code = 404
```

```sql
host LIKE '%.api.%' AND method = 'POST'
```

Full SELECT query:

```sql
SELECT * FROM entries WHERE response_raw LIKE '%password%' ORDER BY timestamp DESC LIMIT 20
```

The `entries` table has the following columns: `id`, `timestamp`, `method`, `host`, `path`, `status_code`, `request_raw`, `response_raw`.
