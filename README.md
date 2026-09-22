# NeoNect Server

NeoNect Server is a stateless API backend for secure messaging relay and public key distribution. It operates with zero knowledge of message payload contents.

The server's primary responsibility is providing an opaque relay model for end-to-end encrypted messaging clients. It manages user authentication, friendship metadata, device registries, and opaque payload routing, without interpreting or decrypting any client messages.

## Architecture

* **HTTP API**: REST-like endpoints for authentication, friendship, device management, and polling.
* **WebSocket**: Real-time delivery stream for connected devices.
* **Authentication**: Token-based bearer authentication and `neonect_sid` secure cookies.
* **Users & Friendship**: Users register with a username and password. Friendships are explicitly established and bi-directional communication is restricted to friends.
* **Devices**: Multi-device support. Each user can register multiple independent devices.
* **Prekeys**: Support for both Curve and Post-Quantum (PQ) prekeys to facilitate client E2EE sessions.
* **Idempotency**: Critical endpoints support `Idempotency-Key` headers.
* **Mailbox / Relay**: Per-device opaque mailbox for offline delivery, protected by message count and byte quotas.
* **SQLite**: Single-file relational persistence layer.

## Security Model

The security boundary of NeoNect is explicitly defined:
* **Authentication & Authorization**: Handled entirely by the server. Sessions are token-based and hashed before persistence.
* **Opaque Message Relay**: The server enforces authorization based on sender/recipient device IDs and friendship status, but **does not decrypt or interpret message payloads**. The server treats encrypted payloads as opaque application data.
* **Prekey Authorization**: Only friends can claim prekeys for a target user.
* **Device Ownership**: Operations on a device (polling, acknowledging messages, establishing WebSocket connections) require active ownership. Revoking a device instantly terminates its sessions and clears its keys.
* **Server Knowledge Restrictions**: The server does not generate, store, or possess client private keys. It does not implement client cryptographic protocols (like the Double Ratchet or X3DH). E2EE must be entirely implemented on the client.

## Setup

### Prerequisites
* Go 1.25.0+
* SQLite3

### Build
```bash
go build -o server ./cmd/server
```

### Run
The server automatically applies necessary database migrations on startup.
```bash
NEONECT_BOOTSTRAP_KEY="<high-entropy-machine-secret>" ./server
```

## Configuration

The server is configured entirely via environment variables.

| Name | Required | Default | Purpose |
|------|----------|---------|---------|
| `NEONECT_BOOTSTRAP_KEY` | **Yes** | - | Cryptographic seed used to verify vault integrity. Must be exactly 32 bytes. |
| `NEONECT_ENV` | No | `beta` | Environment context. Use `production` for strict mode. |
| `NEONECT_DEBUG` | No | `false` | Enable verbose logging. |
| `NEONECT_BIND_ADDR` | No | `0.0.0.0:0` | Interface and port to bind. |
| `NEONECT_DB_DIR` | No | `storage/database` | Directory for SQLite files. |
| `NEONECT_KEY_DIR` | No | `storage/keys` | Directory for vault keys. |
| `NEONECT_MAX_DEVICES` | No | `10` | Maximum allowed devices per user. |
| `NEONECT_ALLOWED_ORIGINS`| No | - | Comma-separated list of CORS origins. |
| `NEONECT_TRUSTED_PROXIES`| No | - | Comma-separated list of trusted proxies for rate-limiting. |
| `NEONECT_READ_TIMEOUT` | No | `15` | Global HTTP read timeout in seconds. |
| `NEONECT_WRITE_TIMEOUT`| No | `15` | Global HTTP write timeout in seconds. |
| `NEONECT_IDLE_TIMEOUT` | No | `60` | Global HTTP idle timeout in seconds. |
| `NEONECT_INTEGRITY_SEED` | No | `integrity_pulse` | Seed string for startup verification. |
| `NEONECT_DB_NAME` | No | `neonect_v1_beta` | Database filename prefix. |
| `NEONECT_MASTER_KEY_FILE`| No | `master.key` | Filename for the encrypted master vault key. |
| `NEONECT_CERT_FILE` | No | `cert.pem` | TLS certificate. |
| `NEONECT_KEY_FILE` | No | `key.pem` | TLS private key. |

## Limits

The server enforces various operational and rate limits:

| Limit | Description |
|-------|-------------|
| **Device Limit** | 10 devices per user (configurable). |
| **Message Quota** | Per-device message limits enforced on the relay. |
| **Body Size** | 4MB global HTTP request body size limit. |
| **WebSocket Frame** | 4MB maximum frame size. |
| **Prekey Batch Size**| Rejects excessively large batches of OPKs. |
| **Username Length** | 3+ characters. |
| **Password Length** | 8+ characters. |

