# OpenAPI spec conventions (`_oas/`)

OpenAPI **3.0.3**. Files are `.yml` with 4-space indentation. Lint with redocly:

```yaml
# redocly.yaml
extends:
    - recommended
rules:
    info-license: off
    operation-2xx-response: off
    operation-4xx-response: off
```

## Source files vs. generated files

Write only in the source files: `models.openapi.yml`, `public.openapi.yml`, `domains/**` and `common/**`.
`_oas/bundled/` is **generated** by `make openapi` (redocly bundle). ❌ Never create, edit or patch files there when you add a schema, parameter or endpoint. Your changes would be overwritten on the next bundle. Change the sources and run `make openapi`.

## Two root files, two jobs

| File | Contains | Generates |
|---|---|---|
| `_oas/models.openapi.yml` | `paths: {}` plus **all** `components` (schemas, parameters, responses, securitySchemes). Each entry is just a `$ref` to its file. | `internal/models` (from the *bundled* copy) |
| `_oas/public.openapi.yml` | `paths:` only, each a `$ref` to a domain file, plus `securitySchemes` re-referenced from models | `internal/public` (handlers, request/response types) |

```yaml
# _oas/models.openapi.yml
openapi: 3.0.3
info:
    title: <Project> Models
    version: 0.0.0
servers:
    - url: 'http://localhost'
      description: Local server
paths: {}
components:
    securitySchemes:
        sessionAuth:
            type: apiKey
            in: cookie
            name: AccessToken
    parameters:
        limit:
            $ref: common/parameters/pagination-parameters.yml#/limit
        widgetId:
            $ref: domains/widgets/parameters/widgets-parameters.yml#/widgetId
    schemas:
        BaseError:
            $ref: common/schemas/BaseError.yml
        WidgetGet:
            $ref: domains/widgets/schemas/WidgetGet.yml
    responses:
        Unauthorized:
            $ref: common/responses/errors/Unauthorized.yml
```

```yaml
# _oas/public.openapi.yml
paths:
    /v1/widgets:
        $ref: './domains/widgets/widgets.yml'
    /v1/widgets/{widgetId}:
        $ref: './domains/widgets/widget.yml'
components:
    securitySchemes:
        sessionAuth:
            $ref: models.openapi.yml#/components/securitySchemes/sessionAuth
```

**Rule:** if Go code has to refer to a type by name (`models.WidgetGet`, `models.Widgets404Error`, `models.WidgetStatus`), that type **must** be registered in `models.openapi.yml`. Keep the registry grouped: common types first, then one block per domain in the same order as `public.openapi.yml`.

## Domain folder

```
_oas/domains/widgets/
├── widgets.yml                    # /v1/widgets           → post (create), get (list)
├── widget.yml                     # /v1/widgets/{widgetId} → get, patch, delete
├── parameters/widgets-parameters.yml
└── schemas/
    ├── WidgetBase.yml
    ├── WidgetCreate.yml
    ├── WidgetGet.yml
    ├── WidgetUpdate.yml
    ├── WidgetStatus.yml           # standalone enums get their own file + registry entry
    └── errors/
        ├── Widgets404Error.yml
        ├── WidgetsCreate400Error.yml
        └── WidgetsUpdate400Error.yml
```

- Path files: the plural name holds the collection and the singular name holds the item. Sub-resources and actions use kebab-case (`commands-history.yml`, `openapi-file.yml`).
- Parameters: one file per domain, a **map keyed by parameter name**, referenced as `...-parameters.yml#/widgetId`.
- `common/` holds cross-domain pieces: `parameters/pagination-parameters.yml`, `responses/errors/{BadRequest,Unauthorized,Forbidden,NotFound,InternalServerError}.yml`, and `schemas/{BaseError,Pagination,Timestamp,Id,IdMedium,IdLong}.yml` plus `schemas/errors/*`.

## `$ref` rules

| From | To | How |
|---|---|---|
| operation file | any component | `'../../models.openapi.yml#/components/schemas/WidgetGet'` |
| schema file | sibling building block | `'./WidgetBase.yml'` (inlined; **not** a named type) |
| schema file | common primitive | `'../../../common/schemas/Timestamp.yml'` |
| error schema | BaseError | `../../../../models.openapi.yml#/components/schemas/BaseError` |
| common response | common error schema | `../../schemas/errors/BadRequestError.yml` |

The number of `../` matters for codegen import mapping. See quirk 1.

## Schema quartet

```yaml
# WidgetBase.yml: every property, nothing required
type: object
properties:
    name:
        type: string
        description: The name of the widget
    description:
        type: string
        description: Optional longer description
    tags:
        type: array
        items:
            type: string
```

```yaml
# WidgetCreate.yml
allOf:
    - $ref: './WidgetBase.yml'
    - type: object
      required:
          - name
```

```yaml
# WidgetGet.yml: same required set as Create + server fields
allOf:
    - $ref: './WidgetBase.yml'
    - type: object
      required:
          - name
    - type: object
      required:
          - id
          - updatedAt
          - createdAt
      properties:
          id:
              $ref: '../../../common/schemas/Id.yml'
          updatedAt:
              $ref: '../../../common/schemas/Timestamp.yml'
          createdAt:
              $ref: '../../../common/schemas/Timestamp.yml'
```

