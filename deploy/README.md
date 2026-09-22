# NeoNect Production Deployment

This directory contains the systemd service unit for the NeoNect server.

## Production Layout

NeoNect runs as a compiled binary supervised by systemd.

- **Service File Location**: `/etc/systemd/system/neonect.service` (copied from `deploy/systemd/neonect.service`)
- **Binary Location**: `/opt/neonect/neonect-server`
- **Working Directory**: `/opt/neonect/`
- **Service User**: `neonect` (dedicated unprivileged user)
- **Required Directories**:
  - `/opt/neonect/storage/database/` - SQLite database files
  - `/opt/neonect/storage/keys/` - Cryptographic keys
- **Configuration Mechanism**: systemd `LoadCredential` from `/etc/neonect/bootstrap.key`

*Note*: The application currently expects `go.mod` to exist in the project root to resolve storage paths correctly. Therefore, you must also copy `go.mod` into the `/opt/neonect/` working directory.

## Installation

1. **Create the Service User**
   ```bash
   sudo useradd -r -s /bin/false -d /opt/neonect neonect
   ```

2. **Setup Directories**
   ```bash
   sudo mkdir -p /opt/neonect/storage/database /opt/neonect/storage/keys
   sudo mkdir -p /etc/neonect
   ```

3. **Deploy Application Files**
   Compile the binary and copy it along with `go.mod`:
   ```bash
   go build -o neonect-server ./cmd/server
   sudo cp neonect-server go.mod /opt/neonect/
   sudo chown -R neonect:neonect /opt/neonect
   sudo chmod 700 /opt/neonect/storage
   ```

4. **Configure Environment**
   Create `/etc/neonect/bootstrap.key` to provide the production bootstrap secret.
   This file must not be committed and should only be readable by root, as systemd will read it and securely pass it to the service as a credential.
   ```bash
   sudo bash -c 'echo "<32-byte-secret-key>" > /etc/neonect/bootstrap.key'
   sudo chown root:root /etc/neonect/bootstrap.key
   sudo chmod 600 /etc/neonect/bootstrap.key
   ```

5. **Install Systemd Unit**
   ```bash
   sudo cp deploy/systemd/neonect.service /etc/systemd/system/
   ```

## Basic Lifecycle Commands

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable neonect
sudo systemctl start neonect
```

Check the status:
```bash
sudo systemctl status neonect
```

Restart the service:
```bash
sudo systemctl restart neonect
```

Stop the service:
```bash
sudo systemctl stop neonect
```

## Logs

NeoNect output is managed by journald. To follow logs in real-time:
```bash
journalctl -u neonect -f
```

