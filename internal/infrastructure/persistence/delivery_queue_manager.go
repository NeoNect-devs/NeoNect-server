package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type sqlDeliveryQueueRepository struct {
	db *sql.DB
}

func NewDeliveryQueueRepository(db *sql.DB) DeliveryQueueRepository {
	return &sqlDeliveryQueueRepository{db: db}
}

func (m *sqlDeliveryQueueRepository) GetLastSequence(ctx context.Context, deviceID string) (int64, error) {
	var currentSequence int64
	err := m.db.QueryRowContext(ctx, Queries.GetLastSequence, deviceID).Scan(&currentSequence)
	return currentSequence, err
}

func (m *sqlDeliveryQueueRepository) Enqueue(ctx context.Context, deviceID string, payload []byte, ttlSeconds int, sequence int64) error {
	currentTime := time.Now().Unix()
	expiryTime := currentTime + int64(ttlSeconds)
	_, err := m.db.ExecContext(ctx, Queries.EnqueueMsg, deviceID, payload, expiryTime, sequence, currentTime)
	return err
}

var ErrDuplicateConflict = errors.New("duplicate message ID with incompatible envelope")
var ErrMailboxQuotaExceeded = errors.New("mailbox quota exceeded")

func (m *sqlDeliveryQueueRepository) EnqueueEnvelope(ctx context.Context, messageID string, senderDeviceID string, recipientDeviceID string, protocolVersion int, payload []byte, ttlSeconds int, sequence int64) error {
	const maxMessages = 1000
	const maxMailboxBytes = 50 * 1024 * 1024 // 50MB
	const maxEnvelopeBytes = 1 * 1024 * 1024 // 1MB

	if len(payload) > maxEnvelopeBytes {
		return errors.New("envelope size exceeds maximum allowed")
	}

	currentTime := time.Now().Unix()
	expiryTime := currentTime + int64(ttlSeconds)

	query := `
	INSERT INTO delivery_queue (message_id, sender_device_id, device_id, protocol_version, payload, expiry, sequence, retry_count, next_retry, ack_deadline)
	SELECT ?, ?, ?, ?, ?, ?, ?, 0, ?, 0
	WHERE (SELECT COUNT(*) FROM delivery_queue WHERE device_id = ?) < ?
	  AND (SELECT IFNULL(SUM(LENGTH(payload)), 0) FROM delivery_queue WHERE device_id = ?) + ? <= ?
	ON CONFLICT(message_id) WHERE message_id IS NOT NULL DO NOTHING
	`

	res, err := m.db.ExecContext(ctx, query,
		messageID, senderDeviceID, recipientDeviceID, protocolVersion, payload, expiryTime, sequence, currentTime,
		recipientDeviceID, maxMessages,
		recipientDeviceID, len(payload), maxMailboxBytes,
	)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		var existingSender, existingRecipient *string
		var existingPayload []byte
		err := m.db.QueryRowContext(ctx, "SELECT sender_device_id, device_id, payload FROM delivery_queue WHERE message_id = ?", messageID).Scan(&existingSender, &existingRecipient, &existingPayload)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrMailboxQuotaExceeded
			}
			return err
		}
		eSender := ""
		if existingSender != nil {
			eSender = *existingSender
		}
		eRecipient := ""
		if existingRecipient != nil {
			eRecipient = *existingRecipient
		}
		if eSender != senderDeviceID || eRecipient != recipientDeviceID || !bytes.Equal(existingPayload, payload) {
			return ErrDuplicateConflict
		}
	}
	return nil
}

func (m *sqlDeliveryQueueRepository) GetQueue(ctx context.Context, deviceID string) ([]DeliveryQueueItem, error) {
	currentTime := time.Now().Unix()
	rows, err := m.db.QueryContext(ctx, Queries.GetQueueByDevice, deviceID, currentTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queuedMessages []DeliveryQueueItem
	for rows.Next() {
		var message DeliveryQueueItem
		var messageExpiry int64
		if err := rows.Scan(&message.ID, &message.Payload, &messageExpiry, &message.Sequence, &message.RetryCount, &message.NextRetry, &message.AckDeadline, &message.MessageID, &message.SenderDeviceID, &message.ProtocolVersion); err != nil {
			return nil, err
		}
		queuedMessages = append(queuedMessages, message)
	}
	return queuedMessages, rows.Err()
}

func (m *sqlDeliveryQueueRepository) UpdateState(ctx context.Context, messageID int64, retryCount int, nextRetry int64, ackDeadline int64) error {
	_, err := m.db.ExecContext(ctx, Queries.UpdateState, retryCount, nextRetry, ackDeadline, messageID)
	return err
}

func (m *sqlDeliveryQueueRepository) ClaimItem(ctx context.Context, messageID int64, nextRetryTimestamp int64, currentNextRetry int64) (bool, error) {
	executionResult, err := m.db.ExecContext(ctx, Queries.ClaimItem, nextRetryTimestamp, messageID, currentNextRetry)
	if err != nil {
		return false, err
	}
	rowsAffectedCount, err := executionResult.RowsAffected()
	return rowsAffectedCount > 0, err
}

func (m *sqlDeliveryQueueRepository) Acknowledge(ctx context.Context, deviceID string, messageID int64) error {
	_, err := m.db.ExecContext(ctx, Queries.DelMsgFromQueue, messageID, deviceID)
	return err
}

func (m *sqlDeliveryQueueRepository) Cleanup(ctx context.Context) error {
	currentTime := time.Now().Unix()
	_, err := m.db.ExecContext(ctx, Queries.CleanupQueue, currentTime)
	return err
}

func (m *sqlDeliveryQueueRepository) GetDueItems(ctx context.Context, currentTime int64) ([]DeliveryQueueItem, error) {
	rows, err := m.db.QueryContext(ctx, Queries.GetDueItems, currentTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dueMessages []DeliveryQueueItem
	for rows.Next() {
		var message DeliveryQueueItem
		if err := rows.Scan(&message.ID, &message.DeviceID, &message.Payload, &message.Sequence, &message.RetryCount, &message.NextRetry, &message.AckDeadline, &message.MessageID, &message.SenderDeviceID, &message.ProtocolVersion); err != nil {
			return nil, err
		}
		dueMessages = append(dueMessages, message)
	}
	return dueMessages, rows.Err()
}

func (m *sqlDeliveryQueueRepository) FanOutMessage(ctx context.Context, deviceIDs []string, payload []byte, ttlSeconds int) error {
	if len(deviceIDs) == 0 {
		return nil
	}
	currentTime := time.Now().Unix()
	expiryTime := currentTime + int64(ttlSeconds)

	var query strings.Builder
	query.WriteString("INSERT INTO delivery_queue (device_id, payload, expiry, sequence, created_at) ")
	var args []interface{}

	for i, deviceID := range deviceIDs {
		if i > 0 {
			query.WriteString(" UNION ALL ")
		}
		query.WriteString("SELECT ?, ?, ?, COALESCE((SELECT sequence FROM delivery_queue WHERE device_id = ? ORDER BY id DESC LIMIT 1), 0) + 1, ?")
		args = append(args, deviceID, payload, expiryTime, deviceID, currentTime)
	}

	_, err := m.db.ExecContext(ctx, query.String(), args...)
	return err
}
