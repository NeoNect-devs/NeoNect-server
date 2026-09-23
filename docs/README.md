# NeoNect Documentation Directory

Welcome to the internal structure details of the NeoNect Server system.

## Navigation

- [API Reference](API.md): Endpoint URLs, payloads, and protocol bounds.
- [Authentication Model](AUTHENTICATION.md): Internal mappings surrounding tokens, storage bounds, and cache boundaries.
- [WebSocket Protocol](WEBSOCKET.md): Handshake, limitations, and realtime framing mappings securely handling origin limits.
- [Protocol Limits](PROTOCOL.md): Explicit delineations defining v1 legacy limits vs v2 Idempotent device bounds accurately mapped against quotas.
- [Environment Configuration](ENVIRONMENT.md): Absolute variables dictating internal production states securely mapping Vaults.
- [Deployment Information](DEPLOYMENT.md): Service boundaries explicitly isolating configurations via systemd mappings cleanly.
- [Architecture](ARCHITECTURE.md): Component interactions bounding logic effectively into structural paths safely mapping HTTP payloads into isolated offline mailboxes.
- [Security Model](SECURITY.md): Threat boundary isolating the cryptography blindly alongside explicitly verified constraints globally.
- [Database](DATABASE.md): Idempotent transactional schema bounds structurally executing operations isolated gracefully avoiding schema fragmentation.
- [Development Guide](DEVELOPMENT.md): Baseline parameters for operating the system locally safely executing comprehensive limits securely.
- [Externally Visible Errors](ERRORS.md): Defined HTTP response formats natively bubbled cleanly avoiding abstract errors.

## Documentation Scope
This documentation describes exactly what the server implements today. It explicitly documents missing elements (e.g. absent migration tracking) and accurately explains legacy behaviors (v1 partial failures).
