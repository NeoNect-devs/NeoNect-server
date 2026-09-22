package security_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestRateLimitBucketIsolation(t *testing.T) {
	// Set strict rate limits for this test so we can easily saturate them
	t.Setenv("NEONECT_MAX_REQ_PER_SEC_IP", "10")
	// No overriding trusted proxies, harness sets 127.0.0.1 which is what we want for loopback proxying
	h := harness.Setup(t)

	client := &http.Client{Timeout: 5 * time.Second} // Use clean client, NO spoofing transport

	// Client A: hits 10 times to max out bucket
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.10")
		resp, _ := client.Do(req)
		resp.Body.Close()
	}

	// 11th request should be rate limited
	reqA, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
	reqA.Header.Set("X-Forwarded-For", "203.0.113.10")
	respA, _ := client.Do(reqA)
	if respA.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Client A bucket was not rate limited! Got %d", respA.StatusCode)
	}
	respA.Body.Close()

	// Client B behind SAME proxy should be completely isolated and succeed!
	reqB, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
	reqB.Header.Set("X-Forwarded-For", "203.0.113.11") // Different IP
	respB, _ := client.Do(reqB)
	if respB.StatusCode != http.StatusOK {
		t.Fatalf("Client B was incorrectly rate limited by Client A's activity! Got %d", respB.StatusCode)
	}
	respB.Body.Close()

	// Client C spoofing: X-Forwarded-For: attacker, 203.0.113.11
	// The rightmost untrusted IP is attacker (if 203.0.113.11 is not trusted).
	// Wait, the logic is right-to-left. 127.0.0.1 is trusted.
	// X-Forwarded-For: "attacker, 203.0.113.11".
	// IP is 203.0.113.11! Which is NOT in trusted proxies.
	// So 203.0.113.11 is selected. And it has 1 request in its bucket (from Client B test above).
	reqC, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
	reqC.Header.Set("X-Forwarded-For", "attacker, 203.0.113.11")
	respC, _ := client.Do(reqC)
	if respC.StatusCode != http.StatusOK {
		t.Fatalf("Spoofed multi-hop was incorrectly parsed. Got %d", respC.StatusCode)
	}
	respC.Body.Close()
}

func TestProxyGetClientIP_EdgeCases(t *testing.T) {
	// We only need 1 request per edge case, but we need to verify the IP extraction.
	// Since we can't easily see the extracted IP directly, we use rate limiting.
	t.Setenv("NEONECT_MAX_REQ_PER_SEC_IP", "1")

	h := harness.Setup(t)
	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		name       string
		headers    map[string]string
		ipToLock   string            // The logical IP that should be locked out
		testHeader map[string]string // The header used for the follow-up request to verify lockout
	}{
		{
			name:       "IPv6 parsing",
			headers:    map[string]string{"X-Forwarded-For": "2001:db8::1"},
			ipToLock:   "2001:db8::1",
			testHeader: map[string]string{"X-Forwarded-For": "2001:db8::1"},
		},
		{
			name:       "Whitespace in XFF",
			headers:    map[string]string{"X-Forwarded-For": "  10.0.0.1  , 2001:db8::2 "},
			ipToLock:   "2001:db8::2", // Right-most untrusted
			testHeader: map[string]string{"X-Forwarded-For": "2001:db8::2"},
		},
		{
			name:       "Empty entries in XFF",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.2,, , 10.0.0.3"},
			ipToLock:   "10.0.0.3",
			testHeader: map[string]string{"X-Forwarded-For": "10.0.0.3"},
		},
		{
			name:    "Malformed IP fallback",
			headers: map[string]string{"X-Forwarded-For": "not-an-ip"},
			// Wait, does isTrustedProxy reject "not-an-ip" and thus select it as the client IP?
			ipToLock:   "not-an-ip",
			testHeader: map[string]string{"X-Forwarded-For": "not-an-ip"},
		},
		{
			name:       "Huge header",
			headers:    map[string]string{"X-Forwarded-For": strings.Repeat("10.0.0.1,", 1000) + "10.0.0.9"},
			ipToLock:   "10.0.0.9",
			testHeader: map[string]string{"X-Forwarded-For": "10.0.0.9"},
		},
		{
			name:       "X-Real-IP fallback",
			headers:    map[string]string{"X-Real-IP": "10.0.0.4"},
			ipToLock:   "10.0.0.4",
			testHeader: map[string]string{"X-Real-IP": "10.0.0.4"},
		},
		{
			name:       "X-Forwarded-For overrides X-Real-IP",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.5", "X-Real-IP": "10.0.0.6"},
			ipToLock:   "10.0.0.5",
			testHeader: map[string]string{"X-Forwarded-For": "10.0.0.5"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// First request consumes the 1 limit
			req1, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
			for k, v := range tc.headers {
				req1.Header.Set(k, v)
			}
			resp1, _ := client.Do(req1)
			resp1.Body.Close()

			// Second request with same identity should be rate limited!
			req2, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
			for k, v := range tc.testHeader {
				req2.Header.Set(k, v)
			}
			resp2, _ := client.Do(req2)
			if resp2.StatusCode != http.StatusTooManyRequests {
				t.Fatalf("Expected IP to be rate limited, got %d", resp2.StatusCode)
			}
			resp2.Body.Close()
		})
	}
}

func TestUntrustedProxy(t *testing.T) {
	// If the server does NOT trust the proxy, X-Forwarded-For is IGNORED.
	// Here, we simulate a test where NEONECT_TRUSTED_PROXIES is EMPTY!
	t.Setenv("NEONECT_MAX_REQ_PER_SEC_IP", "1")

	t.Setenv("NEONECT_TRUSTED_PROXIES", "")

	h := harness.Setup(t)
	client := &http.Client{Timeout: 5 * time.Second}

	// Request 1: hits the limit for 127.0.0.1
	req1, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	resp1, _ := client.Do(req1)
	resp1.Body.Close()

	// Request 2: tries to "spoof" a new IP. But proxy is untrusted!
	// It should be rate limited because 127.0.0.1 is already at limit.
	req2, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
	req2.Header.Set("X-Forwarded-For", "203.0.113.2")
	resp2, _ := client.Do(req2)
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Untrusted proxy was allowed to spoof IP and evade limit! Got %d", resp2.StatusCode)
	}
	resp2.Body.Close()
}
