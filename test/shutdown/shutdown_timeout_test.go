package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestShutdown_TimeoutRespected(t *testing.T) {
	h := harness.Setup(t)

	// Since we cannot change production code, we can block an endpoint by sending a massive payload
	// very slowly, or by holding the connection open.
	// But wait, the easiest way to block work is to hold the WebSocket open and not read from the server,
	// and then trigger a broadcast, BUT we just tested that and it actually completes in 500ms due to WriteControl deadline!

	// What if we send a very slow HTTP request body?
	// The HTTP server will be in state `Active`, and `Shutdown` will wait for it to finish until the context expires.

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/auth", reader)
		req.Header.Set("Content-Type", "application/json")
		resp, err := h.Client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	// Wait for the request to reach the server
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown with a short timeout
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	h.SafeClose(ctx)

	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Errorf("Shutdown took too long, timeout was not respected. Elapsed: %v", elapsed)
	}

	// Close the writer to unblock the request eventually
	writer.Close()
	wg.Wait()
}
