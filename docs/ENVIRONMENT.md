# Environment Variables Reference

NeoNect Server determines runtime state predominantly via environment bounds.

## Critical Configuration

### `NEONECT_ENV`
- **Purpose**: Defines operational mode.
- **Values**: `development`, `beta`, `production`
- **Default**: `beta` (Fails fatally if unsupported).

### `NEONECT_DEBUG`
- **Purpose**: Enable verbose logging.
- **Default**: `false` (Always forced false if ENV is beta or production).

### `NEONECT_BOOTSTRAP_KEY`
- **Purpose**: A strictly enforced 32-byte (AES-256) master encryption key used to unlock the `SystemVault`.
- **Sensitivity**: CRITICAL.

### `NEONECT_BOOTSTRAP_KEY_FILE`
- **Purpose**: Explicitly loads the bootstrap key from a filesystem path, taking precedence over `NEONECT_BOOTSTRAP_KEY`.
- **Sensitivity**: CRITICAL.

### `NEONECT_INTEGRITY_SEED`
- **Purpose**: Seed value used for database/vault integrity verification.
- **Default**: `integrity_pulse`

### `NEONECT_MASTER_KEY_FILE`
- **Purpose**: The filename used to store the encrypted SQLite/Vault master key payload.
- **Default**: `master.key`

## Networking & Filesystem Configuration

### `NEONECT_BIND_ADDR`
- **Purpose**: Server listening interface and port.
- **Default**: `0.0.0.0:0` (random port).

### `NEONECT_NODE_ADDR`
- **Purpose**: Public/Advertised Node Address.
- **Optional**: Yes.

### `NEONECT_STORAGE_ROOT`
- **Purpose**: Global base path for storage directories.
- **Default**: `<project_root>/storage`

### `NEONECT_DB_DIR`
- **Purpose**: The directory containing SQLite DB/WAL artifacts.
- **Default**: `storage/database`

### `NEONECT_DB_NAME`
- **Purpose**: The explicit database filename.
- **Default**: `neonect_v1_beta` (in beta), `neonect_v1_production` (in production).

### `NEONECT_KEY_DIR`
- **Purpose**: The directory path for cryptographic artifacts/TLS keys.
- **Default**: `storage/keys`

### `NEONECT_CERT_FILE` / `NEONECT_KEY_FILE`
- **Purpose**: Filenames for TLS certificate and private key.
- **Default**: `cert.pem` / `key.pem`

## HTTP Timeouts

### `NEONECT_READ_TIMEOUT`
- **Purpose**: Read timeout duration in seconds.
- **Default**: `15`

### `NEONECT_WRITE_TIMEOUT`
- **Purpose**: Write timeout duration in seconds.
- **Default**: `15`

### `NEONECT_IDLE_TIMEOUT`
- **Purpose**: Idle connection timeout in seconds.
- **Default**: `60`

## Security & Storage Limits
### `NEONECT_MAX_DEVICES`
- **Purpose**: Max devices allowed to be registered per user account.
- **Default**: `10`

### `NEONECT_ALLOWED_ORIGINS`
- **Purpose**: Comma-separated strict Origin validations for CORS and WebSocket handshake procedures.
- **Example**: `https://app.neonect.io,https://web.neonect.io`

### `NEONECT_TRUSTED_PROXIES`
- **Purpose**: Comma-separated list of IPs allowed to forward `X-Forwarded-For` for rate limiting.

### Rate Limits
- `NEONECT_MAX_CONN_PER_IP`: Max generic requests limits.
- `NEONECT_MAX_REQ_PER_SEC_IP`: Sustained request bounds.
- `NEONECT_MAX_MSG_PER_SEC`: Max messaging API limits.
- `NEONECT_MAX_DISCOVERY_PER_SEC`: Bound for querying external public keys.

## Operational Limits

### `NEONECT_HTTP_MAX_BODY_BYTES`
- **Purpose**: Controls maximum HTTP request body size.
- **Unit**: Bytes
- **Default**: `4194304` (4 MiB)
- **Safe meaning**: Prevent memory exhaustion from large HTTP requests.

### `NEONECT_WS_MAX_MESSAGE_BYTES`
- **Purpose**: Controls maximum WebSocket read message size.
- **Unit**: Bytes
- **Default**: `4194304` (4 MiB)
- **Safe meaning**: Prevent memory exhaustion from large WebSocket messages.

