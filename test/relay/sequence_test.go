package relay_test

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"NeoNect/test/harness"
)

func TestSequenceUniqueness_Concurrent(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "seq_rec", "Password123!")
	cookieRec := h.Login(t, "seq_rec", "Password123!")
	h.RegisterDevice(t, cookieRec, "dev_rec")

	// Create 5 sender users
	const numSenders = 5
	const msgsPerSender = 20

	var sendCookies []string
	for i := 0; i < numSenders; i++ {
		u := "seq_snd_" + string(rune('A'+i))
		h.RegisterUser(t, u, "Password123!")
		cookie := h.Login(t, u, "Password123!")
		h.RegisterDevice(t, cookie, "dev_"+u)
		sendCookies = append(sendCookies, cookie)
		h.AddFriend(t, cookie, "seq_rec")
		h.AddFriend(t, cookieRec, u)
	}

	startCh := make(chan struct{})
	var wg sync.WaitGroup
	var expectedCount int32

	for i := 0; i < numSenders; i++ {
		for j := 0; j < msgsPerSender; j++ {
			wg.Add(1)
			go func(senderIdx, msgIdx int) {
				defer wg.Done()
				<-startCh
				resp, _ := h.PostJSON(t, "/api/v1/relay/send", map[string]interface{}{
					"from_device_id":   "dev_seq_snd_" + string(rune('A'+senderIdx)),
					"protocol_version": 1, "to_username": "seq_rec",
					"ciphertext": "cGluZw==",
					"timestamp":  time.Now().Unix(),
				}, sendCookies[senderIdx])
				if resp.StatusCode == http.StatusCreated {
					atomic.AddInt32(&expectedCount, 1)
				} else if resp.StatusCode != http.StatusTooManyRequests {
					t.Errorf("Send failed with %d", resp.StatusCode)
				}
			}(i, j)
		}
	}

	// unleash concurrent sends
	close(startCh)
	wg.Wait()

	// Query actual DB queue records to verify uniqueness and monotonicity
	rows, err := h.App.DB.DB().Query(`SELECT sequence FROM delivery_queue WHERE device_id = 'dev_rec' ORDER BY sequence ASC`)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	defer rows.Close()

	var count int
	var lastSeq int64 = 0

	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		count++

		if seq <= lastSeq {
			t.Errorf("Sequence monotonicity or uniqueness violated! Previous: %d, Current: %d", lastSeq, seq)
		}

		// Wait, does the sequence guarantee NO GAPS?
		// If transaction fails, sequence might have a gap.
		// But in a successful run, it should ideally be contiguous from 1 to N because SQLite uses a single thread or simple increment.
		// Wait, NeoNect reads MAX(sequence) and increments it! So it should strictly be lastSeq + 1!
		if seq != lastSeq+1 {
			t.Errorf("Sequence gap detected! Expected %d, got %d", lastSeq+1, seq)
		}

		lastSeq = seq
	}

	if int32(count) != expectedCount {
		t.Errorf("Expected %d messages in queue, found %d", expectedCount, count)
	}
}
