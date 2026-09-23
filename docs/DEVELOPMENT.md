# Development Guide

## Build & Testing
The system natively isolates standard go operations:

```bash
# Build the application
go build -o neonect-server ./cmd/server

# Run tests
go test ./...

# Run concurrency and race conditions natively
go test -count=1 -race ./...
```

## Adding Routes
1. Navigate to `internal/app/app_routes.go`.
2. Map `app.withMethod` binding the expected HTTP verbs strictly preventing unauthorized variations.
3. Establish validations explicitly against JSON structures natively utilizing `parseJSON` ensuring structural size bounds remain tightly encapsulated.

## Modifying Persistence
Database structures modify linearly across `manager.go`.
- Avoid adding Version tracking logic explicitly. Instead, write structural DDL statements bound safely with `IF NOT EXISTS` constructs natively inside the `tx.ExecContext`.
