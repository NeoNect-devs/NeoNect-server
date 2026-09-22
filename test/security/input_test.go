package security_test

import (
	"NeoNect/test/harness"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPInputAbuse(t *testing.T) {
	h := harness.Setup(t)

	// Empty JSON
	resp, _ := h.PostJSON(t, "/api/v1/auth", map[string]interface{}{}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for empty JSON, got %d", resp.StatusCode)
	}

	// Missing fields
	resp, _ = h.PostJSON(t, "/api/v1/auth", map[string]interface{}{
		"username": "onlyuser",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for missing fields, got %d", resp.StatusCode)
	}

	// Malformed JSON (using manual request since PostJSON marshals valid JSON)
	req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/auth", strings.NewReader("{invalidjson}"))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = h.Client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for malformed JSON, got %d", resp.StatusCode)
	}
}
