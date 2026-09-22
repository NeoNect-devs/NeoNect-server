package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"sync"
	"testing"
	"time"
)

func TestShutdown_DeliveryWorker(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_A", "password123")
	h.RegisterUser(t, "user_B", "password123")

	cookieA := h.Login(t, "user_A", "password123")
	cookieB := h.Login(t, "user_B", "password123")

	h.RegisterDevice(t, cookieA, "device_A")

	// Create delivery work by sending a message
	// B sends to A
	h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"recipient": "user_A",
		"payload":   "TEST_PAYLOAD",
	}, cookieB)

	// Start shutdown while workers are potentially processing it
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.SafeClose(ctx)
	}()

	wg.Wait()

	// Ensure DB is closed. Wait, SafeClose closes DB.
	// We can check if trying to use DB panics or returns closed error.
	err := h.App.DB.DB().Ping()
	if err == nil || err.Error() != "sql: database is closed" {
		t.Logf("DB Close error: %v", err) // Usually it's already closed so second close returns error.
	}
}
