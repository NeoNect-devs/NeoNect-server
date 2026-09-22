package persistence

import (
	"context"
	"database/sql"
	"errors"
)

type sqlPrekeyRepository struct {
	db *sql.DB
}

func NewPrekeyRepository(db *sql.DB) PrekeyRepository {
	return &sqlPrekeyRepository{db: db}
}

func (r *sqlPrekeyRepository) UploadPrekeys(
	ctx context.Context,
	deviceID string,
	signedCurve *SignedPrekeyRecord,
	oneTimeCurve []OneTimePrekeyRecord,
	signedPQ *SignedPrekeyRecord,
	oneTimePQ []OneTimePrekeyRecord,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if len(oneTimeCurve) > 0 {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM one_time_curve_prekeys WHERE device_id = ?", deviceID).Scan(&count); err != nil {
			return err
		}
		if count+len(oneTimeCurve) > 10000 {
			return errors.New("prekey limit exceeded")
		}
	}

	if len(oneTimePQ) > 0 {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM one_time_pq_prekeys WHERE device_id = ?", deviceID).Scan(&count); err != nil {
			return err
		}
		if count+len(oneTimePQ) > 10000 {
			return errors.New("prekey limit exceeded")
		}
	}

	if signedCurve != nil {
		query := `INSERT INTO signed_curve_prekeys (device_id, key_id, public_key, signature, created_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(device_id) DO UPDATE SET
				key_id = excluded.key_id,
				public_key = excluded.public_key,
				signature = excluded.signature,
				created_at = CURRENT_TIMESTAMP`
		if _, err := tx.ExecContext(ctx, query, deviceID, signedCurve.KeyID, signedCurve.PublicKey, signedCurve.Signature); err != nil {
			return err
		}
	}

	if signedPQ != nil {
		query := `INSERT INTO signed_pq_prekeys (device_id, key_id, public_key, signature, created_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(device_id) DO UPDATE SET
				key_id = excluded.key_id,
				public_key = excluded.public_key,
				signature = excluded.signature,
				created_at = CURRENT_TIMESTAMP`
		if _, err := tx.ExecContext(ctx, query, deviceID, signedPQ.KeyID, signedPQ.PublicKey, signedPQ.Signature); err != nil {
			return err
		}
	}

	for _, p := range oneTimeCurve {
		query := `INSERT INTO one_time_curve_prekeys (device_id, key_id, public_key, status, created_at)
			VALUES (?, ?, ?, 'AVAILABLE', CURRENT_TIMESTAMP)
			ON CONFLICT(device_id, key_id) DO NOTHING`
		if _, err := tx.ExecContext(ctx, query, deviceID, p.KeyID, p.PublicKey); err != nil {
			return err
		}
	}

	for _, p := range oneTimePQ {
		query := `INSERT INTO one_time_pq_prekeys (device_id, key_id, public_key, status, created_at)
			VALUES (?, ?, ?, 'AVAILABLE', CURRENT_TIMESTAMP)
			ON CONFLICT(device_id, key_id) DO NOTHING`
		if _, err := tx.ExecContext(ctx, query, deviceID, p.KeyID, p.PublicKey); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *sqlPrekeyRepository) ClaimPrekeys(ctx context.Context, deviceID string) (*PrekeyBundle, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bundle := &PrekeyBundle{}

	// 1. Get and consume one-time curve prekey
	bundle.OneTimeCurvePrekey = &OneTimePrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, `
		UPDATE one_time_curve_prekeys
		SET status = 'CONSUMED'
		WHERE device_id = ? AND key_id = (
			SELECT key_id
			FROM one_time_curve_prekeys
			WHERE device_id = ? AND status = 'AVAILABLE'
			ORDER BY key_id ASC LIMIT 1
		)
		RETURNING key_id, public_key`, deviceID, deviceID).
		Scan(&bundle.OneTimeCurvePrekey.KeyID, &bundle.OneTimeCurvePrekey.PublicKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.OneTimeCurvePrekey = nil
		} else {
			return nil, err
		}
	}

	// 2. Get and consume one-time PQ prekey
	bundle.OneTimePQPrekey = &OneTimePrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, `
		UPDATE one_time_pq_prekeys
		SET status = 'CONSUMED'
		WHERE device_id = ? AND key_id = (
			SELECT key_id
			FROM one_time_pq_prekeys
			WHERE device_id = ? AND status = 'AVAILABLE'
			ORDER BY key_id ASC LIMIT 1
		)
		RETURNING key_id, public_key`, deviceID, deviceID).
		Scan(&bundle.OneTimePQPrekey.KeyID, &bundle.OneTimePQPrekey.PublicKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.OneTimePQPrekey = nil
		} else {
			return nil, err
		}
	}

	// 3. Get identity key
	err = tx.QueryRowContext(ctx, "SELECT identity_key FROM devices WHERE device_id = ? AND status = 'ACTIVE'", deviceID).Scan(&bundle.IdentityKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("device not found or inactive")
		}
		return nil, err
	}

	// 4. Get signed curve prekey
	bundle.SignedCurvePrekey = &SignedPrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, "SELECT key_id, public_key, signature FROM signed_curve_prekeys WHERE device_id = ?", deviceID).
		Scan(&bundle.SignedCurvePrekey.KeyID, &bundle.SignedCurvePrekey.PublicKey, &bundle.SignedCurvePrekey.Signature)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.SignedCurvePrekey = nil
		} else {
			return nil, err
		}
	}

	// 5. Get signed PQ prekey
	bundle.SignedPQPrekey = &SignedPrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, "SELECT key_id, public_key, signature FROM signed_pq_prekeys WHERE device_id = ?", deviceID).
		Scan(&bundle.SignedPQPrekey.KeyID, &bundle.SignedPQPrekey.PublicKey, &bundle.SignedPQPrekey.Signature)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.SignedPQPrekey = nil
		} else {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (r *sqlPrekeyRepository) ClaimPrekeysIdempotent(
	ctx context.Context,
	userID int64,
	deviceID string,
	idempotencyKey string,
	fingerprint string,
	buildResponse func(*PrekeyBundle) ([]byte, int, error),
) ([]byte, int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	// Check idempotency record
	var existingFingerprint string
	var existingStatus int
	var existingPayload []byte
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint, http_status, response_payload 
		FROM idempotency_records 
		WHERE user_id = ? AND operation = 'claim' AND idempotency_key = ?`,
		userID, idempotencyKey).Scan(&existingFingerprint, &existingStatus, &existingPayload)

	if err == nil {
		if existingFingerprint != fingerprint {
			return nil, 0, errors.New("idempotency conflict: fingerprint mismatch")
		}
		return existingPayload, existingStatus, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, err
	}

	// Try to claim the lock (insert empty record first) to fail fast on concurrent requests
	_, err = tx.ExecContext(ctx, `
		INSERT INTO idempotency_records (user_id, operation, idempotency_key, request_fingerprint, target_device_id) 
		VALUES (?, 'claim', ?, ?, ?)`,
		userID, idempotencyKey, fingerprint, deviceID)
	if err != nil {
		return nil, 0, errors.New("idempotency conflict: request in progress")
	}

	// Now proceed with normal claim
	bundle := &PrekeyBundle{}

	// 1. Get and consume one-time curve prekey
	bundle.OneTimeCurvePrekey = &OneTimePrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, `
		UPDATE one_time_curve_prekeys
		SET status = 'CONSUMED'
		WHERE device_id = ? AND key_id = (
			SELECT key_id
			FROM one_time_curve_prekeys
			WHERE device_id = ? AND status = 'AVAILABLE'
			ORDER BY key_id ASC LIMIT 1
		)
		RETURNING key_id, public_key`, deviceID, deviceID).
		Scan(&bundle.OneTimeCurvePrekey.KeyID, &bundle.OneTimeCurvePrekey.PublicKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.OneTimeCurvePrekey = nil
		} else {
			return nil, 0, err
		}
	}

	// 2. Get and consume one-time PQ prekey
	bundle.OneTimePQPrekey = &OneTimePrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, `
		UPDATE one_time_pq_prekeys
		SET status = 'CONSUMED'
		WHERE device_id = ? AND key_id = (
			SELECT key_id
			FROM one_time_pq_prekeys
			WHERE device_id = ? AND status = 'AVAILABLE'
			ORDER BY key_id ASC LIMIT 1
		)
		RETURNING key_id, public_key`, deviceID, deviceID).
		Scan(&bundle.OneTimePQPrekey.KeyID, &bundle.OneTimePQPrekey.PublicKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.OneTimePQPrekey = nil
		} else {
			return nil, 0, err
		}
	}

	// 3. Get identity key
	err = tx.QueryRowContext(ctx, "SELECT identity_key FROM devices WHERE device_id = ? AND status = 'ACTIVE'", deviceID).Scan(&bundle.IdentityKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, errors.New("device not found or inactive")
		}
		return nil, 0, err
	}

	// 4. Get signed curve prekey
	bundle.SignedCurvePrekey = &SignedPrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, "SELECT key_id, public_key, signature FROM signed_curve_prekeys WHERE device_id = ?", deviceID).
		Scan(&bundle.SignedCurvePrekey.KeyID, &bundle.SignedCurvePrekey.PublicKey, &bundle.SignedCurvePrekey.Signature)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.SignedCurvePrekey = nil
		} else {
			return nil, 0, err
		}
	}

	// 5. Get signed PQ prekey
	bundle.SignedPQPrekey = &SignedPrekeyRecord{DeviceID: deviceID}
	err = tx.QueryRowContext(ctx, "SELECT key_id, public_key, signature FROM signed_pq_prekeys WHERE device_id = ?", deviceID).
		Scan(&bundle.SignedPQPrekey.KeyID, &bundle.SignedPQPrekey.PublicKey, &bundle.SignedPQPrekey.Signature)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bundle.SignedPQPrekey = nil
		} else {
			return nil, 0, err
		}
	}

	// Call the callback to build the response
	payload, status, err := buildResponse(bundle)
	if err != nil {
		return nil, 0, err
	}

	// Update the idempotency record with the actual response
	_, err = tx.ExecContext(ctx, `
		UPDATE idempotency_records 
		SET http_status = ?, response_payload = ? 
		WHERE user_id = ? AND operation = 'claim' AND idempotency_key = ?`,
		status, payload, userID, idempotencyKey)
	if err != nil {
		return nil, 0, err
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}

	return payload, status, nil
}
