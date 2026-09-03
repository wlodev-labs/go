package postgres

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func Open(ctx context.Context, host, port, user, password, dbname, sslmode string) (*sqlx.DB, error) {
	q := url.Values{}
	q.Set("sslmode", sslmode)

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     dbname,
		RawQuery: q.Encode(),
	}

	db, err := sqlx.ConnectContext(ctx, "pgx", dsn.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to postgres")
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}

func IsUniqueConstraintError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgerrcode.UniqueViolation
	}
	return false
}
