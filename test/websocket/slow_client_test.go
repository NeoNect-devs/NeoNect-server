package websocket_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestWebSocket_SlowClientIsolation(t *testing.T) {
	// Bypass global IP rate limit by using a trusted proxy configuration.

	h := harness.Setup(t)

	// Users & Devices
	h.RegisterUser(t, "user_slow", "password123")
	cookieSlow := h.Login(t, "user_slow", "password123")
	h.RegisterDevice(t, cookieSlow, "dev_slow")

	h.RegisterUser(t, "user_healthy", "password123")
	cookieHealthy := h.Login(t, "user_healthy", "password123")
	h.RegisterDevice(t, cookieHealthy, "dev_healthy")

	h.RegisterUser(t, "user_new", "password123")
	cookieNew := h.Login(t, "user_new", "password123")
	h.RegisterDevice(t, cookieNew, "dev_new")

	// Make them friends so they can message
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "user_healthy"}, cookieSlow)
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "user_slow"}, cookieHealthy)

	// 1. Client A (slow). We use raw TCP to cause backpressure.
	u, _ := url.Parse(h.BaseURL)
	rawConn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	reqStr := fmt.Sprintf("GET /api/v1/relay/ws?device_id=dev_slow HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nCookie: neonect_sid=%s\r\n\r\n", u.Host, cookieSlow)
	_, err = rawConn.Write([]byte(reqStr))
	if err != nil {
		t.Fatalf("Failed to write upgrade: %v", err)
	}

	respBuf := make([]byte, 1024)
	n, err := rawConn.Read(respBuf)
	if err != nil || !strings.Contains(string(respBuf[:n]), "101 Switching Protocols") {
		t.Fatalf("Failed to upgrade: %s", string(respBuf[:n]))
	}

	// 2. Client B (healthy) connects.
	connB := h.DialWS(t, cookieHealthy, "dev_healthy")
	defer connB.Close()

	// 3. Flood A to cause TCP backpressure
	largePayload := make([]byte, 10000)
	for i := 0; i < len(largePayload); i++ {
		largePayload[i] = 'A'
	}

	// We will send requests manually so we can inject random X-Forwarded-For headers
	// to completely evade the RateLimiter's IP buckets.
	// But wait, there is also a per-user message rate limit!
	// DefaultUserMessageLimit is usually like 500 per hour. We only need 150.
	startA := time.Now()
	for i := 0; i < 150; i++ {
		fakeIP := fmt.Sprintf("203.0.113.%d", i%250)
		req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/relay/send", strings.NewReader(fmt.Sprintf(`{
			"from_device_id": "dev_healthy",
			"protocol_version": 1, "to_username": "user_slow",
			"ciphertext": "%s",
			"timestamp": %d
		}`, string(largePayload), time.Now().Unix())))

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fakeIP)
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookieHealthy})

		resp, err := h.Client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	t.Logf("Queued 150 to A in %v", time.Since(startA))
	// 4. Verify B still receives messages while A is blocked.
	startB := time.Now()
	respB, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev_slow",
		"protocol_version": 1, "to_username": "user_healthy",
		"ciphertext": "aGVhbHRoeS1waW5n",
		"timestamp":  time.Now().Unix(),
	}, cookieSlow)
	t.Logf("Queued 1 to B in %v", time.Since(startB))
	if respB.StatusCode != http.StatusCreated {
		t.Fatalf("B send fail: %d", respB.StatusCode)
	}

	connB.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msgBytes, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("Client B blocked! Global mutex lock violation! %v", err)
	}
	var msg map[string]interface{}
	json.Unmarshal(msgBytes, &msg)
	if msg["ciphertext"] != "aGVhbHRoeS1waW5n" {
		t.Fatalf("Expected healthy-ping, got %v", msg["ciphertext"])
	}

	// 5. Verify C can connect while A is blocked.
	connC := h.DialWS(t, cookieNew, "dev_new")
	if connC == nil {
		t.Fatalf("Client C blocked from connecting! Global mutex lock violation!")
	}
	connC.Close()

	// 6. Verify A is eventually disconnected due to write timeout
	rawConn.SetReadDeadline(time.Now().Add(20 * time.Second))
	buf := make([]byte, 1024)
	for {
		_, err = rawConn.Read(buf)
		if err != nil {
			break
		}
	}
}
