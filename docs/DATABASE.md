# Database Model

NeoNect exclusively uses SQLite wrapped over `modernc.org/sqlite` avoiding native CGO dependencies completely.

## Initialization & Migrations
- Schema execution happens during `manager.go` `Initialize()` securely encapsulated inside `BeginTx(ctx, nil)`.
- Migration arrays run explicitly through `defer tx.Rollback()`. Failed operations instantly rollback the entirety of the incomplete transaction, structurally preventing partial schema corruption natively.
- **Migration Tracking**: Version tracking tables are intentionally NOT implemented. Initialization utilizes `IF NOT EXISTS` constructs enforcing natively idempotent upgrades.

## Core Collections
The schema comprises 10 tables:

1. **`users`**
   - **Purpose**: Core identity structure.
   - **Columns**: `id` (INTEGER PRIMARY KEY), `username_hash` (TEXT UNIQUE NOT NULL), `identity_blob` (BLOB NOT NULL).
2. **`user_blocks`**
   - **Purpose**: Stores generic user metadata and sessions. Active tokens are stored here where `block_type = 'SESSION'`.
   - **Columns**: `id` (INTEGER PRIMARY KEY), `user_id` (INTEGER, FK to users), `block_type` (TEXT), `sub_block_id` (TEXT), `payload` (BLOB), `created_at` (DATETIME).
   - **Important**: There is no standalone `sessions` table.
3. **`delivery_queue`**
   - **Purpose**: The core Message mailbox isolating offline delivery queues bounded by device endpoints enforcing limits.
   - **Columns**: `id`, `device_id` (TEXT), `payload` (BLOB), `expiry` (INTEGER), `sequence`, `retry_count`, `next_retry`, `ack_deadline`, `created_at`.
   - **Indexes**: `idx_queue_retry`, `idx_queue_ack_deadline`, `idx_queue_expiry`.
4. **`friendships`**
   - **Purpose**: Bidirectional friendship mappings preventing cross-network spam.
   - **Columns**: `user_id_1`, `user_id_2`, `created_at`.
   - **Constraints**: PRIMARY KEY(`user_id_1`, `user_id_2`), CHECK(`user_id_1 < user_id_2`).
5. **`devices`**
   - **Purpose**: Tracks active identifiers routing public key relationships securely back against `users`.
   - **Columns**: `id`, `device_id` (TEXT UNIQUE), `user_id` (INTEGER, FK to users), `identity_key` (BLOB), `status` (TEXT DEFAULT 'ACTIVE').
6. **`signed_curve_prekeys`**
   - **Columns**: `device_id` (PK, FK to devices ON DELETE CASCADE), `key_id`, `public_key`, `signature`, `created_at`.
7. **`one_time_curve_prekeys`**
   - **Columns**: `device_id` (FK to devices), `key_id`, `public_key`, `status`, `created_at`. PRIMARY KEY(`device_id`, `key_id`).
8. **`signed_pq_prekeys`**
   - **Columns**: `device_id` (PK, FK to devices ON DELETE CASCADE), `key_id`, `public_key`, `signature`, `created_at`.
9. **`one_time_pq_prekeys`**
   - **Columns**: `device_id` (FK to devices), `key_id`, `public_key`, `status`, `created_at`. PRIMARY KEY(`device_id`, `key_id`).
10. **`idempotency_records`**
    - **Purpose**: Guarantees idempotent operations (like prekey claims).
    - **Columns**: `user_id` (INTEGER), `operation` (TEXT), `idempotency_key` (TEXT), `request_fingerprint` (TEXT), `target_device_id` (TEXT), `response_payload` (BLOB), `response_status` (INTEGER), `created_at` (DATETIME). PRIMARY KEY(`user_id`, `operation`, `idempotency_key`).

## Concurrency
- Configured persistently utilizing `PRAGMA journal_mode=WAL` isolating reads simultaneously parallel against targeted writes securely bounded.
