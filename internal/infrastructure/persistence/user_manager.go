package persistence

import (
	"NeoNect/internal/config"
	"context"
	"database/sql"
)

type sqlUserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &sqlUserRepository{db: db}
}

func (m *sqlUserRepository) IsUsernameTaken(ctx context.Context, usernameHash string) (bool, error) {
	var isTaken bool
	err := m.db.QueryRowContext(ctx, Queries.CheckUser, usernameHash).Scan(&isTaken)
	return isTaken, err
}

func (m *sqlUserRepository) CreateUser(ctx context.Context, identityBlob, usernameHash, authenticationKey string) (int64, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insertionResult, err := tx.ExecContext(ctx, Queries.CreateUser, usernameHash, identityBlob)
	if err != nil {
		return 0, err
	}

	newUserID, err := insertionResult.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, Queries.CreateBlock, newUserID, config.BlockTypeAuth, authenticationKey); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return newUserID, nil
}

func (m *sqlUserRepository) GetAuthKey(ctx context.Context, usernameHash string) (string, error) {
	var encodedKey string
	err := m.db.QueryRowContext(ctx, Queries.GetAuth, usernameHash, config.BlockTypeAuth).Scan(&encodedKey)
	if err != nil {
		return "", err
	}
	return encodedKey, nil
}

func (m *sqlUserRepository) GetUserIdByUsername(ctx context.Context, usernameHash string) (int64, error) {
	var userID int64
	err := m.db.QueryRowContext(ctx, Queries.GetUserId, usernameHash).Scan(&userID)
	return userID, err
}

func (m *sqlUserRepository) GetUsernameById(ctx context.Context, userID int64) (string, error) {
	var identityBlob string
	err := m.db.QueryRowContext(ctx, Queries.GetUsername, userID).Scan(&identityBlob)
	return identityBlob, err
}
