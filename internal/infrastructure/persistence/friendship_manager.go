package persistence

import (
	"context"
	"database/sql"
)

type sqlFriendshipRepository struct {
	db *sql.DB
}

func NewFriendshipRepository(db *sql.DB) FriendshipRepository {
	return &sqlFriendshipRepository{db: db}
}

func (r *sqlFriendshipRepository) CreateFriendship(ctx context.Context, userID1, userID2 int64) error {
	if userID1 > userID2 {
		userID1, userID2 = userID2, userID1
	}

	_, err := r.db.ExecContext(ctx, Queries.CreateFriendship, userID1, userID2)
	return err
}

func (r *sqlFriendshipRepository) CheckFriendship(ctx context.Context, userID1, userID2 int64) (bool, error) {
	if userID1 > userID2 {
		userID1, userID2 = userID2, userID1
	}
	var exists bool
	err := r.db.QueryRowContext(ctx, Queries.CheckFriendship, userID1, userID2).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}

func (r *sqlFriendshipRepository) GetFriendsList(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT u.identity_blob FROM users u JOIN friendships f ON (f.user_id_1 = u.id OR f.user_id_2 = u.id) WHERE (f.user_id_1 = ? OR f.user_id_2 = ?) AND u.id != ?", userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		friends = append(friends, username)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if friends == nil {
		friends = []string{}
	}
	return friends, nil
}

func (r *sqlFriendshipRepository) RemoveFriendship(ctx context.Context, userID1, userID2 int64) error {
	if userID1 > userID2 {
		userID1, userID2 = userID2, userID1
	}
	_, err := r.db.ExecContext(ctx, "DELETE FROM friendships WHERE user_id_1 = ? AND user_id_2 = ?", userID1, userID2)
	return err
}
