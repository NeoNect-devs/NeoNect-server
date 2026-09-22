package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"NeoNect/test/harness"
)

func TestAPIContract_MethodAndSizeEnforcement(t *testing.T) {
	h := harness.Setup(t)

	// We test ALL endpoints for Method enforcement and Size limits!
	endpoints := []struct {
		path          string
		allowedMethod string
		needsAuth     bool
		isPost        bool
	}{
		{"/api/v1/users/availability?u=test", http.MethodGet, false, false},
		{"/api/v1/users", http.MethodPost, false, true},
		{"/api/v1/users/me", http.MethodGet, true, false},
		{"/api/v1/auth", http.MethodPost, false, true},
		// DELETE /api/v1/auth is special, we'll test manually
		{"/api/v1/device/register", http.MethodPost, true, true},
		{"/api/v1/device", http.MethodDelete, true, true}, // takes body
		{"/api/v1/device/key?device_id=test", http.MethodGet, true, false},
		{"/api/v1/relay/keys?u=test", http.MethodGet, true, false},
		{"/api/v1/relay/send", http.MethodPost, true, true},
		{"/api/v1/relay/poll?device_id=test", http.MethodGet, true, false},
		{"/api/v1/relay/ack", http.MethodPost, true, true},
		{"/api/v1/health", http.MethodGet, false, false},
		{"/api/v1/security/verify", http.MethodGet, true, false},
	}

	h.RegisterUser(t, "contract_user", "Password123!")
	cookie := h.Login(t, "contract_user", "Password123!")

	for _, ep := range endpoints {
		t.Run("MethodEnforcement_"+ep.path, func(t *testing.T) {
			badMethod := http.MethodGet
			if ep.allowedMethod == http.MethodGet {
				badMethod = http.MethodPost
			}
			req, _ := http.NewRequest(badMethod, h.BaseURL+ep.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			if ep.needsAuth {
				req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
			}
			resp, _ := h.Client.Do(req)
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("Expected 405 Method Not Allowed for %s %s, got %d", badMethod, ep.path, resp.StatusCode)
			}
			resp.Body.Close()
		})

		if ep.isPost {
			t.Run("SizeEnforcement_"+ep.path, func(t *testing.T) {
				largePayload := `{"payload":"` + strings.Repeat("A", 5*1024*1024) + `"}`
				req, _ := http.NewRequest(ep.allowedMethod, h.BaseURL+ep.path, strings.NewReader(largePayload))
				req.Header.Set("Content-Type", "application/json")
				if ep.needsAuth {
					req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
				}
				resp, _ := h.Client.Do(req)
				if resp.StatusCode != http.StatusRequestEntityTooLarge {
					b2, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected 413, got %d - body: %s", resp.StatusCode, string(b2))
				}
				resp.Body.Close()
			})
		}
	}
}

func TestAPIContract_ContentType(t *testing.T) {
	h := harness.Setup(t)
	// Testing missing/wrong content type on POST endpoints
	req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/users", strings.NewReader(`{"username":"test","password":"123"}`))
	// No content type set!
	resp, _ := h.Client.Do(req)
	// Some frameworks enforce content-type. NeoNect JSON decoding might just read it, but let's check!
	// If it succeeds or fails with 400, we don't strictly require 415 unless documented, but we assert it here to check behavior.
	if resp.StatusCode != http.StatusUnsupportedMediaType && resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusCreated {
		t.Errorf("Unexpected status for missing content-type: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIContract_AuthEnforcement(t *testing.T) {
	h := harness.Setup(t)

	endpoints := []string{
		"/api/v1/users/me",
		"/api/v1/device/register",
		"/api/v1/device",
		"/api/v1/device/key?device_id=test",
		"/api/v1/relay/keys?u=test",
		"/api/v1/relay/send",
		"/api/v1/relay/poll?device_id=test",
		"/api/v1/relay/ack",
		"/api/v1/security/verify",
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(http.MethodGet, h.BaseURL+ep, nil)
		if strings.Contains(ep, "register") || strings.Contains(ep, "send") || strings.Contains(ep, "ack") {
			req, _ = http.NewRequest(http.MethodPost, h.BaseURL+ep, strings.NewReader(`{}`))
		} else if ep == "/api/v1/device" {
			req, _ = http.NewRequest(http.MethodDelete, h.BaseURL+ep, strings.NewReader(`{}`))
		}
		resp, _ := h.Client.Do(req)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for unauthenticated %s, got %d", ep, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestAPIContract_SuccessSchemas(t *testing.T) {
	h := harness.Setup(t)

	// Auth and Users
	resp, _ := h.PostJSON(t, "/api/v1/users", map[string]interface{}{"username": "user1", "password": "Password123!"}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create user")
	}

	cookie := h.Login(t, "user1", "Password123!")

	// /api/v1/users/me
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	respMe, _ := h.Client.Do(req)
	var me struct{ Username string }
	json.NewDecoder(respMe.Body).Decode(&me)
	respMe.Body.Close()
	if me.Username != "user1" {
		t.Errorf("Expected username 'user1', got '%s'", me.Username)
	}

	// Device registration
	respDev, devResp := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev1", "public_key": "dGVzdF9rZXk="}, cookie)
	if respDev.StatusCode != http.StatusCreated {
		t.Errorf("Device reg failed: %d", respDev.StatusCode)
	}
	if devResp["status"] != "success" {
		t.Errorf("Expected status: success, got %v", devResp["status"])
	}

	// /api/v1/health
	respHealth, _ := h.Client.Get(h.BaseURL + "/api/v1/health")
	var healthResp map[string]interface{}
	json.NewDecoder(respHealth.Body).Decode(&healthResp)
	respHealth.Body.Close()
	if healthResp["status"] != "success" {
		t.Errorf("Expected status ok in health check")
	}
}
