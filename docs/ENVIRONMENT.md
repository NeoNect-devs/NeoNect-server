# Environment Configuration

## What is an Environment Variable?
An environment variable is a dynamic value that affects the way running processes behave. NeoNect Server reads limits, endpoints, and credentials from the environment at startup.
*Note: NeoNect does NOT automatically parse `.env` files. You must export them via your shell or deployment system.*

**How to set one (Linux/macOS):**
```bash
export NEONECT_HTTP_MAX_BODY_BYTES=4194304
```

---

## 🔒 Required Secrets

### `NEONECT_BOOTSTRAP_KEY`
- **Required**: YES (Or NEONECT_BOOTSTRAP_KEY_FILE)
- **Type**: String (Exactly 32 bytes required for AES-256)
- **Purpose**: The master encryption key to unlock the server's Vault.

### `NEONECT_BOOTSTRAP_KEY_FILE`
- **Required**: YES (Or NEONECT_BOOTSTRAP_KEY)
- **Type**: Path string
- **Purpose**: A file path containing the 32-byte master encryption key (useful for systemd `LoadCredential`).

---

## 🚦 General Configuration & Limits

### `NEONECT_ENV`
- **Required**: Optional
- **Default**: `beta`
- **Purpose**: Determines the operating environment (`development`, `beta`, `production`).

### `NEONECT_DEBUG`
- **Required**: Optional
- **Default**: `false`
- **Purpose**: Enable debug logging (forced false in beta/production).

### `NEONECT_BIND_ADDR`
- **Required**: Optional
- **Default**: `0.0.0.0:0` (Random port)
- **Purpose**: Binding address and port for the HTTP/WebSocket server.

### `NEONECT_ALLOWED_ORIGINS`
- **Required**: Optional
- **Default**: Empty
- **Purpose**: Comma-separated list of allowed CORS origins.

### `NEONECT_TRUSTED_PROXIES`
- **Required**: Optional
- **Default**: Empty
- **Purpose**: Comma-separated list of trusted upstream proxies.

### `NEONECT_MAX_DEVICES`
- **Required**: Optional
- **Default**: `10`
- **Purpose**: Max bound devices per user.

### `NEONECT_INTEGRITY_SEED`
- **Required**: Optional
- **Default**: `integrity_pulse`
- **Purpose**: Used for database/vault integrity checks.

### `NEONECT_MASTER_KEY_FILE`
- **Required**: Optional
- **Default**: `master.key`
- **Purpose**: Filename to store the encrypted SQLite/Vault master payload.

### `NEONECT_CERT_FILE` & `NEONECT_KEY_FILE`
- **Required**: Optional
- **Default**: `cert.pem`, `key.pem`
- **Purpose**: Paths for TLS certificates (if terminating TLS natively).

---

## 🗄️ Database Paths

### `NEONECT_STORAGE_ROOT`
- **Required**: Optional
- **Default**: `./storage` relative to project root
- **Purpose**: Root directory for keys and databases.

### `NEONECT_DB_DIR`
- **Required**: Optional
- **Default**: `NEONECT_STORAGE_ROOT/database`
- **Purpose**: Location of the SQLite database.

### `NEONECT_KEY_DIR`
- **Required**: Optional
- **Default**: `NEONECT_STORAGE_ROOT/keys`
- **Purpose**: Location of key files.

### `NEONECT_DB_NAME`
- **Required**: Optional
- **Default**: `neonect_v1_beta` (depends on `NEONECT_ENV`)
- **Purpose**: SQLite database filename (without extension).

---

## 🕒 Timeouts & Limits

### `NEONECT_READ_TIMEOUT`, `NEONECT_WRITE_TIMEOUT`
- **Required**: Optional
- **Default**: `15`
- **Unit**: Seconds
- **Purpose**: HTTP server read/write timeouts.

### `NEONECT_IDLE_TIMEOUT`
- **Required**: Optional
- **Default**: `60`
- **Unit**: Seconds
- **Purpose**: HTTP keep-alive timeout.

