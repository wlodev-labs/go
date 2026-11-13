# OpenAPI + oapi-codegen quirks

Each quirk is written as symptom → cause → fix. Read this before you debug "weird" generated code.

## 1. Duplicate or inline types appear in `public.gen.go` instead of `externalRef0.X`
**Cause:** `generate.go` maps external refs to the models package by **literal relative path string**:
```go
var defaultImportMapping = map[string]string{
    "../../models.openapi.yml":          "api/internal/models",
    "../../../models.openapi.yml":       "api/internal/models",
    "../../../../models.openapi.yml":    "api/internal/models",
    "../../../../../models.openapi.yml": "api/internal/models",
}
```
A `$ref` from a file nested one level deeper than any key, or written as `./../../models.openapi.yml`, doesn't match. oapi-codegen then inlines the schema.

**Fix:** keep the `$ref` form canonical (`../../models.openapi.yml#/...`, no leading `./`). If you add a deeper folder, add the matching key.

## 2. A new schema field doesn't show up in `models.*`
**Cause:** `internal/models` is generated from `_oas/bundled/models-openapi-bundled.yml`, not from the source files. You changed the sources but didn't re-bundle.

**Fix:** always run `make openapi`, which bundles and then generates. `make openapi.generate` alone uses the stale bundle.

Related: pointing the models config at `_oas/models.openapi.yml` (the unbundled sources) fails with
`unrecognized external reference 'common/parameters/pagination-parameters.yml'; please provide the known import for this reference using option --import-mapping`.
The registry is made only of cross-file `$ref`s, so it must be bundled first. Use **redocly** for this. kin-openapi's `InternalizeRefs` is not a substitute: it hoists building blocks such as `WidgetBase` into extra components with path-like names, and oapi-codegen v2.8 panics on its output.

## 3. Go type names like `WidgetsCreate200JSONResponseBody` or `WidgetsGet404JSONResponseBodyError`
**Cause:** the operation `$ref`s a schema **file** directly, or defines the object inline. Only components registered in `models.openapi.yml` get stable names.

**Fix:** register the schema in `models.openapi.yml` and `$ref` it through `#/components/schemas/...`. The `{error: ...}` wrapper is intentionally inline. Only the inner error type needs a name.

## 4. Long enum constant names
`AlwaysPrefixEnumValues: true` prefixes every constant with its type name:
```go
models.WidgetsCreate400ErrorCodeInvalidName   // enum value invalid_name of WidgetsCreate400Error.code
models.Pagination400ErrorCodeInvalidLimit
models.WidgetStatusActive
```
This is deliberate, because it prevents collisions between domains that share codes (`invalid_title` exists in several of them). Each operation's 401 and 500 responses also get their own `…401JSONResponseBodyErrorCode` type in `public.gen.go`. Ignore those types and build responses with `models.GenericUnauthorizedResponse(...)`.

## 5. Only one `Set-Cookie` header is sent
**Cause:** stock oapi-codegen generates one `SetCookie string` header field and calls `w.Header().Set`.

