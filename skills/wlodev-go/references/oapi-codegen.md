# oapi-codegen setup

oapi-codegen **v2** runs as a library from `scripts/openapi/generate/generate.go`, not through the CLI and a YAML config. The module needs:
```
github.com/oapi-codegen/oapi-codegen/v2
github.com/oapi-codegen/runtime
github.com/getkin/kin-openapi
```
redocly CLI must be installed (`npm i -g @redocly/cli`).

## Pipeline: `make openapi`
```make
openapi.bundle:
	redocly bundle _oas/models.openapi.yml --output _oas/bundled/models-openapi-bundled.yml
	redocly bundle _oas/public.openapi.yml --output _oas/bundled/public-openapi-bundled.yml

openapi.generate:
	go run scripts/openapi/generate/generate.go

openapi: openapi.bundle openapi.generate
```

| Input | Output | Package | Generate options |
|---|---|---|---|
| `_oas/bundled/models-openapi-bundled.yml` (**bundled**) | `internal/models/models.gen.go` | `models` | `Models`, `Strict`, `EmbeddedSpec`; `SkipPrune` |
| `_oas/public.openapi.yml` (**source**, multi-file) | `internal/public/public.gen.go` | `public` | `StdHTTPServer`, `Models`, `Strict`, `EmbeddedSpec`; `SkipPrune`; custom strict template; `ImportMapping` → models |

Models are generated from the bundle so that every `$ref` resolves into a single package. Public is generated from the sources, so refs to `models.openapi.yml` remain **external refs**. These are mapped to `api/internal/models` and appear as `externalRef0.WidgetGet` in `public.gen.go`.

Both packages use `Compatibility.AlwaysPrefixEnumValues: true` and `OutputOptions.NameNormalizer: "ToCamelCaseWithInitialisms"`, so generated names use Go initialisms (`ID`, `URL`, `API`). The normalizer must be identical in every config, because `public` refers to `models` types by their generated names (quirk 14).

The full script is in [../examples/scripts/openapi/generate/generate.go.txt](../examples/scripts/openapi/generate/generate.go.txt). When you start a new project, copy it and change only the module prefix in `defaultImportMapping`.

## What gets generated in `public`
For each operation `widgetsGet`:
- `WidgetsGetRequestObject{WidgetID externalRef0.WidgetID}`, with JSON tag `widgetId`. The path param is typed as the registered parameter, which is a `string` alias in `models`. `Params WidgetsListParams` appears when there are query params, and `Body *WidgetsUpdateJSONRequestBody` when there is a request body.
- `WidgetsGetResponseObject`, an interface implemented by `WidgetsGet200JSONResponse`, `WidgetsGet404JSONResponse` and the rest
- a method on `StrictServerInterface`: `WidgetsGet(ctx, WidgetsGetRequestObject) (WidgetsGetResponseObject, error)`
- `ServerInterface` (raw) + `HandlerFromMux` + `NewStrictHandlerWithOptions` + `GetSpec()` (embedded spec, used by the auth middleware)

Response type shapes:
```go
// success: named type converted from the model
type WidgetsGet200JSONResponse externalRef0.WidgetGet
// inline-wrapped error: struct with Error field
type WidgetsGet404JSONResponse struct{ Error externalRef0.Widgets404Error `json:"error"` }
// component response: embeds the models response
type WidgetsGet401JSONResponse struct{ externalRef0.UnauthorizedJSONResponse }
// no body
type WidgetsDelete200Response struct{}
```

## Wiring (`cmd/api/main.go`)
```go
spec, err := public.GetSpec()
if err != nil {
    logger.Fatal().Err(err).Send()
}
spec.Servers = nil // quirk 8

handler := public.NewStrictHandlerWithOptions(
    server,
    []public.StrictMiddlewareFunc{},
    public.StrictHTTPServerOptions{
        RequestErrorHandlerFunc: utils.GetRequestErrorHandler(logger), // bad JSON → 400 {error:{code:bad_request}}
    },
)
mux := http.NewServeMux()
public.HandlerFromMux(handler, mux)
httpServer := &http.Server{Addr: addr, Handler: authClient.Middleware(mux)}
```

`Server` in `main.go` embeds `*public.PublicServer`. If there are several API surfaces (for example a private API), each gets its own `<name>.openapi.yml`, its own `internal/<name>` package, and its own entry in the `configs` slice of `generate.go`. All of them share `models.openapi.yml`.

## Rules
- ❌ Never hand-edit `*.gen.go` or `_oas/bundled/*`. Change the YAML and regenerate.
- ✅ Commit the generated files and the bundles, so the build needs neither redocly nor a codegen step.
- ✅ `.air.toml` excludes `_oas` and `models` from watching, so regenerating doesn't trigger a rebuild loop. Run `make openapi` manually.
- ✅ After regenerating, `go build ./...` shows every handler whose signature changed. Fix all of them before moving on.
