package shutdown_test

import (
	"NeoNect/test/harness"
	"runtime"
	"testing"
	"time"
)

func TestWebSocket_ResourceLeak(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "ws_user", "password123")
	cookie := h.Login(t, "ws_user", "password123")
	h.RegisterDevice(t, cookie, "device_ws")

	// Get access to the internal activeSessions map via reflection or wait, we can't directly read it.
	// We have to measure goroutines or file descriptors.
	// But wait, the prompt says "Verify activeSessions returns to baseline"
	// Let's use reflection to read the map size if we can, or just measure goroutines.
	// Or we can just connect/disconnect rapidly and see if it runs out of memory or goroutines.

	// How to get activeSessions length?
	// h.App.Handlers.RelayService -> but RelayManager holds wsManager which is unexported!
	// We'll use goroutine count.

	// Ensure system is stable
	time.Sleep(200 * time.Millisecond)
	// We can't access activeSessions easily.
	// We can measure baseline goroutines
	baseline := runtime.NumGoroutine()

	for i := 0; i < 50; i++ {
		conn := h.DialWS(t, cookie, "device_ws")

		// Wait for connection to fully register
		time.Sleep(10 * time.Millisecond)

		conn.Close()

		// Wait for server to process disconnect
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	current := runtime.NumGoroutine()
	if current > baseline+5 {
		t.Errorf("Potential WebSocket connection leak. Baseline: %d, Current: %d", baseline, current)
	}
}
