# Security Model

NeoNect establishes a rigorous boundary explicitly separating client cryptography from server persistence.

## Implemented Security Controls

- **Cryptographic Blindness**: The server acts strictly as an envelope relay. Ciphertext payloads are entirely opaque. No identity keys or device private material cross into server memory unencrypted.
- **Strict Device Authorization**: Active endpoints mandate cryptographically validated Device Ownership checks natively mapping the inbound `from_device_id` precisely against the authenticated session UID.
- **Session Protections**:
  - Employs strict SHA-256 transformations immediately against incoming raw tokens, permanently storing exclusively hashed configurations in memory cache and the SQLite backend.
  - Actively polls via `evictionLoop` sweeping expired sessions transparently.
- **Horizontal Escaping**: Protected via `CheckFriendship` checks preventing unbounded targeting bounds between unregistered networks natively.
- **Denial of Service Limits**:
  - Connections natively restricted via `MaxGlobalWebSockets` bounds.
  - Payloads automatically restricted universally via `http.MaxBytesReader`.
  - Rate limiting strictly bounds TimeWindow states dropping entirely idle IP maps preventing dictionary attacks against RAM bounds.
  - Mailboxes evaluate strict count and bytes limits natively inside explicit database execution preventing memory inflation entirely.

## Design Intentions
- Vault engines lock all resting server artifacts utilizing AES-256 GCM authenticated cryptography.
- System boundaries omit memory-intensive KDFs explicitly favoring securely injected hardware 32-byte bootstrap keys structurally preventing offline password derivation attacks entirely.

## Out Of Scope
- Message indexing or inspection.
- Migration Version Tables (relies on pure SQLite DDL idempotency via pragma lookups rather than arbitrary version history numbers).
