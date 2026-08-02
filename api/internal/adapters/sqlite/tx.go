package sqlite

import (
	"context"
	"database/sql"
)

type ctxKey struct{}

// WithTx returns a context that makes tx the target of repo queries.
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

func txFrom(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(ctxKey{}).(*sql.Tx)
	return tx
}

// querier is the slice of *sql.DB and *sql.Tx used by repo methods.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// TxManager runs a function inside a transaction. The transaction is carried
// in the context, so every repo call made with that context joins it; a repo
// call made without a transaction context runs against the pool directly.
type TxManager struct {
	db *sql.DB
}

// NewTxManager returns a TxManager bound to db.
func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

// InTx runs fn inside a transaction. A nil return commits; any other return
// rolls back everything fn wrote.
func (m *TxManager) InTx(ctx context.Context, fn func(context.Context) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(WithTx(ctx, tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