**Fix:** the custom template `scripts/openapi/generate/templates/strict/strict-interface.tmpl` turns a header named `Set-Cookie` into `SetCookie []string` and emits `w.Header().Add` in a loop (see oapi-codegen discussion #1802). In the spec:
```yaml
headers:
    Set-Cookie:
        description: Session tokens
        schema:
            type: array
            items:
                type: string
```
When you bump oapi-codegen, diff the upstream `strict/strict-interface.tmpl` and reapply only the blocks between the `CUSTOM CODE BEGIN/END` markers.

## 6. SSE / streaming endpoints don't flush, or you need the raw `http.ResponseWriter`
**Cause:** strict handlers return a value. They can't write progressively or upgrade a connection.

**Fix:** keep the operation in the spec, with `text/event-stream` and `application/json` content in the 200 response and `stream` as a query param. The strict method then becomes a stub. Wrap the strict handler in a struct that embeds the generated `ServerInterface` and overrides that one method with the raw `(w http.ResponseWriter, r *http.Request, params XParams)` signature:
```go
type AssistantWrapper struct {
    ServerInterface // = strict handler
    // deps...
}
func (s *AssistantWrapper) AssistantTurn(w http.ResponseWriter, r *http.Request, params AssistantTurnParams) { ... }
// main.go: public.HandlerFromMux(wrapper, mux)
```
Write events with `utils.WriteSSEEvent(w, http.NewResponseController(w), payload)`. Describe the event union with `oneOf` + `discriminator` (quirk 9).

## 7. PATCH can't clear a field to `null`
**Cause:** `XUpdate` is "Base with nothing required", so a property becomes `*T`. In the handler, `nil` means **not sent**, which maps to `UpdateField{Set:false}`. "Sent as null" can't be told apart from "not sent".

**Current convention** (use it unless the user explicitly needs clearing):
```go
if request.Body.Description != nil {
    payload.Description = postgres.UpdateField[string]{Set: true, Value: request.Body.Description}
}
```
**When clearing is needed:** mark the property `nullable: true` and set `OutputOptions.NullableType: true` in `generate.go`. Nullable fields then become `nullable.Nullable[T]` (`github.com/oapi-codegen/nullable`). Map all three states:
```go
if request.Body.Description.IsSpecified() {
    f := postgres.UpdateField[string]{Set: true}
    if !request.Body.Description.IsNull() {
        f.Value = ptr.String(request.Body.Description.MustGet())
    }
    payload.Description = f
}
```
Ask the user before turning `nullable-type` on, because it changes generated types project-wide.

## 8. The auth middleware returns 404 for every route
**Cause:** the kin-openapi router built from the embedded spec matches the `servers:` URL. The spec's `servers` entry is only a placeholder.

**Fix:** set `spec.Servers = nil` in `main.go` before `auth.New`. Keep `servers:` in the YAML anyway, because redocly's `recommended` ruleset requires it.

## 9. `oneOf` + `discriminator`
The mapping values are **file paths**, not component refs:
```yaml
oneOf:
    - $ref: './StreamTextEvent.yml'
    - $ref: './StreamDoneEvent.yml'
discriminator:
    propertyName: event
    mapping:
        textDelta: './StreamTextEvent.yml'
        done: './StreamDoneEvent.yml'
```
Each variant has `event` with a single-value enum. The generated union type exposes `As<Variant>()`, `From<Variant>()` and `Discriminator()`.

## 10. A handler logs `CriticalAuthenticationPass`
**Cause:** the auth middleware decides which routes need authentication **from the spec's `security`**. An operation without `security:` is treated as public, so the request reaches the handler without an auth context.

**Fix:** every operation declares `security` explicitly. Use `security: []` only for truly public endpoints (health, openapi file, login, signup, refresh).

## 11. Response shapes: wrapped and unwrapped
- Success bodies are the schema **itself** (`WidgetGet`), with no `{data: ...}` envelope. Lists are the exception: they use `{data, pagination}`.
- Error bodies are always enveloped in `{error: {...}}`.

In Go, a success response is a conversion (`WidgetsGet200JSONResponse(models.ToWidgetGet(...))`), while an error response is a struct with an `Error` field.

## 12. `models.gen.go` contains types you never referenced
`SkipPrune: true` keeps unused components on purpose, because `internal/models` is the shared type library for handlers, services and utils. Don't remove "unused" schemas from the registry without checking for Go usages.

## 13. `GET /v1/openapi` serves the bundled public spec
The handler reads `_oas/bundled/public-openapi-bundled.yml` at runtime, relative to the working directory. The bundled files are committed, and Docker images must copy `_oas/bundled/`.

## 14. Generated `Id` vs hand-written `ID`
**Cause:** oapi-codegen's default name normalizer (`ToCamelCase`) produces `Id`, `WidgetId` and `ApiKey`, which clashes with the house rule of Go initialisms (`ID`, `URL`, `API`).

**Fix:** `generate.go` sets `OutputOptions.NameNormalizer: string(codegen.NameNormalizerFunctionToCamelCaseWithInitialisms)` in **every** config (models and public). Result:
```go
type WidgetsGetRequestObject struct {
    WidgetID externalRef0.WidgetID `json:"widgetId"` // wire name unchanged
}
```
- If only one config sets it, `public.gen.go` refers to `externalRef0.WidgetID` while `models.gen.go` defines `WidgetId`, and the build fails.
- It affects **all** initialisms in the default list (ID, URL, API, HTTP, JSON, UUID, DB, TS, ...). For example, an operation `authApiKeyGet` becomes `AuthAPIKeyGet`.
- Words that aren't split by case are left alone: `generalOpenapiGet` stays `GeneralOpenapiGet`.
- Add project-specific acronyms with `AdditionalInitialisms`. That option only works together with this normalizer.

**Migrating an existing project:** set the option, run `make openapi`, then `go build ./...`. Every compile error is a rename (`request.NoteId` → `request.NoteID`, `models.NoteGet{Id: ...}` → `ID`). Rename the hand-written `Id`, `PublicId` and `UserId` identifiers (dbmodels, queries, auth context) in the same change, so the project never has both styles. JSON, SQL and URLs don't change, so this is not a breaking API change.
