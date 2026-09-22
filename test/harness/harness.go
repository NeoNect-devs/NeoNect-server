package harness

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"NeoNect/internal/app"
	"NeoNect/security"

	"github.com/gorilla/websocket"
)

type SpoofingRoundTripper struct {
	Transport http.RoundTripper
}

func (rt *SpoofingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("X-Forwarded-For") == "" {
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", time.Now().UnixNano()%250))
	}
	return rt.Transport.RoundTrip(req)
}

type Harness struct {
	BaseURL    string
	WsURL      string
	App        *app.App
	Client     *http.Client
	StorageDir string
	IsClosed   bool
	Server     *http.Server
}

func (h *Harness) SafeClose(ctx context.Context) {
	if !h.IsClosed {
		h.IsClosed = true
		if h.Server != nil {
			_ = h.Server.Shutdown(ctx)
		}
		h.App.Close(ctx)
	}
}

func Setup(t *testing.T) *Harness {
	t.Helper()
	tempDir := t.TempDir()
	dbDir := filepath.Join(tempDir, "database")
	keyDir := filepath.Join(tempDir, "keys")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatalf("err: %v", err)
	}

	masterKeyPath := filepath.Join(keyDir, "master.key")

	envKeys := []string{"NEONECT_BOOTSTRAP_KEY", "NEONECT_DB_DIR", "NEONECT_KEY_DIR", "NEONECT_ENV", "NEONECT_TRUSTED_PROXIES", "NEONECT_MAX_REQ_PER_SEC_IP", "NEONECT_MAX_MSG_PER_SEC", "NEONECT_MAX_DISCOVERY_PER_SEC", "NEONECT_BIND_ADDR"}
	savedEnv := make(map[string]*string)
	for _, k := range envKeys {
		if val, ok := os.LookupEnv(k); ok {
			v := val
			savedEnv[k] = &v
		} else {
			savedEnv[k] = nil
		}
	}
	t.Cleanup(func() {
		for k, v := range savedEnv {
			if v == nil {
				os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	})

	_ = os.Setenv("NEONECT_BOOTSTRAP_KEY", "0123456789abcdef0123456789abcdef")
	err := security.CreateMasterKey(masterKeyPath, []byte("test-master-secret-1234567890123"))
	if err != nil {
		t.Fatalf("Failed to create master key: %v", err)
	}

	_ = os.Setenv("NEONECT_DB_DIR", dbDir)
	_ = os.Setenv("NEONECT_KEY_DIR", keyDir)
	_ = os.Setenv("NEONECT_ENV", "development")
	if _, ok := os.LookupEnv("NEONECT_TRUSTED_PROXIES"); !ok {
		os.Setenv("NEONECT_TRUSTED_PROXIES", "127.0.0.1")
	}
	if os.Getenv("NEONECT_MAX_REQ_PER_SEC_IP") == "" {
		os.Setenv("NEONECT_MAX_REQ_PER_SEC_IP", "10000")
	}
	if os.Getenv("NEONECT_MAX_MSG_PER_SEC") == "" {
		os.Setenv("NEONECT_MAX_MSG_PER_SEC", "10000")
	}
	if os.Getenv("NEONECT_MAX_DISCOVERY_PER_SEC") == "" {
		os.Setenv("NEONECT_MAX_DISCOVERY_PER_SEC", "10000")
	}

	application := app.NewApp(masterKeyPath)

	mux := http.NewServeMux()
	application.RegisterRoutes(mux)

	mw := application.RecoverMiddleware(
		application.TimeoutMiddleware(
			application.HeadersMiddleware(
				application.CorsMiddleware(
					application.BodyLimitMiddleware(
						application.HSTSMiddleware(
							application.Handlers.RateLimitMiddleware(mux),
						),
					),
				),
			),
		),
	)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: mw, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(listener)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", port)

	client := &http.Client{Timeout: 5 * time.Second, Transport: &SpoofingRoundTripper{Transport: http.DefaultTransport}}

	for i := 0; i < 50; i++ {
		resp, err := client.Get(baseURL + "/api/v1/health")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	h := &Harness{
		BaseURL:    baseURL,
		WsURL:      wsURL,
		App:        application,
		Client:     client,
		StorageDir: tempDir,
		Server:     srv,
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		h.SafeClose(ctx)
	})

	return h
}

func (h *Harness) PostJSON(t *testing.T, endpoint string, payload interface{}, cookie string) (*http.Response, map[string]interface{}) {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, h.BaseURL+endpoint, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	var res map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	_ = resp.Body.Close()
	return resp, res
}

func (h *Harness) GetJSON(t *testing.T, endpoint string, cookie string) (*http.Response, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+endpoint, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	var res map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	_ = resp.Body.Close()
	return resp, res
}

func (h *Harness) RegisterUser(t *testing.T, username, password string) {
	t.Helper()
	resp, _ := h.PostJSON(t, "/api/v1/users", map[string]string{"username": username, "password": password}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to register user: %d", resp.StatusCode)
	}
}

func (h *Harness) Login(t *testing.T, username, password string) string {
	t.Helper()
	resp, _ := h.PostJSON(t, "/api/v1/auth", map[string]string{"username": username, "password": password}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login: %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "neonect_sid" {
			return c.Value
		}
	}
	t.Fatalf("No session cookie found")
	return ""
}

func (h *Harness) RegisterDevice(t *testing.T, cookie, deviceID string) {
	t.Helper()
	pubKey := make([]byte, 32)
	rand.Read(pubKey)
	resp, _ := h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  deviceID,
		"public_key": "cHVibGljS2V5",
	}, cookie)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Failed to register device: %d", resp.StatusCode)
	}
}

func (h *Harness) DialWS(t *testing.T, cookie, deviceID string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{Proxy: http.ProxyURL(nil)}
	headers := http.Header{}
	headers.Add("Cookie", "neonect_sid="+cookie)
	conn, _, err := dialer.Dial(fmt.Sprintf("%s/api/v1/relay/ws?device_id=%s", h.WsURL, deviceID), headers)
	if err != nil {
		t.Fatalf("Failed to dial WS: %v", err)
	}
	return conn
}

func (h *Harness) AddFriend(t *testing.T, cookie, targetUsername string) {
	t.Helper()
	resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{"username": targetUsername}, cookie)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		t.Fatalf("Failed to add friend: %d", resp.StatusCode)
	}
}
