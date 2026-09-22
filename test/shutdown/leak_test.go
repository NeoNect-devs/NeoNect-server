package shutdown_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"NeoNect/test/harness"
)

// TestServer_ResourceLeak tests for goroutine leaks after establishing multiple
// connections and performing operations, followed by a shutdown.
func TestServer_ResourceLeak(t *testing.T) {
	// Measure baseline goroutines before test
	baseline := runtime.NumGoroutine()

	func() {
		h := harness.Setup(t)

		h.RegisterUser(t, "leak_user", "password123")
		cookie := h.Login(t, "leak_user", "password123")

		// Create multiple devices and connections
		for i := 0; i < 10; i++ {
			deviceID := "device_" + string(rune('A'+i))
			h.RegisterDevice(t, cookie, deviceID)
			conn := h.DialWS(t, cookie, deviceID)
			defer conn.Close()
		}

		// Allow connections to establish
		time.Sleep(200 * time.Millisecond)

		// Trigger application shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		h.SafeClose(ctx)
	}()

	// Wait for goroutines to clean up
	time.Sleep(500 * time.Millisecond)

	// Force garbage collection
	runtime.GC()

	current := runtime.NumGoroutine()
	// Allow a small threshold for background system goroutines that might have spawned
	if current > baseline+5 {
		t.Errorf("Potential goroutine leak detected. Baseline: %d, Current: %d", baseline, current)
	}
}
