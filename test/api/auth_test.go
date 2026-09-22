package api_test

import (
	"NeoNect/test/harness"
	"net/http"
	"testing"
)

func TestAuthenticationFlow(t *testing.T) {
	h := harness.Setup(t)

	// Valid Registration
	resp, _ := h.PostJSON(t, "/api/v1/users", map[string]string{
		"username": "testuser_auth",
		"password": "ValidPassword123",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d", resp.StatusCode)
	}

	// Duplicate Registration
	resp, _ = h.PostJSON(t, "/api/v1/users", map[string]string{
		"username": "testuser_auth",
		"password": "ValidPassword123",
	}, "")
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("Expected 409 Conflict, got %d", resp.StatusCode)
	}

	// Valid Login
	resp, res := h.PostJSON(t, "/api/v1/auth", map[string]string{
		"username": "testuser_auth",
		"password": "ValidPassword123",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}
	if res["status"] != "success" {
		t.Errorf("Expected status success")
	}

	cookie := ""
	for _, c := range resp.Cookies() {
		if c.Name == "neonect_sid" {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Errorf("Expected neonect_sid cookie")
	}

	// Invalid Password
	resp, _ = h.PostJSON(t, "/api/v1/auth", map[string]string{
		"username": "testuser_auth",
		"password": "WrongPassword123",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// Health Check (Auth'd)
	resp, _ = h.GetJSON(t, "/api/v1/health", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}
}
