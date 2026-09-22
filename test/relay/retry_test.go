package relay_test

import (
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestRetryBehavior(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "retry_sender", "Password123!")
	cookieSender := h.Login(t, "retry_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_rt_s1")

	h.RegisterUser(t, "retry_recipient", "Password123!")
	cookieRecipient := h.Login(t, "retry_recipient", "Password123!")
	h.RegisterDevice(t, cookieRecipient, "dev_rt_r1")

	h.AddFriend(t, cookieSender, "retry_recipient")
	h.AddFriend(t, cookieRecipient, "retry_sender")

	// Ensure no WS is connected so delivery fails natively

	// Send message
	h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev_rt_s1",
		"protocol_version": 1, "to_username": "retry_recipient",
		"ciphertext": "YnVy",
		"timestamp":  time.Now().Unix(),
	}, cookieSender)

	// Since DefaultSchedulerInterval is 1s, we should see the state update within ~2s.
	// We'll poll the DB.
	var finalRetryCount int
	var finalNextRetry int64
	var found bool

	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for background worker to update retry state")
		case <-ticker.C:
			err := h.App.DB.DB().QueryRow("SELECT retry_count, next_retry FROM delivery_queue WHERE device_id = 'dev_rt_r1'").Scan(&finalRetryCount, &finalNextRetry)
			if err != nil {
				continue
			}
			if finalRetryCount > 0 {
				found = true
			}
		}
		if found {
			break
		}
	}

	if finalRetryCount != 1 {
		t.Errorf("Expected retry_count to increment to 1, got %d", finalRetryCount)
	}

	now := time.Now().Unix()
	// Next retry should be ~60 seconds in the future
	if finalNextRetry <= now {
		t.Errorf("Expected next_retry to be advanced into the future, got %d (now is %d)", finalNextRetry, now)
	}
}
