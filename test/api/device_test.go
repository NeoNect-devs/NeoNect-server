package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"NeoNect/test/harness"
)

func TestDeviceFlow(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "deviceuser1", "Password1234")
	cookie1 := h.Login(t, "deviceuser1", "Password1234")

	resp, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %d", resp.StatusCode)
	}

	resp, res := h.GetJSON(t, "/api/v1/relay/keys?u=deviceuser1", cookie1)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}
	devices, _ := res["devices"].([]interface{})
	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %v", len(devices))
	}
}

func TestDeviceMultipleAndDuplicate(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	// 1. Multiple devices
	resp1, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)
	if resp1.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created for dev1, got %d", resp1.StatusCode)
	}

	resp2, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev2",
		"public_key": "cHVibGljS2V5",
	}, cookie1)
	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201 Created for dev2, got %d", resp2.StatusCode)
	}

	// 2. Duplicate device registration
	resp3, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)
	if resp3.StatusCode != http.StatusConflict && resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected Conflict or Bad Request for duplicate dev1, got %d", resp3.StatusCode)
	}
}

func TestDeviceInvalidInput(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	// Invalid public key
	resp, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "not-base64-!@#",
	}, cookie1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid public key, got %d", resp.StatusCode)
	}

	// Empty device ID
	resp2, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "",
		"public_key": "cHVibGljS2V5",
	}, cookie1)
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for empty device id, got %d", resp2.StatusCode)
	}
}

func TestDeviceRevocationEdgeCases(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.RegisterUser(t, "user2", "Password1234")
	cookie2 := h.Login(t, "user2", "Password1234")

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	deleteJSON := func(deviceID, cookie string) *http.Response {
		payload := map[string]string{"device_id": deviceID}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/device", bytes.NewBuffer(body))
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
		req.Header.Set("Content-Type", "application/json")
		resp, _ := h.Client.Do(req)
		return resp
	}

	// Cross-user device revocation attempt
	resp := deleteJSON("dev1", cookie2)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for cross-user revocation, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Revoke own device
	resp2 := deleteJSON("dev1", cookie1)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for revocation, got %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	// Already revoked device / idempotent revocation
	resp3 := deleteJSON("dev1", cookie1)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for already-revoked device revocation, got %d", resp3.StatusCode)
	}
	resp3.Body.Close()

	// Nonexistent device revocation
	resp4 := deleteJSON("dev-nonexistent", cookie1)
	if resp4.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for nonexistent device revocation, got %d", resp4.StatusCode)
	}
	resp4.Body.Close()

	// Cross-user device listing attempt
	resp5, _ := h.GetJSON(t, "/api/v1/relay/keys?u=user1", cookie2)
	if resp5.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for cross-user device enumeration, got %d", resp5.StatusCode)
	}

	// User listing their own devices (should be 0 because revoked)
	resp6, res6 := h.GetJSON(t, "/api/v1/relay/keys?u=user1", cookie1)
	if resp6.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp6.StatusCode)
	}
	devices, _ := res6["devices"].([]interface{})
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices for revoked user, got %v", len(devices))
	}
}

func TestDeviceRegistrationRollback(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_rollback", "Password1234")
	cookie := h.Login(t, "user_rollback", "Password1234")

	// Register dev1
	resp1, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie)
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", resp1.StatusCode)
	}

	// Try to register dev1 again (duplicate) -> will fail
	resp2, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "YmFkX2tleQ==",
	}, cookie)
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("Expected Conflict for duplicate dev1, got %d", resp2.StatusCode)
	}

	// Verify dev1's public key was NOT overwritten (rollback/conflict handled)
	// Note: device/key requires Auth! We need to use req
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/device/key?device_id=dev1", nil)
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	resp4, _ := h.Client.Do(req)
	var dev struct {
		PublicKey string `json:"public_key"`
	}
	json.NewDecoder(resp4.Body).Decode(&dev)
	resp4.Body.Close()
	if dev.PublicKey != "cHVibGljS2V5" {
		t.Errorf("Expected original key 'cHVibGljS2V5', got %v", dev.PublicKey)
	}
}
