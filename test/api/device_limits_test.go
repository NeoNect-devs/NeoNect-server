package api_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"NeoNect/test/harness"
)

func TestDeviceLimitInvariant_Concurrent(t *testing.T) {
	// Let's set a small limit so we hit it easily.
	// But we must NOT use os.Setenv because that was proven not to work fully in harness unless restored,
	// Wait, harness restores it if set BEFORE Setup. But app_init parses NEONECT_MAX_DEVICES.
	os.Setenv("NEONECT_MAX_DEVICES", "5")

	h := harness.Setup(t)

	h.RegisterUser(t, "limit_race_user", "password123!")
	cookie := h.Login(t, "limit_race_user", "password123!")

	const numConcurrent = 150
	var wg sync.WaitGroup
	var successCount int32
	var rateLimitCount int32
	var conflictCount int32 // 400 Bad Request is what the handler returns for device limits

	// Pre-generate payloads so we don't block in the goroutine
	startCh := make(chan struct{})

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			deviceID := fmt.Sprintf("race_device_%d", idx)
			payload := map[string]interface{}{
				"device_id":  deviceID,
				"public_key": base64.StdEncoding.EncodeToString([]byte("dummy-key")),
			}

			<-startCh // Wait for barrier

			resp, _ := h.PostJSON(t, "/api/v1/device/register", payload, cookie)

			switch resp.StatusCode {
			case http.StatusCreated:
				atomic.AddInt32(&successCount, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt32(&rateLimitCount, 1)
			case http.StatusBadRequest:
				atomic.AddInt32(&conflictCount, 1)
			case http.StatusConflict:
				atomic.AddInt32(&conflictCount, 1)
			}
		}(i)
	}

	// Start the race
	close(startCh)
	wg.Wait()

	// Let's inspect the actual DB
	var actualCount int
	// Let's query by user_id instead.
	err := h.App.DB.DB().QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&actualCount)
	if err != nil {
		t.Fatalf("Failed to query DB: %v", err)
	}

	if actualCount > 5 {
		t.Errorf("Device limit invariant violated! Found %d devices in DB, max is 5", actualCount)
	}
	if int32(actualCount) != successCount {
		t.Errorf("Mismatch between success HTTP responses (%d) and actual devices in DB (%d)", successCount, actualCount)
	}

	t.Logf("Results -> Success: %d, RateLimited: %d, RejectedLimit: %d", successCount, rateLimitCount, conflictCount)
}
