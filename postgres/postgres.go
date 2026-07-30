package postgres

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func Open(host, port, user, password, dbname, sslmode string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := db.Ping(); err != nil {
		return nil, errors.WithStack(err)
	}
	return db, nil
}

func IsUniqueConstraintError(err error) bool {
	var pqErr *pq.Error
	if ok := errors.As(err, &pqErr); ok {
		return pqErr.Code == "23505"
	}
	return false
}
