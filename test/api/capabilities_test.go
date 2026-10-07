package api_test

import (
	"context"
	"net/http"
	"testing"
	"NeoNect/test/harness"
)

func TestCapabilities_ReturnsEffectiveConfig(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(context.Background())

	resp, data := h.GetJSON(t, "/api/v1/capabilities", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	if status, ok := data["status"].(string); !ok || status != "success" {
		t.Errorf("Expected status success, got %v", data["status"])
	}

	if version, ok := data["version"].(float64); !ok || int(version) != 1 {
		t.Errorf("Expected version 1, got %v", data["version"])
	}

	limits, ok := data["limits"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected limits object")
	}

	if v, ok := limits["max_http_body_bytes"].(float64); !ok || int64(v) != h.App.Config.HttpMaxBodyBytes {
		t.Errorf("Expected max_http_body_bytes %d, got %v", h.App.Config.HttpMaxBodyBytes, limits["max_http_body_bytes"])
	}

	if v, ok := limits["max_envelope_bytes"].(float64); !ok || int(v) != h.App.Config.MaxEnvelopeBytes {
		t.Errorf("Expected max_envelope_bytes %d, got %v", h.App.Config.MaxEnvelopeBytes, limits["max_envelope_bytes"])
	}

	if v, ok := limits["max_devices_per_user"].(float64); !ok || int(v) != h.App.Config.MaxDevicesPerUser {
		t.Errorf("Expected max_devices_per_user %d, got %v", h.App.Config.MaxDevicesPerUser, limits["max_devices_per_user"])
	}
}
