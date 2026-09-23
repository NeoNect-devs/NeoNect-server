package websocket_test

import (
	"NeoNect/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWebSocket_CheckOrigin(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		reqOrigin      string
		expectAllowed  bool
	}{
		{
			name:           "Empty Origin preserves native CLI contract",
			allowedOrigins: []string{"http://localhost"},
			reqOrigin:      "",
			expectAllowed:  true,
		},
		{
			name:           "Allowed Exact Origin succeeds",
			allowedOrigins: []string{"http://allowed.example"},
			reqOrigin:      "http://allowed.example",
			expectAllowed:  true,
		},
		{
			name:           "Rejected Origin fails",
			allowedOrigins: []string{"http://allowed.example"},
			reqOrigin:      "http://evil.example",
			expectAllowed:  false,
		},
		{
			name:           "Wildcard Configuration rejects browser origin",
			allowedOrigins: []string{"*"},
			reqOrigin:      "http://evil.example",
			expectAllowed:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wsManager := service.NewWebSocketManager(tc.allowedOrigins)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wsManager.HandleConnection(w, r, "test-device")
			}))
			defer wsManager.Shutdown()
			defer srv.Close()

			u := "ws" + strings.TrimPrefix(srv.URL, "http")

			dialer := websocket.Dialer{}
			header := http.Header{}

			// Unconditionally set the Origin header to guarantee the key exists.
			// This prevents the Dialer from potentially synthesizing its own Origin
			// and explicitly tests the empty-Origin contract.
			header["Origin"] = []string{tc.reqOrigin}

			conn, resp, err := dialer.Dial(u, header)
			if tc.expectAllowed {
				if err != nil {
					t.Fatalf("Expected connection to succeed, got error: %v", err)
				}
				if resp.StatusCode != http.StatusSwitchingProtocols {
					t.Fatalf("Expected status 101, got %d", resp.StatusCode)
				}
				conn.Close()
			} else {
				if err == nil {
					conn.Close()
					t.Fatalf("Expected connection to fail, but it succeeded")
				}
				if resp == nil {
					t.Fatalf("Expected a rejection response, got nil")
				}
				if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusBadRequest {
					t.Errorf("Expected rejection status 403 or 400, got %d", resp.StatusCode)
				}
			}
		})
	}
}
