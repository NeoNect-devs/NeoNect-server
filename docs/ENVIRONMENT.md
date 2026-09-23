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
