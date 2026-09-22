package api_test

import (
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestFriendshipAPI(t *testing.T) {
	h := harness.Setup(t)

	// Setup users
	h.RegisterUser(t, "user_alice", "Password123!")
	cookieAlice := h.Login(t, "user_alice", "Password123!")

	h.RegisterUser(t, "user_bob", "Password123!")
	cookieBob := h.Login(t, "user_bob", "Password123!")

	getFriendshipCount := func() int {
		var count int
		err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count friendships: %v", err)
		}
		return count
	}

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_bob",
		}, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 0 {
			t.Errorf("Expected 0 friendships, got %d", count)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "",
		}, cookieAlice)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 0 {
			t.Errorf("Expected 0 friendships, got %d", count)
		}
	})

	t.Run("target user does not exist", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "non_existent_user",
		}, cookieAlice)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 0 {
			t.Errorf("Expected 0 friendships, got %d", count)
		}
	})

	t.Run("self-add attempt", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_alice",
		}, cookieAlice)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 0 {
			t.Errorf("Expected 0 friendships, got %d", count)
		}
	})

	t.Run("authenticated user can add an existing user", func(t *testing.T) {
		resp, res := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_bob",
		}, cookieAlice)
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201 Created, got %d", resp.StatusCode)
		}
		if res["status"] != "success" {
			t.Errorf("Expected status success in response, got %v", res["status"])
		}
		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship, got %d", count)
		}

		var u1, u2 int64
		err := h.App.DB.DB().QueryRow("SELECT user_id_1, user_id_2 FROM friendships").Scan(&u1, &u2)
		if err != nil {
			t.Fatalf("Failed to query friendship pair: %v", err)
		}
		if u1 >= u2 {
			t.Errorf("Expected user_id_1 < user_id_2, got %d >= %d", u1, u2)
		}
	})

	t.Run("duplicate friendship", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_bob",
		}, cookieAlice)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship, got %d", count)
		}
	})

	t.Run("duplicate friendship reversed", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_alice",
		}, cookieBob)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected exactly 1 friendship, got %d", count)
		}
	})

	t.Run("database failure returns 500, not 404", func(t *testing.T) {
		h.RegisterUser(t, "user_dbfail", "Password123!")

		// Simulate database failure
		h.App.DB.DB().Close()

		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_dbfail",
		}, cookieAlice)

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500 Internal Server Error when DB is closed, got %d", resp.StatusCode)
		}
	})
}
