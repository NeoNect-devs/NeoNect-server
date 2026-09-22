package api_test

import (
	"net/http"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestPresence_OnlineOffline(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(t.Context())

	h.RegisterUser(t, "userA", "password123")
	cookieA := h.Login(t, "userA", "password123")

	// Offline initially
	res, body := h.GetJSON(t, "/api/v1/presence?u=userA", cookieA)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", res.StatusCode)
	}
	if online, ok := body["online"].(bool); !ok || online {
		t.Fatal("Expected offline")
	}

	h.RegisterDevice(t, cookieA, "devA")
	ws := h.DialWS(t, cookieA, "devA")

	time.Sleep(50 * time.Millisecond) // Let connection register

	res, body = h.GetJSON(t, "/api/v1/presence?u=userA", cookieA)
	if online, ok := body["online"].(bool); !ok || !online {
		t.Fatal("Expected online")
	}

	ws.Close()
	time.Sleep(50 * time.Millisecond) // Let connection unregister

	res, body = h.GetJSON(t, "/api/v1/presence?u=userA", cookieA)
	if online, ok := body["online"].(bool); !ok || online {
		t.Fatal("Expected offline after disconnect")
	}
}

func TestPresence_UnknownUser(t *testing.T) {
	h := harness.Setup(t)
	defer h.SafeClose(t.Context())

	h.RegisterUser(t, "userA", "password123")
	cookieA := h.Login(t, "userA", "password123")

	res, body := h.GetJSON(t, "/api/v1/presence?u=unknownUser", cookieA)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", res.StatusCode)
	}
	if online, ok := body["online"].(bool); !ok || online {
		t.Fatal("Expected offline for unknown user")
	}
}
