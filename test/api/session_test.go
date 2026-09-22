package api_test

import (
	"NeoNect/test/harness"
	"net/http"
	"testing"
)

func TestSessionFlow(t *testing.T) {
	h := harness.Setup(t)
	h.RegisterUser(t, "sessionuser", "Password123")

	// Valid session
	cookie := h.Login(t, "sessionuser", "Password123")

	// Health check
	resp, _ := h.GetJSON(t, "/api/v1/users/me", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for valid session")
	}

	// Malformed Bearer
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.struct")
	resp, _ = h.Client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for malformed Bearer, got %d", resp.StatusCode)
	}

	// Invalid Token
	resp, _ = h.GetJSON(t, "/api/v1/users/me", "invalid_cookie_string")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for invalid cookie, got %d", resp.StatusCode)
	}
}
