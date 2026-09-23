# Client Integration Guide

This guide walks through the exact lifecycle necessary to securely interface with NeoNect Server.

## 1. Setup & Authentication
1. **Registration**: Execute `POST /api/v1/users` supplying `username` and `password`. The server responds with `user_id`.
2. **Authentication**: Execute `POST /api/v1/auth` supplying credentials. The server responds with an opaque token.
3. **Session Maintenance**: Inject the token natively via the `Authorization: Bearer <token>` header across subsequent requests, or explicitly utilize the returned `session` cookie.

## 2. Device Management
1. **Device Creation**: Clients must generate a local cryptographic identity (private/public key pairs) natively outside server boundaries.
2. **Device Registration**: Execute `POST /api/v1/device/register` supplying an arbitrary client-generated `device_id` and the identity `public_key` in Base64 natively.
3. **Public Prekeys**: Submit bundles utilizing `POST /api/v1/keys/upload`.

## 3. Communication Setup
1. **Friendship Setup**: `POST /api/v1/friends` natively linking mutual networks preventing cross-network spam explicitly returning 201 Created.
2. **Obtaining Keys**: Run `POST /api/v1/keys/claim` requesting specific target properties. The endpoint natively accepts `Idempotency-Key` headers safely bounding duplicated state failures natively returning 200 OK structures.

## 4. Messaging
1. **Cryptographic Operations**: **(CLIENT RESPONSIBILITY)** Encrypt payloads securely utilizing the retrieved identity prekeys natively outside server memory bounds. The server strictly treats data opaque.
2. **Sending (Protocol V2)**:
   - Construct a payload bounding `protocol_version: 2`, `recipient_device_id`, and a globally unique `message_id`.
   - Execute `POST /api/v1/relay/send`. The server explicitly verifies quotas natively rejecting operations via `413 Payload Too Large` when limits exceed securely.
3. **Sending (Protocol V1 Legacy)**:
   - Construct a payload bounding `protocol_version: 1`, targeting `to_username`. The server gracefully duplicates payloads natively targeting all attached devices via `FanOutMessage`.

## 5. Message Receipt
1. **Offline Retrieval**: Execute `GET /api/v1/relay/poll`. The server responds smoothly with active structures isolated gracefully within database operations natively.
2. **Realtime WebSocket**: Connect utilizing `GET /api/v1/relay/ws?device_id={id}` securely mapping standard `Authorization` headers. The server pushes limits safely mapping real-time payloads transparently.
3. **Acknowledgement**: Explicitly call `POST /api/v1/relay/ack` identifying the `message_id` bounding the integer explicitly removing offline data cleanly avoiding duplicated fetches seamlessly.

## 6. Limits & Constraints
- **Mailboxes**: 1,000 Messages OR 50MB per targeted device natively limiting structural inflation.
- **HTTP Payload Size**: 4 MB maximum per request body natively restricted by `http.MaxBytesReader` globally.
- **WebSocket Bounds**: Natively capped securely minimizing concurrent operations aggressively cleanly.
- **Data Boundaries**: Never transmit raw private keys smoothly maintaining plaintext structural isolation definitively offline exclusively.
