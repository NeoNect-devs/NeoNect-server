# Troubleshooting

This document covers actual verified issues you might encounter while deploying or running NeoNect Server.

---

### Symptom: Missing or Invalid Bootstrap Key
**Cause**: The application requires exactly 32 bytes (AES-256) for the `NEONECT_BOOTSTRAP_KEY` or `NEONECT_BOOTSTRAP_KEY_FILE` to start safely.
**Diagnose**: The server exits immediately with a `FATAL Vault Initialization Failed: secure vault: invalid key size for AES-256` message.
**Safe Fix**: Export a strict 32-byte variable (e.g. `export NEONECT_BOOTSTRAP_KEY="12345678901234567890123456789012"`) in your shell or credential file.

### Symptom: Invalid ENV value
**Cause**: Supplying a negative integer or malformed string to a bounded configuration variable (like `NEONECT_HTTP_MAX_BODY_BYTES`).
**Diagnose**: The server fails fast during `GetStrictEnvInt` parsing.
**Safe Fix**: Provide a valid positive integer based on the unit requirements defined in `ENVIRONMENT.md`.

### Symptom: `SQLITE_BUSY` or Database Locks
**Cause**: SQLite allows only one writer at a time. Multiple concurrent server processes might be attempting to access the same database file (`neonect_v1_beta`).
**Diagnose**: Check logs for `database is locked` or `busy timeout` errors.
**Safe Fix**: Ensure only one instance of the NeoNect server is running against the directory. 

### Symptom: Port already in use
**Cause**: The configured HTTP/WebSocket port (e.g., via `NEONECT_BIND_ADDR="127.0.0.1:8080"`) is bound by another application.
**Diagnose**: The server crashes on startup with `bind: address already in use`.
**Safe Fix**: Identify the blocking process using `lsof -i :8080` and terminate it, or change the server's binding port.

### Symptom: Health endpoint unavailable
**Cause**: The server failed to start, the path is wrong, or the port is randomized.
**Diagnose**: `curl http://127.0.0.1:8080/api/v1/health` returns Connection Refused or 404.
**Safe Fix**: Confirm the server's stdout to see which random port it bound to (if `NEONECT_BIND_ADDR` wasn't explicitly exported) and ensure you are using the correct `/api/v1/health` path.

### Symptom: go.sum / dependency mismatch
**Cause**: The `go.mod` file was modified without synchronizing the cryptographic checksums in `go.sum`.
**Diagnose**: `go build` or `go run` fails citing missing hashes or checksum mismatches.
**Safe Fix**: Run `go mod tidy` to clean and regenerate the required dependencies. Do NOT bypass sum checks.
