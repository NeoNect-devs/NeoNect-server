package shutdown_test

import (
	"NeoNect/test/harness"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestHTTP_ResourceLeak(t *testing.T) {
	h := harness.Setup(t)

	// Ensure system is stable
	time.Sleep(200 * time.Millisecond)

	baseline := runtime.NumGoroutine()

	for i := 0; i < 200; i++ {
		req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
		resp, err := h.Client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	current := runtime.NumGoroutine()
	if current > baseline+5 {
		t.Errorf("Potential HTTP connection/goroutine leak. Baseline: %d, Current: %d", baseline, current)
	}
}
