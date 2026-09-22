package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"testing"
	"time"
)

func TestShutdown_DatabaseLifecycle(t *testing.T) {
	h := harness.Setup(t)

	// Cause some DB activity
	h.RegisterUser(t, "db_user", "password123")
	cookie := h.Login(t, "db_user", "password123")
	h.RegisterDevice(t, cookie, "device_db")

	h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"recipient": "db_user",
		"payload":   "TEST",
	}, cookie)

	// Close the app
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h.SafeClose(ctx)

	// DB is now closed. Try to use it.
	err := h.App.DB.DB().Ping()
	if err == nil {
		t.Errorf("Expected DB to be closed, but Ping succeeded")
	} else if err.Error() != "sql: database is closed" {
		t.Errorf("Expected 'sql: database is closed', got: %v", err)
	}

	// Make sure no background worker tries to query the DB and panic
	// Wait a bit to ensure background workers are dead
	time.Sleep(500 * time.Millisecond)

	// At this point we just ensure no panics occurred.
}
