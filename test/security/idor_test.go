package security_test

import (
	"NeoNect/test/harness"
	"net/http"
	"testing"
)

func TestIDOR(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "alice", "Password1234")
	cookieAlice := h.Login(t, "alice", "Password1234")
	h.RegisterDevice(t, cookieAlice, "alice_dev")

	h.RegisterUser(t, "bob", "Password1234")
	cookieBob := h.Login(t, "bob", "Password1234")
	h.RegisterDevice(t, cookieBob, "bob_dev")

	// Alice tries to poll Bob's queue
	resp, _ := h.GetJSON(t, "/api/v1/relay/poll?device_id=bob_dev", cookieAlice)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("IDOR Vulnerability: Expected 401 Unauthorized when polling another user's queue, got %d", resp.StatusCode)
	}

	// Alice tries to fetch Bob's keys without being friends
	resp2, _ := h.GetJSON(t, "/api/v1/relay/keys?u=bob", cookieAlice)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("IDOR Vulnerability: Expected 401 Unauthorized when fetching non-friend keys, got %d", resp2.StatusCode)
	}

	// Alice tries to send a message to Bob without being friends (Protocol 1)
	resp3, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "alice_dev",
		"protocol_version": 1,
		"to_username":      "bob",
		"ciphertext":       "YmFzZTY0",
	}, cookieAlice)
	if resp3.StatusCode != http.StatusForbidden {
		t.Errorf("IDOR Vulnerability: Expected 403 Forbidden when sending to non-friend, got %d", resp3.StatusCode)
	}

	// Alice tries to send a message to Bob without being friends (Protocol 2)
	resp4, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":      "alice_dev",
		"protocol_version":    2,
		"recipient_device_id": "bob_dev",
		"message_id":          "msg-123",
		"ciphertext":          "YmFzZTY0",
	}, cookieAlice)
	if resp4.StatusCode != http.StatusForbidden {
		t.Errorf("IDOR Vulnerability: Expected 403 Forbidden when sending envelope to non-friend, got %d", resp4.StatusCode)
	}
}
