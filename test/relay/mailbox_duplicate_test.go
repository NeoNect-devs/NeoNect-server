package relay_test

import (
	"encoding/base64"
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestMailbox_DuplicateDifferentRecipient(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "dup_sender2", "Password123!")
	cookieSender := h.Login(t, "dup_sender2", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_sender2")

	h.RegisterUser(t, "dup_rec_a", "Password123!")
	cookieRecA := h.Login(t, "dup_rec_a", "Password123!")
	h.RegisterDevice(t, cookieRecA, "dev_rec_a")

	h.RegisterUser(t, "dup_rec_b", "Password123!")
	cookieRecB := h.Login(t, "dup_rec_b", "Password123!")
	h.RegisterDevice(t, cookieRecB, "dev_rec_b")

	h.AddFriend(t, cookieSender, "dup_rec_a")
	h.AddFriend(t, cookieRecA, "dup_sender2")

	h.AddFriend(t, cookieSender, "dup_rec_b")
	h.AddFriend(t, cookieRecB, "dup_sender2")

	// Submit once to rec_a
	resp1, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "dup_same_id",
		"from_device_id":      "dev_sender2",
		"protocol_version":    2,
		"recipient_device_id": "dev_rec_a",
		"ciphertext":          base64.StdEncoding.EncodeToString([]byte("msg1")),
	}, cookieSender)

	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to send first envelope: %d", resp1.StatusCode)
	}

	// Submit same message_id to rec_b
	resp2, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"message_id":          "dup_same_id",
		"from_device_id":      "dev_sender2",
		"protocol_version":    2,
		"recipient_device_id": "dev_rec_b",
		"ciphertext":          base64.StdEncoding.EncodeToString([]byte("msg1")),
	}, cookieSender)

	if resp2.StatusCode == http.StatusCreated {
		t.Fatalf("Server silently allowed same message_id to be used for different recipient devices, violating C.")
	}
}
