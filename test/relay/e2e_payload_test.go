package relay_test

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestE2EPayloadOpacityAndIntegrity(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "e2e_sender", "Password1234")
	cookieSender := h.Login(t, "e2e_sender", "Password1234")
	h.RegisterDevice(t, cookieSender, "e2e_sender_dev")

	h.RegisterUser(t, "e2e_recipient", "Password1234")
	cookieRecipient := h.Login(t, "e2e_recipient", "Password1234")
	h.RegisterDevice(t, cookieRecipient, "e2e_recipient_dev")
	h.AddFriend(t, cookieSender, "e2e_recipient")
	h.AddFriend(t, cookieRecipient, "e2e_sender")

	// Create an arbitrary binary payload to verify that the server does not
	// attempt to parse or modify it (Opacity & Integrity)
	secretPayload := []byte{0x00, 0xff, 0x11, 0xee, 0x22, 0xdd, 0x33, 0xcc}
	ciphertext := base64.StdEncoding.EncodeToString(secretPayload)

	resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "e2e_sender_dev",
		"protocol_version": 1, "to_username": "e2e_recipient",
		"ciphertext": ciphertext,
		"timestamp":  time.Now().Unix(),
	}, cookieSender)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created for send, got %d", resp.StatusCode)
	}

	respPoll, resPoll := h.GetJSON(t, "/api/v1/relay/poll?device_id=e2e_recipient_dev", cookieRecipient)
	if respPoll.StatusCode != http.StatusOK {
		t.Fatalf("Failed to poll messages, status: %d", respPoll.StatusCode)
	}

	messages, ok := resPoll["messages"].([]interface{})
	if !ok || len(messages) != 1 {
		t.Fatalf("Expected exactly 1 message, got %d", len(messages))
	}

	msgMap, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid message format")
	}

	receivedCiphertext, ok := msgMap["ciphertext"].(string)
	if !ok {
		t.Fatalf("Missing or invalid ciphertext field in response")
	}

	if receivedCiphertext != ciphertext {
		t.Errorf("E2E Payload Integrity failed: sent %s, received %s", ciphertext, receivedCiphertext)
	}
}
