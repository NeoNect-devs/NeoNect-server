package concurrency_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func runConcurrencyTest(t *testing.T, h *harness.Harness, devID string, ownerCookie string, numOPKs int, numRequests int) {
	// Upload OPKs
	if numOPKs > 0 {
		var opks []map[string]interface{}
		for i := 0; i < numOPKs; i++ {
			opks = append(opks, map[string]interface{}{
				"key_id":     i + 1,
				"public_key": base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s_opk%d", devID, i+1))),
			})
		}
		resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
			"device_id":              devID,
			"one_time_curve_prekeys": opks,
		}, ownerCookie)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Failed to upload OPKs for %s", devID)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedKeyIDs := make(map[float64]int)
	errorsList := []error{}
	successCount := 0

	var claimerCookies []string
	for i := 0; i < numRequests; i++ {
		username := fmt.Sprintf("claimer_%s_%d", devID, i)
		h.RegisterUser(t, username, "Password1234")
		c := h.Login(t, username, "Password1234")
		claimerCookies = append(claimerCookies, c)
		// Add friend to owner
		ownerName := fmt.Sprintf("owner_%s", devID)
		h.PostJSON(t, "/api/v1/friends", map[string]interface{}{
			"username": ownerName,
		}, c)
	}

	startCh := make(chan struct{})

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		cookie := claimerCookies[i]
		go func() {
			defer wg.Done()
			<-startCh
			resp, claimResp := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{
				"target_device": devID,
			}, cookie)

			mu.Lock()
			defer mu.Unlock()
			if resp.StatusCode != http.StatusOK {
				errorsList = append(errorsList, fmt.Errorf("bad status: %d", resp.StatusCode))
				return
			}
			successCount++
			if opk, ok := claimResp["one_time_curve_prekey"].(map[string]interface{}); ok && opk != nil {
				keyID := opk["key_id"].(float64)
				claimedKeyIDs[keyID]++
			}
		}()
	}

	close(startCh)
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	for id, count := range claimedKeyIDs {
		if count > 1 {
			t.Errorf("[%s] KeyID %v was claimed %d times!", devID, id, count)
		}
	}

	expectedKeys := numOPKs
	if numRequests < numOPKs {
		expectedKeys = numRequests
	}

	if len(claimedKeyIDs) != expectedKeys {
		t.Errorf("[%s] Expected %d unique claimed keys, got %d", devID, expectedKeys, len(claimedKeyIDs))
	}
	if successCount != numRequests {
		t.Errorf("[%s] Expected %d successful requests (some with nil OPKs), got %d", devID, numRequests, successCount)
	}

	var consumedCount int
	err := h.App.DB.DB().QueryRow(`SELECT COUNT(*) FROM one_time_curve_prekeys WHERE device_id = ? AND status = 'CONSUMED'`, devID).Scan(&consumedCount)
	if err != nil {
		t.Fatalf("Failed to query DB: %v", err)
	}
	if consumedCount != expectedKeys {
		t.Errorf("[%s] Expected %d consumed keys in DB, got %d", devID, expectedKeys, consumedCount)
	}
}

func TestPrekeyClaimConcurrency_1OPK_20Req(t *testing.T) {
	h := harness.Setup(t)
	h.RegisterUser(t, "owner_dev1", "Password1234")
	cookie1 := h.Login(t, "owner_dev1", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev1", "public_key": "cHVibGljS2V5"}, cookie1)

	runConcurrencyTest(t, h, "dev1", cookie1, 1, 20)
}

func TestPrekeyClaimConcurrency_20OPK_20Req(t *testing.T) {
	h := harness.Setup(t)
	h.RegisterUser(t, "owner_dev2", "Password1234")
	cookie1 := h.Login(t, "owner_dev2", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev2", "public_key": "cHVibGljS2V5"}, cookie1)

	runConcurrencyTest(t, h, "dev2", cookie1, 20, 20)
}

func TestPrekeyClaimConcurrency_50OPK_100Req(t *testing.T) {
	h := harness.Setup(t)
	h.RegisterUser(t, "owner_dev3", "Password1234")
	cookie1 := h.Login(t, "owner_dev3", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev3", "public_key": "cHVibGljS2V5"}, cookie1)

	runConcurrencyTest(t, h, "dev3", cookie1, 50, 100)
}
