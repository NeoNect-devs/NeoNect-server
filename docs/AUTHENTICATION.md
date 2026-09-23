# Authentication Model

NeoNect Server employs a secure session token-based architecture.

## Trust Boundary
- **Server Knowledge**: Knows the user account, device identifiers, and hashed session tokens.
- **Server Blindness**: Does NOT possess or decrypt device private keys or client identities. Authentication verifies server-side connection trust only, not end-to-end cryptographic identity.

## Lifecycle
1. **Registration**: `POST /api/v1/users` creates the account and securely stores a bcrypt-hashed password.
2. **Login**: `POST /api/v1/auth` verifies credentials and generates a high-entropy session token.
3. **Session Usage**: The server hashes the raw token (`SHA-256`) before committing it to memory and SQLite. The raw token is NEVER persisted or logged.
4. **Token Transmission**: Clients attach the raw token via the `Authorization: Bearer <token>` header or implicitly via the secure `session` HTTP cookie.
5. **Expiration & Eviction**: Sessions automatically enforce `config.SessionDuration` lifetime. The `evictionLoop` gracefully sweeps the database every 5 minutes for expired sessions without blocking requests.

## Devices
Device ownership is fundamentally bound to the authenticated user's session.
- Devices must be explicitly registered via `POST /api/v1/device/register` using an active session.
- Outbound requests (e.g., `SendRelayMessage`, `RevokeDevice`) cryptographically verify that `request.DeviceID` correctly belongs to the authenticated user ID of the requesting session.
