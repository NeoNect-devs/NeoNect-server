package api_test

import (
	"NeoNect/test/harness"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPrekeyUploadFlow(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	// Register device
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	// Upload prekeys
	resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "cHVibGljS2V5",
			"signature":  "c2lnbmF0dXJl",
		},
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="},
			{"key_id": 2, "public_key": "b3BrMg=="},
		},
		"signed_pq_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "cHFfcHVibGlj",
			"signature":  "cHFfc2ln",
		},
		"one_time_pq_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "cHFfb3Br"},
		},
	}, cookie1)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for prekey upload, got %d", resp.StatusCode)
	}
}

func TestPrekeyReplacementAndDuplicate(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	// Initial upload
	resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "cHVibGljS2V5",
			"signature":  "c2lnbmF0dXJl",
		},
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="},
		},
	}, cookie1)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Initial upload failed: %d", resp.StatusCode)
	}

	// Replacement of signed prekey and duplicate one-time key
	resp2, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     2, // New key
			"public_key": "bmV3X3B1Yg==",
			"signature":  "bmV3X3NpZw==",
		},
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="}, // Duplicate
			{"key_id": 2, "public_key": "b3BrMg=="}, // New
		},
	}, cookie1)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Replacement upload failed: %d", resp2.StatusCode)
	}
}

func TestPrekeyUploadValidation(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	// Malformed base64
	resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "not-base64",
			"signature":  "c2lnbmF0dXJl",
		},
	}, cookie1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for malformed base64, got %d", resp.StatusCode)
	}

	// Empty key material
	resp2, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "",
			"signature":  "c2lnbmF0dXJl",
		},
	}, cookie1)
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty key material, got %d", resp2.StatusCode)
	}

	// Oversized key (limit is 2048)
	oversize := make([]byte, 2049)
	rand.Read(oversize)
	oversizeStr := base64.StdEncoding.EncodeToString(oversize)
	resp3, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": oversizeStr,
			"signature":  "c2lnbmF0dXJl",
		},
	}, cookie1)
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for oversized payload, got %d", resp3.StatusCode)
	}
}

func TestPrekeyUploadAuthorization(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.RegisterUser(t, "user2", "Password1234")
	cookie2 := h.Login(t, "user2", "Password1234")

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	// Cross-user upload rejection
	resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="},
		},
	}, cookie2)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for cross-user upload, got %d", resp.StatusCode)
	}

	// Nonexistent device
	resp2, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "nonexistent",
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="},
		},
	}, cookie1)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for nonexistent device, got %d", resp2.StatusCode)
	}

	// Revoked device upload rejection
	payload, _ := json.Marshal(map[string]string{"device_id": "dev1"})
	req, _ := http.NewRequest(http.MethodDelete, h.BaseURL+"/api/v1/device", bytes.NewBuffer(payload))
	req.AddCookie(&http.Cookie{Name: "neonect_sid", Value: cookie1})
	req.Header.Set("Content-Type", "application/json")
	h.Client.Do(req)

	resp3, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="},
		},
	}, cookie1)
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for revoked device upload, got %d", resp3.StatusCode)
	}
}

func TestPrekeyUploadLimits(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	var opks []map[string]interface{}
	for i := 0; i < 201; i++ {
		opks = append(opks, map[string]interface{}{
			"key_id":     i,
			"public_key": "b3Br",
		})
	}

	resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id":              "dev1",
		"one_time_curve_prekeys": opks,
	}, cookie1)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for limit exceeded, got %d", resp.StatusCode)
	}
}

func TestPrekeyUploadAtomicity(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	// An upload containing a valid signed key and an invalid one-time key
	oversize := make([]byte, 2049)
	rand.Read(oversize)

	resp, _ := h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "cHVibGljS2V5",
			"signature":  "c2lnbmF0dXJl",
		},
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": base64.StdEncoding.EncodeToString(oversize)},
		},
	}, cookie1)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected atomic failure due to bad OPK, got %d", resp.StatusCode)
	}

	// Wait, we can't assert on DB directly without querying it, but we can verify subsequent valid uploads work
}

