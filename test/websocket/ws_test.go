package websocket_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"
	"github.com/gorilla/websocket"
)

func TestWebSocket_ConcurrentWrites(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_cw", "password123")
	cookie := h.Login(t, "user_cw", "password123")
	h.RegisterDevice(t, cookie, "dev_cw")

	conn := h.DialWS(t, cookie, "dev_cw")
	defer conn.Close()

	// Spin up 20 senders pushing to the same device to trigger concurrent writes internally
	// Wait, we bypass the API rate limit again? No, we can just spawn 50 messages, which is within the 100 API limit.
	const numMessages = 50
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh
			req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/relay/send", strings.NewReader(fmt.Sprintf(`{"from_device_id": "dev_cw", "protocol_version": 1, "to_username": "user_cw", "ciphertext": "cGF5bG9hZA==", "timestamp": %d}`, time.Now().Unix())))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", idx%250))
			req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
			resp, err := h.Client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("Send failed: %d", resp.StatusCode)
			}
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("Send failed: %d", resp.StatusCode)
			}
		}(i)
	}

	close(startCh)
	wg.Wait()

	// Read all 50 messages on the client side
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	count := 0
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var parsed map[string]interface{}
		json.Unmarshal(msg, &parsed)
		if parsed["ciphertext"] == "cGF5bG9hZA==" {
			count++
		}
		if count == numMessages {
			break
		}
	}

	if count != numMessages {
		t.Fatalf("Expected %d messages, got %d. Concurrent write failure or frame corruption.", numMessages, count)
	}
}

func TestWebSocket_ReconnectPointerIdentity(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_rc", "password123")
	cookie := h.Login(t, "user_rc", "password123")
	h.RegisterDevice(t, cookie, "dev_rc")

	// Client A connects
	connA := h.DialWS(t, cookie, "dev_rc")

	// Client B connects (same device, steals the session)
	connB := h.DialWS(t, cookie, "dev_rc")
	defer connB.Close()

	// The old connection should receive a close message or just close
	connA.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := connA.ReadMessage()
	if err == nil {
		t.Fatalf("Expected connA to be closed by server")
	}
	connA.Close()

	// Wait a moment for connA's read loop to terminate on the server and attempt to clean up
	time.Sleep(500 * time.Millisecond)

	// Verify connB is STILL ALIVE (cleanup from A didn't delete B)
	_, _ = h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev_rc",
		"protocol_version": 1, "to_username": "user_rc",
		"ciphertext": "cmVjb25uZWN0LXRlc3Q=",
		"timestamp":  time.Now().Unix(),
	}, cookie)

	connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := connB.ReadMessage()
	if err != nil {
		t.Fatalf("Client B was incorrectly disconnected during Client A's cleanup! %v", err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(msg, &parsed)
	if parsed["ciphertext"] != "cmVjb25uZWN0LXRlc3Q=" {
		t.Fatalf("Expected reconnect-test")
	}
}

func TestWebSocket_RevocationDuringConnection(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_rev", "password123")
	cookie := h.Login(t, "user_rev", "password123")
	h.RegisterDevice(t, cookie, "dev_rev")

	conn := h.DialWS(t, cookie, "dev_rev")

	// Revoke device
	req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/device", strings.NewReader(`{"device_id":"dev_rev"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	resp, _ := h.Client.Do(req)
	resp.Body.Close()

	// Verify the connection is forcefully closed
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatalf("Expected WebSocket to be forcefully closed upon device revocation")
	}

	// Verify cannot reconnect
	reqWS, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/relay/ws?device_id=dev_rev", nil)
	reqWS.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
	reqWS.Header.Set("Connection", "Upgrade")
	reqWS.Header.Set("Upgrade", "websocket")
	respWS, _ := h.Client.Do(reqWS)
	respWS.Body.Close()
	if respWS.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("Revoked device was able to reconnect!")
	}
}

func TestWebSocket_PingPongClose(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_ping", "password123")
	cookie := h.Login(t, "user_ping", "password123")
	h.RegisterDevice(t, cookie, "dev_ping")

	conn := h.DialWS(t, cookie, "dev_ping")

	// Test Ping
	err := conn.WriteMessage(websocket.PingMessage, []byte("ping-data"))
	if err != nil {
		t.Fatalf("Failed to write ping: %v", err)
	}

	// Server should reply with Pong
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	// gorilla/websocket handles ping/pong internally in ReadMessage by default,
	// but we can set a PingHandler.
	// Wait, the client is conn. We want to read the Pong.
	pongCh := make(chan string, 1)
	conn.SetPongHandler(func(appData string) error {
		pongCh <- appData
		return nil
	})

	// To process control messages, we must call ReadMessage
	go func() {
		conn.ReadMessage()
	}()

	select {
	case p := <-pongCh:
		if p != "ping-data" {
			t.Fatalf("Expected pong data 'ping-data', got %v", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Server did not respond with Pong")
	}

	// Send close frame
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		t.Fatalf("Failed to write close: %v", err)
	}

	// We don't verify server-side cleanup explicitly here because it's asynchronous.
}

func TestWebSocket_PayloadSchema(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_schema", "password123")
	cookie := h.Login(t, "user_schema", "password123")
	h.RegisterDevice(t, cookie, "dev_schema")

	conn := h.DialWS(t, cookie, "dev_schema")
	defer conn.Close()

	_, _ = h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev_schema",
		"protocol_version": 1, "to_username": "user_schema",
		"ciphertext": "Y2lwaGVydGV4dA==", // "ciphertext"
		"timestamp":  123456789,
	}, cookie)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	var msg map[string]interface{}
	err = json.Unmarshal(msgBytes, &msg)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if _, ok := msg["id"]; !ok {
		t.Errorf("Schema validation failed: missing 'id'")
	}
	if ct, ok := msg["ciphertext"]; !ok || ct != "Y2lwaGVydGV4dA==" {
		t.Errorf("Schema validation failed: ciphertext mismatch or missing, got %v", ct)
	}
	if _, ok := msg["sequence"]; !ok {
		t.Errorf("Schema validation failed: missing 'sequence'")
	}
}
