package middleware

import (
	"context"
	"database/sql"

	"github.com/shengli/prism/sdk"
)

// TracedDB wraps a *sql.DB to automatically create spans for database queries.
type TracedDB struct {
	Tracer *sdk.Tracer
	DB     *sql.DB
}

// WrapDB wraps a *sql.DB with tracing.
func WrapDB(tracer *sdk.Tracer, db *sql.DB) *TracedDB {
	return &TracedDB{Tracer: tracer, DB: db}
}

func (t *TracedDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx, span := t.Tracer.StartSpan(ctx, "sql.Query", sdk.WithKind(sdk.KindClient))
	defer t.Tracer.FinishSpan(span)

	span.SetTag("db.type", "sql")
	span.SetTag("db.statement", truncateSQL(query, 500))

	rows, err := t.DB.QueryContext(ctx, query, args...)
	if err != nil {
		span.SetError(err)
	}
	return rows, err
}

func (t *TracedDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	_, span := t.Tracer.StartSpan(ctx, "sql.QueryRow", sdk.WithKind(sdk.KindClient))
	defer t.Tracer.FinishSpan(span)

	span.SetTag("db.type", "sql")
	span.SetTag("db.statement", truncateSQL(query, 500))

	return t.DB.QueryRowContext(ctx, query, args...)
}

func (t *TracedDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx, span := t.Tracer.StartSpan(ctx, "sql.Exec", sdk.WithKind(sdk.KindClient))
	defer t.Tracer.FinishSpan(span)

	span.SetTag("db.type", "sql")
	span.SetTag("db.statement", truncateSQL(query, 500))

	result, err := t.DB.ExecContext(ctx, query, args...)
	if err != nil {
		span.SetError(err)
	}
	return result, err
}

func (t *TracedDB) Close() error {
	return t.DB.Close()
}

func truncateSQL(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
