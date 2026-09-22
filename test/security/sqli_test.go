package security_test

import (
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestSQLInjection(t *testing.T) {
	h := harness.Setup(t)

	// Try common SQL injection payloads in the registration endpoint
	payloads := []string{
		"' OR 1=1 --",
		"\" OR 1=1 --",
		"admin' --",
		"'; DROP TABLE users; --",
	}

	for _, payload := range payloads {
		t.Run("Payload_"+payload, func(t *testing.T) {
			resp, _ := h.PostJSON(t, "/api/v1/users", map[string]string{
				"username": payload,
				"password": "Password123!",
			}, "")

			// Should either successfully register the weird username safely (escaped),
			// or reject it as invalid format, but definitely NOT cause an internal server error.
			if resp.StatusCode == http.StatusInternalServerError {
				t.Errorf("Potential SQL injection vulnerability with payload %s: received 500", payload)
			}

			// Same for auth
			respAuth, _ := h.PostJSON(t, "/api/v1/auth", map[string]string{
				"username": payload,
				"password": "Password123!",
			}, "")

			if respAuth.StatusCode == http.StatusInternalServerError {
				t.Errorf("Potential SQL injection vulnerability in auth with payload %s: received 500", payload)
			}
		})
	}
}
