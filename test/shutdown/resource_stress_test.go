package shutdown_test

import (
	"NeoNect/test/harness"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestShutdown_ResourceStress(t *testing.T) {
	h := harness.Setup(t)

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	baseline := runtime.NumGoroutine()

	h.RegisterUser(t, "stress_user", "password123")
	// 1. Multiple sessions
	for i := 0; i < 10; i++ {

		cookie := h.Login(t, "stress_user", "password123")
		if cookie != "" {
			h.RegisterDevice(t, cookie, "stress_device")
		}
	}
	cookie := h.Login(t, "stress_user", "password123")

	// 2. Many HTTP Requests
	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
		resp, _ := h.Client.Do(req)
		resp.Body.Close()
	}

	// 3. WS connect/disconnect
	for i := 0; i < 10; i++ {
		conn := h.DialWS(t, cookie, "stress_device")
		conn.Close()
	}

	// 4. Relay operations
	for i := 0; i < 50; i++ {
		h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
			"recipient": "stress_user",
			"payload":   "PAYLOAD",
		}, cookie)
	}

	time.Sleep(1 * time.Second)
	runtime.GC()

	current := runtime.NumGoroutine()
	if current > baseline+15 { // allow a bit more leeway for background HTTP connections kept alive by the transport
		t.Errorf("Potential resource leak after stress. Baseline: %d, Current: %d", baseline, current)
	}
}
