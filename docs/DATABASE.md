# Database & Persistence

NeoNect Server v1 relies exclusively on **SQLite** for asynchronous mailbox storage, routing queues, and metadata tracking.

## 📍 Database Location
By default, the database is generated as `neonect_v1_beta` (or `neonect_v1_production` based on `NEONECT_ENV`) at `./storage/database/` relative to the server's execution root. In a standard production deployment, this evaluates to `/opt/neonect/storage/database/neonect_v1_production`.

## ⚙️ Operational Expectations (WAL Mode)
The server natively executes SQLite PRAGMAs (e.g., `PRAGMA journal_mode=WAL`) upon startup to enhance concurrent read/write throughput.
- **WAL Files**: You will see `-wal` and `-shm` files alongside the primary database. **Never delete these files** manually; doing so corrupts the active database transaction state.
- **Multiple Processes**: Do **NOT** run multiple NeoNect server instances concurrently against the exact same SQLite database file. It will result in `SQLITE_BUSY` locking collisions. Clustering is NOT supported in v1.

## 💾 Safe Backup Procedure
Because WAL mode is active, executing a standard `cp neonect_v1_production backup.db` while the server is running is unsafe.

Instead, execute a live backup using the SQLite CLI:
```bash
sqlite3 /opt/neonect/storage/database/neonect_v1_production "VACUUM INTO 'backup_$(date +%s).db';"
```
This safely locks and streams a consistent snapshot without terminating the NeoNect process.

## 🔒 Permissions
The system user executing the NeoNect binary must have read/write access not just to the database file itself, but to the **directory** containing it (to allow for the creation of WAL and lock files).
