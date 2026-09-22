package relay_test

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestMailbox_SendAndPoll(t *testing.T) {
	h := harness.Setup(t)

	// Sender user
	h.RegisterUser(t, "mailbox_sender", "Password123!")
	cookieSender := h.Login(t, "mailbox_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_sender")

	// Recipient user with two devices
	h.RegisterUser(t, "mailbox_rec", "Password123!")
	cookieRec := h.Login(t, "mailbox_rec", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_rec1")
	h.RegisterDevice(t, cookieRec, "dev_rec2")

	h.AddFriend(t, cookieSender, "mailbox_rec")
	h.AddFriend(t, cookieRec, "mailbox_sender")

	ciphertext := "c2VjcmV0IG1lc3NhZ2U=" // "secret message" in base64

	// Send to dev_rec1 only
	resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "msg_123",
		"from_device_id":      "dev_sender",
		"protocol_version":    2,
		"recipient_device_id": "dev_rec1",
		"ciphertext":          ciphertext,
	}, cookieSender)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to send envelope: expected 201, got %d", resp.StatusCode)
	}

	// Poll from dev_rec1
	time.Sleep(100 * time.Millisecond) // wait for worker to pick up if needed
	respRec1, bodyRec1 := h.GetJSON(t, "/api/v1/relay/poll?device_id=dev_rec1", cookieRec)
	if respRec1.StatusCode != http.StatusOK {
		t.Fatalf("Failed to poll dev_rec1: %d", respRec1.StatusCode)
	}

	messages1, ok := bodyRec1["messages"].([]interface{})
	if !ok || len(messages1) != 1 {
		t.Fatalf("Expected 1 message for dev_rec1, got %v", messages1)
	}
	msg1 := messages1[0].(map[string]interface{})
	if msg1["ciphertext"] != ciphertext {
		t.Errorf("Ciphertext mismatch for dev_rec1")
	}
	if msg1["message_id"] != "msg_123" {
		t.Errorf("Expected message_id=msg_123, got %v", msg1["message_id"])
	}
	if msg1["sender_device_id"] != "dev_sender" {
		t.Errorf("Expected sender_device_id=dev_sender, got %v", msg1["sender_device_id"])
	}

	// Poll from dev_rec2 - should be empty
	respRec2, bodyRec2 := h.GetJSON(t, "/api/v1/relay/poll?device_id=dev_rec2", cookieRec)
	if respRec2.StatusCode != http.StatusOK {
		t.Fatalf("Failed to poll dev_rec2: %d", respRec2.StatusCode)
	}
	messages2, ok := bodyRec2["messages"].([]interface{})
	if !ok || len(messages2) != 0 {
		t.Fatalf("Expected 0 messages for dev_rec2, got %v", messages2)
	}
}

func TestMailbox_DuplicateSubmission(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "dup_sender", "Password123!")
	cookieSender := h.Login(t, "dup_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_sender")

	h.RegisterUser(t, "dup_rec", "Password123!")
	cookieRec := h.Login(t, "dup_rec", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_rec")

	h.AddFriend(t, cookieSender, "dup_rec")
	h.AddFriend(t, cookieRec, "dup_sender")

	ciphertext1 := base64.StdEncoding.EncodeToString([]byte("msg1"))
	ciphertext2 := base64.StdEncoding.EncodeToString([]byte("msg2"))

	// Submit once
	resp1, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "dup_123",
		"from_device_id":      "dev_sender",
		"protocol_version":    2,
		"recipient_device_id": "dev_rec",
		"ciphertext":          ciphertext1,
	}, cookieSender)

	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to send first envelope: %d", resp1.StatusCode)
	}

	// Submit same message_id again with EXACT same payload (Case A - should succeed with 201)
	resp2, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "dup_123",
		"from_device_id":      "dev_sender",
		"protocol_version":    2,
		"recipient_device_id": "dev_rec",
		"ciphertext":          ciphertext1,
	}, cookieSender)

	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to send exact duplicate envelope: expected 201, got %d", resp2.StatusCode)
	}

	// Submit same message_id with DIFFERENT payload (Case B - should fail with 409)
	resp3, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "dup_123",
		"from_device_id":      "dev_sender",
		"protocol_version":    2,
		"recipient_device_id": "dev_rec",
		"ciphertext":          ciphertext2,
	}, cookieSender)

	if resp3.StatusCode != http.StatusConflict {
		t.Fatalf("Server allowed conflicting envelope (different payload): expected 409, got %d", resp3.StatusCode)
	}

	// Poll from dev_rec
	time.Sleep(100 * time.Millisecond)
	respRec, bodyRec := h.GetJSON(t, "/api/v1/relay/poll?device_id=dev_rec", cookieRec)
	if respRec.StatusCode != http.StatusOK {
		t.Fatalf("Failed to poll: %d", respRec.StatusCode)
	}

	messages, ok := bodyRec["messages"].([]interface{})
	if !ok || len(messages) != 1 {
		t.Fatalf("Expected exactly 1 message, got %v", messages)
	}
	msg := messages[0].(map[string]interface{})
	if msg["ciphertext"] != ciphertext1 {
		t.Errorf("Expected first ciphertext to be preserved, got %v", msg["ciphertext"])
	}
}

func TestMailbox_SenderAuthorization(t *testing.T) {
	h := harness.Setup(t)

	// Sender user A
	h.RegisterUser(t, "user_a", "Password123!")
	cookieA := h.Login(t, "user_a", "Password123!")
	h.RegisterDevice(t, cookieA, "dev_a")

	// Sender user B
	h.RegisterUser(t, "user_b", "Password123!")
	cookieB := h.Login(t, "user_b", "Password123!")
	h.RegisterDevice(t, cookieB, "dev_b")

	// Try to send as User B's device while authenticated as User A
	resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "msg_auth_1",
		"from_device_id":      "dev_b",
		"protocol_version":    2,
		"recipient_device_id": "dev_a",
		"ciphertext":          "test",
	}, cookieA)

	if resp.StatusCode == http.StatusCreated {
		t.Fatalf("Expected failure when sending from another user's device, got 201")
	}
	t.Logf("Response 1 status: %d", resp.StatusCode)

	// Try to send from non-existent device
	resp2, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "msg_auth_2",
		"from_device_id":      "dev_nonexistent",
		"protocol_version":    2,
		"recipient_device_id": "dev_a",
		"ciphertext":          "test",
	}, cookieA)

	if resp2.StatusCode == http.StatusCreated {
		t.Fatalf("Expected failure when sending from nonexistent device, got 201")
	}
	t.Logf("Response 2 status: %d", resp2.StatusCode)
}
