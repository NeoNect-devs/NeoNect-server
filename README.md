# NeoNect Server

NeoNect Server is an encrypted blind relay for 1:1 messaging. It securely routes offline messages, file attachments (within configured envelope limits), and handles multi-device synchronizations without ever inspecting cryptographic plaintexts.

**Version:** 1.0.0

## 🚦 Start Here
If you are new to the repository, please start with our beginner-friendly **[Getting Started Guide](docs/GETTING_STARTED.md)**. It will walk you from cloning the repository to running the local server.

## 📦 Scope of v1.0.0
NeoNect Server v1 intentionally focuses exclusively on:
- 1:1 encrypted messaging.
- Cryptographic blind-relay (the server does not possess user keys).
- Offline delivery via SQLite persistence.
- Multi-device sync architecture.

*Note: Group chats, voice channels, video calls, and native multipart file upload APIs are strictly **NOT** implemented in v1.*

## ⚙️ Quick Prerequisites
- **Go 1.26.0+**
- **Git**
- Unix-like OS recommended

## 🚀 Quick Start (Development)
```bash
git clone https://github.com/NeoNect-devs/NeoNect-server
cd NeoNect-server
go mod tidy
export NEONECT_BOOTSTRAP_KEY="12345678901234567890123456789012"
export NEONECT_BIND_ADDR="127.0.0.1:8080"
go run cmd/server/main.go
```
*Health Check*: `curl http://127.0.0.1:8080/api/v1/health`

*(By default, if NEONECT_BIND_ADDR is not set, the server binds to a random port via `0.0.0.0:0`)*

## 🛠️ Validation
Run the test suite and static analysis:
```bash
go test -count=1 ./...
go vet ./...
```

## 📚 Documentation Map
- **[Full Documentation Index](docs/README.md)**
- **[Configuration & Environments](docs/ENVIRONMENT.md)**
- **[Deployment Guide](deploy/README.md)**
- **[Versioning](docs/VERSIONING.md)**
- **[Troubleshooting](docs/TROUBLESHOOTING.md)**
