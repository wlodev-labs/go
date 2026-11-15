# Database (`database/postgres`)

The stack is Postgres + `sqlx` with the `pgx` stdlib driver (opened by `github.com/wlodev-labs/go/postgres.Open`), with goose for migrations. There is no ORM and no query builder beyond the shared lib helpers.

## Migrations
```make
# database/postgres/makefile
ifneq (,$(wildcard ../../.env))
    include ../../.env
    export
endif

migration_dir=./migrations
goose_pkg=github.com/pressly/goose/v3/cmd/goose@v3.27.1

create:
	@go run $(goose_pkg) -dir $(migration_dir) create $(ARGS) sql
up:
	@go run $(goose_pkg) -dir $(migration_dir) postgres "host=$(DB_HOST) port=$(DB_PORT) user=$(DB_USER) password=$(DB_PASSWORD) dbname=$(DB_NAME) sslmode=$(DB_SSL_MODE)" up
down:
	@go run $(goose_pkg) -dir $(migration_dir) postgres "host=$(DB_HOST) port=$(DB_PORT) user=$(DB_USER) password=$(DB_PASSWORD) dbname=$(DB_NAME) sslmode=$(DB_SSL_MODE)" down
```
- Create a migration with `make -C database/postgres create ARGS=widgets`, which produces `migrations/YYYYMMDDHHMMSS_widgets.sql`. Always create files through goose and never hand-write the timestamp.
- goose runs via `go run ...@version`, so it's pinned and not a module dependency.
- Each file has `-- +goose Up` and `-- +goose Down`. Down must undo Up exactly.
- ❌ Never edit a migration that has been applied. Add a new one (for example `..._widgets_nullable.sql`).

```sql
-- +goose Up
CREATE TABLE widgets (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(12) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tags TEXT[],
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_widgets_user_id_created_at ON widgets (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS widgets;
```
Table conventions: plural snake_case names, `id SERIAL` internal key, `public_id VARCHAR(12) UNIQUE` (or 32/64, matching `publicid.Size*`), owner FK with `ON DELETE CASCADE`, `updated_at` and `created_at` `NOT NULL DEFAULT NOW()`. `VARCHAR(n)` lengths match the validator limits. Index the pagination column together with the owner column. Index every FK column that a `…ByIDs` query filters with `ANY`; a composite index or `UNIQUE` whose first column is that FK counts.

## Client (`postgres.go`, package `postgresdb`)
```go
type Client struct {
    *sqlx.DB
    *queries.Queries
}

type TxClient struct {
    *sqlx.Tx
    *queries.Queries
}

func New(ctx context.Context) (*Client, error) {
    host, ok := os.LookupEnv("DB_HOST")
    if !ok {
        return nil, utils.FormatMissingEnvVarError("DB_HOST")
    }
    // ... DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSL_MODE
    db, err := postgres.Open(ctx, host, port, user, password, dbname, sslmode)
    if err != nil {
        return nil, err
    }
    return &Client{DB: db, Queries: queries.New(db)}, nil
}

func (c *Client) Begin() (*TxClient, error) {
    tx, err := c.DB.Beginx()
    if err != nil {
        return nil, errors.WithStack(err)
    }
    return &TxClient{Tx: tx, Queries: queries.New(tx)}, nil
}
```
The same query methods work on both `*Client` and `*TxClient`, because `queries.New` accepts any `postgres.SQLExecutor`.

Transaction usage:
```go
tx, err := s.db.Begin()
if err != nil { return errors.WithStack(err) }
defer tx.Rollback() // no-op after Commit

if err := tx.WidgetsInsertWidget(dest, payload); err != nil { return err }
if err := tx.WidgetTagsInsert(...); err != nil { return err }
return errors.WithStack(tx.Commit())
```

## `queries/queries.go`
```go
package queries

type Queries struct {
    *postgres.Queries // embeds Insert, InsertWithDest, Update, UpdateWithDest, Delete, BuildWhereClause, BuildUpdateQuery + sqlx methods
}

func New(e postgres.SQLExecutor) *Queries {
    return &Queries{Queries: postgres.NewQueries(e)}
}
```

## dbmodels (`dbmodels/<domain>_dbmodels.go`)
```go
type WidgetStatus string

const (
    WidgetStatusActive   WidgetStatus = "active"
    WidgetStatusArchived WidgetStatus = "archived"
)

type Widget struct {
    ID          int          `db:"id"`
    PublicID    string       `db:"public_id"`
    UserID      int          `db:"user_id"`
    Name        string       `db:"name"`
    Description *string      `db:"description"` // NULL-able column → pointer
    Tags        *[]string    `db:"tags"`
    Status      WidgetStatus `db:"status"`
    UpdatedAt   time.Time    `db:"updated_at"`
    CreatedAt   time.Time    `db:"created_at"`
}

var WidgetColumns = []string{
    "id", "public_id", "user_id", "name", "description", "tags", "status", "updated_at", "created_at",
}

type WidgetInsertPayload struct {
    PublicID    string
    UserID      int
    Name        string
    Description *string
    Tags        *[]string
}

type WidgetUpdatePayload struct {
    Name        postgres.UpdateField[string]    `db:"name"`
    Description postgres.UpdateField[string]    `db:"description"`
    Tags        postgres.UpdateField[[]string]  `db:"tags"`
    Status      postgres.UpdateField[WidgetStatus] `db:"status"`
    UpdatedAt   postgres.UpdateField[time.Time] `db:"updated_at"`
}
```
- `XColumns` must list every column in the struct, in table order. Every SELECT and RETURNING uses `postgres.GetColumns(dbmodels.XColumns)`. ❌ Never write `SELECT *`.
- Nullable columns are pointers. In `UpdateField[T]`, `T` is the non-pointer type, and `Value: nil` writes `NULL`.
- Enums are a string type with constants, and the API mapping converts with `models.WidgetStatus(...)`.

