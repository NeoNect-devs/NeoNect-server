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
	err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count friend_requests: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected exactly 1 friendship in database, got %d", count)
	}

	var u1, u2 int64
	err = h.App.DB.DB().QueryRow("SELECT user_id_1, user_id_2 FROM friend_requests").Scan(&u1, &u2)
	if err != nil {
		t.Fatalf("Failed to query friendship pair: %v", err)
	}
	if u1 >= u2 {
		t.Errorf("Expected user_id_1 < user_id_2, got %d >= %d", u1, u2)
	}
}

func TestAcceptanceConcurrency(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "user_a", "Password123!")
	cookieA := h.Login(t, "user_a", "Password123!")

	h.RegisterUser(t, "user_b", "Password123!")
	cookieB := h.Login(t, "user_b", "Password123!")

	t.Run("Acceptance racing with decline", func(t *testing.T) {
		h.PostJSON(t, "/api/v1/friends", map[string]string{"username": "user_b"}, cookieA)

		var wg sync.WaitGroup
		startCh := make(chan struct{})

		var acceptStatus, declineStatus int

		wg.Add(2)
		go func() {
			defer wg.Done()
			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{"username": "user_a"}, cookieB)
			acceptStatus = resp.StatusCode
		}()
		go func() {
			defer wg.Done()
			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/friends/requests/decline", map[string]string{"username": "user_a"}, cookieB)
			declineStatus = resp.StatusCode
		}()
		close(startCh)
		wg.Wait()

		// One must succeed (200), one must fail (404 Not Found)
		if !((acceptStatus == http.StatusOK && declineStatus == http.StatusNotFound) || (acceptStatus == http.StatusNotFound && declineStatus == http.StatusOK)) {
			t.Errorf("Expected one 200 and one 404, got accept:%d decline:%d", acceptStatus, declineStatus)
		}

		var fCount, rCount int
		h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&fCount)
		h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&rCount)

		if rCount != 0 {
			t.Errorf("Expected 0 pending requests, got %d", rCount)
		}
		if acceptStatus == http.StatusOK && fCount != 1 {
			t.Errorf("Accept succeeded but got %d friendships", fCount)
		}
		if declineStatus == http.StatusOK && fCount != 0 {
			t.Errorf("Decline succeeded but got %d friendships", fCount)
		}

		// Reset for next test
		h.App.DB.DB().Exec("DELETE FROM friendships")
	})

	t.Run("Acceptance racing with cancellation", func(t *testing.T) {
		h.PostJSON(t, "/api/v1/friends", map[string]string{"username": "user_b"}, cookieA)

		var wg sync.WaitGroup
		startCh := make(chan struct{})

		var acceptStatus, cancelStatus int

		wg.Add(2)
		go func() {
			defer wg.Done()
			<-startCh
			resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{"username": "user_a"}, cookieB)
			acceptStatus = resp.StatusCode
		}()
		go func() {
			defer wg.Done()
			<-startCh
			resp, _ := h.DeleteJSON(t, "/api/v1/friends/requests", map[string]string{"username": "user_b"}, cookieA)
			cancelStatus = resp.StatusCode
		}()
		close(startCh)
		wg.Wait()

		if !((acceptStatus == http.StatusOK && cancelStatus == http.StatusNotFound) || (acceptStatus == http.StatusNotFound && cancelStatus == http.StatusOK)) {
			t.Errorf("Expected one 200 and one 404, got accept:%d cancel:%d", acceptStatus, cancelStatus)
		}

		var fCount, rCount int
		h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&fCount)
		h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&rCount)

		if rCount != 0 {
			t.Errorf("Expected 0 pending requests, got %d", rCount)
		}
		if acceptStatus == http.StatusOK && fCount != 1 {
			t.Errorf("Accept succeeded but got %d friendships", fCount)
		}
		if cancelStatus == http.StatusOK && fCount != 0 {
			t.Errorf("Cancel succeeded but got %d friendships", fCount)
		}

		h.App.DB.DB().Exec("DELETE FROM friendships")
	})

	t.Run("Repeated acceptance and duplicate transitions", func(t *testing.T) {
		h.PostJSON(t, "/api/v1/friends", map[string]string{"username": "user_b"}, cookieA)

		var wg sync.WaitGroup
		startCh := make(chan struct{})

		const numConcurrent = 5
		statusCodes := make([]int, numConcurrent)

		wg.Add(numConcurrent)
		for i := 0; i < numConcurrent; i++ {
			go func(idx int) {
				defer wg.Done()
				<-startCh
				resp, _ := h.PostJSON(t, "/api/v1/friends/requests/accept", map[string]string{"username": "user_a"}, cookieB)
				statusCodes[idx] = resp.StatusCode
			}(i)
		}
		close(startCh)
		wg.Wait()

		var successCount int
		var notFoundCount int
		for _, code := range statusCodes {
			if code == http.StatusOK {
				successCount++
			} else if code == http.StatusNotFound {
				notFoundCount++
			}
		}

		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful accept, got %d", successCount)
		}
		if notFoundCount != numConcurrent-1 {
			t.Errorf("Expected %d not found, got %d", numConcurrent-1, notFoundCount)
		}

		var fCount, rCount int
		h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friendships").Scan(&fCount)
		h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM friend_requests").Scan(&rCount)

		if fCount != 1 {
			t.Errorf("Expected 1 accepted friendship, got %d", fCount)
		}
		if rCount != 0 {
			t.Errorf("Expected 0 pending requests, got %d", rCount)
		}
	})
}
