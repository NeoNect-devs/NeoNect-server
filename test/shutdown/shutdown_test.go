package shutdown_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"NeoNect/test/harness"
)

// TestServer_GracefulShutdown verifies that the server can shutdown gracefully,
// closing active connections and stopping background workers without panicking.
func TestServer_GracefulShutdown(t *testing.T) {
	h := harness.Setup(t)

	// Create some initial load
	h.RegisterUser(t, "shutdown_user", "password123")
	cookie := h.Login(t, "shutdown_user", "password123")
	h.RegisterDevice(t, cookie, "device_active")

	conn := h.DialWS(t, cookie, "device_active")

	// Perform a health check before shutdown
	resp, _ := h.GetJSON(t, "/api/v1/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected health check to be OK before shutdown, got %v", resp.StatusCode)
	}

	// Trigger application shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	h.SafeClose(ctx)

	// Verify that the websocket connection is closed or closing
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Errorf("Expected WebSocket connection to be closed after shutdown")
	}

	conn.Close()
}
