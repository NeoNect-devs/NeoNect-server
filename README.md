# NeoNect Server

NeoNect Server operates as a highly secure, blindly-routing relay node powering the NeoNect ecosystem.

## Project Purpose
NeoNect fundamentally delivers encrypted payloads effortlessly bridging offline clients via asynchronous centralized mailboxes. It establishes a secure trust boundary asserting authentication restrictions, isolating active resources gracefully, mapping limits inherently over HTTP and real-time WebSockets without ever interpreting internal cryptographic plaintext boundaries.

## Architecture & Trust Model
- **Cryptographic Blindness**: Designed fundamentally to act natively without cryptographic identity keys. The server exclusively transports opaque `ciphertext` mappings.
- **Relay Mechanism**: Centralizes robust asynchronous SQLite operations limiting abuse intrinsically over strictly bounded mailbox queues.
- **Authentication**: Protects connections dynamically leveraging cached internal `uid` restrictions actively executing explicit `ValidateDeviceOwnership` configurations natively securing endpoint structures.

## Core Capabilities
- Secure user registration, authentication, and transient session lifecycles.
- Asynchronous Offline Envelopes securely persisted mapping strict `message_id` limits via protocol V2 bounding duplicates natively.
- Robust fanout deliveries routing messages across device lists natively utilizing protocol V1 legacy boundaries.
- Native WebSocket routing seamlessly draining active mailboxes securely into connected client boundaries gracefully utilizing synchronized limits cleanly.

## Documentation
Complete architectural and client-integration manuals structurally reside inherently within the nested `docs/` repository boundary safely mapping execution limits deeply.

- **[Detailed Documentation Index](docs/README.md)**

## Development & Building
```bash
go build -o neonect-server ./cmd/server
```
Review the **[Development Guide](docs/DEVELOPMENT.md)** natively understanding testing operations structurally.

## License

NeoNect is licensed under the Apache License, Version 2.0.

See the `LICENSE` file for the complete license text.
