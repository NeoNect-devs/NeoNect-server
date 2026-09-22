package relay_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"NeoNect/test/harness"
)

func TestMailboxQuotaConcurrency(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "quota_sender", "Password123!")
	cookieSender := h.Login(t, "quota_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_qs")

	h.RegisterUser(t, "quota_rec", "Password123!")
	cookieRec := h.Login(t, "quota_rec", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_qr")

	h.AddFriend(t, cookieSender, "quota_rec")
	h.AddFriend(t, cookieRec, "quota_sender")

	// Max messages is 1000. Let's send 990 sequentially to almost fill it.
	for i := 0; i < 990; i++ {
		msgID := fmt.Sprintf("fill_%d", i)
		cipher := base64.StdEncoding.EncodeToString([]byte(msgID))
		resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
			"protocol_version":    2,
			"message_id":          msgID,
			"from_device_id":      "dev_qs",
			"recipient_device_id": "dev_qr",
			"ciphertext":          cipher,
		}, cookieSender)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to pre-fill mailbox at %d: %d", i, resp.StatusCode)
		}
	}

	// Now send 20 messages concurrently. Only 10 should succeed, the rest should get 413.
	var wg sync.WaitGroup
	startCh := make(chan struct{})
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msgID := fmt.Sprintf("conc_%d", idx)
			cipher := base64.StdEncoding.EncodeToString([]byte(msgID))

			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
				"protocol_version":    2,
				"message_id":          msgID,
				"from_device_id":      "dev_qs",
				"recipient_device_id": "dev_qr",
				"ciphertext":          cipher,
			}, cookieSender)

			if resp.StatusCode == http.StatusCreated {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else if resp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Errorf("Unexpected status: %d", resp.StatusCode)
			}
		}(i)
	}

	close(startCh)
	wg.Wait()

	if successCount != 10 {
		t.Errorf("Expected exactly 10 concurrent successes, got %d", successCount)
	}
}
