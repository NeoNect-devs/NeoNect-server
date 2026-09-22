package security_test

import (
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestFriendshipSecurity(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_sec_a", "Password123!")
	cookieA := h.Login(t, "user_sec_a", "Password123!")

	h.RegisterUser(t, "user_sec_b", "Password123!")
	_ = h.Login(t, "user_sec_b", "Password123!")

	h.RegisterUser(t, "user_sec_c", "Password123!")

	getFriendshipCount := func() int {
		var count int
		err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count friendships: %v", err)
		}
		return count
	}

	t.Run("cannot spoof source user ID", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_sec_c",
			"from_id":  "user_sec_b", // Ignored by API
		}, cookieA)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201 Created for A->C, got %d", resp.StatusCode)
		}

		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship, got %d", count)
		}

		respConflict, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_sec_c",
		}, cookieA)
		if respConflict.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", respConflict.StatusCode)
		}

		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship after conflict, got %d", count)
		}
	})

	t.Run("malformed username does not cause crash", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "invalid\\user\"name'",
		}, cookieA)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 404 or 400 for malformed username, got %d", resp.StatusCode)
		}

		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship (A->C from before), got %d", count)
		}
	})

	t.Run("sql injection payloads in username", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "' OR 1=1 --",
		}, cookieA)
		if resp.StatusCode == http.StatusInternalServerError {
			t.Errorf("Potential SQL injection vulnerability: received 500")
		}

		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship (A->C from before), got %d", count)
		}
	})
}
