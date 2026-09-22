package api_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"NeoNect/test/harness"
)

func TestBUG001_PresenceFallThrough(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(t.Context())

	h.RegisterUser(t, "userA", "password123")
	cookieA := h.Login(t, "userA", "password123")

	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/presence?u=unknownUser", nil)
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookieA, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})

	resp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	bodyStr := string(bodyBytes)

	// Count how many times "online" appears in the response body.
	// If the fall-through bug is present, it will write the JSON object twice,
	// so "online" will appear twice.
	count := strings.Count(bodyStr, "online")
	if count > 1 {
		t.Fatalf("BUG-001 regression: Response body contains multiple JSON objects due to fall-through: %s", bodyStr)
	}
	if count == 0 {
		t.Fatalf("Expected valid JSON response, got: %s", bodyStr)
	}
}
