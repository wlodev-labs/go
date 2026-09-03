package postgres

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"
	"github.com/wlodev-labs/go/pagination"
)

var ErrNoRowsAffected = errors.New("no rows affected")

type SQLExecutor interface {
	Queryx(query string, args ...any) (*sqlx.Rows, error)
	QueryRowx(query string, args ...any) *sqlx.Row
	Exec(query string, args ...any) (sql.Result, error)
	Get(dest any, query string, args ...any) error
	Select(dest any, query string, args ...any) error
	NamedExec(query string, arg any) (sql.Result, error)
	Rebind(query string) string
}

type Queries struct {
	SQLExecutor
}

func NewQueries(ex SQLExecutor) *Queries {
	return &Queries{
		SQLExecutor: ex,
	}
}

func (q *Queries) Insert(query string, args ...any) error {
	if resp, err := q.Exec(query, args...); err != nil {
		return errors.WithStack(err)
	} else if rowsAffected, err := resp.RowsAffected(); err != nil {
		return errors.WithStack(err)
	} else if rowsAffected == 0 {
		return errors.WithStack(ErrNoRowsAffected)
	}
	return nil
}

func (q *Queries) InsertWithDest(dest any, returningColumns string, query string, args ...any) error {
	if err := q.Get(dest, fmt.Sprintf("%s RETURNING %s", query, returningColumns), args...); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (q *Queries) Update(query string, args ...any) error {
	if resp, err := q.Exec(query, args...); err != nil {
		return errors.WithStack(err)
	} else if rowsAffected, err := resp.RowsAffected(); err != nil {
		return errors.WithStack(err)
	} else if rowsAffected == 0 {
		return errors.WithStack(ErrNoRowsAffected)
	}
	return nil
}

func (q *Queries) UpdateWithDest(dest any, returningColumns string, query string, args ...any) error {
	if err := q.Get(dest, fmt.Sprintf("%s RETURNING %s", query, returningColumns), args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.WithStack(ErrNoRowsAffected)
		}
		return errors.WithStack(err)
	}
	return nil
}

func (q *Queries) Delete(query string, args ...any) error {
	if resp, err := q.Exec(query, args...); err != nil {
		return errors.WithStack(err)
	} else if rowsAffected, err := resp.RowsAffected(); err != nil {
		return errors.WithStack(err)
	} else if rowsAffected == 0 {
		return errors.WithStack(ErrNoRowsAffected)
	}
	return nil
}

// Where clause builder helps with managing WHERE clauses where e.g. pagination
// is involved with other filters like soft delete and query specific filters.
type WhereCondition struct {
	// Clause can be a simple condition like "id = ?" or a complex one like
	// "name = ? OR age > ?" Use placeholders (?) for arguments instead of
	// numbers (e.g., $1, $2) as it will be handled by the rebind function.
	Clause string
	Args   []any
}

type BuildWhereClausePaginationParams struct {
	// Specify the table prefix and column for time filtering, e.g.
	// "teams.created_at"
	TimeColumn string
	// Pagination parsed data
	Parsed *pagination.Parsed
}

type BuildWhereClauseParams struct {
	// Optional, pagination related parameters
	Pagination *BuildWhereClausePaginationParams
}

// BuildWhereClause takes a slice of WhereCondition and returns a WHERE clause
// string and a slice of arguments. It also takes optional pagination parameters
// to include in the WHERE clause.
//
// Note: The returned query should be passed through the rebind function before
// executing it to ensure that the placeholders are correctly formatted.
func (q *Queries) BuildWhereClause(conditions []WhereCondition, params BuildWhereClauseParams) (string, []any) {
	var clauses []string
	var args []any

	if params.Pagination != nil && params.Pagination.Parsed != nil {
		pagData := params.Pagination.Parsed
		timeCol := params.Pagination.TimeColumn
		if pagData.Until != nil {
			clauses = append(clauses, fmt.Sprintf("%s < ?", timeCol))
			args = append(args, *pagData.Until)
		}

		if pagData.Since != nil {
			clauses = append(clauses, fmt.Sprintf("%s > ?", timeCol))
			args = append(args, *pagData.Since)
		}
	}

	for _, c := range conditions {
		clauses = append(clauses, c.Clause)
		args = append(args, c.Args...)
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

var ErrInvalidUpdatePayload = errors.New("invalid update payload: must be a struct or pointer to struct")
var ErrNoFieldsToUpdate = errors.New("no fields to update")

// BuildUpdateQuery constructs an UPDATE SQL query string and its corresponding
// arguments based on the provided table name, payload struct, and conditions.
//
// The payload struct should have fields tagged with `db` to indicate the
// corresponding database columns. Only fields that implement the UpdateItem
// interface and are set will be included in the SET clause of the query.
func (q *Queries) BuildUpdateQuery(tableName string, payload any, conditions []WhereCondition) (string, []any, error) {
	v := reflect.ValueOf(payload)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return "", nil, ErrInvalidUpdatePayload
	}

	t := v.Type()
	fieldCount := t.NumField()
	sets := make([]string, 0, fieldCount)
	args := make([]any, 0, fieldCount+len(conditions))

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		col := field.Tag.Get("db")
		if col == "" || col == "-" {
			continue
		}

		if pf, ok := v.Field(i).Interface().(UpdateItem); ok && pf.IsSet() {
			sets = append(sets, col+" = ?")
			args = append(args, pf.Get())
		}
	}

	if len(sets) == 0 {
		return "", nil, ErrNoFieldsToUpdate
	}

	var queryBuilder strings.Builder
	// Rough capacity estimate: 50 bytes for base + 20 per set + 40 per condition
	queryBuilder.Grow(50 + (len(sets) * 20) + (len(conditions) * 40))

	queryBuilder.WriteString("UPDATE ")
	queryBuilder.WriteString(tableName)
	queryBuilder.WriteString(" SET ")
	queryBuilder.WriteString(strings.Join(sets, ", "))

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		for i, c := range conditions {
			if c.Clause == "" {
				continue
			}

			if i > 0 {
				queryBuilder.WriteString(" AND ")
			}
			queryBuilder.WriteString("(")
			queryBuilder.WriteString(c.Clause)
			queryBuilder.WriteString(")")
			args = append(args, c.Args...)
		}
	}
	return q.Rebind(queryBuilder.String()), args, nil
}
