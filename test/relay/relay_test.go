package relay_test

import (
	"NeoNect/test/harness"
	"net/http"
	"testing"
	"time"
)

func TestRelayFlow(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "sender", "Password1234")
	cookieSender := h.Login(t, "sender", "Password1234")
	h.RegisterDevice(t, cookieSender, "sender_dev")

	h.RegisterUser(t, "recipient", "Password1234")
	cookieRecipient := h.Login(t, "recipient", "Password1234")
	h.RegisterDevice(t, cookieRecipient, "recipient_dev")

	// Make them friends
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "recipient"}, cookieSender)
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "sender"}, cookieRecipient)

	// Send message
	resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "sender_dev",
		"protocol_version": 1, "to_username": "recipient",
		"ciphertext": "YnVy",
		"timestamp":  time.Now().Unix(),
	}, cookieSender)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created for relay send, got %d", resp.StatusCode)
	}

	// Poll message
	resp, res := h.GetJSON(t, "/api/v1/relay/poll?device_id=recipient_dev", cookieRecipient)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for poll, got %d", resp.StatusCode)
	}
	messages, _ := res["messages"].([]interface{})
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %v", len(messages))
	}
}
