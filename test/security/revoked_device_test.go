package security_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"

	"NeoNect/test/harness"
)

func revokeDevice(t *testing.T, h *harness.Harness, cookie string, deviceID string) {
	payload, _ := json.Marshal(map[string]interface{}{"device_id": deviceID})
	req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/device", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Failed to revoke: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to revoke device, got %d", resp.StatusCode)
	}
}

func TestRevokedDeviceCannotSend(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	h.RegisterUser(t, "rvk_sender", "Password123!")
	cookieSender := h.Login(t, "rvk_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_rvk_s")

	h.RegisterUser(t, "rvk_rec", "Password123!")
	cookieRec := h.Login(t, "rvk_rec", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_rvk_r")

	h.AddFriend(t, cookieSender, "rvk_rec")
	h.AddFriend(t, cookieRec, "rvk_sender")

	cipher := base64.StdEncoding.EncodeToString([]byte("hello"))
	resp1, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 1,
		"from_device_id":   "dev_rvk_s",
		"to_username":      "rvk_rec",
		"ciphertext":       cipher,
	}, cookieSender)
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("active device should send: got %d", resp1.StatusCode)
	}

	revokeDevice(t, h, cookieSender, "dev_rvk_s")

	resp2, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 1,
		"from_device_id":   "dev_rvk_s",
		"to_username":      "rvk_rec",
		"ciphertext":       cipher,
	}, cookieSender)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked device should be rejected: got %d, expected 401", resp2.StatusCode)
	}
}

func TestRevokedDeviceCannotPoll(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	h.RegisterUser(t, "rvk_poller", "Password123!")
	cookie := h.Login(t, "rvk_poller", "Password123!")
	h.RegisterDevice(t, cookie, "dev_rvk_poll")

	revokeDevice(t, h, cookie, "dev_rvk_poll")

	resp, _ := h.GetJSON(t, "/api/v1/relay/poll?device_id=dev_rvk_poll", cookie)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked device poll should be rejected: got %d, expected 401", resp.StatusCode)
	}
}

func TestRevokedDeviceCannotACK(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	h.RegisterUser(t, "rvk_acker", "Password123!")
	cookie := h.Login(t, "rvk_acker", "Password123!")
	h.RegisterDevice(t, cookie, "dev_rvk_ack")

	revokeDevice(t, h, cookie, "dev_rvk_ack")

	resp, _ := h.PostJSON(t, "/api/v1/relay/ack", map[string]interface{}{
		"device_id":  "dev_rvk_ack",
		"message_id": 1,
	}, cookie)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked device ACK should be rejected: got %d, expected 401", resp.StatusCode)
	}
}

func TestRevokedDeviceCannotConnectWS(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	h.RegisterUser(t, "rvk_ws", "Password123!")
	cookie := h.Login(t, "rvk_ws", "Password123!")
	h.RegisterDevice(t, cookie, "dev_rvk_ws")

	revokeDevice(t, h, cookie, "dev_rvk_ws")

	// Try to connect WS
	dialer := websocket.Dialer{}
	headers := http.Header{}
	headers.Add("Cookie", "neonect_sid="+cookie)
	_, resp, err := dialer.Dial(h.WsURL+"/api/v1/relay/ws?device_id=dev_rvk_ws", headers)
	if err == nil {
		t.Fatalf("Expected WS connection to fail for revoked device")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked device WS should be rejected with 401: got %d", resp.StatusCode)
	}
}

func TestRevokedDeviceDisconnectsWS(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	h.RegisterUser(t, "rvk_ws_disc", "Password123!")
	cookie := h.Login(t, "rvk_ws_disc", "Password123!")
	h.RegisterDevice(t, cookie, "dev_rvk_ws_disc")

	// Connect WS while active
	conn := h.DialWS(t, cookie, "dev_rvk_ws_disc")
	var err error
	defer conn.Close()

	// Revoke the device
	revokeDevice(t, h, cookie, "dev_rvk_ws_disc")

	// Wait for disconnect
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("Expected WS connection to be closed after revocation")
	}
}
