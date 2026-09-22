package persistence

import (
	"NeoNect/internal/config"
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

var Queries = struct {
	CheckUser              string
	GetAuth                string
	CreateUser             string
	CreateBlock            string
	CreateDevice           string
	GetUserId              string
	GetDeviceOwner         string
	HasDeviceForUser       string
	HasActiveDeviceForUser string
	GetDevicePublicKey     string
	GetDevicesByUser       string
	CreateSess             string
	GetBySess              string
	GetBySessWithTime      string
	DelSess                string
	GetUsername            string
	EnqueueMsg             string
	EnqueueEnvelopeMsg     string
	GetQueueByDevice       string
	DelMsgFromQueue        string
	UpdateState            string
	ClaimItem              string
	GetLastSequence        string
	GetDueItems            string
	CleanupQueue           string
	CreateFriendship       string
	CheckFriendship        string
	VerifyDb               string
	PragmaJournal          string
	PragmaSync             string
	PragmaBusy             string
	PragmaCache            string
	PragmaForeignKeys      string
	PragmaMmapSize         string
	PragmaTempStore        string
}{
	CheckUser:              "SELECT EXISTS(SELECT 1 FROM users WHERE username_hash = ?)",
	GetAuth:                "SELECT b.payload FROM user_blocks b JOIN users u ON u.id = b.user_id WHERE u.username_hash = ? AND b.block_type = ?",
	CreateUser:             "INSERT INTO users (username_hash, identity_blob) VALUES (?, ?)",
	CreateBlock:            "INSERT INTO user_blocks (user_id, block_type, payload) VALUES (?, ?, ?)",
	CreateDevice:           "INSERT INTO devices (user_id, device_id, identity_key) SELECT ?, ?, ? WHERE (SELECT COUNT(*) FROM devices WHERE user_id = ?) < ?",
	GetUserId:              "SELECT id FROM users WHERE username_hash = ?",
	GetDeviceOwner:         "SELECT user_id FROM devices WHERE device_id = ?",
	HasDeviceForUser:       "SELECT EXISTS(SELECT 1 FROM devices WHERE device_id = ? AND user_id = ?)",
	HasActiveDeviceForUser: "SELECT EXISTS(SELECT 1 FROM devices WHERE device_id = ? AND user_id = ? AND status = 'ACTIVE')",
	GetDevicePublicKey:     "SELECT identity_key FROM devices WHERE device_id = ? AND status = 'ACTIVE'",
	GetDevicesByUser:       "SELECT device_id FROM devices WHERE user_id = ? AND status = 'ACTIVE'",
	CreateSess:             "INSERT INTO user_blocks (user_id, block_type, sub_block_id, payload) VALUES (?, ?, ?, ?)",
	GetBySess:              "SELECT user_id FROM user_blocks WHERE block_type = ? AND sub_block_id = ?",
	GetBySessWithTime:      "SELECT user_id, strftime('%s', created_at) as ts FROM user_blocks WHERE block_type = ? AND sub_block_id = ?",
	DelSess:                "DELETE FROM user_blocks WHERE block_type = ? AND sub_block_id = ?",
	GetUsername:            "SELECT identity_blob FROM users WHERE id = ?",
	EnqueueMsg:             "INSERT INTO delivery_queue (device_id, payload, expiry, sequence, retry_count, next_retry, ack_deadline) VALUES (?, ?, ?, ?, 0, ?, 0)",
	EnqueueEnvelopeMsg:     "INSERT INTO delivery_queue (message_id, sender_device_id, device_id, protocol_version, payload, expiry, sequence, retry_count, next_retry, ack_deadline) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, 0) ON CONFLICT(message_id) WHERE message_id IS NOT NULL DO NOTHING",
	GetQueueByDevice:       "SELECT id, payload, expiry, sequence, retry_count, next_retry, ack_deadline, message_id, sender_device_id, protocol_version FROM delivery_queue WHERE device_id = ? AND expiry > ? ORDER BY sequence ASC",
	DelMsgFromQueue:        "DELETE FROM delivery_queue WHERE id = ? AND device_id = ?",
	UpdateState:            "UPDATE delivery_queue SET retry_count = ?, next_retry = ?, ack_deadline = ? WHERE id = ?",
	ClaimItem:              "UPDATE delivery_queue SET next_retry = ? WHERE id = ? AND next_retry <= ?",
	GetLastSequence:        "SELECT COALESCE(MAX(sequence), 0) FROM delivery_queue WHERE device_id = ?",
	GetDueItems:            "SELECT id, device_id, payload, sequence, retry_count, next_retry, ack_deadline, message_id, sender_device_id, protocol_version FROM delivery_queue WHERE next_retry <= ? ORDER BY next_retry ASC LIMIT 100",
	CleanupQueue:           "DELETE FROM delivery_queue WHERE expiry <= ?",
	CreateFriendship:       "INSERT INTO friendships (user_id_1, user_id_2) VALUES (?, ?)",
	CheckFriendship:        "SELECT EXISTS(SELECT 1 FROM friendships WHERE user_id_1 = ? AND user_id_2 = ?)",
	VerifyDb:               "SELECT 1",
	PragmaJournal:          "PRAGMA journal_mode = WAL",
	PragmaSync:             "PRAGMA synchronous = NORMAL",
	PragmaBusy:             fmt.Sprintf("PRAGMA busy_timeout = %d", config.SqliteBusyTimeout),
	PragmaCache:            fmt.Sprintf("PRAGMA cache_size = %d", config.SqliteCacheSize),
	PragmaForeignKeys:      "PRAGMA foreign_keys = ON",
	PragmaMmapSize:         fmt.Sprintf("PRAGMA mmap_size = %d", config.SqliteMmapSize),
	PragmaTempStore:        "PRAGMA temp_store = MEMORY",
}

const (
	DatabaseSchema = `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username_hash TEXT UNIQUE NOT NULL,
		identity_blob BLOB NOT NULL
	);
	CREATE TABLE IF NOT EXISTS user_blocks (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id      INTEGER NOT NULL,
		block_type   TEXT NOT NULL,
		sub_block_id TEXT,
		payload      BLOB NOT NULL,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	CREATE TABLE IF NOT EXISTS delivery_queue (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id    TEXT NOT NULL,
		payload      BLOB NOT NULL,
		expiry       INTEGER NOT NULL,
		sequence     INTEGER NOT NULL,
		retry_count  INTEGER DEFAULT 0,
		next_retry   INTEGER DEFAULT 0,
		ack_deadline INTEGER DEFAULT 0,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS friendships (
		user_id_1 INTEGER NOT NULL,
		user_id_2 INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(user_id_1, user_id_2),
		CHECK(user_id_1 < user_id_2),
		FOREIGN KEY(user_id_1) REFERENCES users(id),
		FOREIGN KEY(user_id_2) REFERENCES users(id)
	);
	CREATE INDEX IF NOT EXISTS idx_user_silo_time ON user_blocks(user_id, block_type, sub_block_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_session_lookup ON user_blocks(block_type, sub_block_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_device_unique ON user_blocks(block_type, sub_block_id);
	CREATE INDEX IF NOT EXISTS idx_queue_device ON delivery_queue(device_id);
	CREATE INDEX IF NOT EXISTS idx_queue_retry ON delivery_queue(next_retry);
	CREATE INDEX IF NOT EXISTS idx_queue_ack_deadline ON delivery_queue(ack_deadline);
	CREATE INDEX IF NOT EXISTS idx_queue_expiry ON delivery_queue(expiry);`

	Migration1 = `
	CREATE TABLE IF NOT EXISTS devices (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id    TEXT UNIQUE NOT NULL,
		user_id      INTEGER NOT NULL,
		identity_key BLOB NOT NULL,
		status       TEXT NOT NULL DEFAULT 'ACTIVE',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		revoked_at   DATETIME,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	CREATE INDEX IF NOT EXISTS idx_devices_user ON devices(user_id);
	CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
	INSERT INTO devices (device_id, user_id, identity_key, status, created_at, updated_at)
	SELECT sub_block_id, user_id, payload, 'ACTIVE', created_at, created_at
	FROM user_blocks
	WHERE block_type = 'DEVICE' AND sub_block_id IS NOT NULL
	ON CONFLICT(device_id) DO NOTHING;
	`
	Migration2 = `
	CREATE TABLE IF NOT EXISTS signed_curve_prekeys (
		device_id TEXT NOT NULL,
		key_id INTEGER NOT NULL,
		public_key BLOB NOT NULL,
		signature BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(device_id),
		FOREIGN KEY(device_id) REFERENCES devices(device_id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS one_time_curve_prekeys (
		device_id TEXT NOT NULL,
		key_id INTEGER NOT NULL,
		public_key BLOB NOT NULL,
		status TEXT NOT NULL DEFAULT 'AVAILABLE',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(device_id, key_id),
		FOREIGN KEY(device_id) REFERENCES devices(device_id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS signed_pq_prekeys (
		device_id TEXT NOT NULL,
		key_id INTEGER NOT NULL,
		public_key BLOB NOT NULL,
		signature BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(device_id),
		FOREIGN KEY(device_id) REFERENCES devices(device_id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS one_time_pq_prekeys (
		device_id TEXT NOT NULL,
		key_id INTEGER NOT NULL,
		public_key BLOB NOT NULL,
		status TEXT NOT NULL DEFAULT 'AVAILABLE',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(device_id, key_id),
		FOREIGN KEY(device_id) REFERENCES devices(device_id) ON DELETE CASCADE
	);
	`

	Migration3 = `
	CREATE TABLE IF NOT EXISTS idempotency_records (
		user_id INTEGER NOT NULL,
		operation TEXT NOT NULL,
		idempotency_key TEXT NOT NULL,
		request_fingerprint TEXT NOT NULL,
		target_device_id TEXT,
		http_status INTEGER,
		response_payload BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(user_id, operation, idempotency_key),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	`
	Migration4 = `
	ALTER TABLE delivery_queue ADD COLUMN message_id TEXT;
	ALTER TABLE delivery_queue ADD COLUMN sender_device_id TEXT;
	ALTER TABLE delivery_queue ADD COLUMN protocol_version INTEGER DEFAULT 1;
	CREATE UNIQUE INDEX IF NOT EXISTS idx_delivery_queue_msg ON delivery_queue(message_id) WHERE message_id IS NOT NULL;
	`
)

type Database struct {
	databaseConnection *sql.DB
	databasePath       string
}

func NewDatabase(dbPath string) (*Database, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)",
		dbPath, config.SqliteBusyTimeout)

	db, err := sql.Open(config.DriverSqlite, dsn)
	if err != nil {
		return nil, err
	}

	if err := configureSqlitePragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	return &Database{
		databaseConnection: db,
		databasePath:       dbPath,
	}, nil
}

