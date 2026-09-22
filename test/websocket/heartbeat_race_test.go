package websocket_test

import (
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestWebSocket_HeartbeatRace(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_race", "password123")
	cookie := h.Login(t, "user_race", "password123")
	h.RegisterDevice(t, cookie, "dev_race")

	// Old session established
	connOld := h.DialWS(t, cookie, "dev_race")

	// Wait a bit to let it settle
	time.Sleep(50 * time.Millisecond)

	// New session replaces old session
	connNew := h.DialWS(t, cookie, "dev_race")

	// Old connection should be closed by the server
	connOld.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := connOld.ReadMessage()
	if err == nil {
		t.Fatalf("Old connection should be closed by server")
	}

	// Verify device remains online
	res, body := h.GetJSON(t, "/api/v1/presence?u=user_race", cookie)
	if online, ok := body["online"].(bool); !ok || !online {
		t.Fatalf("Device should remain online, got: %v (Status %d)", body, res.StatusCode)
	}

	// Ensure new connection is healthy
	connNew.Close()
}
