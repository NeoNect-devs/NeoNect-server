package websocket_test

import (
	"NeoNect/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebSocket_CheckOrigin(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		reqOrigin      string
		expectAllowed  bool
	}{
		{"Empty Origin", []string{"http://localhost"}, "", true},
		{"Allowed Exact", []string{"http://localhost"}, "http://localhost", true},
		{"Not Allowed", []string{"http://localhost"}, "http://evil.com", false},
		{"Wildcard", []string{"*"}, "http://evil.com", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := service.NewWebSocketManager(tc.allowedOrigins)
			
			req := httptest.NewRequest("GET", "/ws", nil)
			if tc.reqOrigin != "" {
				req.Header.Set("Origin", tc.reqOrigin)
			}
			
			w := httptest.NewRecorder()
			
			// HandleConnection expects a websocket upgrade, if origin is rejected it returns 403 Forbidden.
			ws.HandleConnection(w, req, "test-device")
			
			if !tc.expectAllowed && w.Code != http.StatusForbidden && w.Code != http.StatusBadRequest {
				t.Errorf("Expected rejection, got status %d", w.Code)
			}
		})
	}
}