## Query methods (`queries/<domain>_queries.go`)
Naming: `<Domain><Verb><What>[By<Keys>]`, where the verb is one of `Insert`, `Select`, `Update` or `Delete`. Examples: `WidgetsInsertWidget`, `WidgetsSelectWidgetByID`, `WidgetsSelectWidgetByUserIDAndPublicID`, `WidgetsSelectUserPaginatedWidgets`, `WidgetsUpdateWidget`, `WidgetsDeleteWidget`.

```go
func (q *Queries) WidgetsInsertWidget(dest *dbmodels.Widget, payload dbmodels.WidgetInsertPayload) error {
    return q.InsertWithDest(dest, postgres.GetColumns(dbmodels.WidgetColumns), `
        INSERT INTO widgets (
            public_id,
            user_id,
            name,
            description,
            tags
        ) VALUES ($1, $2, $3, $4, $5)
    `, payload.PublicID, payload.UserID, payload.Name, payload.Description, payload.Tags)
}

func (q *Queries) WidgetsSelectWidgetByUserIDAndPublicID(userID int, publicID string) (*dbmodels.Widget, error) {
    widget := &dbmodels.Widget{}
    return widget, q.Get(widget, `
        SELECT
            `+postgres.GetColumns(dbmodels.WidgetColumns)+`
        FROM widgets
        WHERE
            user_id = $1 AND
            public_id = $2
    `, userID, publicID)
}

type WidgetsSelectUserPaginatedWidgetsParams struct{}

func (q *Queries) WidgetsSelectUserPaginatedWidgets(userID int, pag *pagination.Parsed, params WidgetsSelectUserPaginatedWidgetsParams) ([]*dbmodels.Widget, error) {
    var widgets []*dbmodels.Widget
    whereClause, args := q.BuildWhereClause(
        []postgres.WhereCondition{
            {Clause: "user_id = ?", Args: []any{userID}},
        },
        postgres.BuildWhereClauseParams{
            Pagination: &postgres.BuildWhereClausePaginationParams{Parsed: pag, TimeColumn: "created_at"},
        },
    )
    args = append(args, pag.Limit)
    return widgets, q.Select(&widgets, q.Rebind(`
        SELECT
            `+postgres.GetColumns(dbmodels.WidgetColumns)+`
        FROM widgets
        `+whereClause+`
        ORDER BY created_at DESC
        LIMIT ?
    `), args...)
}

func (q *Queries) WidgetsUpdateWidget(dest *dbmodels.Widget, widgetID int, payload dbmodels.WidgetUpdatePayload) error {
    query, args, err := q.BuildUpdateQuery("widgets", payload, []postgres.WhereCondition{
        {Clause: "id = ?", Args: []any{widgetID}},
    })
    if err != nil {
        return err
    }
    return q.UpdateWithDest(dest, postgres.GetColumns(dbmodels.WidgetColumns), query, args...)
}

func (q *Queries) WidgetsDeleteWidget(widgetID int) error {
    return q.Delete(`
        DELETE FROM widgets
        WHERE id = $1
    `, widgetID)
}
```

### Placeholder quirk
- Static queries use `$1, $2...` directly.
- Queries built with `BuildWhereClause` use `?` **everywhere**, including the `LIMIT ?` you add yourself, and the final SQL is wrapped in `q.Rebind(...)`. Mixing `$n` and `?` in one query breaks.
- `BuildUpdateQuery` already rebinds its output, so you pass it straight to `UpdateWithDest`.

### Return-value quirk
- `q.Get` returns `sql.ErrNoRows` when nothing matches. Handlers turn that into a 404.
- `Insert`, `Update`, `Delete` and `UpdateWithDest` return `postgres.ErrNoRowsAffected` when 0 rows changed.
- `BuildUpdateQuery` returns `postgres.ErrNoFieldsToUpdate` for an empty payload. Guard against this with `IsUpdatePayloadEmpty` in the handler.
- Use `postgres.IsUniqueConstraintError(err)` to turn duplicate-key violations into a 400/409 domain code.

### Related rows: batch loading, not JOINs
Child, join-table and lookup data is loaded with one `<Domain>Select<What>By<Key>IDs(ids []int)` query per level (`WHERE fk = ANY($1)`, deterministic `ORDER BY`), then stitched together in Go. Each query scans into one flat dbmodel. JOINs (with `postgres.GetColumnsWithPrefix("w", cols)`) or `EXISTS` appear only in a parent query that filters, sorts or authorizes by a child column. See [related-data.md](related-data.md) for the full pattern.
