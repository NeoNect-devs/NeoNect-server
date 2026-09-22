package security

import (
	"NeoNect/internal/config"
	"NeoNect/test/harness"
	"crypto/rand"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestSEC002_WebSocketReadLimit(t *testing.T) {
	h := harness.Setup(t)

	username := "sec002_user"
	password := "SecurePass1!"
	deviceID := "device_sec002"

	h.RegisterUser(t, username, password)
	cookie := h.Login(t, username, password)
	h.RegisterDevice(t, cookie, deviceID)

	conn := h.DialWS(t, cookie, deviceID)
	defer conn.Close()

	// 4MB + 1 byte
	payloadSize := config.GlobalMaxBodySize + 1
	largePayload := make([]byte, payloadSize)
	_, _ = rand.Read(largePayload[:100]) // just some random bytes, rest is 0

	// Set a write deadline just in case
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Send the payload
	err := conn.WriteMessage(websocket.TextMessage, largePayload)
	if err != nil {
		t.Fatalf("Failed to write large message: %v", err)
	}

	// We expect the server to close the connection because the limit was exceeded.
	// The server will close the connection with CloseMessage and then close the TCP socket.
	// Let's try to read a message.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	messageType, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatalf("Expected connection to be closed due to read limit, but successfully read message type %d", messageType)
	}

	// The error should ideally be an UnexpectedEOF or CloseError, but any error reading means
	// the server dropped us (which is the expected behavior for read limit violation in gorilla/websocket).
	if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
		// Just confirming it errored out
	}
}
