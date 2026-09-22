package security_test

import (
	"net/http"
	"os"
	"testing"

	"NeoNect/test/harness"
)

func TestTrustedProxy(t *testing.T) {
	os.Setenv("NEONECT_MAX_REQ_PER_SEC_IP", "100")
	h := harness.Setup(t)

	// Simulate requests coming through a proxy by spoofing X-Forwarded-For
	// We want to verify that rate limiting still kicks in, meaning the proxy is either trusted or the real IP is extracted properly.
	var hitRateLimit bool
	for i := 0; i < 150; i++ {
		req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1")
		resp, err := h.Client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			hitRateLimit = true
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	if !hitRateLimit {
		t.Errorf("Expected rate limit (429) when spoofing X-Forwarded-For, but did not hit it")
	}
}
