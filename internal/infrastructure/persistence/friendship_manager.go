package persistence

import (
	"context"
	"database/sql"
	"errors"

	sqlite "modernc.org/sqlite"
)

func isUniqueConstraint(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		return code == 1555 || code == 2067
	}
	return false
}

type sqlFriendshipRepository struct {
	db *sql.DB
}

func NewFriendshipRepository(db *sql.DB) FriendshipRepository {
	return &sqlFriendshipRepository{db: db}
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

func (r *sqlFriendshipRepository) CreateFriendRequest(ctx context.Context, senderID, receiverID int64) error {
	u1, u2 := senderID, receiverID
	if u1 > u2 {
		u1, u2 = u2, u1
	}

	res, err := r.db.ExecContext(ctx, Queries.CreateFriendRequest, u1, u2, senderID, u1, u2)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrDuplicateRecord
		}
		return err
	}

	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrDuplicateRecord
	}

	return nil
}

func (r *sqlFriendshipRepository) GetFriendRequestSender(ctx context.Context, userID1, userID2 int64) (int64, error) {
	if userID1 > userID2 {
		userID1, userID2 = userID2, userID1
	}
	var senderID int64
	err := r.db.QueryRowContext(ctx, Queries.GetFriendRequestSender, userID1, userID2).Scan(&senderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrRecordNotFound
		}
		return 0, err
	}
	return senderID, nil
}

func (r *sqlFriendshipRepository) GetIncomingRequests(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, Queries.GetIncomingRequests, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		requests = append(requests, username)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if requests == nil {
		requests = []string{}
	}
	return requests, nil
}

func (r *sqlFriendshipRepository) GetOutgoingRequests(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, Queries.GetOutgoingRequests, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		requests = append(requests, username)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if requests == nil {
		requests = []string{}
	}
	return requests, nil
}

func (r *sqlFriendshipRepository) AcceptFriendRequest(ctx context.Context, senderID, recipientID int64) error {
	u1, u2 := senderID, recipientID
	if u1 > u2 {
		u1, u2 = u2, u1
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, Queries.DeleteFriendRequest, u1, u2, senderID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	_, err = tx.ExecContext(ctx, Queries.CreateFriendship, u1, u2)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrDuplicateRecord
		}
		return err
	}

	return tx.Commit()
}

func (r *sqlFriendshipRepository) DeclineFriendRequest(ctx context.Context, senderID, recipientID int64) error {
	u1, u2 := senderID, recipientID
	if u1 > u2 {
		u1, u2 = u2, u1
	}
	res, err := r.db.ExecContext(ctx, Queries.DeleteFriendRequest, u1, u2, senderID)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (r *sqlFriendshipRepository) CancelFriendRequest(ctx context.Context, senderID, recipientID int64) error {
	u1, u2 := senderID, recipientID
	if u1 > u2 {
		u1, u2 = u2, u1
	}
	res, err := r.db.ExecContext(ctx, Queries.DeleteFriendRequest, u1, u2, senderID)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}
