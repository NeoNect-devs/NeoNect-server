package websocket_test

import (
	"runtime"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestWebSocket_GoroutineBound(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_churn", "password123")
	cookie := h.Login(t, "user_churn", "password123")
	h.RegisterDevice(t, cookie, "dev_churn")

	time.Sleep(100 * time.Millisecond) // settle
	baseline := runtime.NumGoroutine()

	peak := 0

	for i := 0; i < 50; i++ {
		conn := h.DialWS(t, cookie, "dev_churn")
		time.Sleep(5 * time.Millisecond) // Let goroutines spawn

		current := runtime.NumGoroutine()
		if current > peak {
			peak = current
		}

		conn.Close()
		time.Sleep(5 * time.Millisecond) // Let them die
	}

	time.Sleep(200 * time.Millisecond) // Final settle
	postCleanup := runtime.NumGoroutine()

	t.Logf("Baseline: %d, Peak: %d, Post-Cleanup: %d", baseline, peak, postCleanup)

	// Post-cleanup should be very close to baseline
	if postCleanup > baseline+2 {
		t.Fatalf("Goroutine leak detected! Baseline: %d, Post-Cleanup: %d", baseline, postCleanup)
	}
}
