# Go writing style

## Params structs
Any function that may grow takes a trailing `XParams` struct, **even when it's empty**:
```go
type ToWidgetGetParams struct{}
func ToWidgetGet(dbWidget *dbmodels.Widget, params ToWidgetGetParams) WidgetGet

type WidgetsSelectUserPaginatedWidgetsParams struct{}
func (q *Queries) WidgetsSelectUserPaginatedWidgets(userID int, pag *pagination.Parsed, params WidgetsSelectUserPaginatedWidgetsParams) ([]*dbmodels.Widget, error)
```
Adding a filter later doesn't change any call site. Name it `<FuncName>Params`. Required identifiers stay positional (`userID`, `publicID`) and everything optional goes in Params as pointers.

Constructors with several dependencies put one parameter per line:
```go
func NewPublicServer(
    logger zerolog.Logger,
    db *postgresdb.Client,
    pagination *pagination.Client,
) *PublicServer {
```

## Naming
- **Initialisms are all caps** (standard Go): `ID`, `PublicID`, `UserID`, `WidgetsSelectWidgetByID`, `WidgetsSelectWidgetByUserIDAndPublicID`, and locals `userID`, `publicID`, `widgetID`. The same goes for `URL`, `API`, `HTTP`, `JSON`, `SQL`, `UUID` and `DB` (`apiBaseURL`, `jwtIssuerURL`).
  - Generated code matches because `generate.go` sets `NameNormalizer: ToCamelCaseWithInitialisms` (`request.WidgetID`, `models.WidgetGet.ID`). See quirk 14.
  - Wire names are unaffected. JSON keys, query and path params stay camelCase (`id`, `widgetId`, `userId`), and SQL columns stay snake_case (`public_id`).
  - ❌ Don't write `Id`, `Url` or `Api` in new code. If you touch a file that still has them, rename them within that change.
  - Exception: existing lib APIs keep their exported names (`postgres.SliceToIdsSlice`) until a major version.
- Query methods: `<Domain><Verb><What>[By<Keys>]`. Mappers: `To<Schema>`. Sentinels: `ErrX`. Constants: `<Type><Value>` (`WidgetStatusActive`, `DeploymentEnvironmentProduction`).
- Receivers are one letter: `s` for servers and services, `q` for queries, `c` for clients.
- Files: `<domain>.go` in `internal/public`, `<domain>_queries.go`, `<domain>_dbmodels.go`. A service package splits into files by concern (`errors.go`, `tools.go`, `<action>.go`).

## Errors (`github.com/cockroachdb/errors`)
- Import `github.com/cockroachdb/errors` everywhere, not `errors` or `pkg/errors`. In handlers, the stdlib `errors.Is` also works, but stay consistent within a file.
- Use `errors.WithStack(err)` when an error crosses a boundary you own (SDK call, I/O, sql). Stacks are rendered by `wlodevzerolog.ErrorStackMarshaler`.
- Use `errors.Wrap(ErrSentinel, cause.Error())` to classify a lower-level failure as a domain error. `errors.Is(err, ErrSentinel)` stays true.
- Use `errors.Wrap(err, "failed to connect to postgres")` to add context. Messages are lowercase, with no trailing punctuation.
- Declare sentinels at the top of the file or in `errors.go`: `var ErrInvalidName = errors.New("invalid name")`.
- ❌ Don't log and also return the same error from deep layers. Log once, where it's handled (the handler or `main`).

## Logging (`zerolog`)
- Errors: `s.logger.Error().Err(err).Send()`. The stack comes from the error, so don't add a message that duplicates it.
- Information: `s.logger.Info().Msg("no fields to update in the request body")`.
- Request-scoped fields: `logger.With().Str("method", r.Method).Str("url", r.URL.String()).Logger()`.
- Fatal is only for `main` startup.

## Control flow
- Return early. Avoid deep `else` branches, except for the existing `if resp, err := ...; err != nil {} else if ...` chains in the lib.
- Check `ok` from `os.LookupEnv` and comma-ok context lookups right away.
- Use a `switch { case errors.Is(...): }` for mapping more than one sentinel.

## Comments
- Doc comments on exported identifiers start with the name: `// Parse takes the request parameters ...`. Wrap at about 80 columns.
- Explain *why* (quirks, workarounds, links to upstream issues), not *what*. Mark code patched into generated templates with `CUSTOM CODE BEGIN/END`.
- A package with non-obvious purpose gets a top comment (`// Runpod client acting as a single, unified interface ...`).

## Misc
- Use `time.Now().UTC()` for timestamps you write, and `ptr.Time`, `ptr.String`, `ptr.Int` and `ptr.Bool` for pointers to literals.
- Preallocate slices with known length: `make([]models.WidgetGet, len(items))`.
- Run `gofmt` and `go vet` clean. Imports are a single block sorted by goimports (stdlib, module and third-party mixed, as goimports orders them).
