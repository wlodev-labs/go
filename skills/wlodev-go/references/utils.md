# `utils/`

Project-local helpers that are shared across packages. Check here, and in the shared lib (`lib.md`), before writing a new helper. If a helper is generic enough for every project, suggest moving it into `github.com/wlodev-labs/go`.

## `utils/env.go`
```go
type DeploymentEnvironment string

const (
    DeploymentEnvironmentDevelopment DeploymentEnvironment = "development"
    DeploymentEnvironmentProduction  DeploymentEnvironment = "production"
)

// GetEnvironment reads APP_ENV; anything unknown or missing is production.
func GetEnvironment() DeploymentEnvironment
```
Fail safe: a missing `APP_ENV` means **production**.

## `utils/errors.go`
- `CriticalAuthenticationPass` is the log message for "handler reached without auth context". It always means a spec `security` mistake (quirk 10).
- `FormatMissingEnvVarError(name) error` is used by every `New()` that reads env.
- `GetRequestErrorHandler(logger)` is the `RequestErrorHandlerFunc` for the strict handler. It turns body decode errors into `400 {error:{code:"bad_request"}}` and everything else into a 500.
- `SendErrorResponse(w, r, err, SendErrorResponseParams{StatusCode, Code, Message, DumpError})` writes the `{error:{code,message}}` envelope outside generated code (middleware, wrappers).

## `utils/requests.go`
- `WriteJSONErrorResponse(w, status, code)` is the minimal error envelope, built with `models.BaseError`.
- `WriteSSEEvent(w, rc *http.ResponseController, payload any) error` writes `data: <json>\n\n` and flushes.

## Public IDs: moved to the lib
Public IDs now come from `github.com/wlodev-labs/go/publicid` (see `lib.md`). ❌ Don't create `utils/publicid`. If a project still has one, replace the import: the API is identical.

## `utils/validate`
Thin wrappers around `go-playground/validator/v10`:
```go
validate.Field(name, "min=1,max=255")   // single value, validator tag syntax
validate.Payload(structWithTags)        // struct with `validate:"..."` tags
validate.Enum(v, []T{A, B})             // membership check
validate.Password(pw)                   // min=8,max=128
validate.FirstName / LastName           // required,min=1,max=50
```
Domain validators are small private functions next to their use (`validateWidgetName`). Their limits match the column length in the migration.