## Error Model

All API errors return a standard JSON structure with an appropriate HTTP status code:
```json
{
  "error": "human readable message"
}
```

Status codes used:
* `200 OK` / `201 Created` / `202 Accepted` - Success
* `400 Bad Request` - Validation failure, missing fields, oversized payloads
* `401 Unauthorized` - Missing or invalid session/device
* `403 Forbidden` - Unauthorized sender (e.g. not a friend)
* `404 Not Found` - Resource (user, device, message) does not exist
* `405 Method Not Allowed` - Unsupported HTTP method
* `409 Conflict` - Resource already exists (duplicate username, device ID, idempotency clash)
* `413 Payload Too Large` - Exceeded message quota
* `422 Unprocessable Entity` - Recipient has no registered devices
* `429 Too Many Requests` - Rate limit exceeded
* `500 Internal Server Error` - Unexpected server failure

## Full HTTP API Reference

### System APIs

#### `GET /api/v1/health`
Check server health.
- **Auth**: None
- **Response 200**:
  ```json
  {"status": "success"}
  ```

#### `GET /api/v1/security/verify`
Verify cryptographic vault integrity.
- **Auth**: None
- **Response 200**:
  ```json
  {"integrity_valid": true}
  ```

#### `GET /api/v1/presence?u=<username>`
Check online presence of a user.
- **Auth**: Required
- **Response 200**:
  ```json
  {"username": "bob", "online": true}
  ```

### Authentication APIs

#### `POST /api/v1/users`
Register a new user.
- **Auth**: None
- **Request**:
  ```json
  {"username": "alice", "password": "securepassword123"}
  ```
- **Response 201**:
  ```json
  {"status": "success", "user_id": 1}
  ```

#### `GET /api/v1/users/availability?u=<username>`
Check if a username is available.
- **Auth**: None
- **Response 200**:
  ```json
  {"available": true}
  ```

#### `POST /api/v1/auth`
Login and create a session.
- **Auth**: None
- **Request**:
  ```json
  {"username": "alice", "password": "securepassword123"}
  ```
- **Response 200**:
  ```json
  {"status": "success", "token": "<session-token>"}
  ```

#### `DELETE /api/v1/auth`
Logout and revoke session.
- **Auth**: Required
- **Response 200**:
  ```json
  {"status": "success"}
  ```

#### `GET /api/v1/users/me`
Get current user profile.
- **Auth**: Required
- **Response 200**:
  ```json
  {"user_id": 1, "username": "alice"}
  ```

### Friendship APIs

Friendship is required to fetch another user's prekeys or send them a message.

#### `GET /api/v1/friends`
List friends.
- **Auth**: Required
- **Response 200**:
  ```json
  {"status": "success", "friends": ["bob", "charlie"]}
  ```

#### `POST /api/v1/friends`
Add a friend.
- **Auth**: Required
- **Request**:
  ```json
  {"username": "bob"}
  ```
- **Response 201**:
  ```json
  {"status": "success"}
  ```

#### `DELETE /api/v1/friends`
Remove a friend.
- **Auth**: Required
- **Request**:
  ```json
  {"username": "bob"}
  ```
- **Response 200**:
  ```json
  {"status": "success"}
  ```

### Device APIs

#### `GET /api/v1/devices`
List own devices.
- **Auth**: Required
- **Response 200**:
  ```json
  {"status": "success", "devices": ["dev-1", "dev-2"]}
  ```

#### `POST /api/v1/device/register`
Register a device and its public key.
- **Auth**: Required
- **Request**:
  ```json
  {"device_id": "dev-1", "public_key": "<base64>"}
  ```
- **Response 201**:
  ```json
  {"status": "success"}
  ```

#### `GET /api/v1/device/key?device_id=<device-id>`
Get the public key for a specific own device.
- **Auth**: Required
- **Response 200**:
  ```json
  {"device_id": "dev-1", "public_key": "<base64>"}
  ```

#### `DELETE /api/v1/device`
Revoke a device.
- **Auth**: Required
- **Request**:
  ```json
  {"device_id": "dev-1"}
  ```
- **Response 200**:
  ```json
  {"status": "success"}
  ```

#### `GET /api/v1/relay/keys?u=<target-username>`
Fetch all active device keys for a friend.
- **Auth**: Required (Must be friends)
- **Response 200**:
  ```json
  {"devices": [{"device_id": "dev-1", "public_key": "<base64>"}]}
  ```

