# Deployment Guide

NeoNect Server runs optimally as a standalone Linux process managed by `systemd`.

*Note: Docker configurations are NOT supported natively in this version.*

## 1. Prerequisites
- **Go 1.26.0+** to compile the binary.
- A dedicated, unprivileged system user (e.g., `neonect`).
- Persistent storage for the SQLite database.

## 2. Build the Server
```bash
go build -o neonect-server cmd/server/main.go
```

## 3. Filesystem Configuration
Create the application and data directories based on the `systemd` unit:
```bash
sudo mkdir -p /opt/neonect/storage
sudo cp neonect-server /opt/neonect/
sudo chown -R neonect:neonect /opt/neonect
```
*The SQLite database will be written to `/opt/neonect/storage/database`. Ensure the `neonect` user has full read/write access.*

## 4. Credentials Configuration
Create a secure credential file for the 32-byte bootstrap key:
```bash
sudo mkdir -p /etc/neonect
echo -n "12345678901234567890123456789012" | sudo tee /etc/neonect/bootstrap.key
sudo chmod 600 /etc/neonect/bootstrap.key
```

## 5. Systemd Service Installation
The actual repository deployment config uses `LoadCredential`. Copy the service file provided in the repository to `/etc/systemd/system/neonect.service` or use the provided layout:
```ini
[Unit]
Description=NeoNect Server
Documentation=https://github.com/neonect/neonect
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=neonect
Group=neonect
WorkingDirectory=/opt/neonect
ExecStart=/opt/neonect/neonect-server
Restart=on-failure
RestartSec=5s

StandardOutput=journal
StandardError=journal

# Configuration
LoadCredential=bootstrap.key:/etc/neonect/bootstrap.key
Environment=NEONECT_BOOTSTRAP_KEY_FILE=%d/bootstrap.key
Environment=NEONECT_DB_DIR=/opt/neonect/storage/database
Environment=NEONECT_KEY_DIR=/opt/neonect/storage/keys
Environment=NEONECT_ENV=production

# Security / Sandboxing
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/opt/neonect/storage

[Install]
WantedBy=multi-user.target
```

## 6. Service Management
**Start**:
```bash
sudo systemctl start neonect
```
**Stop**:
```bash
sudo systemctl stop neonect
```
**Restart**:
```bash
sudo systemctl restart neonect
```
**Status**:
```bash
sudo systemctl status neonect
```

## 7. Logs and Health
- **Logs**: View real-time logs via `journalctl -u neonect -f`.
- **Health Check**: `curl http://127.0.0.1:8080/api/v1/health` (Assuming you configure `NEONECT_BIND_ADDR=127.0.0.1:8080` or look up the port).

## 8. Backups
Because SQLite uses Write-Ahead Logging (WAL), do **NOT** simply copy the database file while the server is running. See [DATABASE.md](../docs/DATABASE.md) for the safe `VACUUM INTO` backup procedure.

## 9. Updates and Rollbacks
To update: Compile the new binary, stop the service, replace `/opt/neonect/neonect-server`, and start the service.
To rollback: Replace the binary with the previously compiled version and restart.