### `NEONECT_MAX_ENVELOPE_BYTES`
- **Purpose**: Controls maximum size of an individual envelope payload.
- **Unit**: Bytes
- **Default**: `1048576` (1 MiB)
- **Safe meaning**: Ensure individual envelopes fit comfortably within limits. Must not exceed ingress limits.

### `NEONECT_MAX_MAILBOX_BYTES`
- **Purpose**: Controls the maximum combined bytes allowed in a device's delivery queue mailbox.
- **Unit**: Bytes
- **Default**: `52428800` (50 MiB)
- **Safe meaning**: Limits total storage per offline device.

### `NEONECT_MAX_MAILBOX_MESSAGES`
- **Purpose**: Controls the maximum number of unread messages stored in a device's delivery queue.
- **Unit**: Count
- **Default**: `1000`
- **Safe meaning**: Bounds sequence exhaustion and general queue length.

### `NEONECT_MAX_GLOBAL_WEBSOCKETS`
- **Purpose**: Controls the system-wide maximum number of concurrent WebSocket connections.
- **Unit**: Count
- **Default**: `10000`
- **Safe meaning**: Protects against file descriptor and goroutine exhaustion.

### `NEONECT_DELIVERY_BATCH_SIZE`
- **Purpose**: Maximum number of delivery queue items to fetch in a single background batch.
- **Unit**: Count
- **Default**: `100`
- **Safe meaning**: Prevents excessive memory use and prolonged database locks during queue polling.

### `NEONECT_DB_MAX_OPEN_CONNS`
- **Purpose**: Maximum number of open connections to the SQLite database.
- **Unit**: Count
- **Default**: `10`
- **Safe meaning**: Ensures predictable concurrency for the WAL-mode database.

### `NEONECT_DB_MAX_IDLE_CONNS`
- **Purpose**: Maximum number of idle connections retained in the database pool.
- **Unit**: Count
- **Default**: `5`
- **Safe meaning**: Recycles connections during low load. Must be <= NEONECT_DB_MAX_OPEN_CONNS.

### `NEONECT_DB_CONN_MAX_LIFETIME_SECONDS`
- **Purpose**: Maximum lifetime of a database connection before being forcefully retired.
- **Unit**: Seconds
- **Default**: `3600` (1 hour)
- **Safe meaning**: Mitigates potential subtle connection state leaks.

### `NEONECT_SQLITE_BUSY_TIMEOUT_MS`
- **Purpose**: Time SQLite will wait for a lock before returning SQLITE_BUSY.
- **Unit**: Milliseconds
- **Default**: `10000` (10 seconds)
- **Safe meaning**: Prevents queries from failing immediately under high concurrency.

### `NEONECT_SQLITE_CACHE_SIZE`
- **Purpose**: Suggested SQLite page cache size.
- **Unit**: Pages (if positive) or Kibibytes (if negative)
- **Default**: `-32000` (approx 32 MiB)
- **Safe meaning**: Controls memory footprint of the SQLite engine.

### `NEONECT_SQLITE_MMAP_SIZE_BYTES`
- **Purpose**: Maximum bytes SQLite is allowed to map via mmap.
- **Unit**: Bytes
- **Default**: `268435456` (256 MiB)
- **Safe meaning**: Accelerates read performance at the expense of virtual memory space.

### `NEONECT_REQUEST_CONTEXT_TIMEOUT_SECONDS`
- **Purpose**: Global timeout enforced on HTTP request contexts.
- **Unit**: Seconds
- **Default**: `30`
- **Safe meaning**: Prevents stalled requests from permanently consuming resources.

### `NEONECT_WS_PING_PERIOD_SECONDS`
- **Purpose**: Frequency of WebSocket ping frames sent to clients.
- **Unit**: Seconds
- **Default**: `30`
- **Safe meaning**: Keeps the connection alive and detects dead peers.

### `NEONECT_WS_PONG_WAIT_SECONDS`
- **Purpose**: Time allowed for a client to reply to a ping before connection termination.
- **Unit**: Seconds
- **Default**: `45`
- **Safe meaning**: Enforces active liveness. Must be strictly greater than NEONECT_WS_PING_PERIOD_SECONDS.

### `NEONECT_MAX_AUTH_PER_SEC_IP`
- **Purpose**: Maximum authentication attempts per second per IP address.
- **Unit**: Count/Second
- **Default**: `5`
- **Safe meaning**: Protects against brute-force attacks on authentication endpoints.
