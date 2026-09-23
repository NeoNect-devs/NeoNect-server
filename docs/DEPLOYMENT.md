# Deployment Guide

NeoNect natively targets standard Linux runtime assumptions utilizing Systemd.

## Building the Binary
```bash
go build -o neonect-server ./cmd/server
```

## Credential Sideloading
Systemd deployments utilize `LoadCredential` semantics safely mapping the bootstrap key via disk rather than environment scopes preventing leaking across `ps` and generic logs.
1. The administrator provisions the key inside `/etc/neonect/bootstrap.key`.
2. Systemd `LoadCredential` routes the secret strictly into the service bounds.
3. The server natively inspects `NEONECT_BOOTSTRAP_KEY_FILE` directly from the bounded path securely.

## Logging
The server exclusively utilizes the internal `AppLogger` pushing operational logs to standard out, intentionally crafted to be consumed seamlessly by `journald`.
- **Secret Disclosure**: Core paths strictly omit password, private key, token, or ciphertext outputs natively preventing log poisoning.

## Storage
- `NEONECT_DB_DIR`: Contains the primary SQLite artifact alongside `db-wal` operations.
- **Isolation Verification**: The server actively processes storage path resolutions utilizing `filepath.EvalSymlinks`, blocking sibling directory traversals accurately failing-closed on misconfigured root paths.

## Graceful Shutdown
The application listens for `SIGTERM` / `SIGINT` routing gracefully down the stack.
- HTTP server halts incoming traffic.
- Websocket handlers terminate heartbeat threads cleanly draining active channels.
- SQLite connections synchronize and close securely preventing corrupted uncommitted boundaries.
