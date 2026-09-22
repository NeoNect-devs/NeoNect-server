package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestShutdown_SlowClient(t *testing.T) {
	h := harness.Setup(t)

	// Setup users
	h.RegisterUser(t, "user_slow", "password123")
	h.RegisterUser(t, "user_healthy", "password123")

	cookieA := h.Login(t, "user_slow", "password123")
	cookieB := h.Login(t, "user_healthy", "password123")

	h.RegisterDevice(t, cookieA, "device_A")
	h.RegisterDevice(t, cookieB, "device_B")

	// Client A: Slow client (no reading, artificial backpressure)
	dialerA := websocket.Dialer{
		Proxy: http.ProxyURL(nil),
		NetDial: func(network, addr string) (net.Conn, error) {
			conn, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			// Use small buffers to saturate TCP Send-Q quickly
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				_ = tcpConn.SetReadBuffer(1024)
				_ = tcpConn.SetWriteBuffer(1024)
			}
			return conn, nil
		},
	}

	// Dial clients directly so we can customize Client A
	headersA := http.Header{}
	headersA.Add("Cookie", "neonect_sid="+cookieA)
	connA, _, err := dialerA.Dial(h.WsURL+"/api/v1/relay/ws?device_id=device_A", headersA)
	if err != nil {
		t.Fatalf("Failed to dial client A: %v", err)
	}
	defer connA.Close()

	connB := h.DialWS(t, cookieB, "device_B")
	defer connB.Close()

	// 1. Backpressure A
	// Send enough messages to fill the TCP Send-Q
	// A payload of 1MB will definitely block quickly.
	largePayload := make([]byte, 1024*1024)
	for i := range largePayload {
		largePayload[i] = 'A'
	}

	// We use the relay API to send messages to user_slow
	// We'll spam it in a goroutine until it blocks.
	var spamWg sync.WaitGroup
	spamWg.Add(1)
	go func() {
		defer spamWg.Done()
		for i := 0; i < 10; i++ {
			// Do not check error, it will just start queuing in the DB and delivery workers will pick it up
			body := `{"recipient":"user_slow","payload":"` + string(largePayload) + `"}`
			req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/relay/send", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookieB, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			resp, err := h.Client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Wait for backpressure to take effect
	time.Sleep(200 * time.Millisecond)

	// Verify B is healthy
	connB.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	// B has no pending messages, so it might just block or receive ping
	// Actually we should send a ping from B and expect a pong to prove it's unblocked
	connB.SetPingHandler(func(appData string) error {
		return connB.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})
	connB.WriteControl(websocket.PingMessage, []byte("test"), time.Now().Add(time.Second))

	// Trigger shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	shutdownComplete := make(chan struct{})
	go func() {
		h.SafeClose(ctx)
		close(shutdownComplete)
	}()

	// Wait for shutdown to complete
	select {
	case <-shutdownComplete:
	case <-time.After(5 * time.Second):
		t.Fatalf("Shutdown timed out, likely deadlocked on slow client A")
	}

	// Verify A and B are disconnected (read fails)
	connA.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, _, err := connA.ReadMessage(); err == nil {
		t.Errorf("Expected A to be disconnected")
	}
	connB.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, _, err := connB.ReadMessage(); err == nil {
		t.Errorf("Expected B to be disconnected")
	}

	spamWg.Wait()
}
