# Handlers (`internal/public`)

## Server struct
```go
// internal/public/server.go
type PublicServer struct {
    logger     zerolog.Logger
    db         *postgresdb.Client
    pagination *pagination.Client
    widgets    *widgets.Service // domain services from services/
}

func NewPublicServer(
    logger zerolog.Logger,
    db *postgresdb.Client,
    pagination *pagination.Client,
    widgetsService *widgets.Service,
) *PublicServer {
    return &PublicServer{logger: logger, db: db, pagination: pagination, widgets: widgetsService}
}
```
Dependencies are unexported fields set by a constructor with one parameter per line. There is no global state.

## File layout
There is one file per domain (`widgets.go`) with the CRUD methods in spec order (create, list, get, update, delete). Small private validators sit at the bottom (`validateWidgetName`). Handler-local sentinel errors live in `errors.go`.

## Anatomy of a handler
```go
func (s *PublicServer) WidgetsGet(ctx context.Context, request WidgetsGetRequestObject) (WidgetsGetResponseObject, error) {
    // 1. auth guard (only when the operation has sessionAuth)
    authUser, passed := auth.GetAuthUserContext(ctx)
    if !passed {
        s.logger.Error().Msg(utils.CriticalAuthenticationPass)
        return WidgetsGet401JSONResponse(WidgetsGet401JSONResponse{
            UnauthorizedJSONResponse: models.GenericUnauthorizedResponse(models.GenericUnauthorizedResponseParams{}),
        }), nil
    }

    // 2. load / act; every error is logged, then mapped to a typed response
    widget, err := s.db.WidgetsSelectWidgetByUserIDAndPublicID(authUser.ID, request.WidgetID)
    if err != nil {
        s.logger.Error().Err(err).Send()
        if errors.Is(err, sql.ErrNoRows) {
            return WidgetsGet404JSONResponse(WidgetsGet404JSONResponse{
                Error: models.Widgets404Error{Code: models.Widgets404ErrorCodeWidgetNotFound},
            }), nil
        }
        return WidgetsGet500JSONResponse(WidgetsGet500JSONResponse{
            InternalServerErrorJSONResponse: models.GenericInternalServerErrorResponse(models.GenericInternalServerErrorResponseParams{}),
        }), nil
    }

    // 3. map db → API model
    return WidgetsGet200JSONResponse(models.ToWidgetGet(widget, models.ToWidgetGetParams{})), nil
}
```

Rules:
- ✅ Return `(response, nil)` on **every** path. A non-nil `error` would skip the typed response and reach the generic error handler.
- ✅ Wrap the return in the response type explicitly, as in `WidgetsGet404JSONResponse(WidgetsGet404JSONResponse{...})`. This is house style: it makes the response type visible at the return site.
- ✅ Call `s.logger.Error().Err(err).Send()` before returning any 4xx/5xx caused by an error. Use `Info()` for normal no-ops.
- ✅ Always scope by owner: query by `(authUser.ID, publicID)`, never by public id alone.
- ✅ Validation failures map to a specific enum code (`models.WidgetsCreate400ErrorCodeInvalidName`).
- ❌ Don't put business rules that services or other handlers also need into the handler. Move them to `services/<domain>` and map their sentinel errors (below).

## Mapping service errors
```go
widget, err := s.widgets.Create(ctx, authUser.ID, widgets.CreateParams{Name: request.Body.Name})
if err != nil {
    s.logger.Error().Err(err).Send()
    switch {
    case errors.Is(err, widgets.ErrInvalidName):
        return WidgetsCreate400JSONResponse(WidgetsCreate400JSONResponse{
            Error: models.WidgetsCreate400Error{Code: models.WidgetsCreate400ErrorCodeInvalidName},
        }), nil
    default:
        return WidgetsCreate500JSONResponse(WidgetsCreate500JSONResponse{
            InternalServerErrorJSONResponse: models.GenericInternalServerErrorResponse(models.GenericInternalServerErrorResponseParams{}),
        }), nil
    }
}
```

