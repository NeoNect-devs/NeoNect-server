package persistence

import (
	"context"
	"database/sql"
)

type sqlIntegrityRepository struct {
	db *sql.DB
}

func NewIntegrityRepository(db *sql.DB) IntegrityRepository {
	return &sqlIntegrityRepository{db: db}
}

func (m *sqlIntegrityRepository) VerifyDatabase(ctx context.Context) bool {
	var validationResult int
	err := m.db.QueryRowContext(ctx, Queries.VerifyDb).Scan(&validationResult)
	return err == nil && validationResult == 1
}
