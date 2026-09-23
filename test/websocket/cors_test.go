package websocket_test

import (
	"NeoNect/internal/app"
	"NeoNect/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorsMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		allowedOrigins []string
		reqOrigin      string
		expectOrigin   string
		expectCreds    string
		expectVary     string
		expectStatus   int
	}{
		{
			name:           "Empty Origin",
			method:         http.MethodGet,
			allowedOrigins: []string{"http://localhost"},
			reqOrigin:      "",
			expectOrigin:   "",
			expectCreds:    "",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "Exact Match GET",
			method:         http.MethodGet,
			allowedOrigins: []string{"http://localhost", "https://app.neonect.com"},
			reqOrigin:      "https://app.neonect.com",
			expectOrigin:   "https://app.neonect.com",
			expectCreds:    "true",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "OPTIONS Preflight with Exact Match",
			method:         http.MethodOptions,
			allowedOrigins: []string{"https://app.neonect.com"},
			reqOrigin:      "https://app.neonect.com",
			expectOrigin:   "https://app.neonect.com",
			expectCreds:    "true",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "Rejected Origin",
			method:         http.MethodGet,
			allowedOrigins: []string{"http://localhost"},
			reqOrigin:      "http://evil.com",
			expectOrigin:   "",
			expectCreds:    "",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "Wildcard Match",
			method:         http.MethodGet,
			allowedOrigins: []string{"*"},
			reqOrigin:      "http://evil.com",
			expectOrigin:   "*",
			expectCreds:    "",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "OPTIONS Preflight with Wildcard",
			method:         http.MethodOptions,
			allowedOrigins: []string{"*"},
			reqOrigin:      "https://evil.com",
			expectOrigin:   "*",
			expectCreds:    "",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "Mixed Configuration - Exact Match Wins",
			method:         http.MethodOptions,
			allowedOrigins: []string{"*", "https://app.neonect.com"},
			reqOrigin:      "https://app.neonect.com",
			expectOrigin:   "https://app.neonect.com",
			expectCreds:    "true",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
		{
			name:           "Mixed Configuration - Wildcard Fallback",
			method:         http.MethodOptions,
			allowedOrigins: []string{"*", "https://app.neonect.com"},
			reqOrigin:      "https://evil.com",
			expectOrigin:   "*",
			expectCreds:    "",
			expectVary:     "Origin",
			expectStatus:   http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			application := &app.App{
				Config: config.AppConfig{
					AllowedOrigins: tc.allowedOrigins,
				},
			}

			dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			})

			handler := application.CorsMiddleware(dummyHandler)

			req := httptest.NewRequest(tc.method, "/api/v1/health", nil)
			if tc.reqOrigin != "" {
				req.Header.Set("Origin", tc.reqOrigin)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			expectedStatusCode := tc.expectStatus
			if tc.method != http.MethodOptions {
				expectedStatusCode = http.StatusAccepted
			}

			if w.Code != expectedStatusCode {
				t.Errorf("Expected HTTP status %d, got %d", expectedStatusCode, w.Code)
			}

			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.expectOrigin {
				t.Errorf("Expected ACAO %q, got %q", tc.expectOrigin, got)
			}

			if got := w.Header().Get("Access-Control-Allow-Credentials"); got != tc.expectCreds {
				t.Errorf("Expected ACAC %q, got %q", tc.expectCreds, got)
			}

			if got := w.Header().Get("Vary"); got != tc.expectVary {
				t.Errorf("Expected Vary %q, got %q", tc.expectVary, got)
			}

			if w.Header().Get("Access-Control-Allow-Methods") == "" {
				t.Errorf("Expected Access-Control-Allow-Methods to be present")
			}
			if w.Header().Get("Access-Control-Allow-Headers") == "" {
				t.Errorf("Expected Access-Control-Allow-Headers to be present")
			}
		})
	}
}
