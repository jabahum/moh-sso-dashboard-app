package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// Store is the Observatory query set plus transactions, copied from the host's
// internal/db/sqlc Store.
type Store interface {
	Querier
	ExecTx(ctx context.Context, fn func(q Querier) error) error
}

type SQLStore struct {
	db *sql.DB
	*Queries
}

func NewStore(db *sql.DB) *SQLStore {
	return &SQLStore{
		db:      db,
		Queries: New(db),
	}
}

func (s *SQLStore) DB() *sql.DB {
	return s.db
}

func (s *SQLStore) ExecTx(ctx context.Context, fn func(q Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin tx: %w", err)
	}

	q := New(tx)

	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx rollback error: %v, original error: %w", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}
