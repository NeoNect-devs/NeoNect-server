package relay_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"NeoNect/test/harness"
)

func TestMailboxQuotaV1(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "quota_sender_v1", "Password123!")
	cookieSender := h.Login(t, "quota_sender_v1", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_qs_v1")

	h.RegisterUser(t, "quota_rec_v1", "Password123!")
	cookieRec := h.Login(t, "quota_rec_v1", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_qr_v1_1")
	h.RegisterDevice(t, cookieRec, "dev_qr_v1_2")

	h.AddFriend(t, cookieSender, "quota_rec_v1")
	h.AddFriend(t, cookieRec, "quota_sender_v1")

	cipher := base64.StdEncoding.EncodeToString([]byte("initial message"))
	resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 1,
		"from_device_id":   "dev_qs_v1",
		"to_username":      "quota_rec_v1",
		"ciphertext":       cipher,
	}, cookieSender)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created for initial message, got %d", resp.StatusCode)
	}

	for i := 0; i < 989; i++ {
		cipher := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("fill_%d", i)))
		resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
			"protocol_version": 1,
			"from_device_id":   "dev_qs_v1",
			"to_username":      "quota_rec_v1",
			"ciphertext":       cipher,
		}, cookieSender)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to pre-fill mailbox at %d: %d", i, resp.StatusCode)
		}
	}

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cipher := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("conc_%d", idx)))

			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
				"protocol_version": 1,
				"from_device_id":   "dev_qs_v1",
				"to_username":      "quota_rec_v1",
				"ciphertext":       cipher,
			}, cookieSender)

			if resp.StatusCode == http.StatusCreated {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else if resp.StatusCode != http.StatusRequestEntityTooLarge {
				{
					b, _ := io.ReadAll(resp.Body)
					t.Errorf("Unexpected status: %d, body: %s", resp.StatusCode, string(b))
				}
			}
		}(i)
	}

	close(startCh)
	wg.Wait()

	if successCount != 10 {
		t.Errorf("Expected exactly 10 concurrent successes, got %d", successCount)
	}

	h.RegisterDevice(t, cookieRec, "dev_qr_v1_3")

	cipher2 := base64.StdEncoding.EncodeToString([]byte("new device message"))
	resp2, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 1,
		"from_device_id":   "dev_qs_v1",
		"to_username":      "quota_rec_v1",
		"ciphertext":       cipher2,
	}, cookieSender)
	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("Expected success for multi-device when at least one has quota, got %d", resp2.StatusCode)
	}

	for i := 0; i < 999; i++ {
		cipher := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("fill3_%d", i)))
		resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
			"protocol_version": 1,
			"from_device_id":   "dev_qs_v1",
			"to_username":      "quota_rec_v1",
			"ciphertext":       cipher,
		}, cookieSender)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Failed to fill device 3 at %d: %d", i, resp.StatusCode)
		}
	}

	resp3, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"protocol_version": 1,
		"from_device_id":   "dev_qs_v1",
		"to_username":      "quota_rec_v1",
		"ciphertext":       cipher2,
	}, cookieSender)
	if resp3.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413 when ALL devices full, got %d", resp3.StatusCode)
	}
}

func TestMailboxQuotaV1Bytes(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "quota_sender_bytes", "Password123!")
	cookieSender := h.Login(t, "quota_sender_bytes", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_qs_b")

	h.RegisterUser(t, "quota_rec_bytes", "Password123!")
	cookieRec := h.Login(t, "quota_rec_bytes", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_qr_b")

	h.AddFriend(t, cookieSender, "quota_rec_bytes")
	h.AddFriend(t, cookieRec, "quota_sender_bytes")

	// Payload is exactly 700KB *before* base64. After base64 it's exactly 933,336 bytes.
	// This fits comfortably within the 1MB maxEnvelopeBytes constraint.
	payloadBytes := make([]byte, 700*1024)
	for i := 0; i < len(payloadBytes); i++ {
		payloadBytes[i] = 'A'
	}
	cipher := base64.StdEncoding.EncodeToString(payloadBytes)

	successCount := 0
	for i := 0; i < 80; i++ {
		resp, body := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
			"protocol_version": 1,
			"from_device_id":   "dev_qs_b",
			"to_username":      "quota_rec_bytes",
			"ciphertext":       cipher,
		}, cookieSender)

		if resp.StatusCode == http.StatusCreated {
			successCount++
		} else if resp.StatusCode == http.StatusRequestEntityTooLarge {
			// Expected after limit reached
		} else {
			t.Fatalf("Unexpected status code at iteration %d: %d, body: %v", i, resp.StatusCode, body)
		}
	}

	if successCount == 80 {
		t.Errorf("Expected mailbox bytes quota to kick in, but all 80 large messages succeeded")
	}
	if successCount == 0 {
		t.Errorf("Expected some messages to succeed, but got 0")
	}
}
