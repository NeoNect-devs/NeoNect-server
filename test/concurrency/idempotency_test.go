package concurrency_test

import (
	"NeoNect/test/harness"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func doIdempotentClaim(t *testing.T, h *harness.Harness, targetDevice, idempotencyKey, cookie string) (*http.Response, map[string]interface{}) {
	payload := map[string]interface{}{
		"target_device": targetDevice,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, h.BaseURL+"/api/v1/keys/claim", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	var respBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&respBody)

	return resp, respBody
}

func TestIdempotency_A_20ConcurrentIdentical(t *testing.T) {
	h := harness.Setup(t)
	devID := "dev_idem_A"
	h.RegisterUser(t, "owner_A", "Password1234")
	ownerCookie := h.Login(t, "owner_A", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": devID, "public_key": "cHVibGljS2V5"}, ownerCookie)

	// Upload 5 OPKs
	var opks []map[string]interface{}
	for i := 0; i < 5; i++ {
		opks = append(opks, map[string]interface{}{
			"key_id":     i + 1,
			"public_key": base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s_opk%d", devID, i+1))),
		})
	}
	h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id":              devID,
		"one_time_curve_prekeys": opks,
	}, ownerCookie)

	claimerName := "claimer_A"
	h.RegisterUser(t, claimerName, "Password1234")
	claimerCookie := h.Login(t, claimerName, "Password1234")
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "owner_A"}, claimerCookie)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	conflictCount := 0
	responses := make([]map[string]interface{}, 20)

	startCh := make(chan struct{})

	key := "idemp-key-A"
	for i := 0; i < 20; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			<-startCh
			resp, body := doIdempotentClaim(t, h, devID, key, claimerCookie)
			mu.Lock()
			defer mu.Unlock()
			if resp.StatusCode == http.StatusOK {
				successCount++
				responses[idx] = body
			} else if resp.StatusCode == http.StatusConflict {
				conflictCount++
			} else {
				t.Errorf("Unexpected status: %d", resp.StatusCode)
			}
		}()
	}

	close(startCh)
	wg.Wait()

	// Wait for any remaining async
	time.Sleep(100 * time.Millisecond)

	if successCount == 0 {
		t.Fatalf("Expected at least one success")
	}

	// Because of SQLite write locks, some concurrent requests might get 409 Conflict ("request in progress")
	// The ones that succeeded MUST have identical responses
	var firstSuccess map[string]interface{}
	for _, r := range responses {
		if r != nil {
			if firstSuccess == nil {
				firstSuccess = r
			} else {
				// compare
				fBytes, _ := json.Marshal(firstSuccess)
				rBytes, _ := json.Marshal(r)
				if string(fBytes) != string(rBytes) {
					t.Fatalf("Responses differ: %s vs %s", string(fBytes), string(rBytes))
				}
			}
		}
	}

	// Verify only 1 OPK was consumed
	var consumedCount int
	h.App.DB.DB().QueryRow(`SELECT COUNT(*) FROM one_time_curve_prekeys WHERE device_id = ? AND status = 'CONSUMED'`, devID).Scan(&consumedCount)
	if consumedCount != 1 {
		t.Fatalf("Expected 1 consumed OPK, got %d", consumedCount)
	}

	// Verify we can retry after it's done and get same response
	respRetry, bodyRetry := doIdempotentClaim(t, h, devID, key, claimerCookie)
	if respRetry.StatusCode != http.StatusOK {
		t.Fatalf("Retry failed: %d", respRetry.StatusCode)
	}
	fBytes, _ := json.Marshal(firstSuccess)
	rBytes, _ := json.Marshal(bodyRetry)
	if string(fBytes) != string(rBytes) {
		t.Fatalf("Retry response differs: %s vs %s", string(fBytes), string(rBytes))
	}
}

