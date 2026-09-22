package websocket_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"

	"github.com/gorilla/websocket"
)

func tryDialWS(h *harness.Harness, cookie, deviceID string) (*websocket.Conn, int, error) {
	dialer := websocket.Dialer{Proxy: http.ProxyURL(nil)}
	headers := http.Header{}
	headers.Add("Cookie", "neonect_sid="+cookie)
	conn, resp, err := dialer.Dial(fmt.Sprintf("%s/api/v1/relay/ws?device_id=%s", h.WsURL, deviceID), headers)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	return conn, statusCode, err
}

func TestWebSocket_ConnectionLimit(t *testing.T) {
	h := harness.Setup(t)

	var activeConns []*websocket.Conn
	var mu sync.Mutex

	successCount := 0
	rejectCount := 0

	var wg sync.WaitGroup

	// Max limit is 10. We launch 15 concurrent connections from the same IP.
	// Since httptest.Server uses 127.0.0.1 for all clients without X-Forwarded-For,
	// this acts as a single IP address test.
	for i := 0; i < 15; i++ {
		username := fmt.Sprintf("user_lim_%d", i)
		h.RegisterUser(t, username, "password123")
		cookie := h.Login(t, username, "password123")
		deviceID := fmt.Sprintf("dev_lim_%d", i)
		h.RegisterDevice(t, cookie, deviceID)

		wg.Add(1)
		go func(c, id string) {
			defer wg.Done()
			conn, statusCode, err := tryDialWS(h, c, id)

			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				activeConns = append(activeConns, conn)
				successCount++
			} else {
				if statusCode == http.StatusTooManyRequests {
					rejectCount++
				} else {
					t.Errorf("Unexpected error dialing: %v, status %d", err, statusCode)
				}
			}
		}(cookie, deviceID)
	}

	wg.Wait()

	if successCount != 10 {
		t.Fatalf("Expected exactly 10 successful connections, got %d", successCount)
	}
	if rejectCount != 5 {
		t.Fatalf("Expected exactly 5 rejected connections, got %d", rejectCount)
	}

	// Now close all connections
	for _, conn := range activeConns {
		conn.Close()
	}

	// Wait a moment for the server to process the close events and release limits
	time.Sleep(200 * time.Millisecond)

	// Now we should be able to connect again from the same IP
	h.RegisterUser(t, "user_lim_extra", "password123")
	cookieExtra := h.Login(t, "user_lim_extra", "password123")
	h.RegisterDevice(t, cookieExtra, "dev_lim_extra")

	conn, statusCode, err := tryDialWS(h, cookieExtra, "dev_lim_extra")
	if err != nil {
		t.Fatalf("Failed to reconnect after freeing limit: %v, status %d", err, statusCode)
	}
	conn.Close()
}
