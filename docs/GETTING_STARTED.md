# NeoNect Server — Getting Started

## What This Project Is
NeoNect Server is an encrypted blind relay for 1:1 messaging. It facilitates secure offline message delivery and multi-device syncing. It intentionally does not implement group chat, voice, video channels, or native file upload APIs in this version.

## Prerequisites
- **Go**: Version 1.26.0+ (check `go.mod` for the exact minimum version).
- **Git**: Required to clone and version control the repository.
- A Unix-like operating system (Linux/macOS) is recommended for production.

## 1. Clone the Repository
```bash
git clone https://github.com/NeoNect-devs/NeoNect-server
```

## 2. Enter the Repository
```bash
cd NeoNect-server
```

## 3. Synchronize Dependencies
Before running the application, download and verify the Go modules:
```bash
go mod tidy
```
This command parses your code, adds missing module requirements to `go.mod`, and removes unused ones. The resulting `go.sum` contains cryptographic hashes of specific module versions to ensure reproducible and secure builds. It must always be tracked in Git.

## 4. Configure Environment Variables
NeoNect uses **Environment Variables** for configuration instead of hard-coded values. 
*Note: NeoNect does NOT automatically load a `.env` file natively.* You must pass them through your shell, systemd, or execution wrapper.

A 32-byte bootstrap key is strictly required:
```bash
export NEONECT_BOOTSTRAP_KEY="12345678901234567890123456789012"
```

To bind the server to a fixed local port (instead of the OS-assigned random default):
```bash
export NEONECT_BIND_ADDR="127.0.0.1:8080"
```

## 5. Start the Server
Start the development server with the following command:
```bash
go run cmd/server/main.go
```

## 6. Verify the Server
Once started, the server exposes a health endpoint. Verify it by accessing:
```bash
curl http://127.0.0.1:8080/api/v1/health
```
*(If you did not export NEONECT_BIND_ADDR, check the server stdout to see which random port it bound to).*

## 7. Stop the Server
Press `Ctrl+C` in your terminal to safely stop the server.

## 8. Run Tests
To ensure your local environment is sound, run the complete test suite:
```bash
go test -count=1 ./...
```

## 9. Run Static Analysis
Run the Go static analysis tool to check for structural code errors:
```bash
go vet ./...
```

## 10. SQLite Data
NeoNect uses a local SQLite database for persistence. By default, it is created at `./storage/database/neonect_v1_beta` relative to the server execution path.

## 11. Logs
Server logs are emitted to standard output (stdout) and standard error (stderr). If deploying via systemd, these are managed by `journalctl`.

## 12. Common First-Time Problems
- **Database Locked (`SQLITE_BUSY`)**: Ensure no other instance of the server is running against the same database file.
- **Port already in use**: Another service is bound to the configured HTTP/WebSocket port.
- **Missing/Invalid Bootstrap Key**: The server requires exactly 32 bytes for AES-256 and will exit with `FATAL Vault Initialization Failed: secure vault: invalid key size for AES-256` if the key is missing or incorrectly sized.
