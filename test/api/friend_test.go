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

	h.RegisterUser(t, "user_charlie", "Password123!")
	cookieCharlie := h.Login(t, "user_charlie", "Password123!")

	getFriendshipCount := func() int {
		var count int
		err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count friendships: %v", err)
		}
		return count
	}

	getRequestCount := func() int {
		var count int
		err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count requests: %v", err)
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
	})

	t.Run("target user does not exist", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "non_existent_user",
		}, cookieAlice)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found, got %d", resp.StatusCode)
		}
	})

	t.Run("self-add attempt", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_alice",
		}, cookieAlice)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request, got %d", resp.StatusCode)
		}
	})

	t.Run("authenticated user can send friend request", func(t *testing.T) {
		resp, res := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_bob",
		}, cookieAlice)
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201 Created, got %d", resp.StatusCode)
		}
		if res["status"] != "success" {
			t.Errorf("Expected status success in response, got %v", res["status"])
		}
		if count := getFriendshipCount(); count != 0 {
			t.Errorf("Expected 0 friendship, got %d", count)
		}
		if count := getRequestCount(); count != 1 {
			t.Errorf("Expected exactly 1 request, got %d", count)
		}
	})

	t.Run("duplicate friend request", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_bob",
		}, cookieAlice)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", resp.StatusCode)
		}
	})

	t.Run("reverse friend request is conflict", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_alice",
		}, cookieBob)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", resp.StatusCode)
		}
	})

	t.Run("unauthorized user cannot accept request", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{
			"username": "user_bob",
		}, cookieCharlie)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("sender cannot accept own request", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{
			"username": "user_bob",
		}, cookieAlice)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("recipient can accept friend request", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{
			"username": "user_alice",
		}, cookieBob)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
		if count := getFriendshipCount(); count != 1 {
			t.Errorf("Expected 1 friendship, got %d", count)
		}
		if count := getRequestCount(); count != 0 {
			t.Errorf("Expected 0 requests, got %d", count)
		}
	})

	t.Run("cannot accept non-existent request", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{
			"username": "user_alice",
		}, cookieBob)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found, got %d", resp.StatusCode)
		}
	})

	t.Run("sender cannot decline own request", func(t *testing.T) {
		h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_charlie",
		}, cookieAlice)

		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/decline", map[string]string{
			"username": "user_charlie",
		}, cookieAlice)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("unauthorized user cannot decline request", func(t *testing.T) {
		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/decline", map[string]string{
			"username": "user_alice",
		}, cookieBob) // bob is unrelated to alice-charlie
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 403 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("decline friend request", func(t *testing.T) {
		// Charlie declines the request from Alice
		resp, _ := h.PostJSON(t, "/api/v1/friends/requests/decline", map[string]string{
			"username": "user_alice",
		}, cookieCharlie)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
		if count := getRequestCount(); count != 0 {
			t.Errorf("Expected 0 requests, got %d", count)
		}
	})

	t.Run("recipient cannot cancel incoming request", func(t *testing.T) {
		h.PostJSON(t, "/api/v1/friends", map[string]string{
			"username": "user_charlie",
		}, cookieAlice)

		resp, _ := h.DeleteJSON(t, "/api/v1/friends/requests", map[string]string{
			"username": "user_alice",
		}, cookieCharlie)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}
	})

	t.Run("unauthorized user cannot cancel request", func(t *testing.T) {
		resp, _ := h.DeleteJSON(t, "/api/v1/friends/requests", map[string]string{
			"username": "user_charlie",
		}, cookieBob) // bob is unrelated
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 403 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("cancel friend request", func(t *testing.T) {
		// Alice cancels her request to Charlie
		resp, _ := h.DeleteJSON(t, "/api/v1/friends/requests", map[string]string{
			"username": "user_charlie",
		}, cookieAlice)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}
		if count := getRequestCount(); count != 0 {
			t.Errorf("Expected 0 requests, got %d", count)
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