func TestIdempotency_B_ConcurrentConflicting(t *testing.T) {
	h := harness.Setup(t)
	h.RegisterUser(t, "owner_B", "Password1234")
	ownerCookie := h.Login(t, "owner_B", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev_B1", "public_key": "cHVibGljS2V5"}, ownerCookie)
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev_B2", "public_key": "cHVibGljS2V5"}, ownerCookie)

	claimerName := "claimer_B"
	h.RegisterUser(t, claimerName, "Password1234")
	claimerCookie := h.Login(t, claimerName, "Password1234")
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "owner_B"}, claimerCookie)

	key := "idemp-key-B"

	// Request 1: dev_B1
	resp1, _ := doIdempotentClaim(t, h, "dev_B1", key, claimerCookie)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("Req 1 failed: %d", resp1.StatusCode)
	}

	// Request 2: dev_B2 (same key, different target)
	resp2, _ := doIdempotentClaim(t, h, "dev_B2", key, claimerCookie)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("Req 2 should be conflict, got: %d", resp2.StatusCode)
	}
}

func TestIdempotency_C_DifferentUsersSameKey(t *testing.T) {
	h := harness.Setup(t)
	h.RegisterUser(t, "owner_C", "Password1234")
	ownerCookie := h.Login(t, "owner_C", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev_C", "public_key": "cHVibGljS2V5"}, ownerCookie)

	h.RegisterUser(t, "claimer_C1", "Password1234")
	c1 := h.Login(t, "claimer_C1", "Password1234")
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "owner_C"}, c1)

	h.RegisterUser(t, "claimer_C2", "Password1234")
	c2 := h.Login(t, "claimer_C2", "Password1234")
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "owner_C"}, c2)

	key := "idemp-key-C"

	resp1, _ := doIdempotentClaim(t, h, "dev_C", key, c1)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("Req 1 failed")
	}

	resp2, _ := doIdempotentClaim(t, h, "dev_C", key, c2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Req 2 failed, expected OK because namespace should be isolated")
	}
}

func TestIdempotency_D_ResponseLoss(t *testing.T) {
	h := harness.Setup(t)
	devID := "dev_D"
	h.RegisterUser(t, "owner_D", "Password1234")
	ownerCookie := h.Login(t, "owner_D", "Password1234")
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": devID, "public_key": "cHVibGljS2V5"}, ownerCookie)

	opks := []map[string]interface{}{{
		"key_id":     1,
		"public_key": base64.StdEncoding.EncodeToString([]byte("opk1")),
	}}
	h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id":              devID,
		"one_time_curve_prekeys": opks,
	}, ownerCookie)

	claimerName := "claimer_D"
	h.RegisterUser(t, claimerName, "Password1234")
	claimerCookie := h.Login(t, claimerName, "Password1234")
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "owner_D"}, claimerCookie)

	key := "idemp-key-D"

	resp1, body1 := doIdempotentClaim(t, h, devID, key, claimerCookie)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("Req 1 failed")
	}

	// Retry
	resp2, body2 := doIdempotentClaim(t, h, devID, key, claimerCookie)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Req 2 failed")
	}

	b1, _ := json.Marshal(body1)
	b2, _ := json.Marshal(body2)
	if string(b1) != string(b2) {
		t.Fatalf("Responses differ")
	}

	var consumedCount int
	h.App.DB.DB().QueryRow(`SELECT COUNT(*) FROM one_time_curve_prekeys WHERE device_id = ? AND status = 'CONSUMED'`, devID).Scan(&consumedCount)
	if consumedCount != 1 {
		t.Fatalf("Expected 1 consumed OPK")
	}
}

func TestIdempotency_E_TransactionFailure(t *testing.T) {
	h := harness.Setup(t)
	claimerName := "claimer_E"
	h.RegisterUser(t, claimerName, "Password1234")
	claimerCookie := h.Login(t, claimerName, "Password1234")

	key := "idemp-key-E"

	// Request invalid device
	resp1, _ := doIdempotentClaim(t, h, "invalid_device", key, claimerCookie)
	if resp1.StatusCode == http.StatusOK {
		t.Fatalf("Expected failure")
	}

	// Ensure no record is left in DB
	var count int
	h.App.DB.DB().QueryRow(`SELECT COUNT(*) FROM idempotency_records WHERE idempotency_key = ?`, key).Scan(&count)
	if count != 0 {
		t.Fatalf("Expected 0 idempotency records for failed transaction, got %d", count)
	}
}
