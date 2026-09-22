package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type sqlDeviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) DeviceRepository {
	return &sqlDeviceRepository{db: db}
}

func (m *sqlDeviceRepository) RegisterDevice(ctx context.Context, userID int64, deviceID string, publicKey []byte, maxDevices int) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, Queries.CreateDevice, userID, deviceID, publicKey, userID, maxDevices)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.New("conflict")
		}
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("maximum device limit reached")
	}

	return tx.Commit()
}

func (m *sqlDeviceRepository) GetDeviceOwner(ctx context.Context, deviceID string) (int64, error) {
	var userID int64
	err := m.db.QueryRowContext(ctx, Queries.GetDeviceOwner, deviceID).Scan(&userID)
	return userID, err
}

func (m *sqlDeviceRepository) IsDeviceOwnedByUser(ctx context.Context, userID int64, deviceID string) (bool, error) {
	var isOwned bool
	err := m.db.QueryRowContext(ctx, Queries.HasDeviceForUser, deviceID, userID).Scan(&isOwned)
	return isOwned, err
}

func (m *sqlDeviceRepository) IsActiveDeviceOwnedByUser(ctx context.Context, userID int64, deviceID string) (bool, error) {
	var isOwned bool
	err := m.db.QueryRowContext(ctx, Queries.HasActiveDeviceForUser, deviceID, userID).Scan(&isOwned)
	return isOwned, err
}

func (m *sqlDeviceRepository) GetDevicePublicKey(ctx context.Context, deviceID string) ([]byte, error) {
	var protectedKey []byte
	err := m.db.QueryRowContext(ctx, Queries.GetDevicePublicKey, deviceID).Scan(&protectedKey)
	if err != nil {
		return nil, err
	}
	return protectedKey, nil
}

func (m *sqlDeviceRepository) GetDevicesByUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, Queries.GetDevicesByUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userDevices []string
	for rows.Next() {
		var currentDeviceID string
		if err := rows.Scan(&currentDeviceID); err != nil {
			return nil, err
		}
		userDevices = append(userDevices, currentDeviceID)
	}
	return userDevices, rows.Err()
}

func (m *sqlDeviceRepository) GetDevicesAndKeysByUser(ctx context.Context, userID int64) ([]DeviceKey, error) {
	query := "SELECT device_id, identity_key FROM devices WHERE user_id = ? AND status = 'ACTIVE'"
	rows, err := m.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userDevices []DeviceKey
	for rows.Next() {
		var currentDeviceID string
		var protectedKey []byte
		if err := rows.Scan(&currentDeviceID, &protectedKey); err != nil {
			return nil, err
		}
		userDevices = append(userDevices, DeviceKey{
			DeviceID:  currentDeviceID,
			PublicKey: protectedKey,
		})
	}
	return userDevices, rows.Err()
}

func (m *sqlDeviceRepository) DeleteDevice(ctx context.Context, userID int64, deviceID string) error {
	query := "UPDATE devices SET status = 'REVOKED', revoked_at = CURRENT_TIMESTAMP WHERE user_id = ? AND device_id = ? AND status = 'ACTIVE'"
	_, err := m.db.ExecContext(ctx, query, userID, deviceID)
	return err
}
