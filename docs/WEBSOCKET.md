# WebSocket Protocol Reference

NeoNect supports real-time envelope delivery over secure WebSockets.

## Endpoint
`GET /api/v1/relay/ws?device_id=<device>`

## Handshake & Authentication
Clients connect and present their credentials via standard HTTP mechanisms (e.g. `session` cookie or headers if the client library permits). The server verifies the session and ensures the `device_id` provided is owned by the authenticated user.

## Origin Protection
The `CheckOrigin` policy demands strict Origin matching.
- **Empty Origins**: Non-browser clients (e.g. mobile/CLI) may send an empty Origin and are permitted.
- **Allowed Origins**: Configured via `NEONECT_ALLOWED_ORIGINS`. Only exact matches pass. If `*` is specified, it strictly looks for a literal `*` rather than dynamically bypassing all checks, securing native origin behavior.

## Rate Limits & Connection Caps
- Global connections are strictly capped by `MaxGlobalWebSockets` (default 10,000) using a channel semaphore (`connSem`).
- Connections exceeding the limit gracefully receive a `503 Service Unavailable`.

## Connection Lifecycle
1. **Heartbeat**: The server issues a Ping frame every `PingPeriod`. Clients must respond with a Pong before `PongWait` expires.
2. **Read Loop**: The server continuously reads frames to detect client disconnects natively.
3. **Write Serialization**: Application payload writes and automated heartbeat Pings are explicitly protected by a mutex lock (`sess.mu.Lock()`) to guarantee concurrency safety within the Gorilla WebSocket bounds.
4. **Disconnection**: On failure, the server breaks the read loop, fires the defer blocks, and permanently removes the session map reference. Reconnection requires a fresh handshake.

## Delivery Protocol
WebSockets push real-time updates as JSON envelopes.
```json
{
  "id": 123,
  "message_id": "...",
  "sender_device_id": "...",
  "protocol_version": 2,
  "ciphertext": "base64..."
}
```
Clients must subsequently call `POST /api/v1/relay/ack` with the local SQLite `id` (e.g., `123`) to explicitly dequeue the message from the mailbox.
