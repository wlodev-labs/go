# Project layout

The Go module name is short and local (`module api`), so imports look like `api/internal/models`.

```
.
├── cmd/api/main.go                 # wiring only, no business logic
├── _oas/                           # OpenAPI sources (see openapi-spec.md)
│   ├── models.openapi.yml          # component registry → internal/models
│   ├── public.openapi.yml          # paths → internal/public
│   ├── bundled/                    # redocly output, generated, committed, served at /v1/openapi
│   ├── common/{parameters,responses,schemas}/
│   └── domains/<domain>/...
├── internal/
│   ├── models/
│   │   ├── models.gen.go           # generated from bundled models spec
│   │   ├── generic.go              # GenericXxxResponse builders (401/403/404/500/400)
│   │   └── mappings.go             # ToXxx(dbmodel, ToXxxParams{}) db → API
│   └── public/
│       ├── public.gen.go           # generated strict server
│       ├── server.go               # PublicServer struct + NewPublicServer
│       ├── errors.go               # handler-local sentinel errors
│       └── <domain>.go             # one file per OpenAPI domain/tag
├── auth/                           # middleware enforcing spec `security`, auth context
├── database/postgres/
│   ├── makefile                    # goose create/up/down
│   ├── migrations/                 # YYYYMMDDHHMMSS_<name>.sql
│   ├── postgres.go                 # package postgresdb: Client, TxClient, New(ctx)
│   ├── dbmodels/<domain>_dbmodels.go
│   └── queries/{queries.go,<domain>_queries.go}
├── services/<name>/                # external clients + domain services (see services.md)
├── utils/                          # env.go, errors.go, requests.go, validate/ (public IDs: wlodev-labs/go/publicid)
├── scripts/openapi/generate/       # generate.go + templates/strict/strict-interface.tmpl
├── makefile                        # dev, openapi, openapi.bundle, openapi.generate
├── redocly.yaml
├── .air.toml                       # hot reload: go build -o tmp/api ./cmd/api
└── .env.example                    # every env var, grouped with comment headers
```

## Package naming quirks
- `database/postgres` is **package `postgresdb`**, so it doesn't clash with `github.com/wlodev-labs/go/postgres`. Import it as `postgresdb "api/database/postgres"`.
- A service that wraps an SDK of the same name is suffixed `internal`: `services/supabase` is `package supabaseinternal`, imported as `supabaseinternal "api/services/supabase"`.
- The shared zerolog package is aliased `wlodevzerolog` next to `github.com/rs/zerolog`.

## `cmd/api/main.go` wiring order
1. `zerolog.ErrorStackMarshaler = wlodevzerolog.ErrorStackMarshaler`, then `logger := wlodevzerolog.New(utils.GetEnvironment() == utils.DeploymentEnvironmentDevelopment)`
2. `db, err := postgresdb.New(ctx)`
3. External clients (`supabaseinternal.New()`, ...), then domain services (`calendar.New(logger, db)`)
4. `paginationClient := pagination.New(20, 1, 100)`. These values must match `common/parameters/pagination-parameters.yml` (default 20, min 1, max 100).
5. `spec, _ := public.GetSpec()`, then `spec.Servers = nil` (see quirk 8)
6. `authClient, _ := auth.New(logger, db, ..., spec)`
7. `public.NewStrictHandlerWithOptions(server, nil, public.StrictHTTPServerOptions{RequestErrorHandlerFunc: utils.GetRequestErrorHandler(logger)})`
8. `public.HandlerFromMux(handler, http.NewServeMux())`
9. `http.Server{Handler: authClient.Middleware(mux)}`

Every constructor failure ends in `logger.Fatal().Err(err).Send()`.

Env loading uses `_ "github.com/joho/godotenv/autoload"`.

## Makefile targets
- `make dev`: runs air (plus any sidecar processes) with `trap 'kill 0' INT TERM EXIT`.
- `make openapi`: `openapi.bundle` (redocly) followed by `openapi.generate` (go run).
- `make -C database/postgres create ARGS=<name>` / `up` / `down`: goose migrations. They read `../../.env`.
