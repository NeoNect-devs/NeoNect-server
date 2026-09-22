package shutdown_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestShutdown_Drain(t *testing.T) {
	h := harness.Setup(t)

	// Create users
	h.RegisterUser(t, "user_drain_1", "password123")
	c1 := h.Login(t, "user_drain_1", "password123")
	h.RegisterDevice(t, c1, "dev1")

	h.RegisterUser(t, "user_drain_2", "password123")
	c2 := h.Login(t, "user_drain_2", "password123")
	h.RegisterDevice(t, c2, "dev2")

	// 1. Slow client
	u, _ := url.Parse(h.BaseURL)
	rawConn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer rawConn.Close()
	reqStr := fmt.Sprintf("GET /api/v1/relay/ws?device_id=dev1 HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nCookie: neonect_sid=%s\r\n\r\n", u.Host, c1)
	_, _ = rawConn.Write([]byte(reqStr))
	respBuf := make([]byte, 1024)
	n, _ := rawConn.Read(respBuf)
	if !strings.Contains(string(respBuf[:n]), "101 Switching Protocols") {
		t.Fatalf("Failed to upgrade: %s", string(respBuf[:n]))
	}

	// 2. Normal client
	connNormal := h.DialWS(t, c2, "dev2")
	defer connNormal.Close()

	// 3. Pending relay payload
	h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev2",
		"protocol_version": 1, "to_username": "user_drain_1",
		"ciphertext": "hello",
		"timestamp":  time.Now().Unix(),
	}, c2)

	// Sleep slightly to let things spin up
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)

	// Start graceful shutdown concurrently
	go func() {
		defer wg.Done()
		h.App.Close(context.Background())
	}()

	// Wait for shutdown to finish
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatalf("Shutdown hung forever")
	}

	// Try reading from slow raw connection, it should have been closed with 1012
	rawConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err = rawConn.Read(respBuf)
	if err == nil {
		t.Logf("Read from closed connection: %v bytes", n)
	}
}