```yaml
# WidgetUpdate.yml: PATCH body, everything optional → pointers in Go
allOf:
    - $ref: './WidgetBase.yml'
```

Fields that exist only on update (e.g. `status`) go in an extra `allOf` item of `XUpdate`. Fields that exist only on read go in `XGet`.

Generated Go: `WidgetCreate.Name string`, `WidgetCreate.Description *string`, and `WidgetUpdate.Name *string`.

## Operations

```yaml
# widgets.yml (excerpt)
post:
    operationId: widgetsCreate          # <domain><Action>, camelCase
    summary: Create a new widget
    description: Create a new widget.
    tags:
        - Widgets
    security:
        - sessionAuth: []               # ALWAYS explicit; `security: []` for public
    requestBody:
        required: true
        content:
            application/json:
                schema:
                    $ref: '../../models.openapi.yml#/components/schemas/WidgetCreate'
    responses:
        '200':                          # quoted status codes; create returns 200 with XGet
            description: Widget created successfully
            content:
                application/json:
                    schema:
                        $ref: '../../models.openapi.yml#/components/schemas/WidgetGet'
        '400':
            description: Something went wrong with the request
            content:
                application/json:
                    schema:
                        type: object
                        required:
                            - error
                        properties:
                            error:
                                $ref: '../../models.openapi.yml#/components/schemas/WidgetsCreate400Error'
        '401':
            $ref: ../../models.openapi.yml#/components/responses/Unauthorized
        '500':
            $ref: ../../models.openapi.yml#/components/responses/InternalServerError
```

| Action | Method | operationId | Success |
|---|---|---|---|
| create | POST collection | `widgetsCreate` | 200 + `WidgetGet` |
| list | GET collection | `widgetsList` | 200 + `{data, pagination}` |
| get | GET item | `widgetsGet` | 200 + `WidgetGet` |
| update | PATCH item | `widgetsUpdate` | 200 + `WidgetGet` |
| delete | DELETE item | `widgetsDelete` | 200, no body |

Paths are versioned: `/v1/...`. Path params are named `<resource>Id` (`widgetId`). On the wire, names stay camelCase with `Id`, while the generated Go field is `WidgetID` (name normalizer, quirk 14).

## Errors

```yaml
# common/schemas/BaseError.yml
type: object
required:
    - code
properties:
    code:
        type: string
        example: bad_request
    message:
        type: string
```

```yaml
# domains/widgets/schemas/errors/WidgetsCreate400Error.yml
allOf:
    - $ref: ../../../../models.openapi.yml#/components/schemas/BaseError
    - type: object
      properties:
          code:
              type: string
              enum:
                  - 'invalid_name'
                  - 'invalid_description'
```

- Names: `<Domain><Op?><Status>Error`. Examples: `Widgets404Error` (shared by get/update/delete), `WidgetsCreate400Error`, `WidgetsUpdate400Error`, `Pagination400Error`.
- Codes are snake_case and specific (`invalid_<field>`, `<resource>_not_found`, `end_time_before_start_time`). The client switches on `code`, so never reuse a generic `bad_request` for a domain error.
- 400/404 are written **inline** in the operation as `{error: $ref}` (see above). 401/500 (and generic 400/403/404) use `components/responses/*`.
- A list endpoint's 400 is `Pagination400Error` (code `invalid_limit`). If the list has its own filters, build a domain error that includes `invalid_limit` too.

## Pagination

```yaml
get:
    operationId: widgetsList
    tags:
        - Widgets
        - Pagination
    parameters:
        - $ref: '../../models.openapi.yml#/components/parameters/since'
        - $ref: '../../models.openapi.yml#/components/parameters/until'
        - $ref: '../../models.openapi.yml#/components/parameters/limit'
    responses:
        '200':
            description: A list of widgets retrieved successfully
            content:
                application/json:
                    schema:
                        type: object
                        required:
                            - data
                            - pagination
                        properties:
                            data:
                                type: array
                                items:
                                    $ref: '../../models.openapi.yml#/components/schemas/WidgetGet'
                            pagination:
                                $ref: '../../models.openapi.yml#/components/schemas/Pagination'
```

Cursor pagination is time-based: `until` returns items older than the timestamp and `since` returns items newer. `Pagination` is `{count, next, prev}`, where `next` and `prev` are nullable date-times. See `lib.md` for the server side.

## Enums
- Standalone enums (`WidgetStatus`, `Modality`) get their own schema file and a registry entry. The DB model mirrors them with its own string type, and `mappings.go` converts with `models.WidgetStatus(db.Status)`.
- Enum values are lowercase snake_case for API-facing values. Old code may have UPPER_CASE (`CommandIntent`); don't add more of those.

## Writing style
- `summary` is a short imperative phrase ("Get a widget by ID"). `description` is a full sentence ending in a period.
- Every property has a `description`. Primitives in `common/` carry an `example`.
- Success `description` names the resource and outcome: "Widget updated successfully". Use "Something went wrong with the request" for 400 and "Requested resource was not found" for 404.
- Tags are PascalCase domain names (`Widgets`). Add `Pagination` on list endpoints.
