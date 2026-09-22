package relay_test

import (
	"encoding/base64"
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestProtocolVersionBoundary(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "pb_sender", "Password123!")
	cookieSender := h.Login(t, "pb_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_pb_s")

	h.RegisterUser(t, "pb_rec", "Password123!")
	cookieRec := h.Login(t, "pb_rec", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_pb_r")

	h.AddFriend(t, cookieSender, "pb_rec")
	h.AddFriend(t, cookieRec, "pb_sender")

	cipher := base64.StdEncoding.EncodeToString([]byte("test"))

	// 1. omitted protocol_version -> reject (400)
	resp1, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id": "dev_pb_s",
		"to_username":    "pb_rec",
		"ciphertext":     cipher,
	}, cookieSender)
	if resp1.StatusCode != http.StatusBadRequest {
		t.Fatalf("omitted protocol_version expected 400, got %d", resp1.StatusCode)
	}

	// 2. protocol_version = 1 -> explicit legacy compatibility mode -> success (201)
	resp2, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 1,
		"from_device_id":   "dev_pb_s",
		"to_username":      "pb_rec",
		"ciphertext":       cipher,
	}, cookieSender)
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("protocol_version=1 expected 201, got %d", resp2.StatusCode)
	}

	// 3. protocol_version = 2 -> strict device-bound mode -> success (201)
	resp3, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version":    2,
		"message_id":          "msg_v2_1",
		"from_device_id":      "dev_pb_s",
		"recipient_device_id": "dev_pb_r",
		"ciphertext":          cipher,
	}, cookieSender)
	if resp3.StatusCode != http.StatusCreated {
		t.Fatalf("protocol_version=2 expected 201, got %d", resp3.StatusCode)
	}

	// 4. unsupported version (e.g. 3) -> reject (400)
	resp4, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version":    3,
		"message_id":          "msg_v3_1",
		"from_device_id":      "dev_pb_s",
		"recipient_device_id": "dev_pb_r",
		"ciphertext":          cipher,
	}, cookieSender)
	if resp4.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsupported protocol_version expected 400, got %d", resp4.StatusCode)
	}

	// 5. version 2 missing message_id -> reject (400)
	resp5, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version":    2,
		"from_device_id":      "dev_pb_s",
		"recipient_device_id": "dev_pb_r",
		"ciphertext":          cipher,
	}, cookieSender)
	if resp5.StatusCode != http.StatusBadRequest {
		t.Fatalf("version 2 missing message_id expected 400, got %d", resp5.StatusCode)
	}

	// 6. version 2 missing recipient_device_id -> reject (400)
	resp6, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 2,
		"message_id":       "msg_v2_2",
		"from_device_id":   "dev_pb_s",
		"to_username":      "pb_rec",
		"ciphertext":       cipher,
	}, cookieSender)
	if resp6.StatusCode != http.StatusBadRequest {
		t.Fatalf("version 2 missing recipient_device_id expected 400, got %d", resp6.StatusCode)
	}
}