### `NEONECT_REQUEST_CONTEXT_TIMEOUT_SECONDS`
- **Required**: Optional
- **Default**: `30`
- **Unit**: Seconds
- **Purpose**: Max duration for processing a single HTTP request context.

### `NEONECT_MAX_AUTH_PER_SEC_IP`, `NEONECT_MAX_CONN_PER_IP`, `NEONECT_MAX_REQ_PER_SEC_IP`, `NEONECT_MAX_MSG_PER_SEC`, `NEONECT_MAX_DISCOVERY_PER_SEC`
- **Required**: Optional
- **Purpose**: Operational rate limits for abuse mitigation.

---

## 🌐 HTTP

### `NEONECT_HTTP_MAX_BODY_BYTES`
- **Required**: Optional
- **Default**: 4194304 (4 MiB)
- **Unit**: Bytes
- **Purpose**: Enforces the maximum HTTP request body size globally.

---

## 🔌 WebSocket

### `NEONECT_WS_MAX_MESSAGE_BYTES`
- **Required**: Optional
- **Default**: 4194304 (4 MiB)
- **Unit**: Bytes
- **Purpose**: Maximum read limit for incoming WebSocket frames.

### `NEONECT_WS_PING_PERIOD_SECONDS`
- **Required**: Optional
- **Default**: 30
- **Unit**: Seconds
- **Purpose**: Ping heartbeat interval.

### `NEONECT_WS_PONG_WAIT_SECONDS`
- **Required**: Optional
- **Default**: 45
- **Unit**: Seconds
- **Purpose**: Maximum time to wait for a pong response (Must exceed ping period).

### `NEONECT_MAX_GLOBAL_WEBSOCKETS`
- **Required**: Optional
- **Default**: 10000
- **Unit**: Count
- **Purpose**: Global cap on concurrent WebSocket connections.

---

## 📦 Mailbox / Relay

### `NEONECT_MAX_ENVELOPE_BYTES`
- **Required**: Optional
- **Default**: 1048576 (1 MiB)
- **Unit**: Bytes
- **Purpose**: Maximum size of a stored encrypted relay envelope. This is the absolute limit for transporting arbitrary ciphertext (including chunked file attachments sent via standard messaging).

### `NEONECT_MAX_MAILBOX_BYTES`
- **Required**: Optional
- **Default**: 52428800 (50 MiB)
- **Unit**: Bytes
- **Purpose**: Storage quota allocated per active mailbox.

### `NEONECT_MAX_MAILBOX_MESSAGES`
- **Required**: Optional
- **Default**: 1000
- **Unit**: Count
- **Purpose**: Maximum number of envelopes queued in a mailbox.

### `NEONECT_DELIVERY_BATCH_SIZE`
- **Required**: Optional
- **Default**: 100
- **Unit**: Count
- **Purpose**: Number of messages retrieved atomically during offline fanout.

---

## 🗄️ Database Tuning

### `NEONECT_DB_MAX_OPEN_CONNS` (Default: 10)
### `NEONECT_DB_MAX_IDLE_CONNS` (Default: 5)
### `NEONECT_DB_CONN_MAX_LIFETIME_SECONDS` (Default: 3600)

### `NEONECT_SQLITE_BUSY_TIMEOUT_MS`
- **Required**: Optional
- **Default**: 10000
- **Unit**: Milliseconds
- **Purpose**: Time SQLite waits for a lock before returning `SQLITE_BUSY`.

### `NEONECT_SQLITE_CACHE_SIZE`
- **Required**: Optional
- **Default**: -32000
- **Unit**: Pages (Negative = Kibibytes in SQLite)
- **Purpose**: Memory allocated for SQLite caching.

### `NEONECT_SQLITE_MMAP_SIZE_BYTES`
- **Required**: Optional
- **Default**: 268435456 (256 MiB)
- **Unit**: Bytes
- **Purpose**: Maximum memory-mapped I/O boundary.
