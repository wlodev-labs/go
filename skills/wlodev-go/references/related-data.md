# Related data: batch loading without JOINs

Use this whenever a response contains rows from more than one table: children (`widget.parts`), a join table with its own data (`widget_parts` → `parts`), or a many-to-one lookup (`widget.category`).

## The approach
Load **one table per query**, then stitch the rows together in Go. Each relation level costs exactly one query, whatever the number of rows: a widgets list with parts is 3 queries (widgets, widget_parts, parts), never 1 + N. This is batch loading, also called an application-side join, the same thing `prefetch_related` and GraphQL dataloaders do.

Why it beats a JOIN for child data:
- **Pagination stays correct.** `LIMIT 20` applies to parents. A JOIN on a one-to-many relation returns one row per child, so `LIMIT` cuts a parent's children in half and breaks the `next`/`prev` cursor.
- **No row multiplication.** Parent columns aren't repeated for every child, and you don't de-duplicate in Go.
- **Flat dbmodels.** Every query scans into exactly one dbmodel. No relation pointer fields, no `db:"part.name"` prefixes, no nullable embedded structs from LEFT JOINs.
- **Reusable queries.** `PartsSelectPartsByIDs` serves every feature that references parts.
- **Simple, index-friendly SQL.** `fk = ANY($1)` uses the FK index directly.

