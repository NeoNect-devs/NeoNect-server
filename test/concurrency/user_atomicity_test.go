package concurrency_test

import (
	"NeoNect/test/harness"
	"net/http"
	"testing"
)

func TestCreateUser_Atomicity(t *testing.T) {
	h := harness.Setup(t)

	// 1. Inject a trigger to forcefully fail the user_blocks insert.
	// This simulates a database failure or disk-full scenario exactly on the second insert.
	_, err := h.App.DB.DB().Exec(`
		CREATE TRIGGER force_fail BEFORE INSERT ON user_blocks 
		BEGIN 
			SELECT RAISE(ABORT, 'forced failure') WHERE NEW.block_type = 'AUTH'; 
		END;
	`)
	if err != nil {
		t.Fatalf("Failed to create trigger: %v", err)
	}

	// 2. Attempt to create a user. The second insert (user_blocks) will fail.
	resp, _ := h.PostJSON(t, "/api/v1/users", map[string]string{
		"username": "atomicuser",
		"password": "Password123!",
	}, "")

	if resp.StatusCode == http.StatusCreated {
		t.Errorf("Expected registration to fail due to trigger, got %d", resp.StatusCode)
	}

	// 3. Drop the trigger so subsequent operations work normally
	_, err = h.App.DB.DB().Exec(`DROP TRIGGER force_fail`)
	if err != nil {
		t.Fatalf("Failed to drop trigger: %v", err)
	}

	// 4. Verify the users table was rolled back.
	// We attempt to register the exact same username.
	// If the transaction failed to rollback, the username hash will still exist
	// in the `users` table, and the unique constraint will block this new registration.
	resp2, res2 := h.PostJSON(t, "/api/v1/users", map[string]string{
		"username": "atomicuser",
		"password": "Password123!",
	}, "")

	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("Atomicity violated! Expected second registration to succeed, but got %d: %v", resp2.StatusCode, res2)
	}

	// 5. Verify the new user can successfully authenticate, proving all records are intact.
	resp3, _ := h.PostJSON(t, "/api/v1/auth", map[string]string{
		"username": "atomicuser",
		"password": "Password123!",
	}, "")

	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected authentication to succeed after rollback and recreation, got %d", resp3.StatusCode)
	}
}