func configureSqlitePragmas(db *sql.DB) error {
	if _, err := db.Exec(Queries.PragmaJournal); err != nil {
		return err
	}
	if _, err := db.Exec(Queries.PragmaSync); err != nil {
		return err
	}
	if _, err := db.Exec(Queries.PragmaBusy); err != nil {
		return err
	}
	if _, err := db.Exec(Queries.PragmaCache); err != nil {
		return err
	}
	if _, err := db.Exec(Queries.PragmaForeignKeys); err != nil {
		return err
	}
	if _, err := db.Exec(Queries.PragmaMmapSize); err != nil {
		return err
	}
	if _, err := db.Exec(Queries.PragmaTempStore); err != nil {
		return err
	}
	return nil
}

func (m *Database) Initialize(ctx context.Context) error {
	if _, err := m.databaseConnection.ExecContext(ctx, DatabaseSchema); err != nil {
		return err
	}
	if _, err := m.databaseConnection.ExecContext(ctx, Migration1); err != nil {
		return err
	}
	if _, err := m.databaseConnection.ExecContext(ctx, Migration2); err != nil {
		return err
	}
	if _, err := m.databaseConnection.ExecContext(ctx, Migration3); err != nil {
		return err
	}

	for _, stmt := range strings.Split(Migration4, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if strings.HasPrefix(strings.ToUpper(stmt), "ALTER TABLE ") && strings.Contains(strings.ToUpper(stmt), " ADD COLUMN ") {
			parts := strings.Fields(stmt)
			if len(parts) >= 6 {
				table := parts[2]
				col := parts[5]
				var count int
				check := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", table, col)
				if err := m.databaseConnection.QueryRowContext(ctx, check).Scan(&count); err == nil && count > 0 {
					continue
				}
			}
		}

		if _, err := m.databaseConnection.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

func (m *Database) Close() error {
	if m.databaseConnection == nil {
		return nil
	}
	return m.databaseConnection.Close()
}

func (m *Database) DB() *sql.DB {
	return m.databaseConnection
}