## List with pagination
```go
pag, err := s.pagination.Parse(pagination.ParseParams{
    Limit: request.Params.Limit,
    Until: request.Params.Until,
    Since: request.Params.Since,
})
if err != nil {
    s.logger.Error().Err(err).Send()
    if errors.Is(err, pagination.ErrInvalidLimit) {
        return WidgetsList400JSONResponse(WidgetsList400JSONResponse{
            Error: models.Pagination400Error{Code: models.Pagination400ErrorCodeInvalidLimit},
        }), nil
    }
    return WidgetsList500JSONResponse(/* generic 500 */), nil
}

items, err := s.db.WidgetsSelectUserPaginatedWidgets(authUser.ID, pag, queries.WidgetsSelectUserPaginatedWidgetsParams{})
// ...error → 500

data := make([]models.WidgetGet, len(items))
for i, item := range items {
    data[i] = models.ToWidgetGet(item, models.ToWidgetGetParams{})
}
pagResp := pagination.GetResponseData(items, func(w *dbmodels.Widget) time.Time { return w.CreatedAt })

return WidgetsList200JSONResponse(WidgetsList200JSONResponse{
    Data:       data,
    Pagination: models.ToPagination(pagResp, models.ToPaginationParams{}),
}), nil
```
The time column passed to `GetResponseData` must match the `TimeColumn` used in the query and its `ORDER BY`.

## Update (PATCH)
```go
widget, err := s.db.WidgetsSelectWidgetByUserIDAndPublicID(authUser.ID, request.WidgetID) // 404/500 as in Get

payload := dbmodels.WidgetUpdatePayload{}
if request.Body.Name != nil {
    if err := validateWidgetName(*request.Body.Name); err != nil {
        s.logger.Error().Err(err).Send()
        return WidgetsUpdate400JSONResponse(/* invalid_name */), nil
    }
    payload.Name = postgres.UpdateField[string]{Set: true, Value: request.Body.Name}
}
if request.Body.Description != nil {
    payload.Description = postgres.UpdateField[string]{Set: true, Value: request.Body.Description}
}

if postgres.IsUpdatePayloadEmpty(payload) {
    s.logger.Info().Msg("no fields to update in the request body")
    return WidgetsUpdate200JSONResponse(models.ToWidgetGet(widget, models.ToWidgetGetParams{})), nil
}
payload.UpdatedAt = postgres.UpdateField[time.Time]{Set: true, Value: ptr.Time(time.Now().UTC())}

updated := &dbmodels.Widget{}
if err := s.db.WidgetsUpdateWidget(updated, widget.ID, payload); err != nil { /* 500 */ }
return WidgetsUpdate200JSONResponse(models.ToWidgetGet(updated, models.ToWidgetGetParams{})), nil
```
Check for an empty payload **before** setting `UpdatedAt`. Otherwise the payload is never empty, and `BuildUpdateQuery` only bumps the timestamp. For clearing to `null`, see quirk 7 in `openapi-quirks.md`.

## Delete
Look up by owner (404), call `s.db.WidgetsDeleteWidget(widget.ID)`, then return `WidgetsDelete200Response{}`.

## Mappers (`internal/models/mappings.go`)
```go
type ToWidgetGetParams struct{}

func ToWidgetGet(dbWidget *dbmodels.Widget, params ToWidgetGetParams) WidgetGet {
    return WidgetGet{
        ID:          dbWidget.PublicID, // API id is ALWAYS the public id
        Name:        dbWidget.Name,
        Description: dbWidget.Description,
        Status:      WidgetStatus(dbWidget.Status),
        UpdatedAt:   dbWidget.UpdatedAt,
        CreatedAt:   dbWidget.CreatedAt,
    }
}
```
Name mappers `To<SchemaName>`. Each takes a pointer to the db model and an empty Params struct. They live in package `models`, next to the generated code.

## Generic error builders (`internal/models/generic.go`)
`GenericBadRequestResponse`, `GenericUnauthorizedResponse`, `GenericForbiddenResponse`, `GenericNotFoundResponse` and `GenericInternalServerErrorResponse` each take `Generic<X>ResponseParams{Message *string}` and return the `components/responses` type. Use them for every generic 4xx/5xx. Don't build `UnauthorizedError{...}` by hand.
