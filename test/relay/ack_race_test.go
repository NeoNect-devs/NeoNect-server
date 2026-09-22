package relay_test

import (
	"sync"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestAckRaceBehavior(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "ack_race_sender", "Password123!")
	cookieSender := h.Login(t, "ack_race_sender", "Password123!")
	h.RegisterDevice(t, cookieSender, "dev_ack_s1")

	h.RegisterUser(t, "ack_race_recipient", "Password123!")
	cookieRecipient := h.Login(t, "ack_race_recipient", "Password123!")
	h.RegisterDevice(t, cookieRecipient, "dev_ack_r1")
	h.RegisterDevice(t, cookieRecipient, "dev_ack_r2")

	h.AddFriend(t, cookieSender, "ack_race_recipient")
	h.AddFriend(t, cookieRecipient, "ack_race_sender")

	// Send message
	h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
		"from_device_id":   "dev_ack_s1",
		"protocol_version": 1, "to_username": "ack_race_recipient",
		"ciphertext": "YnVy",
		"timestamp":  time.Now().Unix(),
	}, cookieSender)

	// Poll to get the message ID
	_, res := h.GetJSON(t, "/api/v1/relay/poll?device_id=dev_ack_r1", cookieRecipient)
	msgs := res["messages"].([]interface{})
	if len(msgs) == 0 {
		t.Fatalf("Expected message")
	}
	msgID := int64(msgs[0].(map[string]interface{})["id"].(float64))

	// Concurrent ACKs
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	const numAttackers = 20
	// Mix correct device, incorrect device, incorrect user, etc
	for i := 0; i < numAttackers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh

			var deviceID string
			var cookie string
			if idx%4 == 0 {
				deviceID = "dev_ack_r1" // Correct device
				cookie = cookieRecipient
			} else if idx%4 == 1 {
				deviceID = "dev_ack_r2" // Wrong device for this specific message (which is mapped to dev_ack_r1)
				cookie = cookieRecipient
			} else if idx%4 == 2 {
				deviceID = "dev_ack_r1" // Correct device, wrong user
				cookie = cookieSender
			} else {
				deviceID = "dev_ack_s1" // Wrong device, wrong user
				cookie = cookieSender
			}

			h.PostJSON(t, "/api/v1/relay/ack", map[string]interface{}{
				"device_id":  deviceID,
				"message_id": msgID,
			}, cookie)
		}(i)
	}

	// Wait for all to finish
	close(startCh)
	wg.Wait()

	// Verify the database queue is completely clean for this message for dev_ack_r1
	var count int
	err := h.App.DB.DB().QueryRow("SELECT COUNT(*) FROM delivery_queue WHERE id = ?", msgID).Scan(&count)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected message to be deleted exactly once and no longer exist, but found %d", count)
	}
}
