# Services (`services/<name>`)

`services/` holds everything that isn't HTTP and isn't SQL. There are two kinds:

| Kind | Examples | Shape |
|---|---|---|
| **External client**: wraps a third-party API, SDK, socket or model | `supabase` (auth admin), `runpod` (OpenAI-compatible LLM), `s3`, `tts` (unix-socket sidecar) | `Client` struct, `New()` reads env, no logger or db |
| **Domain service**: business logic reused by handlers, tools or jobs | `calendar` (create/update/search events) | `Service` struct, `New(logger, db, ...deps)` |

One package per service, named after the thing it wraps. If the name clashes with the SDK package, add the suffix `internal` (`package supabaseinternal`).

## External client
```go
// Package s3 is a thin client around the S3-compatible object storage used for
// user uploads.
package s3

type Client struct {
    *s3sdk.Client        // embed the SDK so callers keep the full API
    bucket string
}

func New() (*Client, error) {
    endpoint, ok := os.LookupEnv("S3_ENDPOINT")
    if !ok {
        return nil, utils.FormatMissingEnvVarError("S3_ENDPOINT")
    }
    bucket, ok := os.LookupEnv("S3_BUCKET")
    if !ok {
        return nil, utils.FormatMissingEnvVarError("S3_BUCKET")
    }
    // ...build sdk client
    return &Client{Client: sdk, bucket: bucket}, nil
}

type PutObjectParams struct {
    ContentType string
}

func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, params PutObjectParams) error {
    if _, err := c.Client.PutObject(ctx, /* ... */); err != nil {
        return errors.WithStack(err)
    }
    return nil
}
```
- **Embed** the SDK client (`openai.Client`, `*supabase.Client`) and add only the project-specific methods.
- For a hand-written HTTP API, use its own struct embedding `*http.Client` with an explicit `Timeout`, plus `apiBaseURL` and `apiKey` fields (see `newHTTPAuthClient` in supabase).
- For a local sidecar, take the socket or path as a constructor argument (`tts.New("/tmp/app_tts.sock")`). Use a custom `http.Transport` with `DialContext` for unix sockets, and set `ResponseHeaderTimeout` instead of `Timeout` so streams aren't cut off.
- Every new env var goes into `.env.example` under a `# Name` header block.

## Domain service
```go
package widgets

var ErrInvalidName = errors.New("invalid name")
var ErrInvalidDescription = errors.New("invalid description")

type Service struct {
    logger zerolog.Logger
    db     *postgresdb.Client
}

func New(logger zerolog.Logger, db *postgresdb.Client) *Service {
    return &Service{logger: logger, db: db}
}

type CreateParams struct {
    Name        string
    Description *string
}

func (s *Service) Create(ctx context.Context, userID int, params CreateParams) (*dbmodels.Widget, error) {
    if err := validate.Field(params.Name, "min=1,max=255"); err != nil {
        return nil, errors.Wrap(ErrInvalidName, err.Error()) // classify, keep detail
    }

    publicID, err := publicid.New(publicid.NewParams{})
    if err != nil {
        return nil, errors.WithStack(err)
    }

    widget := &dbmodels.Widget{}
    if err := s.db.WidgetsInsertWidget(widget, dbmodels.WidgetInsertPayload{
        PublicID: publicID, UserID: userID, Name: params.Name, Description: params.Description,
    }); err != nil {
        return nil, errors.WithStack(err)
    }
    return widget, nil
}
```
- A service returns **dbmodels**, not API models. Handlers do the mapping.
- Every expected failure is an exported sentinel `ErrX`, so callers can `errors.Is` it into an API error code. Use `errors.Wrap(ErrX, cause.Error())` when there is an underlying error, and `errors.WithStack(ErrX)` when there isn't.
- Signatures are `(ctx, <ids>, XParams)`. Optional inputs are pointers inside Params.
- Constants that encode product decisions are exported at the top (`const DefaultTimezone = "Europe/Warsaw"`).
- ✅ When handlers **and** another caller (LLM tools, background jobs) do the same thing, the logic must live here, not in a handler.

## Wiring
Construct in `main.go`: external clients first, then domain services that depend on them, then the `PublicServer`. Pass dependencies explicitly. There are no package-level singletons or `init()` side effects.