### Prekey APIs

#### `POST /api/v1/keys/upload`
Upload prekeys for a device.
- **Auth**: Required
- **Request**:
  ```json
  {
    "device_id": "dev-1",
    "signed_curve_prekey": {"key_id": 1, "public_key": "<base64>", "signature": "<base64>"},
    "one_time_curve_prekeys": [{"key_id": 1, "public_key": "<base64>"}],
    "signed_pq_prekey": {"key_id": 1, "public_key": "<base64>", "signature": "<base64>"},
    "one_time_pq_prekeys": [{"key_id": 1, "public_key": "<base64>"}]
  }
  ```
- **Response 200**:
  ```json
  {"status": "success"}
  ```

#### `POST /api/v1/keys/claim`
Claim a prekey bundle for a friend's device. Supports `Idempotency-Key` header.
- **Auth**: Required (Must be friends)
- **Request**:
  ```json
  {"target_user": "bob", "target_device": "dev-bob-1"}
  ```
- **Response 200**:
  ```json
  {
    "identity_key": "<base64>",
    "signed_curve_prekey": {"key_id": 1, "public_key": "<base64>", "signature": "<base64>"},
    "one_time_curve_prekey": {"key_id": 1, "public_key": "<base64>"}
  }
  ```

### Messaging APIs

#### `POST /api/v1/relay/send`
Send an opaque ciphertext message to a friend. Supports two protocol versions.
- **Auth**: Required (Must be friends)
- **Request (Protocol Version 2 - Device Bound)**:
  ```json
  {
    "from_device_id": "my-dev",
    "recipient_device_id": "bob-dev",
    "protocol_version": 2,
    "message_id": "msg-123",
    "ciphertext": "<base64>"
  }
  ```
- **Request (Protocol Version 1 - Legacy)**:
  ```json
  {
    "from_device_id": "my-dev",
    "to_username": "bob",
    "protocol_version": 1,
    "ciphertext": "<base64>"
  }
  ```
- **Response 201**:
  ```json
  {"status": "success"}
  ```

#### `GET /api/v1/relay/poll?device_id=<device-id>`
Poll for pending messages for a specific device.
- **Auth**: Required
- **Response 200**:
  ```json
  {
    "messages": [
      {
        "id": 42,
        "message_id": "msg-123",
        "sender_device_id": "alice-dev",
        "protocol_version": 2,
        "ciphertext": "<base64>"
      }
    ]
  }
  ```

#### `POST /api/v1/relay/ack`
Acknowledge receipt of a message, removing it from the server mailbox.
- **Auth**: Required
- **Request**:
  ```json
  {"device_id": "my-dev", "message_id": 42}
  ```
- **Response 200**:
  ```json
  {"status": "success"}
  ```

## WebSocket API

### `GET /api/v1/relay/ws?device_id=<device-id>`
Connect to the real-time message delivery stream for a specific device.
- **Auth**: Required (via `Authorization` header or `neonect_sid` cookie)
- **Behavior**:
  - Requires ownership of the active `device_id`.
  - Replaces any existing WebSocket connection for the same device.
  - Server pushes JSON payload identical to the structure inside the `poll` response when a message arrives.
  - Client must acknowledge messages via the HTTP `ack` endpoint.
  - Revoking the device immediately disconnects its WebSocket.

## Client Integration Flow

1. **Registration/Login**: Client calls `POST /api/v1/users` or `POST /api/v1/auth`. Server returns a token.
2. **Device Registration**: Client generates keys, calls `POST /api/v1/device/register`, and uploads prekeys via `POST /api/v1/keys/upload`.
3. **Friend Operations**: Client calls `POST /api/v1/friends` to add another user.
4. **Device Discovery**: Client calls `GET /api/v1/relay/keys?u=target` to get target's devices.
5. **Key Claiming**: Client calls `POST /api/v1/keys/claim` to claim a prekey for a specific target device to begin an E2EE session.
6. **Message Submission**: Client encrypts the message locally and submits the opaque payload via `POST /api/v1/relay/send`.
7. **Receiving**: Client opens `GET /api/v1/relay/ws` or periodically polls `GET /api/v1/relay/poll`.
8. **Acknowledgement**: Upon successfully receiving and decrypting (or failing to decrypt) the message, the client calls `POST /api/v1/relay/ack`.
9. **Revocation**: If a device is compromised or removed, client calls `DELETE /api/v1/device`. Server immediately invalidates its keys and drops its WebSocket connection.

## Testing

```bash
# Run normal tests
go test -v ./...

# Run race detector tests
go test -v -race ./...
```


