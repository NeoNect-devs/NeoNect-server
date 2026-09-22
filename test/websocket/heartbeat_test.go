package websocket_test

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestWebSocket_HeartbeatIsolation(t *testing.T) {
	h := harness.Setup(t)

	// Users & Devices
	h.RegisterUser(t, "user_slow_hb", "password123")
	cookieSlow := h.Login(t, "user_slow_hb", "password123")
	h.RegisterDevice(t, cookieSlow, "dev_slow_hb")

	h.RegisterUser(t, "user_healthy_hb", "password123")
	cookieHealthy := h.Login(t, "user_healthy_hb", "password123")
	h.RegisterDevice(t, cookieHealthy, "dev_healthy_hb")

	h.RegisterUser(t, "user_new_hb", "password123")
	cookieNew := h.Login(t, "user_new_hb", "password123")
	h.RegisterDevice(t, cookieNew, "dev_new_hb")

	// 1. Client A (slow). Use raw TCP.
	u, _ := url.Parse(h.BaseURL)
	rawConn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()

	reqStr := fmt.Sprintf("GET /api/v1/relay/ws?device_id=dev_slow_hb HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nCookie: neonect_sid=%s\r\n\r\n", u.Host, cookieSlow)
	_, err = rawConn.Write([]byte(reqStr))
	if err != nil {
		t.Fatalf("Failed to write upgrade: %v", err)
	}

	respBuf := make([]byte, 1024)
	n, err := rawConn.Read(respBuf)
	if err != nil || !strings.Contains(string(respBuf[:n]), "101 Switching Protocols") {
		t.Fatalf("Failed to upgrade: %s", string(respBuf[:n]))
	}

	// 2. Client B (healthy).
	connB := h.DialWS(t, cookieHealthy, "dev_healthy_hb")
	defer connB.Close()

	pingChan := make(chan struct{}, 1)
	connB.SetPingHandler(func(appData string) error {
		select {
		case pingChan <- struct{}{}:
		default:
		}
		return connB.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})

	// Run a read loop on B to process the ping
	go func() {
		for {
			if _, _, err := connB.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// 3. Flood A to cause TCP backpressure
	largePayload := make([]byte, 10000)
	for i := 0; i < len(largePayload); i++ {
		largePayload[i] = 'A'
	}

	startA := time.Now()
	for i := 0; i < 200; i++ {
		fakeIP := fmt.Sprintf("203.0.113.%d", i%250)
		req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/relay/send", strings.NewReader(fmt.Sprintf(`{
			"recipient": "user_slow_hb",
			"envelopes": [
				{
					"device_id": "dev_slow_hb",
					"ciphertext": "%s"
				}
			]
		}`, string(largePayload))))

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fakeIP)
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookieHealthy})

		resp, err := h.Client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	t.Logf("Queued messages to A in %v", time.Since(startA))

	// 4. Wait for B's heartbeat. If global loop is blocked by A for 15s (WriteTimeout),
	// B will receive its Ping at 45s instead of 30s. We wait 35s.
	select {
	case <-pingChan:
		t.Logf("Received heartbeat on Client B!")
	case <-time.After(35 * time.Second):
		t.Fatalf("Heartbeat timeout! Global heartbeat loop is likely blocked by slow client A.")
	}

	// 5. Verify C can connect.
	connC := h.DialWS(t, cookieNew, "dev_new_hb")
	if connC == nil {
		t.Fatalf("Client C blocked from connecting!")
	}
	connC.Close()
}
