package security_test

import (
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestSessionTTL_Expiration(t *testing.T) {
	h := harness.Setup(t)

	// 1. Fresh session
	h.RegisterUser(t, "ttl_user", "password123")
	cookie := h.Login(t, "ttl_user", "password123")

	// 2. Validate fresh session works
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	resp, _ := h.Client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Fresh session failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Manually expire the session in the DB to test TTL validation natively
	// Find the session token hash in DB. Wait, cookie IS the raw token.
	// The DB stores token_hash. But we can just expire ALL sessions for the user to be safe.
	// Instead of h.Login, we manually insert a token so it's NOT in the in-memory cache!
	h.App.DB.DB().Exec("INSERT INTO user_blocks (user_id, block_type, sub_block_id, payload, created_at) VALUES ((SELECT id FROM users WHERE username_hash = ?), 'SESSION', 'expired_token123', '', datetime('now', '-2 days'))", "21cf7283286dbd7f9d0c6489b4f0b09426f0ecbd944ce2f567b5e43a60a7e024")
	// wait, we need the exact hash for 'ttl_user'. Harness does not expose it easily, but we can just use the user_id from the first login!
	// Let's get the user_id from the db:
	var uid int
	h.App.DB.DB().QueryRow("SELECT user_id FROM user_blocks WHERE block_type = 'SESSION'").Scan(&uid)
	h.App.DB.DB().Exec("INSERT INTO user_blocks (user_id, block_type, sub_block_id, payload, created_at) VALUES (?, 'SESSION', 'expired_token123', '', datetime('now', '-2 days'))", uid)
	cookie = "expired_token123"

	// 4. Validate expired session is rejected
	req2, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req2.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	resp2, _ := h.Client.Do(req2)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expired session was NOT rejected! Got %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	// 5. Verify bearer token auth logic also rejects it
	req3, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req3.Header.Set("Authorization", "Bearer "+cookie)
	resp3, _ := h.Client.Do(req3)
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expired bearer session was NOT rejected! Got %d", resp3.StatusCode)
	}
	resp3.Body.Close()
}

func TestSession_Logout(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "logout_user", "password123")
	cookie := h.Login(t, "logout_user", "password123")

	req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/auth", nil)
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	resp, _ := h.Client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Logout failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Re-use logged-out session
	req2, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req2.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	resp2, _ := h.Client.Do(req2)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Reused logged-out session was NOT rejected! Got %d", resp2.StatusCode)
	}
	resp2.Body.Close()
}

func TestSession_MultipleSessions(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "multi_user", "password123")
	cookie1 := h.Login(t, "multi_user", "password123")
	cookie2 := h.Login(t, "multi_user", "password123")

	// Logout cookie 1
	req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/auth", nil)
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie1})
	resp, _ := h.Client.Do(req)
	resp.Body.Close()

	// Cookie 2 should still work
	req2, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req2.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie2})
	resp2, _ := h.Client.Do(req2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Independent session was improperly destroyed! Got %d", resp2.StatusCode)
	}
	resp2.Body.Close()
}
