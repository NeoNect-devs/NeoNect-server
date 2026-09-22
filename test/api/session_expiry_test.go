package api_test

import (
	"NeoNect/test/harness"
	"net/http"
	"testing"
)

func TestSessionLogout(t *testing.T) {
	h := harness.Setup(t)

	// Register and login a user
	h.RegisterUser(t, "logout_user", "password123!")
	cookie := h.Login(t, "logout_user", "password123!")

	// Verify session works
	resp, _ := h.GetJSON(t, "/api/v1/users/me", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /users/me, got %d", resp.StatusCode)
	}

	// Perform Logout
	req, err := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/auth", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	logoutResp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Logout request failed: %v", err)
	}
	defer logoutResp.Body.Close()

	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for logout, got %d", logoutResp.StatusCode)
	}

	// Verify session is invalidated
	resp, _ = h.GetJSON(t, "/api/v1/users/me", cookie)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized after logout, got %d", resp.StatusCode)
	}
}

func TestLogoutWithoutSession(t *testing.T) {
	h := harness.Setup(t)

	// Perform Logout without a session
	req, err := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/auth", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	logoutResp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Logout request failed: %v", err)
	}
	defer logoutResp.Body.Close()

	// Handler returns success even if no session to prevent enumeration
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for unauthenticated logout, got %d", logoutResp.StatusCode)
	}
}
