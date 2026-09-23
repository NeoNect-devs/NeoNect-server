# Externally Visible Errors

The server standardizes JSON outputs mapped natively across predictable HTTP status layers safely abstracting explicit backend mechanisms.

## Error Format
All errors return the schema:
```json
{
  "error": "human-readable-string"
}
```

## Status Codes & Explanations

- **`400 Bad Request`**
  - Causes: Malformed JSON, missing mandatory fields (e.g. `recipient_device_id` on protocol V2 limits), prekey upload batch mismatches.
  - *Special Return*: Self-friendship constraints return structurally as bad request bounds.
- **`401 Unauthorized`**
  - Causes: Invalid sessions, unregistered credentials, attempting device modification bounds (`RevokeDevice`) upon external/unowned devices.
- **`403 Forbidden`**
  - Causes: Relaying payloads towards non-friend accounts safely denied without processing.
- **`404 Not Found`**
  - Causes: Target usernames missing, explicit prekey device targeting on invalid devices.
- **`405 Method Not Allowed`**
  - Causes: Issuing incorrect verbs bounds natively by `withMethod` encapsulation logic.
- **`409 Conflict`**
  - Causes: Username registration overlaps natively blocked. Device ID overlaps gracefully triggering native conflict mapping. Prekey idempotency fingerprint mismatches securely block redundant attacks.
- **`413 Payload Too Large`**
  - Causes: Submitting HTTP payloads greater than `GlobalMaxBodySize` (hardcoded to `4 MB` / 4194304 bytes), or hitting the 50 MB mailbox quota (`ErrMailboxQuotaExceeded`) natively preventing storage abuses.
- **`422 Unprocessable Entity`**
  - Causes: V1 relay targets accounts possessing zero attached devices preventing unreachable offline looping.
- **`429 Too Many Requests`**
  - Causes: Exceeding `RateLimiter` boundaries. These apply globally across:
    - IP-based generic request floods.
    - IP-based connection limits.
    - Account-based `SendRelayMessage` frequencies.
    - Account-based `GetRecipientKeys` / `ClaimPrekeys` discovery operations.
  - *Note*: The response relies purely on the JSON `{"error": "too many..."}` response; no `Retry-After` HTTP header is emitted by the server.
- **`503 Service Unavailable`**
  - Causes: Exceeding global `connSem` limits inside WebSocket handshakes smoothly deflecting congestion.
