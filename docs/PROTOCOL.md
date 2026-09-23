# Protocol

NeoNect Server supports two cryptographic routing protocols. The server operates natively as a blind relay. It never decrypts, inspects, or possesses the payload plaintext.

## Protocol V1 (Legacy Fanout)
V1 handles compatibility fanout targeting.
- **Routing**: Targeted by Username (`to_username`).
- **Delivery**: The server dynamically identifies all registered devices for the recipient and duplicates the ciphertext payload into each device's localized mailbox.
- **Quota Limitations**: V1 operates on bulk insertions (`UNION ALL`). If the payload exceeds the mailbox limits (1,000 items / 50MB per device) for a subset of devices, the server drops delivery strictly for the throttled devices without natively bubbling a 413 error back to the client if at least one delivery succeeded.
- **Idempotency**: Lacks native idempotency controls as it omits `message_id`.

## Protocol V2 (Device-Bound Opaque Envelopes)
V2 guarantees strict, sequence-bound end-to-end device delivery.
- **Routing**: Explicitly targeted by Device ID (`recipient_device_id`).
- **Idempotency**: Clients must provide a globally unique `message_id`. The server respects SQLite `ON CONFLICT` barriers preventing duplicate processing safely.
- **Quotas**: Correctly surfaces `ErrMailboxQuotaExceeded` natively as HTTP 413, giving deterministic feedback.

## Cryptographic Boundary
- The server possesses zero cryptographic identity material beyond public keys.
- Client payloads must be encrypted before transmission.
- `SendRelayMessage` accepts the unparsed, opaque Base64 `ciphertext` and drops it strictly into the persistence layer until explicitly dequeued by the recipient.
