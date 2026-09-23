# NeoNect Server API Reference

Base Prefix: `/api/v1`

## Common Headers
- `Authorization`: Required on most endpoints (`Bearer <token>`). Alternatively, `session` cookie can be used.
- `Idempotency-Key`: Supported on specific endpoints (e.g., `/keys/claim`) to safely retry state-mutating requests.

## Global Limits
- **HTTP Request Body**: Maximum payload size is `4 MB` (4194304 bytes) strictly enforced via `http.MaxBytesReader`. This applies to all JSON payloads globally. Note: This differs from the 50 MB internal offline mailbox quota evaluated at the persistence layer.

## Endpoints

### Authentication & Users
- `POST /users` (Register)
  - Body: `{"username": "...", "password": "..."}`
  - Response (201): `{"status": "success", "user_id": 123}`
- `POST /auth` (Login)
  - Body: `{"username": "...", "password": "..."}`
  - Response (200): `{"status": "success", "token": "..."}` (also sets `session` secure cookie)
- `DELETE /auth` (Logout)
  - Response (200): `{"status": "success"}`
- `GET /users/availability?u={username}` (Check Username)
  - Response (200): `{"available": true|false}`
- `GET /users/me` (Profile)
  - Response (200): `{"user_id": 123, "username": "..."}`

### Devices
- `POST /device/register`
  - Body: `{"device_id": "...", "public_key": "base64..."}`
  - Response (201): `{"status": "success"}`
- `GET /devices`
  - Response (200): `{"status": "success", "devices": ["device1", ...]}`
- `GET /device/key?device_id={id}`
  - Response (200): `{"device_id": "...", "public_key": "..."}`
- `DELETE /device`
  - Body: `{"device_id": "..."}`
  - Response (200): `{"status": "success"}`

### Friends
- `GET /friends`
  - Response (200): `{"status": "success", "friends": ["username1"]}`
- `POST /friends`
  - Body: `{"username": "..."}`
  - Response (201): `{"status": "success"}`
- `DELETE /friends`
  - Body: `{"username": "..."}`
  - Response (200): `{"status": "success"}`

### Messaging & Relay
- `POST /relay/send`
  - Body (v1): `{"from_device_id": "...", "protocol_version": 1, "to_username": "...", "ciphertext": "base64...", "timestamp": 1234567890}` (Note: `timestamp` is optional).
  - Body (v2): `{"from_device_id": "...", "recipient_device_id": "...", "protocol_version": 2, "message_id": "...", "ciphertext": "base64...", "timestamp": 1234567890}` (Note: `timestamp` is optional).
  - Response (201): `{"status": "success"}`
- `GET /relay/poll`
  - Response (200): `{"messages": [{"id": 1, "message_id": "...", "sender_device_id": "...", "protocol_version": 2, "ciphertext": "base64..."}]}` (Note: `message_id`, `sender_device_id`, and `protocol_version` are omitted if missing/null in DB natively).
- `POST /relay/ack`
  - Body: `{"device_id": "...", "message_id": 123}` (Local DB ID)
  - Response (200): `{"status": "success"}`
- `GET /relay/keys?u={username}`
  - Response (200): `{"devices": [{"device_id": "...", "public_key": "base64..."}]}`
- `GET /relay/ws?device_id={id}`
  - **Type**: WebSocket Upgrade Endpoint
  - **Behavior**: Real-time push delivery of messaging envelopes. Detailed protocol behaviors are explicitly defined in [WEBSOCKET.md](WEBSOCKET.md).

### Cryptographic Prekeys
- `POST /keys/upload`
  - Body:
    ```json
    {
      "device_id": "...",
      "signed_curve_prekey": {"key_id": 1, "public_key": "base64...", "signature": "base64..."},
      "one_time_curve_prekeys": [{"key_id": 2, "public_key": "base64..."}],
      "signed_pq_prekey": {"key_id": 1, "public_key": "base64...", "signature": "base64..."},
      "one_time_pq_prekeys": [{"key_id": 2, "public_key": "base64..."}]
    }
    ```
    (Note: `signed_curve_prekey`, `one_time_curve_prekeys`, `signed_pq_prekey`, `one_time_pq_prekeys` are optional/`omitempty`).
  - Response (200): `{"status": "success"}`
- `POST /keys/claim`
  - Body: `{"target_user": "...", "target_device": "..."}`
  - Response (200):
    ```json
    {
      "identity_key": "base64...",
      "signed_curve_prekey": {"key_id": 1, "public_key": "base64...", "signature": "base64..."},
      "signed_pq_prekey": {"key_id": 1, "public_key": "base64...", "signature": "base64..."},
      "one_time_curve_prekey": {"key_id": 2, "public_key": "base64..."},
      "one_time_pq_prekey": {"key_id": 2, "public_key": "base64..."}
    }
    ```
    (Note: prekey fields are optional/`omitempty` if exhausted/missing). Supports `Idempotency-Key` header.

### System
- `GET /health` -> 200 OK
- `GET /security/verify` -> System integrity status
- `GET /presence` -> Realtime presence indicators