### When SQL still joins
- ✅ **To filter, sort or authorize the parent by a child column**, put a JOIN or `EXISTS` in the *parent* query (with `postgres.GetColumnsWithPrefix("w", ...)` for the parent's columns). Then load child data separately, as below.
  ```sql
  WHERE w.user_id = ? AND EXISTS (SELECT 1 FROM widget_parts wp WHERE wp.widget_id = w.id AND wp.part_id = ?)
  ```
- ✅ **For counts and aggregates**, run a separate query that returns `(parent_id, count)` with `GROUP BY parent_id`, and attach it with `SliceToResponseMap`.
- ❌ Never fetch child *data* through a JOIN.

## The recipe
1. **Parent query**, scoped by owner (`WidgetsSelectUserPaginatedWidgets`, `WidgetsSelectWidgetByUserIDAndPublicID`).
2. **Each level:** collect the ids (`postgres.SliceToIdsSlice`, or `postgres.SliceToOptionalIdsSlice` for a nullable FK), then run **one** `…ByIDs` query with `= ANY($1)`.
3. **Map bottom-up:**
   - Leaves (no further relation) become API models in a map: `SliceToResponseMap` for 1:1 (`id → PartGet`), `SliceToGroupResponseMap` for 1:N (`widgetID → []WidgetImageGet`).
   - Middle levels look up their leaf, then group under the parent id.
   - The parent mapper receives its children as an argument.
4. **Wrap steps 2–3 in a loader** that every handler returning the schema calls.

## Example: widgets → widget_parts → parts
`widget_parts` is a join table with its own data (`quantity`, `position`), the same shape as a report's devices. `WidgetGet` gains `parts: WidgetPartGet[]`, and each `WidgetPartGet` embeds a `PartGet`.

### Migration
```sql
-- +goose Up
CREATE TABLE parts (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(12) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    sku VARCHAR(64) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_parts_user_id_created_at ON parts (user_id, created_at DESC);

CREATE TABLE widget_parts (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(12) NOT NULL UNIQUE,
    widget_id INTEGER NOT NULL REFERENCES widgets(id) ON DELETE CASCADE,
    part_id INTEGER NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1,
    position INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (widget_id, part_id) -- also serves "widget_id = ANY($1)"
);
CREATE INDEX idx_widget_parts_part_id ON widget_parts (part_id);

-- +goose Down
DROP TABLE IF EXISTS widget_parts;
DROP TABLE IF EXISTS parts;
```
✅ Every FK column that a `…ByIDs` query filters with `ANY` has an index. A composite `UNIQUE`/index whose **first** column is the FK counts.

### Spec
Register `PartGet` and `WidgetPartGet` in `models.openapi.yml` like any other schema. A schema file refers to another *named* schema through the registry, so the generated field gets the named Go type and not an anonymous inline struct:
```yaml
# domains/widgets/schemas/WidgetPartGet.yml
type: object
required: [id, part, quantity, position, createdAt]
properties:
    id:
        $ref: '../../../common/schemas/Id.yml'
    part:
        $ref: '../../../models.openapi.yml#/components/schemas/PartGet'
    quantity:
        type: integer
    position:
        type: integer
    createdAt:
        $ref: '../../../common/schemas/Timestamp.yml'
```
In `WidgetGet.yml`, add `parts` to `required` and as `type: array`, `items: $ref: '../../../models.openapi.yml#/components/schemas/WidgetPartGet'`.

### dbmodels: flat, one struct per table
```go
type Part struct {
    ID        int       `db:"id"`
    PublicID  string    `db:"public_id"`
    UserID    int       `db:"user_id"`
    Name      string    `db:"name"`
    SKU       string    `db:"sku"`
    UpdatedAt time.Time `db:"updated_at"`
    CreatedAt time.Time `db:"created_at"`
}

type WidgetPart struct {
    ID        int       `db:"id"`
    PublicID  string    `db:"public_id"`
    WidgetID  int       `db:"widget_id"`
    PartID    int       `db:"part_id"` // the FK only; no `Part *Part` field
    Quantity  int       `db:"quantity"`
    Position  int       `db:"position"`
    UpdatedAt time.Time `db:"updated_at"`
    CreatedAt time.Time `db:"created_at"`
}
```
Each struct has a matching `XColumns` slice, as in [database.md](database.md).

### Queries
```go
func (q *Queries) WidgetsSelectWidgetPartsByWidgetIDs(widgetIDs []int) ([]*dbmodels.WidgetPart, error) {
    var widgetParts []*dbmodels.WidgetPart
    return widgetParts, q.Select(&widgetParts, `
        SELECT
            `+postgres.GetColumns(dbmodels.WidgetPartColumns)+`
        FROM widget_parts
        WHERE
            widget_id = ANY($1)
        ORDER BY widget_id, position ASC, id ASC
    `, widgetIDs)
}

func (q *Queries) PartsSelectPartsByIDs(ids []int) ([]*dbmodels.Part, error) {
    var parts []*dbmodels.Part
    return parts, q.Select(&parts, `
        SELECT
            `+postgres.GetColumns(dbmodels.PartColumns)+`
        FROM parts
        WHERE
            id = ANY($1)
    `, ids)
}
```
- Naming: `<Domain>Select<What>By<Key>IDs(ids []int)`, with a plural key: `ByIDs`, `ByWidgetIDs`.
- ✅ Child queries that produce a list have a deterministic `ORDER BY` (FK first, then the display order, then `id`). The groups keep that order. Lookup queries (`PartsSelectPartsByIDs`) don't need one, because the result goes into a map.
- `[]int` is passed straight as `$1`; pgx encodes it as an array.

### Mappers: children come in as arguments
```go
type ToPartGetParams struct{}

func ToPartGet(dbPart *dbmodels.Part, params ToPartGetParams) PartGet {
    return PartGet{ID: dbPart.PublicID, Name: dbPart.Name, SKU: dbPart.SKU, CreatedAt: dbPart.CreatedAt}
}

type ToWidgetPartGetParams struct{}

func ToWidgetPartGet(dbWidgetPart *dbmodels.WidgetPart, part PartGet, params ToWidgetPartGetParams) WidgetPartGet {
    return WidgetPartGet{
        ID:        dbWidgetPart.PublicID,
        Part:      part,
        Quantity:  dbWidgetPart.Quantity,
        Position:  dbWidgetPart.Position,
        CreatedAt: dbWidgetPart.CreatedAt,
    }
}

type ToWidgetGetParams struct{}

func ToWidgetGet(dbWidget *dbmodels.Widget, parts []WidgetPartGet, params ToWidgetGetParams) WidgetGet {
    if parts == nil {
        parts = []WidgetPartGet{} // a widget without parts is `[]`, never `null`
    }
    return WidgetGet{
        ID:    dbWidget.PublicID,
        Name:  dbWidget.Name,
        Parts: parts,
        // ...
    }
}
```
- Required related data is a positional argument, between the db model and Params, so a missing argument fails to compile. Optional extras go into the Params struct.
- The parent mapper turns `nil` into an empty slice. That makes "no children" correct for every caller, including a map lookup on a missing key.

### Loader: one per relation, shared by every handler
```go
// loadWidgetPartsGet returns the parts of the given widgets as API models,
// keyed by the widget's internal id. Widgets without parts are absent from
// the map; ToWidgetGet turns the nil lookup into [].
func (s *PublicServer) loadWidgetPartsGet(widgetIDs []int) (map[int][]models.WidgetPartGet, error) {
    if len(widgetIDs) == 0 {
        return map[int][]models.WidgetPartGet{}, nil
    }

    widgetParts, err := s.db.WidgetsSelectWidgetPartsByWidgetIDs(widgetIDs)
    if err != nil {
        return nil, errors.WithStack(err)
    }

    partIDs := postgres.SliceToIdsSlice(widgetParts, func(wp *dbmodels.WidgetPart) int {
        return wp.PartID
    })
    parts, err := s.db.PartsSelectPartsByIDs(partIDs)
    if err != nil {
        return nil, errors.WithStack(err)
    }
    partsByID := postgres.SliceToResponseMap(parts, func(p *dbmodels.Part) (int, models.PartGet) {
        return p.ID, models.ToPartGet(p, models.ToPartGetParams{})
    })

    result := make(map[int][]models.WidgetPartGet, len(widgetIDs))
    for _, wp := range widgetParts {
        part, ok := partsByID[wp.PartID]
        if !ok {
            return nil, errors.Newf("part %d not found for widget part %d", wp.PartID, wp.ID)
        }
        result[wp.WidgetID] = append(result[wp.WidgetID], models.ToWidgetPartGet(wp, part, models.ToWidgetPartGetParams{}))
    }
    return result, nil
}
```
The middle level uses a hand-written loop instead of `SliceToGroupResponseMap`, because the leaf lookup can fail and needs an error path. A leaf relation with no lookup is a single call:
```go
images, err := s.db.WidgetsSelectWidgetImagesByWidgetIDs(widgetIDs)
if err != nil {
    return nil, errors.WithStack(err)
}
return postgres.SliceToGroupResponseMap(images, func(i *dbmodels.WidgetImage) (int, models.WidgetImageGet) {
    return i.WidgetID, models.ToWidgetImageGet(i, models.ToWidgetImageGetParams{})
}), nil
```

### Handlers: Get and List share the loader
```go
// WidgetsGet: after the 404/500 lookup of the widget
partsByWidgetID, err := s.loadWidgetPartsGet([]int{widget.ID})
if err != nil {
    s.logger.Error().Err(err).Send()
    return WidgetsGet500JSONResponse(WidgetsGet500JSONResponse{
        InternalServerErrorJSONResponse: models.GenericInternalServerErrorResponse(models.GenericInternalServerErrorResponseParams{}),
    }), nil
}
return WidgetsGet200JSONResponse(models.ToWidgetGet(widget, partsByWidgetID[widget.ID], models.ToWidgetGetParams{})), nil
```
```go
// WidgetsList: after the paginated parent query
widgetIDs := postgres.SliceToIdsSlice(items, func(w *dbmodels.Widget) int { return w.ID })
partsByWidgetID, err := s.loadWidgetPartsGet(widgetIDs)
if err != nil {
    s.logger.Error().Err(err).Send()
    return WidgetsList500JSONResponse(/* generic 500 */), nil
}

data := make([]models.WidgetGet, len(items))
for i, item := range items {
    data[i] = models.ToWidgetGet(item, partsByWidgetID[item.ID], models.ToWidgetGetParams{})
}
```
Create and Update return `WidgetGet` too, so they call the same loader before mapping. The response order is the order of the parent slice. `SliceToIdsSlice` de-duplicates through a map, so the id order is random, and that's fine.

## Many-to-one lookups
For a parent that references one row (for example `widgets.category_id`, nullable): collect the ids with `SliceToOptionalIdsSlice`, load them with `CategoriesSelectCategoriesByIDs`, index them with `SliceToResponseMap`, and look up each parent's row while mapping:
```go
var category *models.CategoryGet // nullable FK → pointer in the API model
if w.CategoryID != nil {
    c, ok := categoriesByID[*w.CategoryID]
    if !ok {
        return nil, errors.Newf("category %d not found for widget %d", *w.CategoryID, w.ID)
    }
    category = &c
}
```
Pass it to the mapper as a positional argument: `ToWidgetGet(w, parts, category, params)`.

## Rules
- ❌ Never run a query inside a loop over rows. Each relation level is one `…ByIDs` query.
- ❌ Never add relation fields (`Part *Part`, `Parts []*WidgetPart`) to dbmodels. A dbmodel mirrors one table.
- ❌ Mappers never query. They receive the related API models as arguments.
- ✅ Write **one loader per relation**: a private server method `load<Parent><Children>Get(parentIDs []int) (map[int][]models.XGet, error)`, at the bottom of the domain handler file next to the validators. Get calls it with `[]int{x.ID}`, and List with the page's ids. ❌ Never copy the assembly into a second handler.
- ✅ Keep loaders one level deep and compose them. If parts had suppliers, `loadWidgetPartsGet` would call `loadPartSuppliersGet(partIDs)` and pass the result to `ToPartGet`.
- ✅ A missing FK target is an integrity error. Return `errors.Newf(...)` with both ids, and the handler logs it and returns 500.
- ✅ A parent with no children gets `[]`. ❌ Never return 500 or `null` for it. If "at least one child" is a business rule, enforce it when writing, not when reading.
- ✅ Load children only by ids that came from an owner-scoped parent query. ❌ Never pass ids from the request into a `…ByIDs` query; resolve them with an owner-scoped query first (`…ByUserIDAndPublicIDs`).
- ✅ Index every FK column used with `ANY`.
- ✅ Return early for empty ids. `ANY('{}')` is correct but costs a round trip.
- ⚠️ The levels are separate statements, so a row deleted between two queries can surface as a missing-relation 500. It's rare, and accepted for reads.
- If a second server package (for example `internal/admin`) needs the same aggregate, move the db part to `services/<domain>`, returning grouped dbmodels (`SliceToGroupMap`), and keep the mapping in each handler package.

## Helper cheat-sheet
All of these are in `github.com/wlodev-labs/go/postgres`. See [lib.md](lib.md).

| Step | Helper | Result |
|---|---|---|
| Ids of the rows you have, for `ANY($1)` | `SliceToIdsSlice(items, func(*T) int)` | unique `[]int` |
| Same, for a nullable FK | `SliceToOptionalIdsSlice(items, func(*T) *int)` | unique `[]int`, nil values skipped |
| Same, for string keys | `SliceToStringSlice(items, func(*T) string)` | unique `[]string` |
| 1:1 lookup of dbmodels | `SliceToIndexMap(items, func(*T) K)` | `map[K]*T` |
| 1:1 lookup of API models (leaf) | `SliceToResponseMap(items, func(*T) (K, R))` | `map[K]R` |
| 1:N grouping of dbmodels | `SliceToGroupMap(items, func(*T) K)` | `map[K][]*T`, order kept |
| 1:N grouping of API models (leaf) | `SliceToGroupResponseMap(items, func(*T) (K, R))` | `map[K][]R`, order kept |
