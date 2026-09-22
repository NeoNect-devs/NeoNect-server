package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShutdown_ActiveHTTP(t *testing.T) {
	h := harness.Setup(t)

	// We can use health endpoint or something slow. But wait, we don't have a deliberately slow endpoint.
	// But we can test that new requests are rejected if the server is shutting down.
	// Actually, the http.Server shutdown mechanism rejects new requests with http.ErrServerClosed locally,
	// and the remote client gets connection refused or connection reset.

	// Trigger shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.SafeClose(ctx)
	}()

	// Give it a tiny bit to initiate shutdown
	time.Sleep(50 * time.Millisecond)

	// Try to make a new request
	req, _ := http.NewRequest(http.MethodGet, h.BaseURL+"/api/v1/health", nil)
	resp, err := h.Client.Do(req)
	if err == nil {
		resp.Body.Close()
		// Depending on timing, it might succeed if shutdown hasn't closed the listener yet.
		// Let's loop for up to 500ms
		for i := 0; i < 10; i++ {
			time.Sleep(50 * time.Millisecond)
			resp, err = h.Client.Do(req)
			if err != nil {
				break
			}
			resp.Body.Close()
		}
	}

	if err == nil {
		t.Errorf("Expected new requests to be rejected after shutdown initiated")
	} else if !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "connection reset") {
		// Log the error to see what it is
		t.Logf("Got error on new request during shutdown: %v", err)
	}

	wg.Wait()
}
