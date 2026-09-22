package concurrency_test

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestFriendshipConcurrency(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_conc_a", "Password123!")
	cookieA := h.Login(t, "user_conc_a", "Password123!")

	h.RegisterUser(t, "user_conc_b", "Password123!")
	cookieB := h.Login(t, "user_conc_b", "Password123!")

	var wg sync.WaitGroup
	const numConcurrent = 20

	var successCount int32
	var conflictCount int32

	startCh := make(chan struct{})

	// 10 goroutines from A to B
	for i := 0; i < numConcurrent/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
				"username": "user_conc_b",
			}, cookieA)
			if resp.StatusCode == http.StatusCreated {
				atomic.AddInt32(&successCount, 1)
			} else if resp.StatusCode == http.StatusConflict {
				atomic.AddInt32(&conflictCount, 1)
			}
		}()
	}

	// 10 goroutines from B to A (testing reverse constraint atomicity)
	for i := 0; i < numConcurrent/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/friends", map[string]string{
				"username": "user_conc_a",
			}, cookieB)
			if resp.StatusCode == http.StatusCreated {
				atomic.AddInt32(&successCount, 1)
			} else if resp.StatusCode == http.StatusConflict {
				atomic.AddInt32(&conflictCount, 1)
			}
		}()
	}

	// Fire all
	close(startCh)
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful friendship creation, got %d", successCount)
	}

	if conflictCount != numConcurrent-1 {
		t.Errorf("Expected exactly %d conflict responses, got %d", numConcurrent-1, conflictCount)
	}

	var count int
	err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count friendships: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 friendship in database, got %d", count)
	}

	var u1, u2 int64
	err = h.App.DB.DB().QueryRow("SELECT user_id_1, user_id_2 FROM friendships").Scan(&u1, &u2)
	if err != nil {
		t.Fatalf("Failed to query friendship pair: %v", err)
	}
	if u1 >= u2 {
		t.Errorf("Expected user_id_1 < user_id_2, got %d >= %d", u1, u2)
	}
}
