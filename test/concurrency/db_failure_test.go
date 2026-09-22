package concurrency_test

import (
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestDatabaseFailure(t *testing.T) {
	h := harness.Setup(t)

	// Create a valid user
	h.RegisterUser(t, "dbfailuser", "Password123!")

	// Directly access the underlying sql.DB connection and close it to simulate DB failure
	if err := h.App.DB.DB().Close(); err != nil {
		t.Fatalf("Failed to close underlying DB: %v", err)
	}

	// Attempt a DB-backed operation (login)
	resp, res := h.PostJSON(t, "/api/v1/auth", map[string]string{
		"username": "dbfailuser",
		"password": "Password123!",
	}, "")

	// Verify that the application responds with a 500 error and doesn't panic
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized when DB is closed, got %d", resp.StatusCode)
	}

	if res["error"] == "" {
		t.Errorf("Expected error message in response body, got empty")
	}
}

func TestDatabaseFailure_FanOut(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "sender", "Password123!")
	cookieSender := h.Login(t, "sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_s1")

	h.RegisterUser(t, "recipient", "Password123!")
	cookieRec := h.Login(t, "recipient", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_r1")
	h.RegisterDevice(t, cookieRec, "dev_r2")

	// Close DB
	h.App.DB.DB().Close()

	// Attempt fan-out
	resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev_s1",
		"protocol_version": 1, "to_username": "recipient",
		"ciphertext": "YnVy",
		"timestamp":  123456789,
	}, cookieSender)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected 500 on fan-out failure, got %d", resp.StatusCode)
	}
}
