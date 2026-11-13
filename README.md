# github.com/wlodev-labs/go

Shared Go utilities for wlodev services:

| Package | What |
|---|---|
| `postgres` | `Open`, `Queries` (insert/update/delete helpers, `BuildWhereClause`, `BuildUpdateQuery`), `UpdateField`, slice helpers |
| `pagination` | Time-based cursor pagination (`since` / `until` / `limit`) |
| `ptr` | Pointer helpers |
| `publicid` | Public nanoid identifiers (`New`, `Validate`, sizes 12/32/64) |
| `zerolog` | Logger setup + cockroachdb stack marshaler |

```sh
go get github.com/wlodev-labs/go@latest
```

Release: `make publish VERSION=vX.Y.Z`.

## Agent skills

The conventions every service follows (layout, OpenAPI/oapi-codegen, database, handlers, style) are shipped as an agent skill in [skills/](skills/):

```sh
npx skills add wlodev-labs/go --skill wlodev-go
npx skills update
```

See [skills/README.md](skills/README.md).
