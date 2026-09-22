package api_test

import (
	"NeoNect/internal/config"
	"NeoNect/test/harness"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestFriendsAndDevicesEndpoints(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	h.RegisterUser(t, "user_A", "Password123!")
	cookieA := h.Login(t, "user_A", "Password123!")

	h.RegisterUser(t, "user_B", "Password123!")
	cookieB := h.Login(t, "user_B", "Password123!")

	h.RegisterUser(t, "user_C", "Password123!")
	// cookieC := h.Login(t, "user_C", "Password123!")

	h.RegisterDevice(t, cookieA, "device_A1")
	h.RegisterDevice(t, cookieA, "device_A2")
	h.RegisterDevice(t, cookieB, "device_B1")

	// 1. Unauthenticated GET /api/v1/friends
	resp, _ := h.GetJSON(t, "/api/v1/friends", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", resp.StatusCode)
	}

	// 2. Authenticated GET /api/v1/friends (Empty)
	resp, data := h.GetJSON(t, "/api/v1/friends", cookieA)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	friendsList := data["friends"].([]interface{})
	if len(friendsList) != 0 {
		t.Fatalf("Expected 0 friends, got %d", len(friendsList))
	}

	// Add friend B to A
	body := map[string]string{"username": "user_B"}
	resp, _ = h.PostJSON(t, "/api/v1/friends", body, cookieA)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201, got %d", resp.StatusCode)
	}

	// Authenticated GET /api/v1/friends (Has friend B)
	resp, data = h.GetJSON(t, "/api/v1/friends", cookieA)
	friendsList = data["friends"].([]interface{})
	if resp.StatusCode != http.StatusOK || len(friendsList) != 1 || friendsList[0].(string) != "user_B" {
		t.Fatalf("Expected user_B in friends list")
	}

	// 3. Authenticated DELETE /api/v1/friends
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/friends", bytes.NewBuffer(b))
	req.Header.Set("Cookie", config.SessionCookieName+"="+cookieA)
	req.Header.Set("Content-Type", "application/json")
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on delete, got %d", delResp.StatusCode)
	}

	// Verify friend B is gone
	resp, data = h.GetJSON(t, "/api/v1/friends", cookieA)
	friendsList = data["friends"].([]interface{})
	if len(friendsList) != 0 {
		t.Fatalf("Expected 0 friends after delete")
	}

	// 4. Deleting a non-friend
	bodyC := map[string]string{"username": "user_C"}
	bC, _ := json.Marshal(bodyC)
	req2, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/friends", bytes.NewBuffer(bC))
	req2.Header.Set("Cookie", config.SessionCookieName+"="+cookieA)
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 on non-friend delete, got %d", resp2.StatusCode)
	}

	// 5. GET /api/v1/devices
	resp, data = h.GetJSON(t, "/api/v1/devices", cookieA)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	devicesList := data["devices"].([]interface{})
	if len(devicesList) != 2 {
		t.Fatalf("Expected 2 devices for A, got %d", len(devicesList))
	}

	// 6. Cross-user device isolation
	resp, data = h.GetJSON(t, "/api/v1/devices", cookieB)
	devicesList = data["devices"].([]interface{})
	if len(devicesList) != 1 || devicesList[0].(string) != "device_B1" {
		t.Fatalf("Expected 1 device for B")
	}
}