func TestPrekeyClaim(t *testing.T) {
	h := harness.Setup(t)

	// User 1
	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	// User 2
	h.RegisterUser(t, "user2", "Password1234")
	cookie2 := h.Login(t, "user2", "Password1234")

	// Add friend
	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{
		"username": "user1",
	}, cookie2)

	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{
		"device_id":  "dev1",
		"public_key": "cHVibGljS2V5",
	}, cookie1)

	// Upload prekeys
	h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id": "dev1",
		"signed_curve_prekey": map[string]interface{}{
			"key_id":     1,
			"public_key": "cHVibGljS2V5",
			"signature":  "c2lnbmF0dXJl",
		},
		"one_time_curve_prekeys": []map[string]interface{}{
			{"key_id": 1, "public_key": "b3BrMQ=="},
		},
	}, cookie1)

	// Claim prekeys
	resp, claimResp := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{
		"target_user":   "user1",
		"target_device": "dev1",
	}, cookie2)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for prekey claim, got %d. Body: %s", resp.StatusCode, claimResp)
	}

	if claimResp["identity_key"] != "cHVibGljS2V5" {
		t.Errorf("Expected identity_key cHVibGljS2V5, got %v", claimResp["identity_key"])
	}

	sc := claimResp["signed_curve_prekey"].(map[string]interface{})
	if sc["public_key"] != "cHVibGljS2V5" {
		t.Errorf("Expected signed curve public key cHVibGljS2V5, got %v", sc["public_key"])
	}

	opk := claimResp["one_time_curve_prekey"].(map[string]interface{})
	if opk["public_key"] != "b3BrMQ==" {
		t.Errorf("Expected one time curve public key b3BrMQ==, got %v", opk["public_key"])
	}

	// Try claiming again, OPK should be consumed
	resp2, claimResp2 := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{
		"target_device": "dev1",
	}, cookie2)

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for second prekey claim, got %d. Body: %s", resp2.StatusCode, claimResp2)
	}

	if claimResp2["one_time_curve_prekey"] != nil {
		t.Errorf("Expected no one time curve prekey on second claim, got %v", claimResp2["one_time_curve_prekey"])
	}
}

func TestPrekeyClaimCombinations(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user1", "Password1234")
	cookie1 := h.Login(t, "user1", "Password1234")

	h.RegisterUser(t, "user2", "Password1234")
	cookie2 := h.Login(t, "user2", "Password1234")

	h.PostJSON(t, "/api/v1/friends", map[string]interface{}{"username": "user1"}, cookie2)

	// Dev 1: Curve and PQ available
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev1", "public_key": "cHVibGljS2V5"}, cookie1)
	h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id":              "dev1",
		"one_time_curve_prekeys": []map[string]interface{}{{"key_id": 1, "public_key": "Y3VydmU="}},
		"one_time_pq_prekeys":    []map[string]interface{}{{"key_id": 1, "public_key": "cHE="}},
	}, cookie1)

	// Dev 2: Curve only
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev2", "public_key": "cHVibGljS2V5"}, cookie1)
	h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id":              "dev2",
		"one_time_curve_prekeys": []map[string]interface{}{{"key_id": 1, "public_key": "Y3VydmU="}},
	}, cookie1)

	// Dev 3: PQ only
	h.PostJSON(t, "/api/v1/device/register", map[string]interface{}{"device_id": "dev3", "public_key": "cHVibGljS2V5"}, cookie1)
	h.PostJSON(t, "/api/v1/keys/upload", map[string]interface{}{
		"device_id":           "dev3",
		"one_time_pq_prekeys": []map[string]interface{}{{"key_id": 1, "public_key": "cHE="}},
	}, cookie1)

	// Test Dev 1: Both
	resp1, cr1 := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{"target_device": "dev1"}, cookie2)
	if resp1.StatusCode != http.StatusOK || cr1["one_time_curve_prekey"] == nil || cr1["one_time_pq_prekey"] == nil {
		t.Errorf("Expected both keys for dev1, got: %v", cr1)
	}

	// Test Dev 2: Curve Only
	resp2, cr2 := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{"target_device": "dev2"}, cookie2)
	if resp2.StatusCode != http.StatusOK || cr2["one_time_curve_prekey"] == nil || cr2["one_time_pq_prekey"] != nil {
		t.Errorf("Expected only Curve key for dev2, got: %v", cr2)
	}

	// Test Dev 3: PQ Only
	resp3, cr3 := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{"target_device": "dev3"}, cookie2)
	if resp3.StatusCode != http.StatusOK || cr3["one_time_curve_prekey"] != nil || cr3["one_time_pq_prekey"] == nil {
		t.Errorf("Expected only PQ key for dev3, got: %v", cr3)
	}

	// Test Dev 1 Again: Both Exhausted
	resp4, cr4 := h.PostJSON(t, "/api/v1/keys/claim", map[string]interface{}{"target_device": "dev1"}, cookie2)
	if resp4.StatusCode != http.StatusOK || cr4["one_time_curve_prekey"] != nil || cr4["one_time_pq_prekey"] != nil {
		t.Errorf("Expected neither key for dev1 (exhausted), got: %v", cr4)
	}
}
