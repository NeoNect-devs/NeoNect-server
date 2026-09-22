package websocket_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestWebSocket_ABBADeadlock(t *testing.T) {
	h := harness.Setup(t)

	// Register user
	h.RegisterUser(t, "user_dd", "password123")
	cookie := h.Login(t, "user_dd", "password123")

	h.RegisterDevice(t, cookie, "dev_dd_0")
	conn := h.DialWS(t, cookie, "dev_dd_0")
	defer conn.Close()

	// Trigger massive concurrent deliveries and a shutdown
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// 10 concurrent deliveries to device 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh
			req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/relay/send", strings.NewReader(fmt.Sprintf(`{"from_device_id": "dev_dd_0", "protocol_version": 1, "to_username": "user_dd", "ciphertext": "cGF5bG9hZA==", "timestamp": %d}`, time.Now().Unix())))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", idx%250))
			req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie})
			resp, err := h.Client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(i)
	}

	// 1 shutdown simulation
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startCh
		time.Sleep(10 * time.Millisecond)
		h.App.Close(context.Background())
	}()

	close(startCh)

	// Wait with a timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatalf("Deadlock detected: waitgroup did not finish in time. DeliverMessage/Shutdown AB-BA deadlock is present.")
	}
}