## Running the Server

This repository provides a fully self-contained Go server. It manages its own SQLite databases and applies schema initialization automatically on startup.

### 1. Prerequisites

- **Go**: 1.21 or higher (as defined in `go.mod`).
- **CGO**: The project relies on `modernc.org/sqlite`, which is a CGO-free SQLite port. A C compiler is NOT required.

### 2. Clone and Setup

```bash
git clone <repository-url>
cd NeoNect-server-main
```

### 3. Dependencies

Fetch all dependencies:

```bash
go mod tidy
```

### 4. Environment Variables

The server is configured via environment variables.

#### Required Secret
*   **`NEONECT_BOOTSTRAP_KEY`**: (Required) Cryptographic seed used to verify the server's internal vault integrity on startup.
    *   **Constraint**: It **MUST** be exactly 32 bytes long (e.g. 32 ASCII characters).
    *   **Secret**: Yes. Do not share or commit this.

#### Optional Configuration
*   **`NEONECT_ENV`**: Environment context (e.g. `development`, `beta`, `production`). Default: `beta`.
*   **`NEONECT_DEBUG`**: Enable verbose logging (`true` or `false`). Default: `false`.
*   **`NEONECT_BIND_ADDR`**: Network interface and port (e.g. `127.0.0.1:8080`). Default: `0.0.0.0:0` (random port).
*   **`NEONECT_DB_DIR`**: Directory for SQLite files. Default: `storage/database` (created automatically).
*   **`NEONECT_MAX_DEVICES`**: Maximum allowed devices per user. Default: `10`.
*   **`NEONECT_READ_TIMEOUT`**, **`NEONECT_WRITE_TIMEOUT`**, **`NEONECT_IDLE_TIMEOUT`**: HTTP timeouts in seconds.

### 5. Environment Setup Example

For local development, you can export these variables directly in your terminal:

```bash
export NEONECT_BOOTSTRAP_KEY="12345678901234567890123456789012"
export NEONECT_ENV="development"
export NEONECT_BIND_ADDR="127.0.0.1:8080"
```

### 6. Database and Storage

The server uses SQLite. By default, it will automatically create a `storage/database` directory at the project root. On startup, it automatically executes all required `CREATE TABLE IF NOT EXISTS` statements. No external migration tool or pre-existing database directory is required. You will see the database files appear on first boot.

### 7. Build

Build the binary to the root of the repository:

```bash
go build -o server ./cmd/server
```

### 8. Run

Execute the compiled binary:

```bash
./server
```

*(Alternatively, you can run it directly with `go run ./cmd/server`)*

### 9. Verify Startup

If `NEONECT_BIND_ADDR` was set to `127.0.0.1:8080`, verify the server is running by hitting the health endpoint:

```bash
curl http://127.0.0.1:8080/api/v1/health
```

Expected output:
```json
{"status": "success"}
```

### 10. Common Startup Failures

- **Fatal crash on startup (`secure vault: invalid key size for AES-256`)**:
  Your `NEONECT_BOOTSTRAP_KEY` is not exactly 32 bytes. Ensure it is exactly 32 bytes long.
- **Fatal crash on startup (Environment variable not set)**:
  `NEONECT_BOOTSTRAP_KEY` is missing from your environment.
- **Port already in use**:
  Your configured `NEONECT_BIND_ADDR` port is occupied. Change it or kill the blocking process.

---

## Authentication Example for a Generic Client Developer

The environment variables above only configure the physical server. To interact with the messaging APIs, your client application must create user accounts and sessions over HTTP.

Here is a platform-independent flow from a fresh server to sending an authenticated request:

**1. Register a new user account (No auth required)**
```http
POST /api/v1/users
Content-Type: application/json

{"username": "alice", "password": "supersecretpassword"}
```
*(Server returns 201 Created)*

**2. Log in to create a session (No auth required)**
```http
POST /api/v1/auth
Content-Type: application/json

{"username": "alice", "password": "supersecretpassword"}
```
*(Server returns a 200 OK and sets a `neonect_sid` HTTP-only cookie, and returns a JSON payload containing the token)*

**3. Call an authenticated endpoint**
Include the session token (via the `neonect_sid` Cookie or `Authorization: Bearer <token>` header, depending on your client's networking library):
```http
GET /api/v1/users/me
Cookie: neonect_sid=<session-token>
```
*(Server returns 200 OK with your user profile)*

**4. Log out**
```http
DELETE /api/v1/auth
Cookie: neonect_sid=<session-token>
```
*(Server destroys the session and invalidates the token)*
