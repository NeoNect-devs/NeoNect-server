package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestDeviceRevocation(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "revoke_user", "password123!")
	cookie := h.Login(t, "revoke_user", "password123!")

	deviceID := "device-to-revoke"
	h.RegisterDevice(t, cookie, deviceID)

	// Ensure the device can connect
	conn := h.DialWS(t, cookie, deviceID)

	// Revoke the device
	payload := map[string]string{
		"device_id": deviceID,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/device", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Revoke request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for device revocation, got %d", resp.StatusCode)
	}

	// Verify the existing connection was disconnected
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("Expected existing WebSocket connection to be closed after revocation")
	}

	// Verify the device can no longer connect
	req, err = http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/relay/ws?device_id="+deviceID, nil)
	if err != nil {
		t.Fatalf("Failed to create WS request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})

	wsResp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("WS HTTP request failed: %v", err)
	}
	defer wsResp.Body.Close()

	// The WS connection should be rejected with an unauthorized or bad request status
	if wsResp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("Device was able to connect to WS after being revoked")
	}
}
