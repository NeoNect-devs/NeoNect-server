package persistence

import (
	"context"
	"os"
	"testing"
)

func TestDatabase_InitializeAtomicity(t *testing.T) {
	dbPath := "test_atomicity.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Inject failure: Create `devices` table without `user_id` column
	// This will cause `Migration1` to fail when it tries to create an index on `user_id`.
	// Since Migration1 is executed after DatabaseSchema,
	// DatabaseSchema's changes should be rolled back.
	if _, err := db.DB().ExecContext(ctx, "CREATE TABLE devices (id INTEGER PRIMARY KEY);"); err != nil {
		t.Fatalf("Failed to inject failure condition: %v", err)
	}

	// First initialization attempt (should fail due to missing user_id column in our injected table)
	err = db.Initialize(ctx)
	if err == nil {
		t.Fatalf("Expected initialization to fail, but it succeeded")
	}

	// Verify atomicity: if rolled back, 'users' table from DatabaseSchema should NOT exist.
	var count int
	err = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query sqlite_master: %v", err)
	}
	if count > 0 {
		t.Fatalf("Atomicity failure: 'users' table exists, meaning DatabaseSchema was not rolled back")
	}

	// Remove the failure condition (drop the injected table)
	if _, err := db.DB().ExecContext(ctx, "DROP TABLE devices;"); err != nil {
		t.Fatalf("Failed to remove failure condition: %v", err)
	}

	// Retry initialization (should succeed)
	err = db.Initialize(ctx)
	if err != nil {
		t.Fatalf("Expected retry to succeed, got error: %v", err)
	}

	// Verify schema integrity after successful initialization
	tablesToVerify := []string{
		"users", "user_blocks", "delivery_queue", "friendships", // DatabaseSchema
		"devices",                                                                                    // Migration1
		"signed_curve_prekeys", "one_time_curve_prekeys", "signed_pq_prekeys", "one_time_pq_prekeys", // Migration2
		"idempotency_records", // Migration3
	}

	for _, table := range tablesToVerify {
		err = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil || count == 0 {
			t.Errorf("Expected table %s to exist", table)
		}
	}

	// Verify Migration4 (message_id in delivery_queue)
	err = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('delivery_queue') WHERE name='message_id'").Scan(&count)
	if err != nil || count == 0 {
		t.Errorf("Expected column message_id in delivery_queue to exist")
	}
}

func TestDatabase_InitializeIdempotency(t *testing.T) {
	dbPath := "test_idempotency.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Run once
	if err := db.Initialize(ctx); err != nil {
		t.Fatalf("First init failed: %v", err)
	}

	// Run twice (idempotency check)
	if err := db.Initialize(ctx); err != nil {
		t.Fatalf("Second init failed: %v", err)
	}
}
