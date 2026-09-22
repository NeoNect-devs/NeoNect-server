package api_test

import (
	"NeoNect/test/harness"
	"context"
	"net/http"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	resp, data := h.GetJSON(t, "/api/v1/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for health, got %d", resp.StatusCode)
	}

	if status, ok := data["status"].(string); !ok || status != "success" {
		t.Fatalf("Expected status success, got %v", data["status"])
	}
}

func TestHealthEndpoint_Failure(t *testing.T) {
	h := harness.Setup(t)
	// Do not defer h.SafeClose because we will close DB manually and we want to prevent errors during shutdown if any, or we can just defer it.
	defer h.SafeClose(context.Background())

	// Close the DB to simulate failure
	if err := h.App.DB.Close(); err != nil {
		t.Fatalf("Failed to close DB: %v", err)
	}

	resp, _ := h.GetJSON(t, "/api/v1/health", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503 Service Unavailable for health, got %d", resp.StatusCode)
	}
}
