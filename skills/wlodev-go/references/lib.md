# `github.com/wlodev-labs/go` shared library

`go get github.com/wlodev-labs/go@latest`. This is the source of truth: if you need a generic helper, check here first. If it's missing and every project would use it, propose adding it to this lib instead of copying it into the project.

## `postgres`
```go
db, err := postgres.Open(ctx, host, port, user, password, dbname, sslmode) // *sqlx.DB via pgx stdlib
// pool: MaxOpen 25, MaxIdle 25, ConnMaxLifetime 30m, ConnMaxIdleTime 5m

postgres.IsUniqueConstraintError(err) bool
```

### Queries
`postgres.NewQueries(ex SQLExecutor) *Queries`. `SQLExecutor` is satisfied by both `*sqlx.DB` and `*sqlx.Tx`. Embed `*postgres.Queries` in the project's `queries.Queries`.

| Method | Behaviour |
|---|---|
| `Insert(query, args...)` | Exec. `ErrNoRowsAffected` if 0 rows |
| `InsertWithDest(dest, returningColumns, query, args...)` | Appends `RETURNING <cols>`, then `Get` into dest |
| `Update(query, args...)` | Exec. `ErrNoRowsAffected` if 0 rows |
| `UpdateWithDest(dest, returningColumns, query, args...)` | Appends `RETURNING`. Maps `sql.ErrNoRows` to `ErrNoRowsAffected` |
| `Delete(query, args...)` | Exec. `ErrNoRowsAffected` if 0 rows |
| `BuildWhereClause(conds []WhereCondition, BuildWhereClauseParams{Pagination: &BuildWhereClausePaginationParams{Parsed, TimeColumn}})` | Returns `(" WHERE a AND b", args)`, or `""`. `?` placeholders; **you must `Rebind`** |
| `BuildUpdateQuery(table, payload, conds)` | Builds `UPDATE t SET col = ? ... WHERE (c1) AND (c2)` from the **set** `UpdateField`s with `db` tags. Already rebound. `ErrNoFieldsToUpdate` / `ErrInvalidUpdatePayload` |

Pagination in `BuildWhereClause`: `Until` becomes `<TimeColumn> < ?` and `Since` becomes `<TimeColumn> > ?`. Use a table prefix in `TimeColumn` when joining (`"w.created_at"`).

### UpdateField (3-state)
```go
type UpdateField[T any] struct {
    Value *T   // nil + Set → NULL
    Set   bool // false → column untouched
}
postgres.IsUpdatePayloadEmpty(payload) bool // true if no UpdateField is Set
```

### Helpers
```go
postgres.GetColumns(cols)                    // "id, public_id, ..."
postgres.GetColumnsWithPrefix("w", cols)     // `w.id AS "id", ...` for joins (parent filtering only)

// batch loading of related rows (see related-data.md)
postgres.SliceToIdsSlice(items, func(*T) int)           // unique ids for ANY($1); random order
postgres.SliceToOptionalIdsSlice(items, func(*T) *int)  // skips nil FKs
postgres.SliceToStringSlice(items, func(*T) string)     // unique strings for ANY($1)
postgres.SliceToIndexMap(items, func(*T) K) map[K]*T                 // 1:1 lookup, dbmodels
postgres.SliceToResponseMap(items, func(*T) (K, R)) map[K]R          // 1:1 lookup, API models
postgres.SliceToGroupMap(items, func(*T) K) map[K][]*T               // 1:N grouping, dbmodels (≥ v0.5.0)
postgres.SliceToGroupResponseMap(items, func(*T) (K, R)) map[K][]R   // 1:N grouping, API models (≥ v0.5.0)
```
The group helpers keep the input order inside each group, so the child query's `ORDER BY` carries over. A lookup of a key with no items returns `nil`, and the parent mapper turns it into `[]`. Use `SliceToGroupResponseMap` for leaf children. When building a child needs a lookup that can fail (a join-table row and its target), write the loop by hand with an error path. [related-data.md](related-data.md) shows which helper fits which step.
Errors: `ErrNoRowsAffected`, `ErrNoFieldsToUpdate`, `ErrInvalidUpdatePayload`.

## `pagination`
```go
client := pagination.New(defaultLimit, minLimit, maxLimit) // 20, 1, 100. Must match the spec's limit param.

parsed, err := client.Parse(pagination.ParseParams{Limit, Until, Since}) // all *T, straight from request.Params
// err == pagination.ErrInvalidLimit → Pagination400Error{invalid_limit}
// parsed.Limit int (default applied), parsed.Until/Since *time.Time

resp := pagination.GetResponseData(items, func(i T) time.Time { return i.CreatedAt })
// resp.Count, resp.Next = time of LAST item, resp.Prev = time of FIRST item (nil when empty)
```
Items must be ordered `DESC` by the same column. To get the next page, the client sends `until=next`. For newer items it sends `since=prev`.

## `publicid`
Random nanoid identifiers exposed through the API instead of internal numeric ids.
```go
import "github.com/wlodev-labs/go/publicid"

id, err := publicid.New(publicid.NewParams{})                          // 12 chars, [0-9A-Za-z]
id, err := publicid.New(publicid.NewParams{Size: publicid.SizeMedium}) // 32 (SizeLong = 64)
ok := publicid.Validate(id, publicid.AlphabetNumbersCharactersWithUppercase, publicid.SizeShort)
```
- Generate the id in the service right before the insert, and store it in `public_id VARCHAR(<size>) NOT NULL UNIQUE`.
- Sizes 12, 32 and 64 must match the `public_id` column's `VARCHAR(n)` and the `common/schemas/{Id,IdMedium,IdLong}.yml` schemas. Keep all three in sync.
- `Validate` is a cheap shape check. Use it to reject obviously malformed path params before querying. A valid shape doesn't mean the id exists.

## `ptr`
`ptr.Bool`, `ptr.String`, `ptr.Int`, `ptr.Time`. Each returns a pointer to a copy.

## `zerolog`
```go
import (
    "github.com/rs/zerolog"
    wlodevzerolog "github.com/wlodev-labs/go/zerolog"
)

zerolog.ErrorStackMarshaler = wlodevzerolog.ErrorStackMarshaler // cockroachdb stacks, runtime frames trimmed
logger := wlodevzerolog.New(isDev) // console writer in dev, JSON in prod; Timestamp+Stack+Caller; RFC3339
```
Set `ErrorStackMarshaler` **before** the first log call. Without it, `.Err(err)` has no stack.

## Releasing the lib
`make publish VERSION=vX.Y.Z` tags the release, pushes it and warms proxy.golang.org. Projects then run `go get github.com/wlodev-labs/go@vX.Y.Z`. The skill tracks `main`, so if a project pins an older version and an API in this file is missing, upgrade the lib. Don't reimplement the API.
